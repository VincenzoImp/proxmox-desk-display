package store

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
)

type Collector interface {
	Collect(context.Context) (display.State, error)
}

type Cache struct {
	collector    Collector
	pollInterval time.Duration
	staleAfter   time.Duration

	mu       sync.RWMutex
	state    display.State
	lastOK   time.Time
	lastErr  error
	hasState bool
}

func NewCache(collector Collector, pollInterval time.Duration, staleAfter time.Duration) *Cache {
	return &Cache{
		collector:    collector,
		pollInterval: pollInterval,
		staleAfter:   staleAfter,
	}
}

func (c *Cache) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Refresh(ctx); err != nil {
					slog.Warn("refresh failed", "error", err)
				}
			}
		}
	}()
}

func (c *Cache) Refresh(ctx context.Context) error {
	state, err := c.collector.Collect(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.lastErr = err
		if !c.hasState {
			return err
		}
		return err
	}
	c.state = display.Finalize(state)
	c.lastOK = time.Now().UTC()
	c.lastErr = nil
	c.hasState = true
	return nil
}

func (c *Cache) State() (display.State, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.hasState {
		if c.lastErr != nil {
			return display.State{}, c.lastErr
		}
		return display.State{}, errors.New("no state collected yet")
	}
	state := c.state
	if time.Since(c.lastOK) > c.staleAfter {
		state.Stale = true
	}
	return state, nil
}

func (c *Cache) Metadata() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	meta := map[string]any{
		"has_state":     c.hasState,
		"last_ok":       c.lastOK,
		"poll_interval": c.pollInterval.String(),
		"stale_after":   c.staleAfter.String(),
	}
	if c.lastErr != nil {
		meta["last_error"] = c.lastErr.Error()
	}
	return meta
}

type MockCollector struct{}

func NewMockCollector() MockCollector {
	return MockCollector{}
}

func (MockCollector) Collect(context.Context) (display.State, error) {
	state := display.NewState()
	state.Hosts = []display.Host{
		{
			ID:            "mock/lab-a",
			Name:          "Lab A",
			SourceID:      "mock",
			Node:          "lab-a",
			Online:        true,
			CPUPct:        18,
			MemoryPct:     62,
			StoragePct:    71,
			UptimeSec:     123456,
			GuestsRunning: 7,
			GuestsStopped: 1,
			Health:        display.HealthOK,
		},
		{
			ID:            "mock/lab-b",
			Name:          "Lab B",
			SourceID:      "mock",
			Node:          "lab-b",
			Online:        true,
			CPUPct:        31,
			MemoryPct:     48,
			StoragePct:    55,
			UptimeSec:     98765,
			GuestsRunning: 5,
			GuestsStopped: 2,
			Health:        display.HealthOK,
		},
	}
	state.Guests = []display.Guest{
		{
			ID:               "mock/100",
			VMID:             "100",
			Name:             "Home Assistant",
			Type:             "lxc",
			HostID:           "mock/lab-a",
			HostName:         "Lab A",
			SourceID:         "mock",
			Status:           "running",
			CPUPct:           3,
			MaxCPU:           2,
			MemoryPct:        34,
			MemoryUsedBytes:  730144440,
			MemoryTotalBytes: 2147483648,
			DiskPct:          42,
			DiskUsedBytes:    9019431321,
			DiskTotalBytes:   21474836480,
			UptimeSec:        456789,
			Pinned:           true,
			Expected:         "running",
			Health:           display.HealthOK,
		},
		{
			ID:               "mock/101",
			VMID:             "101",
			Name:             "Docker",
			Type:             "qemu",
			HostID:           "mock/lab-b",
			HostName:         "Lab B",
			SourceID:         "mock",
			Status:           "running",
			CPUPct:           14,
			MaxCPU:           4,
			MemoryPct:        58,
			MemoryUsedBytes:  4982162063,
			MemoryTotalBytes: 8589934592,
			DiskPct:          63,
			DiskUsedBytes:    67645734912,
			DiskTotalBytes:   107374182400,
			UptimeSec:        98765,
			Pinned:           true,
			Expected:         "running",
			Health:           display.HealthOK,
		},
	}
	return display.Finalize(state), nil
}
