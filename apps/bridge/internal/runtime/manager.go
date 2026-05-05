package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/configstore"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/proxmox"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/store"
)

type Manager struct {
	mu      sync.RWMutex
	cfg     config.Config
	secrets configstore.Secrets
	store   configstore.Store
	cache   *store.Cache
	mock    bool
}

type Snapshot struct {
	ConfigPath      string
	SecretsPath     string
	Mock            bool
	AdminConfigured bool
	DisplayTokenSet bool
	AdminTokenSet   bool
	Server          config.ServerConfig
	Alerts          config.AlertConfig
	Sources         []SourceSnapshot
}

type SourceSnapshot struct {
	ID          string
	Name        string
	BaseURL     string
	TLSMode     string
	Fingerprint string
	CAFile      string
	TokenSet    bool
}

type SourceDiagnostic struct {
	Message     string
	Fingerprint string
}

type ServerUpdate struct {
	PollIntervalSeconds int
	StaleAfterSeconds   int
	DisplayToken        string
	AdminToken          string
	MemoryWarningPct    int
	MemoryCriticalPct   int
	StorageWarningPct   int
	StorageCriticalPct  int
}

type SourceUpdate struct {
	ID          string
	Name        string
	BaseURL     string
	Token       string
	TLSMode     string
	Fingerprint string
	CAFile      string
}

func NewManager(cfg config.Config, secrets configstore.Secrets, cfgStore configstore.Store, cache *store.Cache, mock bool) *Manager {
	return &Manager{cfg: cfg, secrets: secrets, store: cfgStore, cache: cache, mock: mock}
}

func (m *Manager) Config() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) AdminToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.Server.AdminToken()
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap := Snapshot{
		ConfigPath:      m.store.ConfigPath,
		SecretsPath:     m.store.SecretsPath,
		Mock:            m.mock,
		AdminConfigured: m.cfg.Server.AdminToken() != "",
		DisplayTokenSet: m.cfg.Server.DisplayToken() != "",
		AdminTokenSet:   strings.TrimSpace(m.secrets.AdminToken) != "" || strings.TrimSpace(m.cfg.Server.AdminTokenValue) != "",
		Server:          m.cfg.Server,
		Alerts:          m.cfg.Alerts,
		Sources:         make([]SourceSnapshot, 0, len(m.cfg.Proxmox)),
	}
	for _, source := range m.cfg.Proxmox {
		snap.Sources = append(snap.Sources, SourceSnapshot{
			ID:          source.ID,
			Name:        source.Name,
			BaseURL:     source.BaseURL,
			TLSMode:     source.TLS.Mode,
			Fingerprint: source.TLS.Fingerprint,
			CAFile:      source.TLS.CAFile,
			TokenSet:    source.TokenValue != "" || source.TokenEnv != "",
		})
	}
	return snap
}

func (m *Manager) UpdateServer(ctx context.Context, input ServerUpdate) error {
	cfg, secrets := m.clone()
	if input.PollIntervalSeconds < 5 {
		return errors.New("poll interval must be at least 5 seconds")
	}
	if input.StaleAfterSeconds < input.PollIntervalSeconds {
		return errors.New("stale after must be greater than or equal to poll interval")
	}
	cfg.Server.PollIntervalSeconds = input.PollIntervalSeconds
	cfg.Server.StaleAfterSeconds = input.StaleAfterSeconds
	cfg.Alerts.MemoryWarningPct = clamp(input.MemoryWarningPct, 1, 100)
	cfg.Alerts.MemoryCriticalPct = clamp(input.MemoryCriticalPct, cfg.Alerts.MemoryWarningPct, 100)
	cfg.Alerts.StorageWarningPct = clamp(input.StorageWarningPct, 1, 100)
	cfg.Alerts.StorageCriticalPct = clamp(input.StorageCriticalPct, cfg.Alerts.StorageWarningPct, 100)
	if strings.TrimSpace(input.DisplayToken) != "" {
		secrets.DisplayToken = strings.TrimSpace(input.DisplayToken)
	}
	if strings.TrimSpace(input.AdminToken) != "" {
		secrets.AdminToken = strings.TrimSpace(input.AdminToken)
	}
	configstore.ApplySecrets(&cfg, secrets)
	return m.apply(ctx, cfg, secrets)
}

func (m *Manager) UpsertSource(ctx context.Context, input SourceUpdate) error {
	cfg, secrets := m.clone()
	source, err := sourceFromUpdate(cfg, secrets, input, true)
	if err != nil {
		return err
	}
	id := source.ID
	idx := sourceIndex(cfg, id)
	if token := strings.TrimSpace(input.Token); token != "" {
		if secrets.ProxmoxTokens == nil {
			secrets.ProxmoxTokens = map[string]string{}
		}
		secrets.ProxmoxTokens[id] = token
	}
	if idx >= 0 {
		cfg.Proxmox[idx] = source
	} else {
		cfg.Proxmox = append(cfg.Proxmox, source)
	}
	configstore.ApplySecrets(&cfg, secrets)
	return m.apply(ctx, cfg, secrets)
}

