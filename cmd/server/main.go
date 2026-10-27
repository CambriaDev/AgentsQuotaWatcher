package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cambriadev/agents-quota-watcher/internal/app"
	"github.com/cambriadev/agents-quota-watcher/internal/providers"
	"github.com/cambriadev/agents-quota-watcher/internal/providers/claude"
	"github.com/cambriadev/agents-quota-watcher/internal/providers/copilot"
	"github.com/cambriadev/agents-quota-watcher/internal/providers/deepseek"
	"github.com/cambriadev/agents-quota-watcher/internal/store"
	"github.com/cambriadev/agents-quota-watcher/internal/web"
)

func main() {
	address := envOrDefault("QUOTA_LISTEN_ADDR", "127.0.0.1:48080")
	credentialPath := envOrDefault("QUOTA_CREDENTIAL_FILE", defaultCredentialPath())
	authUsername := requiredEnv("QUOTA_AUTH_USERNAME")
	authPassword := requiredEnv("QUOTA_AUTH_PASSWORD")

	credentials, err := store.NewFileCredentialStore(credentialPath)
	if err != nil {
		log.Fatalf("initialize credential store: %v", err)
	}

	registry := providers.NewRegistry(
		claude.NewProvider(claude.Config{
			APIBaseURL: envOrDefault("CLAUDE_API_BASE_URL", "https://api.anthropic.com"),
		}),
		copilot.NewProvider(copilot.Config{
			APIBaseURL: envOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
		}),
		deepseek.NewProvider(deepseek.Config{
			APIBaseURL: envOrDefault("DEEPSEEK_API_BASE_URL", deepseek.DefaultAPIBaseURL),
		}),
	)
	service := app.NewQuotaService(registry, credentials)
	server, err := web.NewServer(service, authUsername, authPassword)
	if err != nil {
		log.Fatalf("initialize web server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go service.RefreshAll(context.Background())
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			service.RefreshAll(context.Background())
		}
	}()

	go func() {
		log.Printf("Quota Watcher listening on http://%s", address)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve HTTP: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s must be set", name)
	}
	return value
}

func defaultCredentialPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "credentials.json"
	}
	return filepath.Join(home, ".agents-quota-watcher", "credentials.json")
}
