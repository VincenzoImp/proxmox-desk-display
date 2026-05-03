package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/store"
)

func TestDisplayStateRequiresToken(t *testing.T) {
	t.Setenv("DISPLAY_TOKEN", "secret")
	cfg := config.MockConfig()
	cache := store.NewCache(store.NewMockCollector(), cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := New(cfg, cache, false)

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
