package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

type Config struct {
	APIBaseURL string
	HTTPClient *http.Client
}

type Provider struct {
	apiBaseURL string
	client     *http.Client
}

func NewProvider(config Config) *Provider {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Provider{
		apiBaseURL: strings.TrimRight(config.APIBaseURL, "/"),
		client:     client,
	}
}

func (p *Provider) ID() domain.ProviderID { return domain.ProviderClaude }
func (p *Provider) Name() string          { return "Claude" }

func (p *Provider) Refresh(ctx context.Context, credentials domain.Credentials) ([]domain.Metric, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBaseURL+"/api/oauth/usage", nil)
	if err != nil {
		return nil, fmt.Errorf("create Claude usage request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	request.Header.Set("Anthropic-Beta", "oauth-2025-04-20")
	request.Header.Set("Accept", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Claude usage: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Claude usage API returned HTTP %d", response.StatusCode)
	}

	var payload usageResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Claude usage response: %w", err)
	}
	metrics := make([]domain.Metric, 0, 3)
	if payload.FiveHour != nil {
		metrics = append(metrics, payload.FiveHour.metric("five-hour", "5-hour session"))
	}
	if payload.SevenDay != nil {
		metrics = append(metrics, payload.SevenDay.metric("seven-day", "7-day all models"))
	}
	if payload.SevenDaySonnet != nil {
		metrics = append(metrics, payload.SevenDaySonnet.metric("seven-day-sonnet", "7-day Sonnet"))
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("Claude usage response contains no recognized buckets")
	}
	return metrics, nil
}

type usageResponse struct {
	FiveHour       *bucket `json:"five_hour"`
	SevenDay       *bucket `json:"seven_day"`
	SevenDaySonnet *bucket `json:"seven_day_sonnet"`
}

type bucket struct {
	Utilization float64    `json:"utilization"`
	ResetAt     *time.Time `json:"resets_at"`
}

func (b bucket) metric(id, label string) domain.Metric {
	percent := b.Utilization
	if percent <= 1 {
		percent *= 100
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return domain.Metric{ID: id, Label: label, Percent: percent, ResetAt: b.ResetAt}
}
