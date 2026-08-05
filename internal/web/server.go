package web

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/cambriadev/agents-quota-watcher/internal/app"
	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

type Server struct {
	service   *app.QuotaService
	dashboard *template.Template
	auth      basicAuth
}

type basicAuth struct {
	username string
	password string
}

func NewServer(service *app.QuotaService, username, password string) (*Server, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("authentication username and password are required")
	}
	return &Server{
		service:   service,
		dashboard: template.Must(template.New("dashboard").Parse(dashboardHTML)),
		auth:      basicAuth{username: username, password: password},
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleDashboard)
	mux.HandleFunc("GET /api/usage", s.handleUsage)
	mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	mux.HandleFunc("POST /api/accounts", s.createAccount)
	mux.HandleFunc("/api/accounts/", s.handleAccount)
	return securityHeaders(s.auth.middleware(mux))
}

func (s *Server) handleDashboard(writer http.ResponseWriter, request *http.Request) {
	if err := s.dashboard.Execute(writer, nil); err != nil {
		http.Error(writer, "render dashboard", http.StatusInternalServerError)
	}
}

func (s *Server) handleUsage(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, s.service.Snapshots())
}

func (s *Server) handleRefresh(writer http.ResponseWriter, request *http.Request) {
	s.service.RefreshAll(request.Context())
	writeJSON(writer, http.StatusOK, s.service.Snapshots())
}

func (s *Server) handleAccount(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/accounts/")
	parts := strings.Split(path, "/")
	if parts[0] == "" || len(parts) > 2 || (len(parts) == 2 && parts[1] != "refresh") {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodPost:
		if len(parts) != 2 {
			http.NotFound(writer, request)
			return
		}
		if err := s.service.Refresh(request.Context(), parts[0]); err != nil {
			writeError(writer, http.StatusBadGateway, err)
			return
		}
		writeJSON(writer, http.StatusOK, s.service.Snapshots())
	case http.MethodDelete:
		if len(parts) != 1 {
			http.NotFound(writer, request)
			return
		}
		if err := s.service.DeleteAccount(parts[0]); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		writeJSON(writer, http.StatusOK, s.service.Snapshots())
	default:
		writer.Header().Set("Allow", "POST, DELETE")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type createAccountRequest struct {
	Provider    domain.ProviderID  `json:"provider"`
	Alias       string             `json:"alias"`
	Credentials domain.Credentials `json:"credentials"`
}

func (s *Server) createAccount(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	var input createAccountRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, fmt.Errorf("invalid account: %w", err))
		return
	}
	if !input.Credentials.ExpiresAt.IsZero() {
		input.Credentials.ExpiresAt = input.Credentials.ExpiresAt.UTC()
	}
	if _, err := s.service.CreateAccount(input.Provider, input.Alias, input.Credentials); err != nil {
		if strings.Contains(err.Error(), "unknown provider") || strings.Contains(err.Error(), "required") {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		writeError(writer, http.StatusBadGateway, err)
		return
	}
	writeJSON(writer, http.StatusOK, s.service.Snapshots())
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return
	}
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(writer, request)
	})
}

func (auth basicAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(username), []byte(auth.username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(auth.password)) != 1 {
			writer.Header().Set("WWW-Authenticate", `Basic realm="Agents Quota Watcher", charset="UTF-8"`)
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
