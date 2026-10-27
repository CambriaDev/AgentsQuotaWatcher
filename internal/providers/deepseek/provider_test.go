package deepseek

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

func TestRefreshFetchesBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/user/balance" {
			t.Errorf("request = %s %s, want GET /user/balance", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want bearer API key", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"is_available": true,
			"balance_infos": [
				{"currency":"CNY","total_balance":"110.00","granted_balance":"10.00","topped_up_balance":"100.00"},
				{"currency":"USD","total_balance":"0","granted_balance":"0","topped_up_balance":"0"}
			]
		}`))
	}))
	defer server.Close()

	provider := NewProvider(Config{APIBaseURL: server.URL})
	metrics, err := provider.Refresh(context.Background(), domain.Credentials{AccessToken: "test-key"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("metric count = %d, want 2", len(metrics))
	}
	if metrics[0].ID != "balance-cny" || metrics[0].Currency != "CNY" || metrics[0].Amount == nil || *metrics[0].Amount != 110 {
		t.Fatalf("CNY metric = %#v", metrics[0])
	}
	if metrics[0].Available == nil || !*metrics[0].Available {
		t.Fatalf("CNY availability = %v, want true", metrics[0].Available)
	}
	if len(metrics[0].Breakdown) != 2 || metrics[0].Breakdown[0].Amount == nil || *metrics[0].Breakdown[0].Amount != 10 ||
		metrics[0].Breakdown[1].Amount == nil || *metrics[0].Breakdown[1].Amount != 100 {
		t.Fatalf("CNY breakdown = %#v", metrics[0].Breakdown)
	}
	if metrics[1].Amount == nil || *metrics[1].Amount != 0 {
		t.Fatalf("zero USD balance = %#v, want an explicit zero amount", metrics[1])
	}

	encoded, err := json.Marshal(metrics[1])
	if err != nil {
		t.Fatalf("marshal zero balance metric: %v", err)
	}
	if !strings.Contains(string(encoded), `"amount":0`) {
		t.Fatalf("zero balance was omitted from JSON: %s", encoded)
	}
}

func TestRefreshRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing availability", body: `{"balance_infos":[{"currency":"CNY","total_balance":"1","granted_balance":"0","topped_up_balance":"1"}]}`},
		{name: "no balance entries", body: `{"is_available":true,"balance_infos":[]}`},
		{name: "empty currency", body: `{"is_available":true,"balance_infos":[{"currency":"","total_balance":"1","granted_balance":"0","topped_up_balance":"1"}]}`},
		{name: "invalid amount", body: `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"bad","granted_balance":"0","topped_up_balance":"1"}]}`},
		{name: "negative amount", body: `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"-1","granted_balance":"0","topped_up_balance":"1"}]}`},
		{name: "non-finite amount", body: `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"NaN","granted_balance":"0","topped_up_balance":"1"}]}`},
		{name: "duplicate currency", body: `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"1","granted_balance":"0","topped_up_balance":"1"},{"currency":"cny","total_balance":"2","granted_balance":"0","topped_up_balance":"2"}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "key"})
			if err == nil {
				t.Fatal("Refresh() error = nil, want invalid response error")
			}
		})
	}
}

func TestRefreshReportsHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "not authorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "bad-key"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("Refresh() error = %v, want HTTP 401 error", err)
	}
}

func TestRefreshPreservesUnavailableStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"is_available":false,"balance_infos":[{"currency":"CNY","total_balance":"0","granted_balance":"0","topped_up_balance":"0"}]}`))
	}))
	defer server.Close()

	metrics, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "key"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if metrics[0].Available == nil || *metrics[0].Available {
		t.Fatalf("available = %v, want explicit false", metrics[0].Available)
	}
	encoded, err := json.Marshal(metrics[0])
	if err != nil {
		t.Fatalf("marshal unavailable balance metric: %v", err)
	}
	if !strings.Contains(string(encoded), `"available":false`) {
		t.Fatalf("unavailable status was omitted from JSON: %s", encoded)
	}
}

func TestNewProviderUsesDefaultBaseURL(t *testing.T) {
	if got := NewProvider(Config{}).apiBaseURL; got != DefaultAPIBaseURL {
		t.Fatalf("apiBaseURL = %q, want %q", got, DefaultAPIBaseURL)
	}
}

func TestRefreshParsesDecimalAmounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"0.01","granted_balance":"0.00","topped_up_balance":"0.01"}]}`))
	}))
	defer server.Close()

	metrics, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "key"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if math.Abs(*metrics[0].Amount-0.01) > 0.000001 {
		t.Fatalf("amount = %v, want 0.01", *metrics[0].Amount)
	}
}
