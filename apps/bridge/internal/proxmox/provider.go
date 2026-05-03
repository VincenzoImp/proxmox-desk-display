package proxmox

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
)

type Collector struct {
	cfg     config.Config
	clients []*Client
	pinned  map[string]config.PinnedGuest
}

const maxDisplayTasks = 48

func NewCollector(cfg config.Config) (*Collector, error) {
	clients := make([]*Client, 0, len(cfg.Proxmox))
	for _, host := range cfg.Proxmox {
		client, err := NewClient(host)
		if err != nil {
			return nil, fmt.Errorf("create client %s: %w", host.ID, err)
		}
		clients = append(clients, client)
	}

	pinned := map[string]config.PinnedGuest{}
	for _, guest := range cfg.Guests.Pinned {
		pinned[guest.Source+"/"+guest.ID] = guest
	}

	return &Collector{cfg: cfg, clients: clients, pinned: pinned}, nil
}

func (c *Collector) Collect(ctx context.Context) (display.State, error) {
	state := display.NewState()
	type result struct {
		state display.State
		err   error
	}
	results := make(chan result, len(c.clients))
	var wg sync.WaitGroup
	for _, client := range c.clients {
		client := client
		wg.Add(1)
		go func() {
			defer wg.Done()
			sourceState, err := c.collectSource(ctx, client)
			results <- result{state: sourceState, err: err}
		}()
	}
	wg.Wait()
	close(results)

	for res := range results {
		state.Hosts = append(state.Hosts, res.state.Hosts...)
		state.Storages = append(state.Storages, res.state.Storages...)
		state.Disks = append(state.Disks, res.state.Disks...)
		state.Guests = append(state.Guests, res.state.Guests...)
		state.Tasks = append(state.Tasks, res.state.Tasks...)
		state.Alerts = append(state.Alerts, res.state.Alerts...)
	}

	sort.Slice(state.Hosts, func(i, j int) bool { return state.Hosts[i].ID < state.Hosts[j].ID })
	sort.Slice(state.Guests, func(i, j int) bool {
		if state.Guests[i].Pinned != state.Guests[j].Pinned {
			return state.Guests[i].Pinned
		}
		return state.Guests[i].ID < state.Guests[j].ID
	})
	sort.Slice(state.Storages, func(i, j int) bool { return state.Storages[i].ID < state.Storages[j].ID })
	sort.Slice(state.Disks, func(i, j int) bool { return state.Disks[i].ID < state.Disks[j].ID })
	sort.Slice(state.Tasks, func(i, j int) bool {
		if state.Tasks[i].StartedAt != state.Tasks[j].StartedAt {
			return state.Tasks[i].StartedAt > state.Tasks[j].StartedAt
		}
		return state.Tasks[i].ID < state.Tasks[j].ID
	})
	if len(state.Tasks) > maxDisplayTasks {
		state.Tasks = state.Tasks[:maxDisplayTasks]
	}
	sort.Slice(state.Alerts, func(i, j int) bool {
		return severityRank(state.Alerts[i].Severity) > severityRank(state.Alerts[j].Severity)
	})

	return display.Finalize(state), nil
}

