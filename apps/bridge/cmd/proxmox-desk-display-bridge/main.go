package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/configstore"
	appruntime "github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/runtime"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/server"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/store"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/version"
)

func main() {
	var configPath string
	var dataDir string
	var mock bool
	var showVersion bool

	flag.StringVar(&configPath, "config", "", "path to config.yaml; defaults to /data/config.yaml or ./config.yaml when present")
	flag.StringVar(&dataDir, "data-dir", "/data", "persistent data directory for config and secrets")
	flag.BoolVar(&mock, "mock", false, "serve deterministic mock data instead of reading Proxmox")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		}
	}

	cfgStore := configstore.New(dataDir, configPath)
	cfg, secrets, err := cfgStore.Load()
	if err != nil && !mock {
		slog.Error("failed to load config", "path", cfgStore.ConfigPath, "error", err)
		os.Exit(1)
	}

	if mock {
		cfg = config.MockConfig()
	}

	collector, err := appruntime.CollectorForConfig(cfg, mock)
	if err != nil {
		slog.Warn("config is not ready; starting setup collector", "error", err)
		collector = store.NewEmptyCollector()
	}

	cache := store.NewCache(collector, cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	admin := appruntime.NewManager(cfg, secrets, cfgStore, cache, mock)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache.Start(ctx)
	if err := cache.Refresh(ctx); err != nil {
		slog.Warn("initial refresh failed", "error", err)
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           server.New(cfg, cache, mock, admin),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("bridge listening", "addr", srv.Addr, "mock", mock, "version", version.Version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}
}
