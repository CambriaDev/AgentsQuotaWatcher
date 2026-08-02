package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

const DefaultCreditLimit = 1500

func NewProvider(config Config) *Provider {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Provider{apiBaseURL: strings.TrimRight(config.APIBaseURL, "/"), client: client}
}

func (p *Provider) ID() domain.ProviderID { return domain.ProviderCopilot }
func (p *Provider) Name() string          { return "GitHub Copilot" }

func (p *Provider) Refresh(ctx context.Context, credentials domain.Credentials) ([]domain.Metric, error) {
	username := strings.TrimSpace(credentials.Username)
	if username == "" {
		return nil, fmt.Errorf("GitHub username is required for Copilot")
	}
	endpoint := fmt.Sprintf("%s/users/%s/settings/billing/ai_credit/usage",
		p.apiBaseURL, url.PathEscape(username))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create Copilot usage request: %w", err)
	}
	p.setHeaders(request, credentials.AccessToken)
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Copilot usage: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Copilot billing API returned HTTP %d", response.StatusCode)
	}
	var payload usageResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Copilot usage response: %w", err)
	}
	return []domain.Metric{payload.metric(credentials.CreditLimit)}, nil
}

func (p *Provider) setHeaders(request *http.Request, token string) {
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
}

type usageResponse struct {
	UsageItems      []usageItem `json:"usageItems"`
	UsageItemsSnake []usageItem `json:"usage_items"`
}

type usageItem struct {
	Model                 string  `json:"model"`
	GrossQuantity         float64 `json:"grossQuantity"`
	GrossQuantitySnake    float64 `json:"gross_quantity"`
	DiscountQuantity      float64 `json:"discountQuantity"`
	DiscountQuantitySnake float64 `json:"discount_quantity"`
}

func (r usageResponse) metric(configuredLimit int) domain.Metric {
	used := 0.0
	items := r.UsageItems
	if items == nil {
		items = r.UsageItemsSnake
	}
	breakdown := make([]domain.Breakdown, 0, len(items))
	for _, item := range items {
		gross := item.GrossQuantity
		if gross == 0 {
			gross = item.GrossQuantitySnake
		}
		used += gross
		if gross > 0 {
			name := item.Model
			if name == "" {
				name = "Unknown model"
			}
			breakdown = append(breakdown, domain.Breakdown{Name: name, Used: gross})
		}
	}
	limit := float64(configuredLimit)
	if limit == 0 {
		limit = DefaultCreditLimit
	}
	percent := 0.0
	if limit > 0 {
		percent = used / limit * 100
		if percent > 100 {
			percent = 100
		}
	}
	return domain.Metric{
		ID: "ai-credits", Label: "Monthly AI credits",
		Used: used, Limit: limit, Percent: percent, Breakdown: breakdown,
	}
}