func (c *Collector) collectSource(ctx context.Context, client *Client) (display.State, error) {
	state := display.NewState()

	var resources []resource
	if err := client.Get(ctx, "/api2/json/cluster/resources", &resources); err != nil {
		msg := err.Error()
		hostID := client.id + "/unreachable"
		state.Hosts = append(state.Hosts, display.Host{
			ID:       hostID,
			Name:     client.name,
			SourceID: client.id,
			Node:     client.name,
			Online:   false,
			Health:   display.HealthCritical,
			Error:    &msg,
		})
		state.Alerts = append(state.Alerts, display.Alert{
			ID:       client.id + "/source-offline",
			SourceID: client.id,
			HostID:   hostID,
			Severity: display.HealthCritical,
			Title:    client.name + " offline",
			Message:  msg,
		})
		return state, nil
	}

	hostByNode := map[string]*display.Host{}
	for _, r := range resources {
		if r.Type != "node" {
			continue
		}
		nodeName := firstNonEmpty(r.Node, r.Name)
		if nodeName == "" {
			nodeName = client.name
		}
		hostID := client.id + "/" + nodeName
		host := display.Host{
			ID:                hostID,
			Name:              displayName(client.name, nodeName),
			SourceID:          client.id,
			Node:              nodeName,
			Online:            r.Status == "online" || r.Status == "",
			CPUPct:            pctFloat(r.CPU),
			MaxCPU:            r.MaxCPU,
			MemoryPct:         pctInt64(r.Mem, r.MaxMem),
			MemoryUsedBytes:   r.Mem,
			MemoryTotalBytes:  r.MaxMem,
			StoragePct:        pctInt64(r.Disk, r.MaxDisk),
			StorageUsedBytes:  r.Disk,
			StorageTotalBytes: r.MaxDisk,
			StorageMaxPct:     pctInt64(r.Disk, r.MaxDisk),
			UptimeSec:         r.Uptime,
			Health:            display.HealthOK,
		}
		if !host.Online {
			host.Health = display.HealthCritical
		}
		hostByNode[nodeName] = &host
	}

	for node, host := range hostByNode {
		status, err := c.nodeStatus(ctx, client, node)
		if err == nil {
			if status.CPU != nil {
				host.CPUPct = pctFloat(*status.CPU)
			}
			if status.Memory.Total > 0 {
				host.MemoryPct = pctInt64(status.Memory.Used, status.Memory.Total)
				host.MemoryUsedBytes = status.Memory.Used
				host.MemoryTotalBytes = status.Memory.Total
			}
			if status.RootFS.Total > 0 {
				host.StoragePct = pctInt64(status.RootFS.Used, status.RootFS.Total)
				host.StorageUsedBytes = status.RootFS.Used
				host.StorageTotalBytes = status.RootFS.Total
			}
			if status.Uptime > 0 {
				host.UptimeSec = status.Uptime
			}
			if status.CPUInfo.CPUs > 0 {
				host.MaxCPU = status.CPUInfo.CPUs
			}
			host.CPUModel = status.CPUInfo.Model
			host.LoadAvg = status.LoadAvg
			host.PVEVersion = status.PVEVersion
			host.KernelVersion = firstNonEmpty(status.KVersion, status.CurrentKernel.Release)
		}
		if devices, err := c.nodePCI(ctx, client, node); err == nil {
			host.GPUCount, host.GPUSummary = summarizeGPUs(devices)
		} else {
			addDataWarning(host, "pci unavailable")
		}
		if networks, err := c.nodeNetwork(ctx, client, node); err == nil {
			host.NetworkTotal, host.NetworkActive, host.PrimaryAddress = summarizeNetwork(networks)
		} else {
			addDataWarning(host, "network unavailable")
		}
		if services, err := c.nodeServices(ctx, client, node); err == nil {
			host.ServicesTotal, host.ServicesRunning, host.ServicesFailed = summarizeServices(services)
			if host.ServicesFailed > 0 {
				host.Health = maxHealth(host.Health, display.HealthWarning)
				state.Alerts = append(state.Alerts, display.Alert{
					ID:       host.ID + "/services-failed",
					SourceID: host.SourceID,
					HostID:   host.ID,
					Severity: display.HealthWarning,
					Title:    host.Name + " services",
					Message:  fmt.Sprintf("%d failed services", host.ServicesFailed),
				})
			}
		} else {
			addDataWarning(host, "services unavailable")
		}
		if disks, err := c.nodeDisks(ctx, client, node); err == nil {
			for _, disk := range disks {
				displayDisk := display.Disk{
					ID:          client.id + "/" + node + "/" + diskName(disk),
					SourceID:    client.id,
					HostID:      host.ID,
					HostName:    host.Name,
					Node:        node,
					Name:        diskName(disk),
					Model:       firstNonEmpty(disk.Model, disk.Vendor),
					Serial:      disk.Serial,
					Type:        disk.Type,
					UsedBy:      stringify(disk.Used),
					SizeBytes:   disk.Size,
					SMARTHealth: firstNonEmpty(disk.Health, disk.SMARTHealth),
					WearoutPct:  disk.Wearout,
					Health:      diskHealth(firstNonEmpty(disk.Health, disk.SMARTHealth), disk.Wearout),
				}
				host.DiskCount++
				if displayDisk.Health == display.HealthCritical || displayDisk.Health == display.HealthWarning {
					host.DiskIssues++
					host.Health = maxHealth(host.Health, displayDisk.Health)
				}
				state.Disks = append(state.Disks, displayDisk)
			}
		} else {
			addDataWarning(host, "disks unavailable")
		}
	}

	for _, r := range resources {
		if r.Type != "storage" {
			continue
		}
		nodeName := firstNonEmpty(r.Node, "pve")
		hostID := client.id + "/" + nodeName
		storageName := firstNonEmpty(r.Storage, strings.TrimPrefix(r.ID, "storage/"+nodeName+"/"))
		storage := display.Storage{
			ID:             client.id + "/" + nodeName + "/" + storageName,
			Name:           storageName,
			SourceID:       client.id,
			HostID:         hostID,
			HostName:       displayName(client.name, nodeName),
			Node:           nodeName,
			Status:         firstNonEmpty(r.Status, "unknown"),
			PluginType:     r.PluginType,
			Content:        r.Content,
			Shared:         r.Shared != 0,
			DiskPct:        pctInt64(r.Disk, r.MaxDisk),
			DiskUsedBytes:  r.Disk,
			DiskTotalBytes: r.MaxDisk,
			Health:         display.HealthOK,
		}
		c.applyStorageAlerts(&state, &storage)
		if host, ok := hostByNode[nodeName]; ok && storage.DiskPct >= host.StorageMaxPct {
			host.StorageMaxPct = storage.DiskPct
			host.StorageMaxName = storage.Name
		}
		state.Storages = append(state.Storages, storage)
	}

	guestBySourceVMID := map[string]display.Guest{}
	for _, r := range resources {
		if r.Type != "qemu" && r.Type != "lxc" {
			continue
		}
		vmid := strconv.Itoa(r.VMID)
		if vmid == "0" && r.ID != "" {
			vmid = strings.TrimPrefix(r.ID, r.Type+"/")
		}
		hostID := client.id + "/" + r.Node
		guestID := client.id + "/" + vmid
		pin, pinned := c.pinned[client.id+"/"+vmid]
		name := firstNonEmpty(pin.Label, r.Name, vmid)
		guest := display.Guest{
			ID:               guestID,
			VMID:             vmid,
			Name:             name,
			Type:             r.Type,
			HostID:           hostID,
			HostName:         displayName(client.name, r.Node),
			SourceID:         client.id,
			Status:           firstNonEmpty(r.Status, "unknown"),
			CPUPct:           pctFloat(r.CPU),
			MaxCPU:           r.MaxCPU,
			MemoryPct:        pctInt64(r.Mem, r.MaxMem),
			MemoryUsedBytes:  r.Mem,
			MemoryTotalBytes: r.MaxMem,
			DiskPct:          pctInt64(r.Disk, r.MaxDisk),
			DiskUsedBytes:    r.Disk,
			DiskTotalBytes:   r.MaxDisk,
			UptimeSec:        r.Uptime,
			NetInBytes:       r.NetIn,
			NetOutBytes:      r.NetOut,
			DiskReadBytes:    r.DiskRead,
			DiskWriteBytes:   r.DiskWrite,
			Tags:             r.Tags,
			Pinned:           pinned,
			Expected:         pin.Expected,
			Health:           display.HealthOK,
		}
		if cfg, err := c.guestConfig(ctx, client, r.Node, r.Type, vmid); err == nil {
			applyGuestConfig(&guest, cfg)
		} else {
			guest.ConfigWarning = "config unavailable"
		}
		if pinned && pin.Expected != "" && guest.Status != pin.Expected {
			guest.Health = display.HealthWarning
			state.Alerts = append(state.Alerts, display.Alert{
				ID:       guestID + "/unexpected-status",
				SourceID: client.id,
				HostID:   hostID,
				GuestID:  guestID,
				Severity: display.HealthWarning,
				Title:    name + " is " + guest.Status,
				Message:  "expected " + pin.Expected,
			})
		}
		state.Guests = append(state.Guests, guest)
		guestBySourceVMID[client.id+"/"+vmid] = guest
		if host, ok := hostByNode[r.Node]; ok {
			if guest.Status == "running" {
				host.GuestsRunning++
			} else {
				host.GuestsStopped++
			}
		}
	}

	now := time.Now().Unix()
	lastBackupStarted := map[string]int64{}
	for node, host := range hostByNode {
		tasks, err := c.nodeTasks(ctx, client, node)
		if err != nil {
			addDataWarning(host, "tasks unavailable")
			continue
		}
		for _, task := range tasks {
			displayTask := taskDisplay(client, host, task, guestBySourceVMID, now)
			if displayTask.Type == "vzdump" && displayTask.StartedAt >= lastBackupStarted[host.ID] {
				host.LastBackupStatus = displayTask.Status
				if displayTask.StartedAt > 0 {
					host.LastBackupAgeSec = now - displayTask.StartedAt
					lastBackupStarted[host.ID] = displayTask.StartedAt
				}
			}
			if displayTask.Health == display.HealthWarning && displayTask.EndedAt > 0 && now-displayTask.EndedAt <= 86400 {
				host.FailedTasks24h++
			}
			state.Tasks = append(state.Tasks, displayTask)
		}
		if host.FailedTasks24h > 0 {
			host.Health = maxHealth(host.Health, display.HealthWarning)
			state.Alerts = append(state.Alerts, display.Alert{
				ID:       host.ID + "/tasks-failed-24h",
				SourceID: host.SourceID,
				HostID:   host.ID,
				Severity: display.HealthWarning,
				Title:    host.Name + " failed tasks",
				Message:  fmt.Sprintf("%d failed in 24h", host.FailedTasks24h),
			})
		}
	}

	for _, host := range hostByNode {
		c.applyHostAlerts(&state, host)
		state.Hosts = append(state.Hosts, *host)
	}

	return state, nil
}

