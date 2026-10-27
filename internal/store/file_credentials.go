package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

type FileCredentialStore struct {
	path string
	mu   sync.Mutex
}

type credentialFile struct {
	Accounts []domain.Account `json:"accounts"`
}

func NewFileCredentialStore(path string) (*FileCredentialStore, error) {
	if path == "" {
		return nil, errors.New("credential path is required")
	}
	return &FileCredentialStore{path: path}, nil
}

func (s *FileCredentialStore) List() ([]domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, migrate, err := s.read()
	if err != nil {
		return nil, err
	}
	if migrate {
		if err := s.write(values); err != nil {
			return nil, err
		}
	}
	return append([]domain.Account(nil), values.Accounts...), nil
}

func (s *FileCredentialStore) Save(account domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, _, err := s.read()
	if err != nil {
		return err
	}
	for index, current := range values.Accounts {
		if current.ID == account.ID {
			values.Accounts[index] = account
			return s.write(values)
		}
	}
	values.Accounts = append(values.Accounts, account)
	return s.write(values)
}

func (s *FileCredentialStore) Delete(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, _, err := s.read()
	if err != nil {
		return err
	}
	for index, account := range values.Accounts {
		if account.ID == accountID {
			values.Accounts = append(values.Accounts[:index], values.Accounts[index+1:]...)
			return s.write(values)
		}
	}
	return fmt.Errorf("unknown account %q", accountID)
}

func (s *FileCredentialStore) read() (credentialFile, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return credentialFile{}, false, nil
	}
	if err != nil {
		return credentialFile{}, false, fmt.Errorf("read credentials: %w", err)
	}
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(data, &schema); err != nil {
		return credentialFile{}, false, fmt.Errorf("decode credentials: %w", err)
	}
	if _, ok := schema["accounts"]; ok {
		var current credentialFile
		if err := json.Unmarshal(data, &current); err != nil {
			return credentialFile{}, false, fmt.Errorf("decode credentials: %w", err)
		}
		if current.Accounts == nil {
			current.Accounts = []domain.Account{}
		}
		return current, false, nil
	}

	current := credentialFile{}
	var legacy map[domain.ProviderID]domain.Credentials
	if err := json.Unmarshal(data, &legacy); err != nil {
		return credentialFile{}, false, fmt.Errorf("decode credentials: %w", err)
	}
	ids := make([]string, 0, len(legacy))
	for provider := range legacy {
		ids = append(ids, string(provider))
	}
	sort.Strings(ids)
	for _, id := range ids {
		provider := domain.ProviderID(id)
		credentials := legacy[provider]
		if credentials.Connected() {
			current.Accounts = append(current.Accounts, domain.Account{
				ID: "legacy-" + id, Provider: provider, Alias: defaultAlias(provider), Credentials: credentials,
			})
		}
	}
	return current, true, nil
}

func (s *FileCredentialStore) write(values credentialFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".credentials-*")
	if err != nil {
		return fmt.Errorf("create credential file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set credential file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close credential file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace credential file: %w", err)
	}
	return nil
}

func defaultAlias(provider domain.ProviderID) string {
	if provider == domain.ProviderCopilot {
		return "GitHub Copilot"
	}
	if provider == domain.ProviderClaude {
		return "Claude"
	}
	if provider == domain.ProviderDeepSeek {
		return "DeepSeek"
	}
	return string(provider)
}
