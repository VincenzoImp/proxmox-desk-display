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

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/proxmox"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/server"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/store"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/version"
)

func main() {
	var configPath string
	var mock bool
	var showVersion bool

	flag.StringVar(&configPath, "config", "config.yaml", "path to config.yaml")
	flag.BoolVar(&mock, "mock", false, "serve deterministic mock data instead of reading Proxmox")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil && !mock {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}
	if mock {
		cfg = config.MockConfig()
	}

	if err := cfg.Validate(mock); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	var collector store.Collector
	if mock {
		collector = store.NewMockCollector()
	} else {
		collector, err = proxmox.NewCollector(cfg)
		if err != nil {
			slog.Error("failed to create Proxmox collector", "error", err)
			os.Exit(1)
		}
	}

	cache := store.NewCache(collector, cfg.Server.PollInterval(), cfg.Server.StaleAfter())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache.Start(ctx)
	if err := cache.Refresh(ctx); err != nil {
		slog.Warn("initial refresh failed", "error", err)
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           server.New(cfg, cache, mock),
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