func (c *Collector) nodeStatus(ctx context.Context, client *Client, node string) (nodeStatus, error) {
	var status nodeStatus
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/status", &status)
	return status, err
}

func (c *Collector) nodePCI(ctx context.Context, client *Client, node string) ([]pciDevice, error) {
	var devices []pciDevice
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/hardware/pci", &devices)
	return devices, err
}

func (c *Collector) nodeNetwork(ctx context.Context, client *Client, node string) ([]networkInterface, error) {
	var networks []networkInterface
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/network", &networks)
	return networks, err
}

func (c *Collector) nodeServices(ctx context.Context, client *Client, node string) ([]nodeService, error) {
	var services []nodeService
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/services", &services)
	return services, err
}

func (c *Collector) nodeDisks(ctx context.Context, client *Client, node string) ([]nodeDisk, error) {
	var disks []nodeDisk
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/disks/list", &disks)
	return disks, err
}

func (c *Collector) nodeTasks(ctx context.Context, client *Client, node string) ([]nodeTask, error) {
	var tasks []nodeTask
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/tasks?limit=25", &tasks)
	return tasks, err
}

func (c *Collector) guestConfig(ctx context.Context, client *Client, node string, guestType string, vmid string) (map[string]any, error) {
	var cfg map[string]any
	pathType := "qemu"
	if guestType == "lxc" {
		pathType = "lxc"
	}
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/"+pathType+"/"+url.PathEscape(vmid)+"/config", &cfg)
	return cfg, err
}

