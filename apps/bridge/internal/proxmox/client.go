package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
)

type Client struct {
	id      string
	name    string
	baseURL string
	token   string
	http    *http.Client
}

type APIError struct {
	SourceID   string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("proxmox %s returned HTTP %d: %s", e.SourceID, e.StatusCode, strings.TrimSpace(e.Body))
}

func NewClient(host config.ProxmoxHost) (*Client, error) {
	transport, err := transportForTLS(host.TLS)
	if err != nil {
		return nil, err
	}
	return &Client{
		id:      host.ID,
		name:    host.Name,
		baseURL: strings.TrimRight(host.BaseURL, "/"),
		token:   config.NormalizeToken(tokenForHost(host)),
		http: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}, nil
}

func tokenForHost(host config.ProxmoxHost) string {
	if host.TokenValue != "" {
		return host.TokenValue
	}
	return os.Getenv(host.TokenEnv)
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{SourceID: c.id, StatusCode: resp.StatusCode, Body: string(body)}
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("proxmox %s response missing data envelope", c.id)
	}
	return json.Unmarshal(envelope.Data, out)
}

func transportForTLS(cfg config.TLSConfig) (*http.Transport, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	switch cfg.Mode {
	case "system", "":
	case "insecure":
		tlsConfig.InsecureSkipVerify = true
	case "ca_file":
		data, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("failed to parse ca_file %q", cfg.CAFile)
		}
		tlsConfig.RootCAs = pool
	case "fingerprint":
		expected, err := parseFingerprint(cfg.Fingerprint)
		if err != nil {
			return nil, err
		}
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("server did not present a certificate")
			}
			sum := sha256.Sum256(rawCerts[0])
			actual := strings.ToLower(hex.EncodeToString(sum[:]))
			if actual != expected {
				return fmt.Errorf("certificate fingerprint mismatch: got SHA256:%s", actual)
			}
			return nil
		}
	default:
		return nil, fmt.Errorf("unsupported tls mode %q", cfg.Mode)
	}
	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}

func parseFingerprint(input string) (string, error) {
	value := strings.TrimSpace(input)
	value = strings.TrimPrefix(value, "SHA256:")
	value = strings.ReplaceAll(value, ":", "")
	value = strings.ToLower(value)
	if len(value) != 64 {
		return "", fmt.Errorf("SHA256 fingerprint must be 64 hex chars")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", err
	}
	return value, nil
}
