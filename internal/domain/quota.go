package domain

import (
	"context"
	"errors"
	"time"
)

type ProviderID string

const (
	ProviderClaude  ProviderID = "claude"
	ProviderCopilot ProviderID = "copilot"
)

var ErrNotConnected = errors.New("provider is not connected")

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	Username     string    `json:"username,omitempty"`
	CreditLimit  int       `json:"credit_limit,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type Account struct {
	ID          string      `json:"id"`
	Provider    ProviderID  `json:"provider"`
	Alias       string      `json:"alias"`
	Credentials Credentials `json:"credentials"`
}

func (c Credentials) Connected() bool {
	return c.AccessToken != ""
}

type Breakdown struct {
	Name string  `json:"name"`
	Used float64 `json:"used"`
}

type Metric struct {
	ID        string      `json:"id"`
	Label     string      `json:"label"`
	Used      float64     `json:"used"`
	Limit     float64     `json:"limit"`
	Percent   float64     `json:"percent"`
	ResetAt   *time.Time  `json:"reset_at,omitempty"`
	Breakdown []Breakdown `json:"breakdown,omitempty"`
}

type Snapshot struct {
	AccountID string     `json:"account_id"`
	Provider  ProviderID `json:"provider"`
	Name      string     `json:"name"`
	Metrics   []Metric   `json:"metrics"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type Provider interface {
	ID() ProviderID
	Name() string
	Refresh(context.Context, Credentials) ([]Metric, error)
}

type AccountStore interface {
	List() ([]Account, error)
	Save(Account) error
	Delete(string) error
}