func (c *Collector) applyHostAlerts(state *display.State, host *display.Host) {
	if !host.Online {
		state.Alerts = append(state.Alerts, display.Alert{
			ID:       host.ID + "/offline",
			SourceID: host.SourceID,
			HostID:   host.ID,
			Severity: display.HealthCritical,
			Title:    host.Name + " offline",
			Message:  "Proxmox node is not online",
		})
		return
	}
	if host.MemoryPct >= c.cfg.Alerts.MemoryCriticalPct {
		host.Health = maxHealth(host.Health, display.HealthCritical)
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthCritical, "memory critical", fmt.Sprintf("memory is %d%%", host.MemoryPct)))
	} else if host.MemoryPct >= c.cfg.Alerts.MemoryWarningPct {
		host.Health = maxHealth(host.Health, display.HealthWarning)
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthWarning, "memory warning", fmt.Sprintf("memory is %d%%", host.MemoryPct)))
	}
	storagePct := host.StoragePct
	if host.StorageMaxPct > storagePct {
		storagePct = host.StorageMaxPct
	}
	if storagePct >= c.cfg.Alerts.StorageCriticalPct {
		host.Health = maxHealth(host.Health, display.HealthCritical)
		if host.StorageMaxName == "" {
			state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthCritical, "storage critical", fmt.Sprintf("storage is %d%%", storagePct)))
		}
	} else if storagePct >= c.cfg.Alerts.StorageWarningPct {
		host.Health = maxHealth(host.Health, display.HealthWarning)
		if host.StorageMaxName == "" {
			state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthWarning, "storage warning", fmt.Sprintf("storage is %d%%", storagePct)))
		}
	}
}

