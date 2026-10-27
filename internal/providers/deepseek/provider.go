package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

const DefaultAPIBaseURL = "https://api.deepseek.com"

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

func (p *Provider) ID() domain.ProviderID { return domain.ProviderDeepSeek }
func (p *Provider) Name() string          { return "DeepSeek" }

func (p *Provider) Refresh(ctx context.Context, credentials domain.Credentials) ([]domain.Metric, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBaseURL+"/user/balance", nil)
	if err != nil {
		return nil, fmt.Errorf("create DeepSeek balance request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	request.Header.Set("Accept", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request DeepSeek balance: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("DeepSeek balance API returned HTTP %d", response.StatusCode)
	}

	var payload balanceResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode DeepSeek balance response: %w", err)
	}
	if payload.IsAvailable == nil {
		return nil, fmt.Errorf("DeepSeek balance response is missing is_available")
	}
	if len(payload.BalanceInfos) == 0 {
		return nil, fmt.Errorf("DeepSeek balance response contains no balances")
	}

	metrics := make([]domain.Metric, 0, len(payload.BalanceInfos))
	seenCurrencies := make(map[string]struct{}, len(payload.BalanceInfos))
	for _, info := range payload.BalanceInfos {
		currency := strings.TrimSpace(info.Currency)
		if currency == "" {
			return nil, fmt.Errorf("DeepSeek balance response contains an empty currency")
		}
		currencyKey := strings.ToUpper(currency)
		if _, exists := seenCurrencies[currencyKey]; exists {
			return nil, fmt.Errorf("DeepSeek balance response contains duplicate currency %q", currency)
		}
		seenCurrencies[currencyKey] = struct{}{}

		total, err := parseAmount(info.TotalBalance)
		if err != nil {
			return nil, fmt.Errorf("parse DeepSeek total balance for %s: %w", currency, err)
		}
		granted, err := parseAmount(info.GrantedBalance)
		if err != nil {
			return nil, fmt.Errorf("parse DeepSeek granted balance for %s: %w", currency, err)
		}
		toppedUp, err := parseAmount(info.ToppedUpBalance)
		if err != nil {
			return nil, fmt.Errorf("parse DeepSeek topped-up balance for %s: %w", currency, err)
		}

		available := *payload.IsAvailable
		metrics = append(metrics, domain.Metric{
			ID:        "balance-" + strings.ToLower(currency),
			Label:     "API balance",
			Amount:    &total,
			Currency:  currency,
			Available: &available,
			Breakdown: []domain.Breakdown{
				{Name: "Granted", Amount: &granted},
				{Name: "Topped up", Amount: &toppedUp},
			},
		})
	}
	return metrics, nil
}

type balanceResponse struct {
	IsAvailable  *bool         `json:"is_available"`
	BalanceInfos []balanceInfo `json:"balance_infos"`
}

type balanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

func parseAmount(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("amount is empty")
	}
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return 0, fmt.Errorf("amount %q must be a finite non-negative number", value)
	}
	return amount, nil
}
