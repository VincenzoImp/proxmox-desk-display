package config

import "testing"

func TestNormalizeToken(t *testing.T) {
	tests := map[string]string{
		"monitor@pve!desk=secret":             "PVEAPIToken=monitor@pve!desk=secret",
		"PVEAPIToken=monitor@pve!desk=secret": "PVEAPIToken=monitor@pve!desk=secret",
	}
	for in, want := range tests {
		if got := NormalizeToken(in); got != want {
			t.Fatalf("NormalizeToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateRequiresDisplayToken(t *testing.T) {
	t.Setenv("PVE_TOKEN", "monitor@pve!desk=secret")
	cfg := Config{
		Server: ServerConfig{
			Bind:            "127.0.0.1",
			Port:            8765,
			DisplayTokenEnv: "DISPLAY_TOKEN",
		},
		Proxmox: []ProxmoxHost{
			{
				ID:       "lab",
				Name:     "Lab",
				BaseURL:  "https://pve.example:8006",
				TokenEnv: "PVE_TOKEN",
				TLS:      TLSConfig{Mode: "insecure"},
			},
		},
	}
	t.Setenv("DISPLAY_TOKEN", "")
	if err := cfg.Validate(false); err == nil {
		t.Fatal("expected missing display token error")
	}
}

func TestValidateAllowsMockWithoutSecrets(t *testing.T) {
	cfg := MockConfig()
	if err := cfg.Validate(true); err != nil {
		t.Fatalf("mock config should validate: %v", err)
	}
}

func TestValidateAcceptsStoredTokenValue(t *testing.T) {
	t.Setenv("DISPLAY_TOKEN", "display")
	cfg := NewDefault()
	cfg.Proxmox = []ProxmoxHost{
		{
			ID:         "lab",
			Name:       "Lab",
			BaseURL:    "https://pve.example:8006",
			TokenValue: "monitor@pve!desk=secret",
			TLS:        TLSConfig{Mode: "insecure"},
		},
	}
	if err := cfg.Validate(false); err != nil {
		t.Fatalf("stored token config should validate: %v", err)
	}
}

func TestAdminTokenFallsBackToDisplayToken(t *testing.T) {
	t.Setenv("DISPLAY_TOKEN", "display")
	cfg := NewDefault()
	if got := cfg.Server.AdminToken(); got != "display" {
		t.Fatalf("AdminToken() = %q, want display fallback", got)
	}
	cfg.Server.AdminTokenValue = "admin"
	if got := cfg.Server.AdminToken(); got != "admin" {
		t.Fatalf("AdminToken() = %q, want explicit admin token", got)
	}
}
