package store

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/display"
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
		for {
			c.mu.RLock()
			interval := c.pollInterval
			c.mu.RUnlock()
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if err := c.Refresh(ctx); err != nil {
					slog.Warn("refresh failed", "error", err)
				}
			}
		}
	}()
}

func (c *Cache) Refresh(ctx context.Context) error {
	c.mu.RLock()
	collector := c.collector
	c.mu.RUnlock()

	state, err := collector.Collect(ctx)
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

func (c *Cache) Reconfigure(collector Collector, pollInterval time.Duration, staleAfter time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collector = collector
	c.pollInterval = pollInterval
	c.staleAfter = staleAfter
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

type EmptyCollector struct{}

func NewEmptyCollector() EmptyCollector {
	return EmptyCollector{}
}

func (EmptyCollector) Collect(context.Context) (display.State, error) {
	state := display.NewState()
	state.Alerts = []display.Alert{
		{
			ID:       "setup-required",
			Severity: display.HealthWarning,
			Title:    "Bridge setup required",
			Message:  "Open the bridge admin page and add at least one Proxmox source.",
		},
	}
	return display.Finalize(state), nil
}

func (MockCollector) Collect(context.Context) (display.State, error) {
	state := display.NewState()
	now := time.Now().Unix()
	state.Hosts = []display.Host{
		{
			ID:                "mock/lab-a",
			Name:              "Lab A",
			SourceID:          "mock",
			Node:              "lab-a",
			Online:            true,
			CPUPct:            18,
			MaxCPU:            4,
			CPUModel:          "Mock CPU",
			GPUCount:          1,
			GPUSummary:        "Intel UHD Graphics",
			MemoryPct:         62,
			MemoryUsedBytes:   5325759447,
			MemoryTotalBytes:  8589934592,
			StoragePct:        71,
			StorageUsedBytes:  76235669504,
			StorageTotalBytes: 107374182400,
			StorageMaxPct:     71,
			StorageMaxName:    "local",
			UptimeSec:         123456,
			LoadAvg:           []string{"0.24", "0.30", "0.28"},
			PVEVersion:        "pve-manager/mock",
			KernelVersion:     "6.17.0-pve",
			PrimaryAddress:    "192.168.1.10",
			NetworkActive:     2,
			NetworkTotal:      3,
			ServicesRunning:   8,
			ServicesTotal:     8,
			DiskCount:         1,
			LastBackupStatus:  "OK",
			LastBackupAgeSec:  7200,
			GuestsRunning:     7,
			GuestsStopped:     1,
			Health:            display.HealthOK,
		},
		{
			ID:                "mock/lab-b",
			Name:              "Lab B",
			SourceID:          "mock",
			Node:              "lab-b",
			Online:            true,
			CPUPct:            31,
			MaxCPU:            16,
			CPUModel:          "Mock Ryzen",
			GPUCount:          2,
			GPUSummary:        "NVIDIA RTX A2000 +1",
			MemoryPct:         48,
			MemoryUsedBytes:   8246337208,
			MemoryTotalBytes:  17179869184,
			StoragePct:        55,
			StorageUsedBytes:  590558003200,
			StorageTotalBytes: 1073741824000,
			StorageMaxPct:     82,
			StorageMaxName:    "datapool",
			UptimeSec:         98765,
			LoadAvg:           []string{"1.10", "1.05", "0.98"},
			PVEVersion:        "pve-manager/mock",
			KernelVersion:     "6.17.0-pve",
			PrimaryAddress:    "192.168.1.11",
			NetworkActive:     3,
			NetworkTotal:      4,
			ServicesRunning:   7,
			ServicesFailed:    1,
			ServicesTotal:     8,
			DiskCount:         2,
			DiskIssues:        1,
			FailedTasks24h:    1,
			LastBackupStatus:  "ERROR",
			LastBackupAgeSec:  3600,
			GuestsRunning:     5,
			GuestsStopped:     2,
			Health:            display.HealthWarning,
		},
	}
	state.Clusters = []display.Cluster{
		{
			ID:          "mock",
			SourceID:    "mock",
			Name:        "Mock Cluster",
			Quorate:     true,
			NodesOnline: 2,
			NodesTotal:  2,
			Health:      display.HealthOK,
		},
	}
	state.Storages = []display.Storage{
		{
			ID:             "mock/lab-a/local",
			Name:           "local",
			SourceID:       "mock",
			HostID:         "mock/lab-a",
			HostName:       "Lab A",
			Node:           "lab-a",
			Status:         "available",
			PluginType:     "dir",
			Content:        "iso,backup,rootdir",
			DiskPct:        71,
			DiskUsedBytes:  76235669504,
			DiskTotalBytes: 107374182400,
			Health:         display.HealthOK,
		},
		{
			ID:             "mock/lab-b/datapool",
			Name:           "datapool",
			SourceID:       "mock",
			HostID:         "mock/lab-b",
			HostName:       "Lab B",
			Node:           "lab-b",
			Status:         "available",
			PluginType:     "zfspool",
			Content:        "images",
			DiskPct:        82,
			DiskUsedBytes:  880468295680,
			DiskTotalBytes: 1073741824000,
			ContentItems:   1,
			ImageCount:     1,
			Health:         display.HealthWarning,
		},
	}
	state.Networks = []display.Network{
		{
			ID:        "mock/lab-a/vmbr0",
			SourceID:  "mock",
			HostID:    "mock/lab-a",
			HostName:  "Lab A",
			Node:      "lab-a",
			Iface:     "vmbr0",
			Type:      "bridge",
			Active:    true,
			Autostart: true,
			Address:   "192.168.1.10",
			Health:    display.HealthOK,
		},
	}
	state.Services = []display.Service{
		{
			ID:        "mock/lab-b/pvedaemon",
			SourceID:  "mock",
			HostID:    "mock/lab-b",
			HostName:  "Lab B",
			Node:      "lab-b",
			Name:      "pvedaemon",
			State:     "running",
			UnitState: "active",
			Health:    display.HealthOK,
		},
	}
	state.ZFSPools = []display.ZFSPool{
		{
			ID:               "mock/lab-b/datapool",
			SourceID:         "mock",
			HostID:           "mock/lab-b",
			HostName:         "Lab B",
			Node:             "lab-b",
			Name:             "datapool",
			HealthText:       "ONLINE",
			Status:           "ONLINE",
			State:            "ONLINE",
			Scan:             "scrub repaired 0B",
			Errors:           "No known data errors",
			DeviceCount:      4,
			SizeBytes:        1073741824000,
			AllocatedBytes:   880468295680,
			FreeBytes:        193273528320,
			FragmentationPct: 12,
			Health:           display.HealthOK,
		},
	}
	state.Disks = []display.Disk{
		{
			ID:          "mock/lab-a/sda",
			SourceID:    "mock",
			HostID:      "mock/lab-a",
			HostName:    "Lab A",
			Node:        "lab-a",
			Name:        "sda",
			Model:       "Samsung SSD",
			Type:        "ssd",
			UsedBy:      "rootfs",
			SizeBytes:   512110190592,
			SMARTHealth: "PASSED",
			WearoutPct:  4,
			Health:      display.HealthOK,
		},
		{
			ID:          "mock/lab-b/nvme0n1",
			SourceID:    "mock",
			HostID:      "mock/lab-b",
			HostName:    "Lab B",
			Node:        "lab-b",
			Name:        "nvme0n1",
			Model:       "Mock NVMe",
			Type:        "nvme",
			UsedBy:      "datapool",
			SizeBytes:   2000398934016,
			SMARTHealth: "PASSED",
			WearoutPct:  12,
			Health:      display.HealthOK,
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
			OSType:           "debian",
			IPAddress:        "dhcp",
			OnBoot:           true,
			Unprivileged:     true,
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
			OSType:           "l26",
			IPAddress:        "192.168.1.50/24",
			IPAddresses:      []string{"192.168.1.50/24"},
			AgentEnabled:     true,
			AgentAvailable:   true,
			AgentHostname:    "docker",
			AgentOS:          "Debian GNU/Linux",
			OnBoot:           true,
			Protection:       true,
			Disks: []display.GuestDisk{
				{Name: "scsi0", Storage: "datapool", Volume: "datapool:vm-101-disk-0", Size: "100G", Backup: true},
			},
			NICs: []display.GuestNIC{
				{Name: "net0", Model: "virtio", MAC: "AA:BB:CC:DD:EE:FF", Bridge: "vmbr0", Firewall: true},
			},
			Pinned:   true,
			Expected: "running",
			Health:   display.HealthOK,
		},
	}
	state.Snapshots = []display.Snapshot{
		{
			ID:        "mock/101/before-upgrade",
			SourceID:  "mock",
			HostID:    "mock/lab-b",
			HostName:  "Lab B",
			GuestID:   "mock/101",
			GuestName: "Docker",
			VMID:      "101",
			Type:      "qemu",
			Name:      "before-upgrade",
			SnapTime:  now - 86400,
			Health:    display.HealthOK,
		},
	}
	state.Tasks = []display.Task{
		{
			ID:            "mock/lab-b/vzdump",
			SourceID:      "mock",
			HostID:        "mock/lab-b",
			HostName:      "Lab B",
			Node:          "lab-b",
			Type:          "vzdump",
			User:          "root@pam",
			Status:        "ERROR",
			Target:        "101 Docker",
			VMID:          "101",
			GuestName:     "Docker",
			StartedAt:     now - 3600,
			StartedAgeSec: 3600,
			EndedAt:       now - 3500,
			DurationSec:   100,
			Health:        display.HealthWarning,
		},
		{
			ID:            "mock/lab-a/qmstart",
			SourceID:      "mock",
			HostID:        "mock/lab-a",
			HostName:      "Lab A",
			Node:          "lab-a",
			Type:          "qmstart",
			User:          "root@pam",
			Status:        "OK",
			Target:        "100 Home Assistant",
			VMID:          "100",
			GuestName:     "Home Assistant",
			StartedAt:     now - 900,
			StartedAgeSec: 900,
			EndedAt:       now - 880,
			DurationSec:   20,
			Health:        display.HealthOK,
		},
	}
	state.BackupJobs = []display.BackupJob{
		{
			ID:       "mock/backup-nightly",
			SourceID: "mock",
			Storage:  "backup",
			Schedule: "02:00",
			Mode:     "snapshot",
			Enabled:  true,
			All:      true,
			Compress: "zstd",
			Health:   display.HealthOK,
		},
	}
	state.Replications = []display.Replication{
		{
			ID:         "mock/101-0",
			SourceID:   "mock",
			GuestID:    "mock/101",
			GuestName:  "Docker",
			VMID:       "101",
			SourceNode: "lab-b",
			TargetNode: "lab-a",
			Schedule:   "*/15",
			Enabled:    true,
			LastSync:   now - 600,
			Health:     display.HealthOK,
		},
	}
	state.HAResources = []display.HAResource{
		{
			ID:       "mock/vm:101",
			SourceID: "mock",
			SID:      "vm:101",
			Type:     "vm",
			State:    "started",
			Node:     "lab-b",
			Health:   display.HealthOK,
		},
	}
	state.Certificates = []display.Certificate{
		{
			ID:            "mock/lab-a/pveproxy-ssl.pem",
			SourceID:      "mock",
			HostID:        "mock/lab-a",
			HostName:      "Lab A",
			Node:          "lab-a",
			Filename:      "pveproxy-ssl.pem",
			Subject:       "CN=lab-a.local",
			Issuer:        "CN=Mock Lab CA",
			DaysRemaining: 120,
			Health:        display.HealthOK,
		},
	}
	state.StorageItems = []display.StorageItem{
		{
			ID:        "mock/lab-b/datapool/vm-101-disk-0",
			SourceID:  "mock",
			HostID:    "mock/lab-b",
			HostName:  "Lab B",
			Node:      "lab-b",
			Storage:   "datapool",
			Content:   "images",
			VolID:     "datapool:vm-101-disk-0",
			VMID:      "101",
			Format:    "raw",
			SizeBytes: 107374182400,
			Health:    display.HealthOK,
		},
	}
	state.MetricTrends = []display.MetricTrend{
		{
			ID:           "mock/host/Lab A/cpu_pct",
			SourceID:     "mock",
			HostID:       "mock/lab-a",
			ResourceType: "host",
			ResourceName: "Lab A",
			Metric:       "cpu_pct",
			Unit:         "%",
			Timeframe:    "hour",
			Last:         18,
			Values:       []int{12, 14, 13, 15, 18, 16, 19, 18},
		},
		{
			ID:           "mock/qemu/Docker/memory_pct",
			SourceID:     "mock",
			HostID:       "mock/lab-b",
			GuestID:      "mock/101",
			ResourceType: "qemu",
			ResourceName: "Docker",
			Metric:       "memory_pct",
			Unit:         "%",
			Timeframe:    "hour",
			Last:         58,
			Values:       []int{52, 54, 57, 58, 59, 58, 57, 58},
		},
	}
	state.Capabilities = []display.Capability{
		{
			ID:         "mock/apt_updates/lab-a",
			SourceID:   "mock",
			HostID:     "mock/lab-a",
			Name:       "apt_updates",
			Endpoint:   "/api2/json/nodes/lab-a/apt/update",
			Status:     "forbidden",
			HTTPStatus: 403,
			Message:    "HTTP 403",
		},
		{
			ID:         "mock/node_rrd/lab-a",
			SourceID:   "mock",
			HostID:     "mock/lab-a",
			Name:       "node_rrd",
			Endpoint:   "/api2/json/nodes/lab-a/rrddata",
			Status:     "ok",
			HTTPStatus: 200,
		},
	}
	state.Updates = []display.Update{
		{
			ID:               "mock/lab-a/pve-manager",
			SourceID:         "mock",
			HostID:           "mock/lab-a",
			HostName:         "Lab A",
			Node:             "lab-a",
			Package:          "pve-manager",
			CurrentVersion:   "9.1.1",
			CandidateVersion: "9.1.2",
			Health:           display.HealthWarning,
		},
	}
	state.Repositories = []display.Repository{
		{
			ID:       "mock/lab-a/pve-enterprise",
			SourceID: "mock",
			HostID:   "mock/lab-a",
			HostName: "Lab A",
			Node:     "lab-a",
			File:     "/etc/apt/sources.list.d/pve-enterprise.list",
			Enabled:  false,
			Status:   "ok",
			Health:   display.HealthOK,
		},
	}
	state.Subscriptions = []display.Subscription{
		{
			ID:          "mock/lab-a",
			SourceID:    "mock",
			HostID:      "mock/lab-a",
			HostName:    "Lab A",
			Node:        "lab-a",
			Status:      "active",
			ProductName: "Proxmox VE",
			Health:      display.HealthOK,
		},
	}
	return display.Finalize(state), nil
}