func (m *Manager) TestSource(ctx context.Context, input SourceUpdate) (SourceDiagnostic, error) {
	cfg, secrets := m.clone()
	source, err := sourceFromUpdate(cfg, secrets, input, true)
	if err != nil {
		return SourceDiagnostic{}, err
	}
	result, err := proxmox.TestHost(ctx, source)
	if err != nil {
		return SourceDiagnostic{}, err
	}
	message := "connection OK"
	if result.Version != "" {
		message += ": Proxmox VE " + result.Version
		if result.Release != "" {
			message += " (" + result.Release + ")"
		}
	}
	return SourceDiagnostic{Message: message, Fingerprint: result.Fingerprint}, nil
}

func (m *Manager) DetectFingerprint(ctx context.Context, baseURL string) (string, error) {
	return proxmox.DetectFingerprint(ctx, baseURL)
}

func (m *Manager) DeleteSource(ctx context.Context, id string) error {
	id = configstore.SanitizeID(id)
	if id == "" {
		return errors.New("source id is required")
	}
	cfg, secrets := m.clone()
	next := cfg.Proxmox[:0]
	found := false
	for _, source := range cfg.Proxmox {
		if source.ID == id {
			found = true
			continue
		}
		next = append(next, source)
	}
	if !found {
		return fmt.Errorf("source %q was not found", id)
	}
	cfg.Proxmox = next
	delete(secrets.ProxmoxTokens, id)
	configstore.ApplySecrets(&cfg, secrets)
	return m.apply(ctx, cfg, secrets)
}

func (m *Manager) Refresh(ctx context.Context) error {
	return m.cache.Refresh(ctx)
}

func (m *Manager) clone() (config.Config, configstore.Secrets) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.cfg
	cfg.Proxmox = append([]config.ProxmoxHost(nil), m.cfg.Proxmox...)
	cfg.Guests.Pinned = append([]config.PinnedGuest(nil), m.cfg.Guests.Pinned...)
	secrets := m.secrets
	secrets.ProxmoxTokens = map[string]string{}
	for id, token := range m.secrets.ProxmoxTokens {
		secrets.ProxmoxTokens[id] = token
	}
	return cfg, secrets
}

func (m *Manager) apply(ctx context.Context, cfg config.Config, secrets configstore.Secrets) error {
	cfg.ApplyDefaults()
	configstore.ApplySecrets(&cfg, secrets)
	collector, err := CollectorForConfig(cfg, m.mock)
	if err != nil {
		return err
	}
	if err := m.store.Save(cfg, secrets); err != nil {
		return err
	}
	m.cache.Reconfigure(collector, cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	m.mu.Lock()
	m.cfg = cfg
	m.secrets = secrets
	m.mu.Unlock()
	return m.cache.Refresh(ctx)
}

func CollectorForConfig(cfg config.Config, mock bool) (store.Collector, error) {
	if mock {
		return store.NewMockCollector(), nil
	}
	if len(cfg.Proxmox) == 0 {
		return store.NewEmptyCollector(), nil
	}
	if err := cfg.Validate(false); err != nil {
		return nil, err
	}
	return proxmox.NewCollector(cfg)
}

func sourceIndex(cfg config.Config, id string) int {
	for i, source := range cfg.Proxmox {
		if source.ID == id {
			return i
		}
	}
	return -1
}

func sourceFromUpdate(cfg config.Config, secrets configstore.Secrets, input SourceUpdate, requireToken bool) (config.ProxmoxHost, error) {
	id := configstore.SanitizeID(input.ID)
	if id == "" {
		id = configstore.SanitizeID(input.Name)
	}
	if id == "" {
		return config.ProxmoxHost{}, errors.New("source id or name is required")
	}
	idx := sourceIndex(cfg, id)
	var existing config.ProxmoxHost
	if idx >= 0 {
		existing = cfg.Proxmox[idx]
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = existing.Name
	}
	if name == "" {
		name = id
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if baseURL == "" {
		baseURL = existing.BaseURL
	}
	if baseURL == "" {
		return config.ProxmoxHost{}, errors.New("base URL is required")
	}
	tlsMode := strings.TrimSpace(input.TLSMode)
	if tlsMode == "" {
		tlsMode = existing.TLS.Mode
	}
	if tlsMode == "" {
		tlsMode = "fingerprint"
	}
	token := strings.TrimSpace(input.Token)
	tokenValue := token
	if tokenValue == "" {
		tokenValue = strings.TrimSpace(secrets.ProxmoxTokens[id])
	}
	tokenEnv := ""
	if tokenValue == "" {
		tokenEnv = existing.TokenEnv
	}
	if requireToken && tokenValue == "" && tokenEnv == "" {
		return config.ProxmoxHost{}, errors.New("token is required")
	}
	source := config.ProxmoxHost{
		ID:         id,
		Name:       name,
		BaseURL:    baseURL,
		TokenEnv:   tokenEnv,
		TokenValue: tokenValue,
		TLS: config.TLSConfig{
			Mode:        tlsMode,
			Fingerprint: firstNonEmpty(strings.TrimSpace(input.Fingerprint), existing.TLS.Fingerprint),
			CAFile:      firstNonEmpty(strings.TrimSpace(input.CAFile), existing.TLS.CAFile),
		},
	}
	if strings.TrimSpace(input.Fingerprint) == "" && input.TLSMode != "" {
		source.TLS.Fingerprint = ""
	}
	if strings.TrimSpace(input.CAFile) == "" && input.TLSMode != "" {
		source.TLS.CAFile = ""
	}
	return source, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func clamp(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