func (c *Collector) applyStorageAlerts(state *display.State, storage *display.Storage) {
	if storage.Status != "available" && storage.Status != "ok" && storage.Status != "" {
		storage.Health = display.HealthWarning
		state.Alerts = append(state.Alerts, display.Alert{
			ID:       storage.ID + "/status",
			SourceID: storage.SourceID,
			HostID:   storage.HostID,
			Severity: display.HealthWarning,
			Title:    storage.HostName + " storage " + storage.Name,
			Message:  "status is " + storage.Status,
		})
	}
	if storage.DiskPct >= c.cfg.Alerts.StorageCriticalPct {
		storage.Health = display.HealthCritical
		state.Alerts = append(state.Alerts, display.Alert{
			ID:       storage.ID + "/critical",
			SourceID: storage.SourceID,
			HostID:   storage.HostID,
			Severity: display.HealthCritical,
			Title:    storage.HostName + " " + storage.Name + " critical",
			Message:  fmt.Sprintf("storage is %d%%", storage.DiskPct),
		})
	} else if storage.DiskPct >= c.cfg.Alerts.StorageWarningPct && storage.Health != display.HealthCritical {
		storage.Health = display.HealthWarning
		state.Alerts = append(state.Alerts, display.Alert{
			ID:       storage.ID + "/warning",
			SourceID: storage.SourceID,
			HostID:   storage.HostID,
			Severity: display.HealthWarning,
			Title:    storage.HostName + " " + storage.Name + " warning",
			Message:  fmt.Sprintf("storage is %d%%", storage.DiskPct),
		})
	}
}

func alertForHost(host display.Host, severity display.Health, title string, message string) display.Alert {
	return display.Alert{
		ID:       host.ID + "/" + strings.ReplaceAll(title, " ", "-"),
		SourceID: host.SourceID,
		HostID:   host.ID,
		Severity: severity,
		Title:    host.Name + " " + title,
		Message:  message,
	}
}

type resource struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Node       string  `json:"node"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	VMID       int     `json:"vmid"`
	CPU        float64 `json:"cpu"`
	MaxCPU     int     `json:"maxcpu"`
	Mem        int64   `json:"mem"`
	MaxMem     int64   `json:"maxmem"`
	Disk       int64   `json:"disk"`
	MaxDisk    int64   `json:"maxdisk"`
	Uptime     int64   `json:"uptime"`
	NetIn      int64   `json:"netin"`
	NetOut     int64   `json:"netout"`
	DiskRead   int64   `json:"diskread"`
	DiskWrite  int64   `json:"diskwrite"`
	Tags       string  `json:"tags"`
	Storage    string  `json:"storage"`
	PluginType string  `json:"plugintype"`
	Content    string  `json:"content"`
	Shared     int     `json:"shared"`
}

type nodeStatus struct {
	CPU        *float64 `json:"cpu"`
	Uptime     int64    `json:"uptime"`
	LoadAvg    []string `json:"loadavg"`
	PVEVersion string   `json:"pveversion"`
	KVersion   string   `json:"kversion"`
	Memory     struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"memory"`
	RootFS struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"rootfs"`
	CPUInfo struct {
		Model string `json:"model"`
		CPUs  int    `json:"cpus"`
		Cores int    `json:"cores"`
	} `json:"cpuinfo"`
	CurrentKernel struct {
		Release string `json:"release"`
		Version string `json:"version"`
	} `json:"current-kernel"`
}

type pciDevice struct {
	ID                  string `json:"id"`
	Class               string `json:"class"`
	VendorName          string `json:"vendor_name"`
	DeviceName          string `json:"device_name"`
	SubsystemVendorName string `json:"subsystem_vendor_name"`
	SubsystemDeviceName string `json:"subsystem_device_name"`
}

type networkInterface struct {
	Iface     string `json:"iface"`
	Type      string `json:"type"`
	Active    int    `json:"active"`
	Autostart int    `json:"autostart"`
	Method    string `json:"method"`
	Address   string `json:"address"`
	CIDR      string `json:"cidr"`
	Gateway   string `json:"gateway"`
}

type nodeService struct {
	Name        string `json:"name"`
	Service     string `json:"service"`
	State       string `json:"state"`
	UnitState   string `json:"unit-state"`
	Description string `json:"desc"`
}

type nodeDisk struct {
	DevPath     string `json:"devpath"`
	Device      string `json:"device"`
	Model       string `json:"model"`
	Vendor      string `json:"vendor"`
	Serial      string `json:"serial"`
	Type        string `json:"type"`
	Used        any    `json:"used"`
	Size        int64  `json:"size"`
	Health      string `json:"health"`
	SMARTHealth string `json:"smart_health"`
	Wearout     int    `json:"wearout"`
}

type nodeTask struct {
	UPID      string `json:"upid"`
	Node      string `json:"node"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	User      string `json:"user"`
	Status    string `json:"status"`
	StartTime int64  `json:"starttime"`
	EndTime   int64  `json:"endtime"`
}

