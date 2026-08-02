package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cambriadev/agents-quota-watcher/internal/app"
	"github.com/cambriadev/agents-quota-watcher/internal/domain"
	"github.com/cambriadev/agents-quota-watcher/internal/providers"
)

func TestRoutesServeDashboardAndUsage(t *testing.T) {
	service := app.NewQuotaService(providers.NewRegistry(testProvider{}), &memoryStore{})
	server := NewServer(service).Routes()

	dashboard := httptest.NewRecorder()
	server.ServeHTTP(dashboard, httptest.NewRequest(http.MethodGet, "/", nil))
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", dashboard.Code, http.StatusOK)
	}

	usage := httptest.NewRecorder()
	server.ServeHTTP(usage, httptest.NewRequest(http.MethodGet, "/api/usage", nil))
	if usage.Code != http.StatusOK {
		t.Fatalf("usage status = %d, want %d", usage.Code, http.StatusOK)
	}
	if got := usage.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}

type testProvider struct{}

func (testProvider) ID() domain.ProviderID { return domain.ProviderClaude }
func (testProvider) Name() string          { return "Test" }
func (testProvider) Refresh(_ context.Context, _ domain.Credentials) ([]domain.Metric, error) {
	return nil, nil
}

type memoryStore struct{ accounts []domain.Account }

func (s *memoryStore) List() ([]domain.Account, error) {
	return append([]domain.Account(nil), s.accounts...), nil
}
func (s *memoryStore) Save(account domain.Account) error {
	s.accounts = append(s.accounts, account)
	return nil
}
func (s *memoryStore) Delete(accountID string) error { return nil }

func TestCreateAccountDoesNotExposeCredentials(t *testing.T) {
	service := app.NewQuotaService(providers.NewRegistry(testProvider{}), &memoryStore{})
	server := NewServer(service).Routes()
	body := `{"provider":"claude","alias":"Personal","credentials":{"access_token":"secret"}}`
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var snapshots []domain.Snapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshots); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].Name != "Personal" {
		t.Fatalf("snapshots = %#v", snapshots)
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatal("credentials leaked from API response")
	}
}
