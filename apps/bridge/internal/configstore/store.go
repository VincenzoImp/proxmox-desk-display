package configstore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile  = "config.yaml"
	DefaultSecretsFile = "secrets.yaml"
)

type Secrets struct {
	AdminToken    string            `yaml:"admin_token,omitempty"`
	DisplayToken  string            `yaml:"display_token,omitempty"`
	ProxmoxTokens map[string]string `yaml:"proxmox_tokens,omitempty"`
}

type Store struct {
	DataDir     string
	ConfigPath  string
	SecretsPath string
}

func New(dataDir string, configPath string) Store {
	if dataDir == "" {
		dataDir = "/data"
	}
	if configPath == "" {
		configPath = filepath.Join(dataDir, DefaultConfigFile)
	}
	return Store{
		DataDir:     dataDir,
		ConfigPath:  configPath,
		SecretsPath: filepath.Join(dataDir, DefaultSecretsFile),
	}
}

func (s Store) Load() (config.Config, Secrets, error) {
	cfg := config.NewDefault()
	if fileExists(s.ConfigPath) {
		loaded, err := config.Load(s.ConfigPath)
		if err != nil {
			return config.Config{}, Secrets{}, err
		}
		cfg = loaded
	}

	secrets, err := s.LoadSecrets()
	if err != nil {
		return config.Config{}, Secrets{}, err
	}
	ApplySecrets(&cfg, secrets)
	return cfg, secrets, nil
}

func (s Store) LoadSecrets() (Secrets, error) {
	secrets := Secrets{ProxmoxTokens: map[string]string{}}
	if !fileExists(s.SecretsPath) {
		return secrets, nil
	}
	data, err := os.ReadFile(s.SecretsPath)
	if err != nil {
		return Secrets{}, err
	}
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return Secrets{}, err
	}
	if secrets.ProxmoxTokens == nil {
		secrets.ProxmoxTokens = map[string]string{}
	}
	return secrets, nil
}

func (s Store) Save(cfg config.Config, secrets Secrets) error {
	cfg.ApplyDefaults()
	cfg.Server.DisplayTokenValue = ""
	cfg.Server.AdminTokenValue = ""
	for i := range cfg.Proxmox {
		cfg.Proxmox[i].TokenValue = ""
	}
	if secrets.ProxmoxTokens == nil {
		secrets.ProxmoxTokens = map[string]string{}
	}

	if err := os.MkdirAll(s.DataDir, 0o750); err != nil {
		return err
	}
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.ConfigPath, configData, 0o640); err != nil {
		return err
	}
	secretData, err := yaml.Marshal(secrets)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.SecretsPath, secretData, 0o600)
}

func ApplySecrets(cfg *config.Config, secrets Secrets) {
	cfg.Server.DisplayTokenValue = strings.TrimSpace(secrets.DisplayToken)
	cfg.Server.AdminTokenValue = strings.TrimSpace(secrets.AdminToken)
	for i := range cfg.Proxmox {
		if token := strings.TrimSpace(secrets.ProxmoxTokens[cfg.Proxmox[i].ID]); token != "" {
			cfg.Proxmox[i].TokenValue = token
		}
	}
}

func EnsureTokens(secrets *Secrets) error {
	if secrets.ProxmoxTokens == nil {
		secrets.ProxmoxTokens = map[string]string{}
	}
	var err error
	if strings.TrimSpace(secrets.DisplayToken) == "" {
		secrets.DisplayToken, err = randomToken()
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(secrets.AdminToken) == "" {
		secrets.AdminToken, err = randomToken()
		if err != nil {
			return err
		}
	}
	return nil
}

func SanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-", "\\", "-")
	value = replacer.Replace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	value = strings.Trim(b.String(), "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

func SortedProxmoxIDs(cfg config.Config) []string {
	ids := make([]string, 0, len(cfg.Proxmox))
	for _, pve := range cfg.Proxmox {
		ids = append(ids, pve.ID)
	}
	sort.Strings(ids)
	return ids
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return errors.New("empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func randomToken() (string, error) {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
