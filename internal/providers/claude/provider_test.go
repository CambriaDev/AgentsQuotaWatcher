package claude

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

func TestRefreshParsesUsageBuckets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/oauth/usage" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing bearer token")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"five_hour":{"utilization":0.25,"resets_at":"2026-08-01T12:00:00Z"},
			"seven_day":{"utilization":50},
			"seven_day_sonnet":{"utilization":1.2}
		}`))
	}))
	defer server.Close()

	provider := NewProvider(Config{APIBaseURL: server.URL})
	metrics, err := provider.Refresh(context.Background(), domain.Credentials{AccessToken: "token"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("metric count = %d, want 3", len(metrics))
	}
	if metrics[0].Percent != 25 || metrics[1].Percent != 50 || metrics[2].Percent != 1.2 {
		t.Fatalf("unexpected percentages: %#v", metrics)
	}
	wantReset, _ := time.Parse(time.RFC3339, "2026-08-01T12:00:00Z")
	if metrics[0].ResetAt == nil || !metrics[0].ResetAt.Equal(wantReset) {
		t.Fatalf("reset = %v, want %v", metrics[0].ResetAt, wantReset)
	}
}

func TestRefreshRejectsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "token"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want unrecognized response error")
	}
}
