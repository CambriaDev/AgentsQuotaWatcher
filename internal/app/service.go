package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
	"github.com/cambriadev/agents-quota-watcher/internal/providers"
)

type QuotaService struct {
	registry *providers.Registry
	accounts domain.AccountStore

	mu        sync.RWMutex
	snapshots map[string]domain.Snapshot
}

func NewQuotaService(registry *providers.Registry, accounts domain.AccountStore) *QuotaService {
	return &QuotaService{
		registry:  registry,
		accounts:  accounts,
		snapshots: make(map[string]domain.Snapshot),
	}
}

func (s *QuotaService) RefreshAll(ctx context.Context) {
	accounts, err := s.accounts.List()
	if err != nil {
		return
	}
	var group sync.WaitGroup
	for _, account := range accounts {
		group.Add(1)
		go func(account domain.Account) {
			defer group.Done()
			_ = s.Refresh(ctx, account.ID)
		}(account)
	}
	group.Wait()
}

func (s *QuotaService) Refresh(ctx context.Context, accountID string) error {
	account, err := s.account(accountID)
	if err != nil {
		return err
	}
	provider, ok := s.registry.Get(account.Provider)
	if !ok {
		return fmt.Errorf("unknown provider %q", account.Provider)
	}
	metrics, err := provider.Refresh(ctx, account.Credentials)
	if err != nil {
		s.storeError(account, err)
		return err
	}
	now := time.Now().UTC()
	s.storeSnapshot(domain.Snapshot{
		AccountID: account.ID,
		Provider:  account.Provider,
		Name:      account.Alias,
		Metrics:   metrics,
		UpdatedAt: &now,
	})
	return nil
}

func (s *QuotaService) CreateAccount(providerID domain.ProviderID, alias string, credentials domain.Credentials) (domain.Account, error) {
	if _, ok := s.registry.Get(providerID); !ok {
		return domain.Account{}, fmt.Errorf("unknown provider %q", providerID)
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return domain.Account{}, errorsNew("account alias is required")
	}
	if err := validateCredentials(providerID, credentials); err != nil {
		return domain.Account{}, err
	}
	id, err := newAccountID()
	if err != nil {
		return domain.Account{}, err
	}
	account := domain.Account{ID: id, Provider: providerID, Alias: alias, Credentials: credentials}
	if err := s.accounts.Save(account); err != nil {
		return domain.Account{}, err
	}
	_ = s.Refresh(context.Background(), account.ID)
	return account, nil
}

func (s *QuotaService) DeleteAccount(accountID string) error {
	if err := s.accounts.Delete(accountID); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.snapshots, accountID)
	s.mu.Unlock()
	return nil
}

func (s *QuotaService) Snapshots() []domain.Snapshot {
	accounts, err := s.accounts.List()
	if err != nil {
		return nil
	}
	result := make([]domain.Snapshot, 0, len(accounts))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, account := range accounts {
		snapshot, exists := s.snapshots[account.ID]
		if !exists {
			snapshot = domain.Snapshot{AccountID: account.ID, Provider: account.Provider, Name: account.Alias}
		}
		result = append(result, snapshot)
	}
	return result
}

func (s *QuotaService) account(id string) (domain.Account, error) {
	accounts, err := s.accounts.List()
	if err != nil {
		return domain.Account{}, err
	}
	for _, account := range accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return domain.Account{}, fmt.Errorf("unknown account %q", id)
}

func (s *QuotaService) storeError(account domain.Account, err error) {
	s.storeSnapshot(domain.Snapshot{
		AccountID: account.ID,
		Provider:  account.Provider,
		Name:      account.Alias,
		Error:     err.Error(),
	})
}

func (s *QuotaService) storeSnapshot(snapshot domain.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.AccountID] = snapshot
}

func validateCredentials(providerID domain.ProviderID, credentials domain.Credentials) error {
	if !credentials.Connected() {
		return errorsNew("access token is required")
	}
	if providerID == domain.ProviderCopilot && strings.TrimSpace(credentials.Username) == "" {
		return errorsNew("GitHub username is required for Copilot")
	}
	if providerID == domain.ProviderCopilot && credentials.CreditLimit < 0 {
		return errorsNew("Copilot credit limit must not be negative")
	}
	return nil
}

func newAccountID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate account ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

type serviceError string

func (e serviceError) Error() string { return string(e) }

func errorsNew(message string) error { return serviceError(message) }
