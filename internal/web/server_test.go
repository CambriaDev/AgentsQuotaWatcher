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
	webServer, err := NewServer(service, "admin", "password")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	server := webServer.Routes()

	dashboard := httptest.NewRecorder()
	server.ServeHTTP(dashboard, httptest.NewRequest(http.MethodGet, "/", nil))
	if dashboard.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated dashboard status = %d, want %d", dashboard.Code, http.StatusUnauthorized)
	}
	if got := dashboard.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatal("unauthenticated dashboard did not request authentication")
	}

	dashboard = httptest.NewRecorder()
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	dashboardRequest.SetBasicAuth("admin", "password")
	server.ServeHTTP(dashboard, dashboardRequest)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", dashboard.Code, http.StatusOK)
	}

	usage := httptest.NewRecorder()
	usageRequest := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	usageRequest.SetBasicAuth("admin", "password")
	server.ServeHTTP(usage, usageRequest)
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
	webServer, err := NewServer(service, "admin", "password")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	server := webServer.Routes()
	body := `{"provider":"claude","alias":"Personal","credentials":{"access_token":"secret"}}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body))
	request.SetBasicAuth("admin", "password")
	server.ServeHTTP(response, request)
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

func TestNewServerRequiresAuthenticationCredentials(t *testing.T) {
	service := app.NewQuotaService(providers.NewRegistry(testProvider{}), &memoryStore{})
	if _, err := NewServer(service, "", "password"); err == nil {
		t.Fatal("NewServer succeeded without a username")
	}
	if _, err := NewServer(service, "admin", ""); err == nil {
		t.Fatal("NewServer succeeded without a password")
	}
}
