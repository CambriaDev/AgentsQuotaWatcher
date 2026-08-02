package copilot

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

func TestRefreshFetchesMonthlyUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/users/octocat/settings/billing/ai_credit/usage":
			if request.URL.RawQuery != "" {
				t.Fatalf("unexpected query: %s", request.URL.RawQuery)
			}
			if request.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
				t.Fatalf("unexpected API version: %s", request.Header.Get("X-GitHub-Api-Version"))
			}
			_, _ = writer.Write([]byte(`{"usageItems":[
				{"model":"gpt-5","grossQuantity":100.25,"discountQuantity":100.25},
				{"model":"claude","grossQuantity":80.5,"discountQuantity":80.5}
			]}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewProvider(Config{
		APIBaseURL: server.URL,
	})
	metrics, err := provider.Refresh(context.Background(), domain.Credentials{AccessToken: "token", Username: "octocat"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	metric := metrics[0]
	if metric.Used != 180.75 || metric.Limit != DefaultCreditLimit || math.Abs(metric.Percent-12.05) > 0.000001 {
		t.Fatalf("metric = %#v, want used=180.75 limit=1500 percent=12.05", metric)
	}
	if len(metric.Breakdown) != 2 {
		t.Fatalf("breakdown = %#v", metric.Breakdown)
	}
}

func TestRefreshRequiresUsername(t *testing.T) {
	_, err := NewProvider(Config{}).Refresh(context.Background(), domain.Credentials{AccessToken: "token"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want missing username error")
	}
}

func TestUsageResponseSupportsSnakeCase(t *testing.T) {
	metric := usageResponse{UsageItemsSnake: []usageItem{{
		Model: "model", GrossQuantitySnake: 30, DiscountQuantitySnake: 30,
	}}}.metric(0)
	if metric.Used != 30 || metric.Limit != DefaultCreditLimit || metric.Percent != 2 {
		t.Fatalf("metric = %#v", metric)
	}
}

func TestUsageResponseUsesConfiguredCreditLimit(t *testing.T) {
	metric := usageResponse{UsageItems: []usageItem{{Model: "model", GrossQuantity: 200}}}.metric(2000)
	if metric.Limit != 2000 || metric.Percent != 10 {
		t.Fatalf("metric = %#v, want limit=2000 percent=10", metric)
	}
}