func summarizeGPUs(devices []pciDevice) (int, string) {
	names := []string{}
	for _, device := range devices {
		if !isGPUClass(device.Class) {
			continue
		}
		names = append(names, gpuName(device))
	}
	if len(names) == 0 {
		return 0, ""
	}
	summary := names[0]
	if len(names) > 1 {
		summary += fmt.Sprintf(" +%d", len(names)-1)
	}
	return len(names), summary
}

func isGPUClass(class string) bool {
	value := strings.ToLower(strings.TrimSpace(class))
	value = strings.TrimPrefix(value, "0x")
	return strings.HasPrefix(value, "03") ||
		strings.Contains(value, "vga") ||
		strings.Contains(value, "display") ||
		strings.Contains(value, "3d controller")
}

func gpuName(device pciDevice) string {
	vendor := strings.TrimSpace(firstNonEmpty(device.VendorName, device.SubsystemVendorName))
	name := strings.TrimSpace(firstNonEmpty(device.DeviceName, device.SubsystemDeviceName, device.ID))
	if vendor == "" {
		return name
	}
	if name == "" {
		return vendor
	}
	if strings.Contains(strings.ToLower(name), strings.ToLower(vendor)) {
		return name
	}
	return vendor + " " + name
}

func summarizeNetwork(networks []networkInterface) (int, int, string) {
	total := 0
	active := 0
	primary := ""
	for _, network := range networks {
		if network.Iface == "" || network.Iface == "lo" {
			continue
		}
		total++
		if network.Active != 0 {
			active++
		}
		address := firstNonEmpty(network.Address, network.CIDR)
		if primary == "" && network.Active != 0 && address != "" {
			primary = address
		}
	}
	return total, active, primary
}

func summarizeServices(services []nodeService) (int, int, int) {
	total := 0
	running := 0
	failed := 0
	for _, service := range services {
		name := firstNonEmpty(service.Name, service.Service)
		if name == "" {
			continue
		}
		total++
		state := strings.ToLower(firstNonEmpty(service.State, service.UnitState))
		if strings.Contains(state, "running") || strings.Contains(state, "active") {
			running++
		}
		if strings.Contains(state, "failed") {
			failed++
		}
	}
	return total, running, failed
}

func diskName(disk nodeDisk) string {
	name := firstNonEmpty(disk.DevPath, disk.Device)
	name = strings.TrimPrefix(name, "/dev/")
	if name == "" {
		return "disk"
	}
	return name
}

func diskHealth(smartHealth string, wearout int) display.Health {
	value := strings.ToLower(strings.TrimSpace(smartHealth))
	switch {
	case strings.Contains(value, "fail"), strings.Contains(value, "bad"), strings.Contains(value, "critical"):
		return display.HealthCritical
	case strings.Contains(value, "warn"):
		return display.HealthWarning
	case strings.Contains(value, "pass"), strings.Contains(value, "ok"), strings.Contains(value, "healthy"):
		return display.HealthOK
	case value == "" || value == "unknown":
		return display.HealthUnknown
	default:
		return display.HealthUnknown
	}
}

