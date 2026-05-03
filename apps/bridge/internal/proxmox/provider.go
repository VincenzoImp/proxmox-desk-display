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

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
)

type Collector struct {
	cfg     config.Config
	clients []*Client
	pinned  map[string]config.PinnedGuest
}

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
		state.Guests = append(state.Guests, res.state.Guests...)
		state.Alerts = append(state.Alerts, res.state.Alerts...)
	}

	sort.Slice(state.Hosts, func(i, j int) bool { return state.Hosts[i].ID < state.Hosts[j].ID })
	sort.Slice(state.Guests, func(i, j int) bool {
		if state.Guests[i].Pinned != state.Guests[j].Pinned {
			return state.Guests[i].Pinned
		}
		return state.Guests[i].ID < state.Guests[j].ID
	})
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
			ID:         hostID,
			Name:       displayName(client.name, nodeName),
			SourceID:   client.id,
			Node:       nodeName,
			Online:     r.Status == "online" || r.Status == "",
			CPUPct:     pctFloat(r.CPU),
			MemoryPct:  pctInt64(r.Mem, r.MaxMem),
			StoragePct: pctInt64(r.Disk, r.MaxDisk),
			UptimeSec:  r.Uptime,
			Health:     display.HealthOK,
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
			}
			if status.RootFS.Total > 0 {
				host.StoragePct = pctInt64(status.RootFS.Used, status.RootFS.Total)
			}
			if status.Uptime > 0 {
				host.UptimeSec = status.Uptime
			}
		}
	}

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
			ID:        guestID,
			VMID:      vmid,
			Name:      name,
			Type:      r.Type,
			HostID:    hostID,
			SourceID:  client.id,
			Status:    firstNonEmpty(r.Status, "unknown"),
			CPUPct:    pctFloat(r.CPU),
			MemoryPct: pctInt64(r.Mem, r.MaxMem),
			Pinned:    pinned,
			Expected:  pin.Expected,
			Health:    display.HealthOK,
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
		if host, ok := hostByNode[r.Node]; ok {
			if guest.Status == "running" {
				host.GuestsRunning++
			} else {
				host.GuestsStopped++
			}
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
		host.Health = display.HealthCritical
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthCritical, "memory critical", fmt.Sprintf("memory is %d%%", host.MemoryPct)))
	} else if host.MemoryPct >= c.cfg.Alerts.MemoryWarningPct {
		host.Health = display.HealthWarning
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthWarning, "memory warning", fmt.Sprintf("memory is %d%%", host.MemoryPct)))
	}
	if host.StoragePct >= c.cfg.Alerts.StorageCriticalPct {
		host.Health = display.HealthCritical
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthCritical, "storage critical", fmt.Sprintf("storage is %d%%", host.StoragePct)))
	} else if host.StoragePct >= c.cfg.Alerts.StorageWarningPct {
		if host.Health != display.HealthCritical {
			host.Health = display.HealthWarning
		}
		state.Alerts = append(state.Alerts, alertForHost(*host, display.HealthWarning, "storage warning", fmt.Sprintf("storage is %d%%", host.StoragePct)))
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
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Node    string  `json:"node"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	VMID    int     `json:"vmid"`
	CPU     float64 `json:"cpu"`
	Mem     int64   `json:"mem"`
	MaxMem  int64   `json:"maxmem"`
	Disk    int64   `json:"disk"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
}

type nodeStatus struct {
	CPU    *float64 `json:"cpu"`
	Uptime int64    `json:"uptime"`
	Memory struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"memory"`
	RootFS struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"rootfs"`
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
