package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/configstore"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
	appruntime "github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/runtime"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/store"
)

func TestDisplayStateRequiresToken(t *testing.T) {
	t.Setenv("DISPLAY_TOKEN", "secret")
	cfg := config.MockConfig()
	cache := store.NewCache(store.NewMockCollector(), cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg, cache, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/display-state", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/display-state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestFullStateKeepsInventoryWhileDisplayStateIsCompact(t *testing.T) {
	t.Setenv("DISPLAY_TOKEN", "secret")
	cfg := config.MockConfig()
	cache := store.NewCache(inventoryCollector{}, cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg, cache, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/display-state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("display-state status = %d", res.Code)
	}
	var compact display.State
	if err := json.Unmarshal(res.Body.Bytes(), &compact); err != nil {
		t.Fatal(err)
	}
	if len(compact.MetricTrends) != 0 || len(compact.StorageItems) != 0 {
		t.Fatalf("display-state was not compact: %#v", compact)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/full-state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("full-state status = %d", res.Code)
	}
	var full display.State
	if err := json.Unmarshal(res.Body.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	if len(full.MetricTrends) != 1 || len(full.StorageItems) != 1 {
		t.Fatalf("full-state missing inventory: %#v", full)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/detail-state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("detail-state status = %d", res.Code)
	}
	var detail display.DetailState
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.MetricTrends) != 1 || len(detail.StorageItems) != 1 {
		t.Fatalf("detail-state missing bounded inventory: %#v", detail)
	}
}

func TestAdminRequiresAdminTokenWhenConfigured(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Server.DisplayTokenValue = "display"
	cache := store.NewCache(store.NewEmptyCollector(), cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfgStore := configstore.New(t.TempDir(), "")
	manager := appruntime.NewManager(cfg, configstore.Secrets{DisplayToken: "display"}, cfgStore, cache, false)
	handler := New(cfg, cache, false, manager)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want unauthorized", res.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "display")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want OK", res.Code)
	}
}

func TestAdminRejectsCrossOriginPost(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Server.DisplayTokenValue = "display"
	cache := store.NewCache(store.NewEmptyCollector(), cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfgStore := configstore.New(t.TempDir(), "")
	manager := appruntime.NewManager(cfg, configstore.Secrets{DisplayToken: "display"}, cfgStore, cache, false)
	handler := New(cfg, cache, false, manager)

	req := httptest.NewRequest(http.MethodPost, "/admin/refresh", nil)
	req.Host = "bridge.local"
	req.Header.Set("Origin", "http://evil.local")
	req.SetBasicAuth("admin", "display")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want forbidden", res.Code)
	}
}

type inventoryCollector struct{}

func (inventoryCollector) Collect(context.Context) (display.State, error) {
	state := display.NewState()
	state.MetricTrends = []display.MetricTrend{{ID: "trend"}}
	state.StorageItems = []display.StorageItem{{ID: "item"}}
	state.Capabilities = []display.Capability{{ID: "cap", Status: "forbidden"}}
	return state, nil
}
