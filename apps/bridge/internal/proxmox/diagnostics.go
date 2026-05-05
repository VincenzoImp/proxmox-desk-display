package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
)

type TestResult struct {
	Version     string
	Release     string
	Fingerprint string
}

func TestHost(ctx context.Context, host config.ProxmoxHost) (TestResult, error) {
	result := TestResult{}
	if host.TLS.Mode == "fingerprint" && strings.TrimSpace(host.TLS.Fingerprint) == "" {
		fingerprint, err := DetectFingerprint(ctx, host.BaseURL)
		if err != nil {
			return result, err
		}
		host.TLS.Fingerprint = fingerprint
		result.Fingerprint = fingerprint
	}
	client, err := NewClient(host)
	if err != nil {
		return result, err
	}
	var version struct {
		Version string `json:"version"`
		Release string `json:"release"`
	}
	if err := client.Get(ctx, "/api2/json/version", &version); err != nil {
		return result, err
	}
	result.Version = version.Version
	result.Release = version.Release
	if result.Fingerprint == "" && host.TLS.Mode == "fingerprint" {
		result.Fingerprint = host.TLS.Fingerprint
	}
	return result, nil
}

func DetectFingerprint(ctx context.Context, baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("fingerprint detection requires an https URL")
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName(parsed.Hostname()),
	})
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("server did not present a certificate")
	}
	sum := sha256.Sum256(certs[0].Raw)
	return "SHA256:" + strings.ToLower(hex.EncodeToString(sum[:])), nil
}

func serverName(host string) string {
	if net.ParseIP(host) != nil {
		return ""
	}
	return host
}
