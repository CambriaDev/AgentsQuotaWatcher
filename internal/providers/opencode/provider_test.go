package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

func TestRefreshFetchesGoUsageWindows(t *testing.T) {
	resetAt := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/zen/go/v1/usage" {
			t.Errorf("request = %s %s, want GET /zen/go/v1/usage", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer go-key" {
			t.Errorf("authorization = %q, want bearer key", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("accept = %q, want application/json", request.Header.Get("Accept"))
		}
		_, _ = writer.Write([]byte(`{"usage":{
			"rolling":{"status":"ok","percent":12,"resetsAt":"2026-11-04T00:00:00Z"},
			"weekly":{"status":"ok","percent":34,"resetsAt":"2026-11-09T00:00:00Z"},
			"monthly":{"status":"ok","percent":56,"resetsAt":"2026-12-01T00:00:00Z"}
		}}`))
	}))
	defer server.Close()

	provider := NewProvider(Config{APIBaseURL: server.URL + "/zen/go/v1"})
	metrics, err := provider.Refresh(context.Background(), domain.Credentials{AccessToken: "go-key"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("metrics = %#v, want three quota windows", metrics)
	}
	if metrics[0].ID != "rolling" || metrics[0].Label != "Rolling (5-hour)" ||
		metrics[0].Percent != 12 || metrics[0].ResetAt == nil || !metrics[0].ResetAt.Equal(resetAt) {
		t.Fatalf("rolling metric = %#v", metrics[0])
	}
	if metrics[1].ID != "weekly" || metrics[1].Percent != 34 || metrics[1].ResetAt == nil {
		t.Fatalf("weekly metric = %#v", metrics[1])
	}
	if metrics[2].ID != "monthly" || metrics[2].Percent != 56 || metrics[2].ResetAt == nil {
		t.Fatalf("monthly metric = %#v", metrics[2])
	}
}

func TestRefreshRejectsInvalidPercent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"usage":{"rolling":{"percent":101}}}`))
	}))
	defer server.Close()

	_, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "key"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want invalid percent error")
	}
}

func TestRefreshRejectsResponseWithoutQuotaWindows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"usage":{}}`))
	}))
	defer server.Close()

	_, err := NewProvider(Config{APIBaseURL: server.URL}).Refresh(context.Background(), domain.Credentials{AccessToken: "key"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want missing windows error")
	}
}
