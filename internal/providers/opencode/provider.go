package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

const DefaultAPIBaseURL = "https://opencode.ai/zen/go/v1"

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
	apiBaseURL := strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	return &Provider{apiBaseURL: apiBaseURL, client: client}
}

func (p *Provider) ID() domain.ProviderID { return domain.ProviderOpenCode }
func (p *Provider) Name() string          { return "OpenCode Go" }

func (p *Provider) Refresh(ctx context.Context, credentials domain.Credentials) ([]domain.Metric, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBaseURL+"/usage", nil)
	if err != nil {
		return nil, fmt.Errorf("create OpenCode Go usage request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	request.Header.Set("Accept", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request OpenCode Go usage: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenCode Go usage API returned HTTP %d", response.StatusCode)
	}

	var payload usageResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode OpenCode Go usage response: %w", err)
	}

	metrics := make([]domain.Metric, 0, 3)
	for _, window := range []struct {
		id    string
		label string
		usage *usageWindow
	}{
		{id: "rolling", label: "Rolling (5-hour)", usage: payload.Usage.Rolling},
		{id: "weekly", label: "Weekly", usage: payload.Usage.Weekly},
		{id: "monthly", label: "Monthly", usage: payload.Usage.Monthly},
	} {
		if window.usage == nil {
			continue
		}
		if window.usage.Percent == nil || math.IsNaN(*window.usage.Percent) ||
			math.IsInf(*window.usage.Percent, 0) || *window.usage.Percent < 0 || *window.usage.Percent > 100 {
			return nil, fmt.Errorf("OpenCode Go %s usage contains an invalid percent", window.id)
		}
		metrics = append(metrics, domain.Metric{
			ID:      window.id,
			Label:   window.label,
			Percent: *window.usage.Percent,
			ResetAt: window.usage.ResetsAt,
		})
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("OpenCode Go usage response contains no recognized quota windows")
	}
	return metrics, nil
}

type usageResponse struct {
	Usage struct {
		Rolling *usageWindow `json:"rolling"`
		Weekly  *usageWindow `json:"weekly"`
		Monthly *usageWindow `json:"monthly"`
	} `json:"usage"`
}

type usageWindow struct {
	Percent  *float64   `json:"percent"`
	ResetsAt *time.Time `json:"resetsAt"`
}
