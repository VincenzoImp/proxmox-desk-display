package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Proxmox []ProxmoxHost `yaml:"proxmox"`
	Alerts  AlertConfig   `yaml:"alerts"`
	Guests  GuestConfig   `yaml:"guests"`
}

type ServerConfig struct {
	Bind                string `yaml:"bind"`
	Port                int    `yaml:"port"`
	DisplayTokenEnv     string `yaml:"display_token_env"`
	PollIntervalSeconds int    `yaml:"poll_interval_seconds"`
	StaleAfterSeconds   int    `yaml:"stale_after_seconds"`
}

type ProxmoxHost struct {
	ID       string    `yaml:"id"`
	Name     string    `yaml:"name"`
	BaseURL  string    `yaml:"base_url"`
	TokenEnv string    `yaml:"token_env"`
	TLS      TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Mode        string `yaml:"mode"`
	Fingerprint string `yaml:"fingerprint"`
	CAFile      string `yaml:"ca_file"`
}

type AlertConfig struct {
	MemoryWarningPct   int `yaml:"memory_warning_pct"`
	MemoryCriticalPct  int `yaml:"memory_critical_pct"`
	StorageWarningPct  int `yaml:"storage_warning_pct"`
	StorageCriticalPct int `yaml:"storage_critical_pct"`
}

type GuestConfig struct {
	Pinned []PinnedGuest `yaml:"pinned"`
}

type PinnedGuest struct {
	Source   string `yaml:"source"`
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Expected string `yaml:"expected"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func MockConfig() Config {
	cfg := Config{
		Server: ServerConfig{
			Bind:                "0.0.0.0",
			Port:                8765,
			DisplayTokenEnv:     "DISPLAY_TOKEN",
			PollIntervalSeconds: 10,
			StaleAfterSeconds:   45,
		},
		Alerts: AlertConfig{
			MemoryWarningPct:   85,
			MemoryCriticalPct:  95,
			StorageWarningPct:  80,
			StorageCriticalPct: 90,
		},
	}
	return cfg
}

func (c *Config) applyDefaults() {
	if c.Server.Bind == "" {
		c.Server.Bind = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8765
	}
	if c.Server.DisplayTokenEnv == "" {
		c.Server.DisplayTokenEnv = "DISPLAY_TOKEN"
	}
	if c.Server.PollIntervalSeconds == 0 {
		c.Server.PollIntervalSeconds = 10
	}
	if c.Server.StaleAfterSeconds == 0 {
		c.Server.StaleAfterSeconds = 45
	}
	if c.Alerts.MemoryWarningPct == 0 {
		c.Alerts.MemoryWarningPct = 85
	}
	if c.Alerts.MemoryCriticalPct == 0 {
		c.Alerts.MemoryCriticalPct = 95
	}
	if c.Alerts.StorageWarningPct == 0 {
		c.Alerts.StorageWarningPct = 80
	}
	if c.Alerts.StorageCriticalPct == 0 {
		c.Alerts.StorageCriticalPct = 90
	}
	for i := range c.Proxmox {
		if c.Proxmox[i].TLS.Mode == "" {
			c.Proxmox[i].TLS.Mode = "fingerprint"
		}
	}
}

func (c Config) Validate(mock bool) error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if c.Server.DisplayToken() == "" && !mock {
		return fmt.Errorf("display token env %q is empty", c.Server.DisplayTokenEnv)
	}
	if !mock && len(c.Proxmox) == 0 {
		return errors.New("at least one proxmox entry is required")
	}
	seen := map[string]bool{}
	for _, pve := range c.Proxmox {
		if pve.ID == "" {
			return errors.New("proxmox.id is required")
		}
		if seen[pve.ID] {
			return fmt.Errorf("duplicate proxmox.id %q", pve.ID)
		}
		seen[pve.ID] = true
		if pve.Name == "" {
			return fmt.Errorf("proxmox %q name is required", pve.ID)
		}
		if pve.BaseURL == "" {
			return fmt.Errorf("proxmox %q base_url is required", pve.ID)
		}
		if pve.TokenEnv == "" {
			return fmt.Errorf("proxmox %q token_env is required", pve.ID)
		}
		if os.Getenv(pve.TokenEnv) == "" {
			return fmt.Errorf("proxmox %q token env %q is empty", pve.ID, pve.TokenEnv)
		}
		switch pve.TLS.Mode {
		case "fingerprint":
			if pve.TLS.Fingerprint == "" {
				return fmt.Errorf("proxmox %q tls.fingerprint is required for fingerprint mode", pve.ID)
			}
		case "ca_file":
			if pve.TLS.CAFile == "" {
				return fmt.Errorf("proxmox %q tls.ca_file is required for ca_file mode", pve.ID)
			}
		case "system", "insecure":
		default:
			return fmt.Errorf("proxmox %q has unsupported tls.mode %q", pve.ID, pve.TLS.Mode)
		}
	}
	return nil
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Bind, s.Port)
}

func (s ServerConfig) DisplayToken() string {
	return os.Getenv(s.DisplayTokenEnv)
}

func (s ServerConfig) PollInterval() time.Duration {
	return time.Duration(s.PollIntervalSeconds) * time.Second
}

func (s ServerConfig) StaleAfter() time.Duration {
	return time.Duration(s.StaleAfterSeconds) * time.Second
}

func NormalizeToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(token, "PVEAPIToken=") {
		return token
	}
	return "PVEAPIToken=" + token
}
