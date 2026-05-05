package runtime

import (
	"testing"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/configstore"
)

func TestSourceFromUpdateUsesSavedTokenAndExistingFields(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Proxmox = []config.ProxmoxHost{
		{
			ID:      "lab",
			Name:    "Lab",
			BaseURL: "https://pve.example:8006",
			TLS:     config.TLSConfig{Mode: "fingerprint", Fingerprint: "SHA256:abc"},
		},
	}
	secrets := configstore.Secrets{ProxmoxTokens: map[string]string{"lab": "monitor@pve!desk=secret"}}

	got, err := sourceFromUpdate(cfg, secrets, SourceUpdate{ID: "lab"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.TokenValue != "monitor@pve!desk=secret" {
		t.Fatalf("TokenValue = %q", got.TokenValue)
	}
	if got.BaseURL != "https://pve.example:8006" || got.TLS.Fingerprint != "SHA256:abc" {
		t.Fatalf("existing fields not preserved: %#v", got)
	}
}
