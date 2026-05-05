package configstore

import (
	"path/filepath"
	"testing"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
)

func TestStoreSavesConfigAndSecretsSeparately(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, "")
	cfg := config.NewDefault()
	cfg.Server.DisplayTokenValue = "display"
	cfg.Proxmox = []config.ProxmoxHost{
		{
			ID:         "lab",
			Name:       "Lab",
			BaseURL:    "https://pve.example:8006",
			TokenValue: "monitor@pve!desk=secret",
			TLS:        config.TLSConfig{Mode: "insecure"},
		},
	}
	secrets := Secrets{
		AdminToken:    "admin",
		DisplayToken:  "display",
		ProxmoxTokens: map[string]string{"lab": "monitor@pve!desk=secret"},
	}

	if err := store.Save(cfg, secrets); err != nil {
		t.Fatal(err)
	}
	loaded, loadedSecrets, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server.DisplayToken() != "display" {
		t.Fatalf("display token was not applied")
	}
	if len(loaded.Proxmox) != 1 || loaded.Proxmox[0].TokenValue != "monitor@pve!desk=secret" {
		t.Fatalf("Proxmox token was not restored: %#v", loaded.Proxmox)
	}
	if loadedSecrets.AdminToken != "admin" {
		t.Fatalf("admin token was not restored")
	}
	if store.ConfigPath != filepath.Join(dir, DefaultConfigFile) {
		t.Fatalf("unexpected config path %q", store.ConfigPath)
	}
}

func TestSanitizeID(t *testing.T) {
	if got := SanitizeID("Jonsbo N4.local"); got != "jonsbo-n4-local" {
		t.Fatalf("SanitizeID() = %q", got)
	}
}