func taskDisplay(client *Client, host *display.Host, task nodeTask, guests map[string]display.Guest, now int64) display.Task {
	vmid := ""
	guestName := ""
	target := task.ID
	if _, err := strconv.Atoi(task.ID); err == nil && task.ID != "" {
		vmid = task.ID
		if guest, ok := guests[client.id+"/"+vmid]; ok {
			guestName = guest.Name
			target = guest.VMID + " " + guest.Name
		}
	}
	status := firstNonEmpty(task.Status, "running")
	health := display.HealthOK
	if task.Status != "" && strings.ToUpper(task.Status) != "OK" {
		health = display.HealthWarning
	}
	duration := int64(0)
	if task.EndTime > 0 && task.StartTime > 0 {
		duration = task.EndTime - task.StartTime
	}
	startedAge := int64(0)
	if task.StartTime > 0 {
		startedAge = maxInt64(0, now-task.StartTime)
	}
	id := firstNonEmpty(task.UPID, host.ID+"/"+task.Type+"/"+strconv.FormatInt(task.StartTime, 10))
	return display.Task{
		ID:            id,
		SourceID:      client.id,
		HostID:        host.ID,
		HostName:      host.Name,
		Node:          host.Node,
		Type:          task.Type,
		User:          task.User,
		Status:        status,
		Target:        target,
		VMID:          vmid,
		GuestName:     guestName,
		StartedAt:     task.StartTime,
		StartedAgeSec: startedAge,
		EndedAt:       task.EndTime,
		DurationSec:   duration,
		Health:        health,
	}
}

func applyGuestConfig(guest *display.Guest, cfg map[string]any) {
	if tags := stringValue(cfg["tags"]); tags != "" && guest.Tags == "" {
		guest.Tags = tags
	}
	if cores := intValue(cfg["cores"]); cores > 0 && guest.MaxCPU == 0 {
		guest.MaxCPU = cores
	}
	if memoryMB := int64Value(cfg["memory"]); memoryMB > 0 && guest.MemoryTotalBytes == 0 {
		guest.MemoryTotalBytes = memoryMB * 1024 * 1024
	}
	guest.OSType = firstNonEmpty(stringValue(cfg["ostype"]), stringValue(cfg["arch"]))
	guest.OnBoot = boolValue(cfg["onboot"])
	guest.Protection = boolValue(cfg["protection"])
	guest.Template = boolValue(cfg["template"])
	guest.Unprivileged = boolValue(cfg["unprivileged"])
	guest.AgentEnabled = agentEnabled(cfg["agent"])
	guest.IPAddress = guestIPAddress(cfg)
}

func guestIPAddress(cfg map[string]any) string {
	for _, key := range []string{"ipconfig0", "ipconfig1", "net0", "net1"} {
		value := stringValue(cfg[key])
		if value == "" {
			continue
		}
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "ip=") {
				return strings.TrimPrefix(part, "ip=")
			}
		}
	}
	return ""
}

func agentEnabled(value any) bool {
	if boolValue(value) {
		return true
	}
	text := strings.ToLower(stringValue(value))
	return strings.Contains(text, "enabled=1") || strings.Contains(text, "enabled=true")
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		return text == "1" || text == "true" || text == "yes" || text == "on"
	default:
		return false
	}
}

func intValue(value any) int {
	return int(int64Value(value))
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func stringify(value any) string {
	text := stringValue(value)
	if text == "<nil>" {
		return ""
	}
	return text
}

func addDataWarning(host *display.Host, message string) {
	for _, existing := range host.DataWarnings {
		if existing == message {
			return
		}
	}
	host.DataWarnings = append(host.DataWarnings, message)
}

func maxHealth(a, b display.Health) display.Health {
	if severityRank(b) > severityRank(a) {
		return b
	}
	return a
}

func maxInt64(a, b int64) int64 {
	if b > a {
		return b
	}
	return a
}

func pctFloat(value float64) int {
	if value <= 0 {
		return 0
	}
	return clampPct(int(math.Round(value * 100)))
}

func pctInt64(used, total int64) int {
	if used <= 0 || total <= 0 {
		return 0
	}
	return clampPct(int(math.Round(float64(used) / float64(total) * 100)))
}

func clampPct(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func displayName(sourceName, nodeName string) string {
	if sourceName == "" || sourceName == nodeName {
		return nodeName
	}
	return sourceName + " / " + nodeName
}

func severityRank(h display.Health) int {
	switch h {
	case display.HealthCritical:
		return 3
	case display.HealthWarning:
		return 2
	case display.HealthOK:
		return 1
	default:
		return 0
	}
}
