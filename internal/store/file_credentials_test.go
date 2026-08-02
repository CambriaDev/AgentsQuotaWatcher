package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cambriadev/agents-quota-watcher/internal/domain"
)

func TestFileCredentialStoreSupportsMultipleAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	store, err := NewFileCredentialStore(path)
	if err != nil {
		t.Fatalf("NewFileCredentialStore() error = %v", err)
	}
	first := domain.Account{ID: "first", Provider: domain.ProviderCopilot, Alias: "Work", Credentials: domain.Credentials{AccessToken: "one"}}
	second := domain.Account{ID: "second", Provider: domain.ProviderCopilot, Alias: "Personal", Credentials: domain.Credentials{AccessToken: "two"}}
	if err := store.Save(first); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	if err := store.Save(second); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}
	accounts, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(accounts) != 2 || accounts[0] != first || accounts[1] != second {
		t.Fatalf("List() = %#v, want both accounts", accounts)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %#o, want 0600", info.Mode().Perm())
	}
	if err := store.Delete(first.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	accounts, err = store.List()
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(accounts) != 1 || accounts[0] != second {
		t.Fatalf("List() after delete = %#v, want second account only", accounts)
	}
}

func TestFileCredentialStoreMigratesLegacyCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	legacy := `{"copilot":{"access_token":"secret","username":"octocat","credit_limit":1500}}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatalf("write legacy credentials: %v", err)
	}
	store, err := NewFileCredentialStore(path)
	if err != nil {
		t.Fatalf("NewFileCredentialStore() error = %v", err)
	}
	accounts, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != "legacy-copilot" || accounts[0].Alias != "GitHub Copilot" {
		t.Fatalf("migrated accounts = %#v", accounts)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}
	if string(data) == legacy || !contains(string(data), `"accounts"`) {
		t.Fatalf("credentials were not migrated: %s", data)
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
