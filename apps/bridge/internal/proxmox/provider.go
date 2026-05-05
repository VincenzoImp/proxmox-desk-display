package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/VincenzoImp/proxmox-desk-display/apps/bridge/internal/display"
)

type Collector struct {
	cfg     config.Config
	clients []*Client
	pinned  map[string]config.PinnedGuest
}

const (
	maxDisplayTasks     = 48
	maxDisplayUpdates   = 64
	maxDisplayRepos     = 48
	maxDisplaySnapshots = 96
	maxDisplayServices  = 96
	maxDisplayNetworks  = 64
	maxStorageItems     = 256
	maxMetricTrends     = 384
	maxTrendSamples     = 24
	maxCapabilities     = 512
	maxClusterOptions   = 64
	maxGuestDisks       = 12
	maxGuestNICs        = 8
	maxGuestIPs         = 8
	maxGuestFilesystems = 8
)

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
		state.Clusters = append(state.Clusters, res.state.Clusters...)
		state.Hosts = append(state.Hosts, res.state.Hosts...)
		state.Storages = append(state.Storages, res.state.Storages...)
		state.Disks = append(state.Disks, res.state.Disks...)
		state.Networks = append(state.Networks, res.state.Networks...)
		state.Services = append(state.Services, res.state.Services...)
		state.ZFSPools = append(state.ZFSPools, res.state.ZFSPools...)
		state.Guests = append(state.Guests, res.state.Guests...)
		state.Snapshots = append(state.Snapshots, res.state.Snapshots...)
		state.Tasks = append(state.Tasks, res.state.Tasks...)
		state.BackupJobs = append(state.BackupJobs, res.state.BackupJobs...)
		state.Replications = append(state.Replications, res.state.Replications...)
		state.HAResources = append(state.HAResources, res.state.HAResources...)
		state.Certificates = append(state.Certificates, res.state.Certificates...)
		state.StorageItems = append(state.StorageItems, res.state.StorageItems...)
		state.MetricTrends = append(state.MetricTrends, res.state.MetricTrends...)
		state.ClusterOptions = append(state.ClusterOptions, res.state.ClusterOptions...)
		state.CephClusters = append(state.CephClusters, res.state.CephClusters...)
		state.Capabilities = append(state.Capabilities, res.state.Capabilities...)
		state.Updates = append(state.Updates, res.state.Updates...)
		state.Repositories = append(state.Repositories, res.state.Repositories...)
		state.Subscriptions = append(state.Subscriptions, res.state.Subscriptions...)
		state.Alerts = append(state.Alerts, res.state.Alerts...)
	}

	sort.Slice(state.Clusters, func(i, j int) bool { return state.Clusters[i].ID < state.Clusters[j].ID })
	sort.Slice(state.Hosts, func(i, j int) bool { return state.Hosts[i].ID < state.Hosts[j].ID })
	sort.Slice(state.Guests, func(i, j int) bool {
		if state.Guests[i].Pinned != state.Guests[j].Pinned {
			return state.Guests[i].Pinned
		}
		return state.Guests[i].ID < state.Guests[j].ID
	})
	sort.Slice(state.Storages, func(i, j int) bool { return state.Storages[i].ID < state.Storages[j].ID })
	sort.Slice(state.Disks, func(i, j int) bool { return state.Disks[i].ID < state.Disks[j].ID })
	sort.Slice(state.Networks, func(i, j int) bool { return state.Networks[i].ID < state.Networks[j].ID })
	if len(state.Networks) > maxDisplayNetworks {
		state.Networks = state.Networks[:maxDisplayNetworks]
	}
	sort.Slice(state.Services, func(i, j int) bool { return state.Services[i].ID < state.Services[j].ID })
	if len(state.Services) > maxDisplayServices {
		state.Services = state.Services[:maxDisplayServices]
	}
	sort.Slice(state.ZFSPools, func(i, j int) bool { return state.ZFSPools[i].ID < state.ZFSPools[j].ID })
	sort.Slice(state.Snapshots, func(i, j int) bool {
		if state.Snapshots[i].SnapTime != state.Snapshots[j].SnapTime {
			return state.Snapshots[i].SnapTime > state.Snapshots[j].SnapTime
		}
		return state.Snapshots[i].ID < state.Snapshots[j].ID
	})
	if len(state.Snapshots) > maxDisplaySnapshots {
		state.Snapshots = state.Snapshots[:maxDisplaySnapshots]
	}
	sort.Slice(state.Tasks, func(i, j int) bool {
		if state.Tasks[i].StartedAt != state.Tasks[j].StartedAt {
			return state.Tasks[i].StartedAt > state.Tasks[j].StartedAt
		}
		return state.Tasks[i].ID < state.Tasks[j].ID
	})
	if len(state.Tasks) > maxDisplayTasks {
		state.Tasks = state.Tasks[:maxDisplayTasks]
	}
	sort.Slice(state.BackupJobs, func(i, j int) bool { return state.BackupJobs[i].ID < state.BackupJobs[j].ID })
	sort.Slice(state.Replications, func(i, j int) bool { return state.Replications[i].ID < state.Replications[j].ID })
	sort.Slice(state.HAResources, func(i, j int) bool { return state.HAResources[i].ID < state.HAResources[j].ID })
	sort.Slice(state.Certificates, func(i, j int) bool { return state.Certificates[i].ID < state.Certificates[j].ID })
	sort.Slice(state.StorageItems, func(i, j int) bool {
		if state.StorageItems[i].CreatedAt != state.StorageItems[j].CreatedAt {
			return state.StorageItems[i].CreatedAt > state.StorageItems[j].CreatedAt
		}
		return state.StorageItems[i].ID < state.StorageItems[j].ID
	})
	if len(state.StorageItems) > maxStorageItems {
		state.StorageItems = state.StorageItems[:maxStorageItems]
	}
	sort.Slice(state.MetricTrends, func(i, j int) bool { return state.MetricTrends[i].ID < state.MetricTrends[j].ID })
	if len(state.MetricTrends) > maxMetricTrends {
		state.MetricTrends = state.MetricTrends[:maxMetricTrends]
	}
	sort.Slice(state.ClusterOptions, func(i, j int) bool { return state.ClusterOptions[i].ID < state.ClusterOptions[j].ID })
	if len(state.ClusterOptions) > maxClusterOptions {
		state.ClusterOptions = state.ClusterOptions[:maxClusterOptions]
	}
	sort.Slice(state.CephClusters, func(i, j int) bool { return state.CephClusters[i].ID < state.CephClusters[j].ID })
	sort.Slice(state.Capabilities, func(i, j int) bool { return state.Capabilities[i].ID < state.Capabilities[j].ID })
	if len(state.Capabilities) > maxCapabilities {
		state.Capabilities = state.Capabilities[:maxCapabilities]
	}
	sort.Slice(state.Updates, func(i, j int) bool { return state.Updates[i].ID < state.Updates[j].ID })
	if len(state.Updates) > maxDisplayUpdates {
		state.Updates = state.Updates[:maxDisplayUpdates]
	}
	sort.Slice(state.Repositories, func(i, j int) bool { return state.Repositories[i].ID < state.Repositories[j].ID })
	if len(state.Repositories) > maxDisplayRepos {
		state.Repositories = state.Repositories[:maxDisplayRepos]
	}
	sort.Slice(state.Subscriptions, func(i, j int) bool { return state.Subscriptions[i].ID < state.Subscriptions[j].ID })
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

	cluster := display.Cluster{
		ID:       client.id,
		SourceID: client.id,
		Name:     client.name,
		Health:   display.HealthOK,
		Quorate:  true,
	}
	if statuses, err := c.clusterStatus(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "cluster_status", "/api2/json/cluster/status", nil)
		applyClusterStatus(&cluster, statuses)
		if !cluster.Quorate {
			cluster.Health = display.HealthCritical
			state.Alerts = append(state.Alerts, display.Alert{
				ID:       client.id + "/cluster-quorum",
				SourceID: client.id,
				Severity: display.HealthCritical,
				Title:    client.name + " quorum",
				Message:  "cluster is not quorate",
			})
		}
	} else {
		addCapability(&state, client.id, "", "", "cluster_status", "/api2/json/cluster/status", err)
		addClusterWarning(&cluster, "cluster status unavailable")
	}
	if options, err := c.clusterOptions(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "cluster_options", "/api2/json/cluster/options", nil)
		state.ClusterOptions = append(state.ClusterOptions, clusterOptionsDisplay(client, options)...)
	} else {
		addCapability(&state, client.id, "", "", "cluster_options", "/api2/json/cluster/options", err)
		addClusterWarning(&cluster, "cluster options unavailable")
	}
	if jobs, err := c.clusterBackupJobs(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "backup_jobs", "/api2/json/cluster/backup", nil)
		for _, job := range jobs {
			state.BackupJobs = append(state.BackupJobs, backupJobDisplay(client, job))
		}
	} else {
		addCapability(&state, client.id, "", "", "backup_jobs", "/api2/json/cluster/backup", err)
		addClusterWarning(&cluster, "backup jobs unavailable")
	}
	if haResources, err := c.clusterHAResources(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "ha_resources", "/api2/json/cluster/ha/resources", nil)
		for _, resource := range haResources {
			displayResource := haResourceDisplay(client, resource)
			if displayResource.Health == display.HealthWarning || displayResource.Health == display.HealthCritical {
				cluster.Health = maxHealth(cluster.Health, displayResource.Health)
				state.Alerts = append(state.Alerts, display.Alert{
					ID:       displayResource.ID + "/ha",
					SourceID: client.id,
					Severity: displayResource.Health,
					Title:    client.name + " HA " + displayResource.SID,
					Message:  firstNonEmpty(displayResource.State, displayResource.RequestState, "not ok"),
				})
			}
			state.HAResources = append(state.HAResources, displayResource)
		}
	} else {
		addCapability(&state, client.id, "", "", "ha_resources", "/api2/json/cluster/ha/resources", err)
		addClusterWarning(&cluster, "HA unavailable")
	}
	if cephCluster, err := c.cephCluster(ctx, client); err == nil && cephCluster != nil {
		addCapability(&state, client.id, "", "", "ceph", "/api2/json/cluster/ceph/status", nil)
		if cephCluster.Health == display.HealthWarning || cephCluster.Health == display.HealthCritical {
			cluster.Health = maxHealth(cluster.Health, cephCluster.Health)
			state.Alerts = append(state.Alerts, display.Alert{
				ID:       cephCluster.ID + "/health",
				SourceID: client.id,
				Severity: cephCluster.Health,
				Title:    client.name + " Ceph",
				Message:  firstNonEmpty(cephCluster.HealthText, "not healthy"),
			})
		}
		state.CephClusters = append(state.CephClusters, *cephCluster)
	} else if err != nil {
		addCapability(&state, client.id, "", "", "ceph", "/api2/json/cluster/ceph/status", err)
	}
	storageConfigByName := map[string]storageConfig{}
	if configs, err := c.clusterStorage(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "storage_config", "/api2/json/storage", nil)
		for _, cfg := range configs {
			storageConfigByName[cfg.Storage] = cfg
		}
	} else {
		addCapability(&state, client.id, "", "", "storage_config", "/api2/json/storage", err)
		addClusterWarning(&cluster, "storage config unavailable")
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
				host.MemoryAvailableBytes = status.Memory.Available
			}
			if status.Swap.Total > 0 {
				host.SwapPct = pctInt64(status.Swap.Used, status.Swap.Total)
				host.SwapUsedBytes = status.Swap.Used
				host.SwapTotalBytes = status.Swap.Total
			}
			if status.RootFS.Total > 0 {
				host.StoragePct = pctInt64(status.RootFS.Used, status.RootFS.Total)
				host.StorageUsedBytes = status.RootFS.Used
				host.StorageTotalBytes = status.RootFS.Total
			}
			host.IOWaitPct = pctFloat(status.Wait)
			host.KSMSharedBytes = status.KSM.Shared
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
		if certs, err := c.nodeCertificates(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "node_certificates", "/api2/json/nodes/"+node+"/certificates/info", nil)
			for _, cert := range certs {
				displayCert := certificateDisplay(client, host, cert, time.Now())
				if displayCert.Health == display.HealthWarning || displayCert.Health == display.HealthCritical {
					host.Health = maxHealth(host.Health, displayCert.Health)
					state.Alerts = append(state.Alerts, display.Alert{
						ID:       displayCert.ID + "/expiry",
						SourceID: client.id,
						HostID:   host.ID,
						Severity: displayCert.Health,
						Title:    host.Name + " cert " + displayCert.Filename,
						Message:  fmt.Sprintf("expires in %d days", displayCert.DaysRemaining),
					})
				}
				state.Certificates = append(state.Certificates, displayCert)
			}
		} else {
			addCapability(&state, client.id, host.ID, "", "node_certificates", "/api2/json/nodes/"+node+"/certificates/info", err)
			addDataWarning(host, "certificates unavailable")
		}
		if rrd, err := c.nodeRRDData(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "node_rrd", "/api2/json/nodes/"+node+"/rrddata", nil)
			state.MetricTrends = append(state.MetricTrends, nodeMetricTrends(client, host, rrd)...)
		} else {
			addCapability(&state, client.id, host.ID, "", "node_rrd", "/api2/json/nodes/"+node+"/rrddata", err)
			addDataWarning(host, "node trend unavailable")
		}
		if devices, err := c.nodePCI(ctx, client, node); err == nil {
			host.GPUCount, host.GPUSummary = summarizeGPUs(devices)
		} else {
			addDataWarning(host, "pci unavailable")
		}
		if networks, err := c.nodeNetwork(ctx, client, node); err == nil {
			host.NetworkTotal, host.NetworkActive, host.PrimaryAddress = summarizeNetwork(networks)
			for _, network := range networks {
				if network.Iface == "" || network.Iface == "lo" {
					continue
				}
				state.Networks = append(state.Networks, networkDisplay(client, host, network))
			}
		} else {
			addDataWarning(host, "network unavailable")
		}
		if services, err := c.nodeServices(ctx, client, node); err == nil {
			host.ServicesTotal, host.ServicesRunning, host.ServicesFailed = summarizeServices(services)
			for _, service := range services {
				displayService := serviceDisplay(client, host, service)
				if displayService.Name == "" {
					continue
				}
				state.Services = append(state.Services, displayService)
			}
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
		if pools, err := c.nodeZFSPools(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "zfs_pools", "/api2/json/nodes/"+node+"/disks/zfs", nil)
			for _, pool := range pools {
				displayPool := zfsPoolDisplay(client, host, pool)
				if detail, err := c.nodeZFSPoolDetail(ctx, client, node, displayPool.Name); err == nil {
					applyZFSPoolDetail(&displayPool, detail)
				} else {
					addCapability(&state, client.id, host.ID, "", "zfs_pool_detail", "/api2/json/nodes/"+node+"/disks/zfs/"+displayPool.Name, err)
				}
				if displayPool.Health == display.HealthWarning || displayPool.Health == display.HealthCritical {
					host.Health = maxHealth(host.Health, displayPool.Health)
					state.Alerts = append(state.Alerts, display.Alert{
						ID:       displayPool.ID + "/zfs",
						SourceID: client.id,
						HostID:   host.ID,
						Severity: displayPool.Health,
						Title:    host.Name + " ZFS " + displayPool.Name,
						Message:  firstNonEmpty(displayPool.HealthText, displayPool.Status, "not healthy"),
					})
				}
				state.ZFSPools = append(state.ZFSPools, displayPool)
			}
		} else {
			addCapability(&state, client.id, host.ID, "", "zfs_pools", "/api2/json/nodes/"+node+"/disks/zfs", err)
			addDataWarning(host, "zfs unavailable")
		}
		if updates, err := c.nodeAPTUpdates(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "apt_updates", "/api2/json/nodes/"+node+"/apt/update", nil)
			host.UpdatesAvailable = len(updates)
			for _, update := range updates {
				state.Updates = append(state.Updates, updateDisplay(client, host, update))
			}
		} else {
			addCapability(&state, client.id, host.ID, "", "apt_updates", "/api2/json/nodes/"+node+"/apt/update", err)
			addDataWarning(host, "updates unavailable")
		}
		if repos, err := c.nodeAPTRepositories(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "apt_repositories", "/api2/json/nodes/"+node+"/apt/repositories", nil)
			for _, repo := range repositoriesDisplay(client, host, repos) {
				state.Repositories = append(state.Repositories, repo)
			}
		} else {
			addCapability(&state, client.id, host.ID, "", "apt_repositories", "/api2/json/nodes/"+node+"/apt/repositories", err)
			addDataWarning(host, "repositories unavailable")
		}
		if subscription, err := c.nodeSubscription(ctx, client, node); err == nil {
			addCapability(&state, client.id, host.ID, "", "subscription", "/api2/json/nodes/"+node+"/subscription", nil)
			displaySubscription := subscriptionDisplay(client, host, subscription)
			host.SubscriptionStatus = displaySubscription.Status
			if displaySubscription.Health == display.HealthWarning {
				host.Health = maxHealth(host.Health, display.HealthWarning)
			}
			state.Subscriptions = append(state.Subscriptions, displaySubscription)
		} else {
			addCapability(&state, client.id, host.ID, "", "subscription", "/api2/json/nodes/"+node+"/subscription", err)
			addDataWarning(host, "subscription unavailable")
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
		if cfg, ok := storageConfigByName[storageName]; ok {
			applyStorageConfig(&storage, cfg)
		}
		if storageContentQueryable(storage) {
			if items, err := c.storageContent(ctx, client, nodeName, storageName); err == nil {
				addCapability(&state, client.id, hostID, "", "storage_content", "/api2/json/nodes/"+nodeName+"/storage/"+storageName+"/content", nil)
				for _, item := range items {
					displayItem := storageItemDisplay(client, storage, item)
					applyStorageItemSummary(&storage, displayItem)
					state.StorageItems = append(state.StorageItems, displayItem)
				}
			} else {
				addCapability(&state, client.id, hostID, "", "storage_content", "/api2/json/nodes/"+nodeName+"/storage/"+storageName+"/content", err)
			}
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
		if current, err := c.guestStatus(ctx, client, r.Node, r.Type, vmid); err == nil {
			addCapability(&state, client.id, hostID, guest.ID, "guest_status", "/api2/json/nodes/"+r.Node+"/"+r.Type+"/"+vmid+"/status/current", nil)
			applyGuestStatus(&guest, current)
		} else {
			addCapability(&state, client.id, hostID, guest.ID, "guest_status", "/api2/json/nodes/"+r.Node+"/"+r.Type+"/"+vmid+"/status/current", err)
			guest.ConfigWarning = firstNonEmpty(guest.ConfigWarning, "status unavailable")
		}
		if rrd, err := c.guestRRDData(ctx, client, r.Node, r.Type, vmid); err == nil {
			addCapability(&state, client.id, hostID, guest.ID, "guest_rrd", "/api2/json/nodes/"+r.Node+"/"+r.Type+"/"+vmid+"/rrddata", nil)
			state.MetricTrends = append(state.MetricTrends, guestMetricTrends(client, guest, rrd)...)
		} else {
			addCapability(&state, client.id, hostID, guest.ID, "guest_rrd", "/api2/json/nodes/"+r.Node+"/"+r.Type+"/"+vmid+"/rrddata", err)
		}
		if snapshots, err := c.guestSnapshots(ctx, client, r.Node, r.Type, vmid); err == nil {
			for _, snapshot := range snapshots {
				if snapshot.Name == "current" {
					continue
				}
				state.Snapshots = append(state.Snapshots, snapshotDisplay(client, guest, snapshot))
			}
		} else {
			guest.ConfigWarning = firstNonEmpty(guest.ConfigWarning, "snapshots unavailable")
		}
		if guest.Type == "qemu" && guest.Status == "running" && guest.AgentEnabled {
			if err := c.applyGuestAgent(ctx, client, r.Node, vmid, &guest); err != nil {
				addCapability(&state, client.id, hostID, guest.ID, "qemu_guest_agent", "/api2/json/nodes/"+r.Node+"/qemu/"+vmid+"/agent", err)
				guest.AgentWarning = "agent unavailable"
			} else {
				addCapability(&state, client.id, hostID, guest.ID, "qemu_guest_agent", "/api2/json/nodes/"+r.Node+"/qemu/"+vmid+"/agent", nil)
			}
		}
		if guest.Type == "lxc" && guest.Status == "running" {
			if interfaces, err := c.containerInterfaces(ctx, client, r.Node, vmid); err == nil {
				addCapability(&state, client.id, hostID, guest.ID, "lxc_interfaces", "/api2/json/nodes/"+r.Node+"/lxc/"+vmid+"/interfaces", nil)
				ips := containerIPAddresses(interfaces)
				if len(ips) > 0 {
					guest.IPAddresses = ips
					if placeholderIPAddress(guest.IPAddress) {
						guest.IPAddress = ips[0]
					}
				}
			} else {
				addCapability(&state, client.id, hostID, guest.ID, "lxc_interfaces", "/api2/json/nodes/"+r.Node+"/lxc/"+vmid+"/interfaces", err)
			}
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

	if replications, err := c.clusterReplications(ctx, client); err == nil {
		addCapability(&state, client.id, "", "", "replication", "/api2/json/cluster/replication", nil)
		for _, replication := range replications {
			if replication.Source != "" && replication.ID != "" {
				if status, err := c.replicationStatus(ctx, client, replication.Source, replication.ID); err == nil {
					addCapability(&state, client.id, client.id+"/"+replication.Source, "", "replication_status", "/api2/json/nodes/"+replication.Source+"/replication/"+replication.ID+"/status", nil)
					applyReplicationStatus(&replication, status)
				} else {
					addCapability(&state, client.id, client.id+"/"+replication.Source, "", "replication_status", "/api2/json/nodes/"+replication.Source+"/replication/"+replication.ID+"/status", err)
				}
			}
			displayReplication := replicationDisplay(client, replication, guestBySourceVMID)
			if displayReplication.Health == display.HealthWarning || displayReplication.Health == display.HealthCritical {
				cluster.Health = maxHealth(cluster.Health, displayReplication.Health)
				state.Alerts = append(state.Alerts, display.Alert{
					ID:       displayReplication.ID + "/replication",
					SourceID: client.id,
					GuestID:  displayReplication.GuestID,
					Severity: displayReplication.Health,
					Title:    client.name + " replication " + displayReplication.ID,
					Message:  firstNonEmpty(displayReplication.Error, "not healthy"),
				})
			}
			state.Replications = append(state.Replications, displayReplication)
		}
	} else {
		addCapability(&state, client.id, "", "", "replication", "/api2/json/cluster/replication", err)
		addClusterWarning(&cluster, "replication unavailable")
	}

	state.Clusters = append(state.Clusters, cluster)

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

func (c *Collector) clusterStatus(ctx context.Context, client *Client) ([]clusterStatus, error) {
	var statuses []clusterStatus
	err := client.Get(ctx, "/api2/json/cluster/status", &statuses)
	return statuses, err
}

func (c *Collector) clusterBackupJobs(ctx context.Context, client *Client) ([]backupJob, error) {
	var jobs []backupJob
	err := client.Get(ctx, "/api2/json/cluster/backup", &jobs)
	return jobs, err
}

func (c *Collector) clusterHAResources(ctx context.Context, client *Client) ([]haResource, error) {
	var resources []haResource
	err := client.Get(ctx, "/api2/json/cluster/ha/resources", &resources)
	return resources, err
}

func (c *Collector) clusterReplications(ctx context.Context, client *Client) ([]replicationJob, error) {
	var replications []replicationJob
	err := client.Get(ctx, "/api2/json/cluster/replication", &replications)
	return replications, err
}

func (c *Collector) replicationStatus(ctx context.Context, client *Client, node string, id string) (replicationStatus, error) {
	var status replicationStatus
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/replication/"+url.PathEscape(id)+"/status", &status)
	return status, err
}

func (c *Collector) clusterOptions(ctx context.Context, client *Client) (map[string]any, error) {
	var options map[string]any
	err := client.Get(ctx, "/api2/json/cluster/options", &options)
	return options, err
}

func (c *Collector) clusterStorage(ctx context.Context, client *Client) ([]storageConfig, error) {
	var storages []storageConfig
	err := client.Get(ctx, "/api2/json/storage", &storages)
	return storages, err
}

func (c *Collector) nodeZFSPools(ctx context.Context, client *Client, node string) ([]zfsPool, error) {
	var pools []zfsPool
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/disks/zfs", &pools)
	return pools, err
}

func (c *Collector) nodeZFSPoolDetail(ctx context.Context, client *Client, node string, pool string) (zfsPoolDetail, error) {
	var detail zfsPoolDetail
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/disks/zfs/"+url.PathEscape(pool), &detail)
	return detail, err
}

func (c *Collector) nodeCertificates(ctx context.Context, client *Client, node string) ([]nodeCertificate, error) {
	var certs []nodeCertificate
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/certificates/info", &certs)
	return certs, err
}

func (c *Collector) nodeRRDData(ctx context.Context, client *Client, node string) ([]rrdPoint, error) {
	var points []rrdPoint
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/rrddata?timeframe=hour&cf=AVERAGE", &points)
	return points, err
}

func (c *Collector) nodeAPTUpdates(ctx context.Context, client *Client, node string) ([]aptUpdate, error) {
	var updates []aptUpdate
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/apt/update", &updates)
	return updates, err
}

func (c *Collector) nodeAPTRepositories(ctx context.Context, client *Client, node string) (aptRepositories, error) {
	var repos aptRepositories
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/apt/repositories", &repos)
	return repos, err
}

func (c *Collector) nodeSubscription(ctx context.Context, client *Client, node string) (subscription, error) {
	var sub subscription
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/subscription", &sub)
	return sub, err
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

func (c *Collector) guestSnapshots(ctx context.Context, client *Client, node string, guestType string, vmid string) ([]guestSnapshot, error) {
	var snapshots []guestSnapshot
	pathType := "qemu"
	if guestType == "lxc" {
		pathType = "lxc"
	}
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/"+pathType+"/"+url.PathEscape(vmid)+"/snapshot", &snapshots)
	return snapshots, err
}

func (c *Collector) guestStatus(ctx context.Context, client *Client, node string, guestType string, vmid string) (guestStatus, error) {
	var status guestStatus
	pathType := "qemu"
	if guestType == "lxc" {
		pathType = "lxc"
	}
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/"+pathType+"/"+url.PathEscape(vmid)+"/status/current", &status)
	return status, err
}

func (c *Collector) guestRRDData(ctx context.Context, client *Client, node string, guestType string, vmid string) ([]rrdPoint, error) {
	var points []rrdPoint
	pathType := "qemu"
	if guestType == "lxc" {
		pathType = "lxc"
	}
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/"+pathType+"/"+url.PathEscape(vmid)+"/rrddata?timeframe=hour&cf=AVERAGE", &points)
	return points, err
}

func (c *Collector) containerInterfaces(ctx context.Context, client *Client, node string, vmid string) ([]containerInterface, error) {
	var interfaces []containerInterface
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/lxc/"+url.PathEscape(vmid)+"/interfaces", &interfaces)
	return interfaces, err
}

func (c *Collector) storageContent(ctx context.Context, client *Client, node string, storage string) ([]storageContent, error) {
	var contents []storageContent
	err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/storage/"+url.PathEscape(storage)+"/content", &contents)
	return contents, err
}

func (c *Collector) cephCluster(ctx context.Context, client *Client) (*display.CephCluster, error) {
	var status cephStatus
	if err := client.Get(ctx, "/api2/json/cluster/ceph/status", &status); err != nil {
		return nil, err
	}
	var df cephDF
	_ = client.Get(ctx, "/api2/json/cluster/ceph/df", &df)
	healthText := strings.TrimPrefix(status.Health.Status, "HEALTH_")
	if healthText == "" {
		healthText = status.Health.Status
	}
	used := df.Summary.TotalUsedBytes
	total := df.Summary.TotalBytes
	return &display.CephCluster{
		ID:             client.id + "/ceph",
		SourceID:       client.id,
		FSID:           status.FSID,
		HealthText:     healthText,
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: df.Summary.TotalAvailBytes,
		UsagePct:       pctInt64(used, total),
		OSDs:           status.OSDMap.OSDMap.NumOSDs,
		OSDsUp:         status.OSDMap.OSDMap.NumUp,
		OSDsIn:         status.OSDMap.OSDMap.NumIn,
		PGs:            status.PGMap.NumPGs,
		Health:         healthFromText(healthText),
	}, nil
}

func (c *Collector) applyGuestAgent(ctx context.Context, client *Client, node string, vmid string, guest *display.Guest) error {
	var firstErr error
	var info agentInfo
	if err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/qemu/"+url.PathEscape(vmid)+"/agent/info", &info); err == nil {
		guest.AgentAvailable = true
		guest.AgentVersion = info.Result.Version
		guest.AgentCommandCount = len(info.Result.SupportedCommands)
	} else {
		firstErr = err
	}
	var hostname agentHostname
	if err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/qemu/"+url.PathEscape(vmid)+"/agent/get-host-name", &hostname); err == nil {
		guest.AgentAvailable = true
		guest.AgentHostname = hostname.Result.HostName
	} else {
		firstErr = err
	}
	var osInfo agentOSInfo
	if err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/qemu/"+url.PathEscape(vmid)+"/agent/get-osinfo", &osInfo); err == nil {
		guest.AgentAvailable = true
		guest.AgentOS = firstNonEmpty(osInfo.Result.PrettyName, osInfo.Result.Name, osInfo.Result.ID)
	} else if firstErr == nil {
		firstErr = err
	}
	var networks agentNetworkInterfaces
	if err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/qemu/"+url.PathEscape(vmid)+"/agent/network-get-interfaces", &networks); err == nil {
		guest.AgentAvailable = true
		guest.IPAddresses = agentIPAddresses(networks)
		if placeholderIPAddress(guest.IPAddress) && len(guest.IPAddresses) > 0 {
			guest.IPAddress = guest.IPAddresses[0]
		}
	} else if firstErr == nil {
		firstErr = err
	}
	var fsInfo agentFSInfo
	if err := client.Get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/qemu/"+url.PathEscape(vmid)+"/agent/get-fsinfo", &fsInfo); err == nil {
		guest.AgentAvailable = true
		guest.Filesystems = agentFilesystems(fsInfo)
	} else if firstErr == nil {
		firstErr = err
	}
	if guest.AgentAvailable {
		return nil
	}
	return firstErr
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
	Wait       float64  `json:"wait"`
	Memory     struct {
		Used      int64 `json:"used"`
		Total     int64 `json:"total"`
		Available int64 `json:"available"`
	} `json:"memory"`
	Swap struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"swap"`
	KSM struct {
		Shared int64 `json:"shared"`
	} `json:"ksm"`
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

type nodeCertificate struct {
	Filename      string   `json:"filename"`
	Subject       string   `json:"subject"`
	Issuer        string   `json:"issuer"`
	SANs          []string `json:"san"`
	Fingerprint   string   `json:"fingerprint"`
	PublicKeyType string   `json:"public-key-type"`
	PublicKeyBits int      `json:"public-key-bits"`
	NotBefore     int64    `json:"notbefore"`
	NotAfter      int64    `json:"notafter"`
}

type storageConfig struct {
	Storage    string `json:"storage"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	Path       string `json:"path"`
	Pool       string `json:"pool"`
	Mountpoint string `json:"mountpoint"`
	Nodes      string `json:"nodes"`
	Shared     any    `json:"shared"`
}

type storageContent struct {
	VolID        string         `json:"volid"`
	Content      string         `json:"content"`
	Format       string         `json:"format"`
	Size         int64          `json:"size"`
	CTime        int64          `json:"ctime"`
	VMID         any            `json:"vmid"`
	Notes        string         `json:"notes"`
	Protected    int            `json:"protected"`
	Verification map[string]any `json:"verification"`
	Verified     int64          `json:"verified"`
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
	Iface       string `json:"iface"`
	Type        string `json:"type"`
	Active      int    `json:"active"`
	Autostart   int    `json:"autostart"`
	Method      string `json:"method"`
	Address     string `json:"address"`
	CIDR        string `json:"cidr"`
	Gateway     string `json:"gateway"`
	BridgePorts string `json:"bridge_ports"`
	Slaves      string `json:"slaves"`
	VLANAware   int    `json:"vlan-aware"`
	Comments    string `json:"comments"`
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

type clusterStatus struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	NodeID   int    `json:"nodeid"`
	Online   int    `json:"online"`
	Quorate  int    `json:"quorate"`
	Version  int    `json:"version"`
	Nodes    int    `json:"nodes"`
	Expected int    `json:"expected"`
}

type backupJob struct {
	ID       string `json:"id"`
	Storage  string `json:"storage"`
	Schedule string `json:"schedule"`
	Mode     string `json:"mode"`
	Enabled  int    `json:"enabled"`
	All      int    `json:"all"`
	VMID     string `json:"vmid"`
	Compress string `json:"compress"`
	MailTo   string `json:"mailto"`
}

type haResource struct {
	SID          string `json:"sid"`
	Type         string `json:"type"`
	State        string `json:"state"`
	RequestState string `json:"request_state"`
	Group        string `json:"group"`
	Node         string `json:"node"`
	MaxRestart   int    `json:"max_restart"`
	MaxRelocate  int    `json:"max_relocate"`
}

type replicationJob struct {
	ID             string `json:"id"`
	Guest          int    `json:"guest"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	Schedule       string `json:"schedule"`
	Rate           int64  `json:"rate"`
	Disable        int    `json:"disable"`
	Status         string `json:"status"`
	State          string `json:"state"`
	Duration       int64  `json:"duration"`
	FailCount      int    `json:"fail_count"`
	LastSync       int64  `json:"last_sync"`
	NextSync       int64  `json:"next_sync"`
	LastSyncStatus string `json:"last_sync_status"`
	Error          string `json:"error"`
	Type           string `json:"type"`
}

type replicationStatus struct {
	Status         string `json:"status"`
	State          string `json:"state"`
	Duration       int64  `json:"duration"`
	FailCount      int    `json:"fail_count"`
	LastSync       int64  `json:"last_sync"`
	NextSync       int64  `json:"next_sync"`
	LastSyncStatus string `json:"last_sync_status"`
	Error          string `json:"error"`
}

type zfsPool struct {
	Name   string `json:"name"`
	Pool   string `json:"pool"`
	Health string `json:"health"`
	Status string `json:"status"`
	Size   int64  `json:"size"`
	Alloc  int64  `json:"alloc"`
	Free   int64  `json:"free"`
	Frag   int    `json:"frag"`
	Dedup  any    `json:"dedup"`
}

type zfsPoolDetail struct {
	Name     string          `json:"name"`
	State    string          `json:"state"`
	Status   string          `json:"status"`
	Scan     string          `json:"scan"`
	Errors   string          `json:"errors"`
	Leaf     int             `json:"leaf"`
	Read     int64           `json:"read"`
	Write    int64           `json:"write"`
	Checksum int64           `json:"cksum"`
	Children []zfsPoolDetail `json:"children"`
}

type rrdPoint struct {
	Time               int64   `json:"time"`
	CPU                float64 `json:"cpu"`
	MaxCPU             float64 `json:"maxcpu"`
	Mem                float64 `json:"mem"`
	MemHost            float64 `json:"memhost"`
	MemUsed            float64 `json:"memused"`
	MemAvailable       float64 `json:"memavailable"`
	MaxMem             float64 `json:"maxmem"`
	MemTotal           float64 `json:"memtotal"`
	Disk               float64 `json:"disk"`
	MaxDisk            float64 `json:"maxdisk"`
	RootUsed           float64 `json:"rootused"`
	RootTotal          float64 `json:"roottotal"`
	NetIn              float64 `json:"netin"`
	NetOut             float64 `json:"netout"`
	DiskRead           float64 `json:"diskread"`
	DiskWrite          float64 `json:"diskwrite"`
	IOWait             float64 `json:"iowait"`
	PressureIOSome     float64 `json:"pressureiosome"`
	PressureIOFull     float64 `json:"pressureiofull"`
	PressureCPUSome    float64 `json:"pressurecpusome"`
	PressureCPUFull    float64 `json:"pressurecpufull"`
	PressureMemorySome float64 `json:"pressurememorysome"`
	PressureMemoryFull float64 `json:"pressurememoryfull"`
}

type guestStatus struct {
	Status             string  `json:"status"`
	QMPStatus          string  `json:"qmpstatus"`
	CPU                float64 `json:"cpu"`
	CPUs               int     `json:"cpus"`
	Mem                int64   `json:"mem"`
	MemHost            int64   `json:"memhost"`
	MaxMem             int64   `json:"maxmem"`
	Swap               int64   `json:"swap"`
	MaxSwap            int64   `json:"maxswap"`
	Disk               int64   `json:"disk"`
	MaxDisk            int64   `json:"maxdisk"`
	DiskRead           int64   `json:"diskread"`
	DiskWrite          int64   `json:"diskwrite"`
	NetIn              int64   `json:"netin"`
	NetOut             int64   `json:"netout"`
	Uptime             int64   `json:"uptime"`
	PID                int     `json:"pid"`
	Tags               string  `json:"tags"`
	RunningQEMU        string  `json:"running-qemu"`
	PressureCPUSome    any     `json:"pressurecpusome"`
	PressureCPUFull    any     `json:"pressurecpufull"`
	PressureIOSome     any     `json:"pressureiosome"`
	PressureIOFull     any     `json:"pressureiofull"`
	PressureMemorySome any     `json:"pressurememorysome"`
	PressureMemoryFull any     `json:"pressurememoryfull"`
	HA                 struct {
		Managed int `json:"managed"`
	} `json:"ha"`
}

type containerInterface struct {
	Name            string `json:"name"`
	Inet            string `json:"inet"`
	Inet6           string `json:"inet6"`
	HardwareAddress string `json:"hardware-address"`
	HWAddr          string `json:"hwaddr"`
	IPAddresses     []struct {
		IPAddress     string `json:"ip-address"`
		IPAddressType string `json:"ip-address-type"`
		Prefix        any    `json:"prefix"`
	} `json:"ip-addresses"`
}

type cephStatus struct {
	FSID   string `json:"fsid"`
	Health struct {
		Status string `json:"status"`
	} `json:"health"`
	OSDMap struct {
		OSDMap struct {
			NumOSDs int `json:"num_osds"`
			NumUp   int `json:"num_up_osds"`
			NumIn   int `json:"num_in_osds"`
		} `json:"osdmap"`
	} `json:"osdmap"`
	PGMap struct {
		NumPGs int `json:"num_pgs"`
	} `json:"pgmap"`
}

type cephDF struct {
	Summary struct {
		TotalBytes      int64 `json:"total_bytes"`
		TotalUsedBytes  int64 `json:"total_used_bytes"`
		TotalAvailBytes int64 `json:"total_avail_bytes"`
	} `json:"summary"`
}

type aptUpdate struct {
	Package          string `json:"Package"`
	Title            string `json:"Title"`
	CurrentVersion   string `json:"CurrentVersion"`
	CandidateVersion string `json:"Version"`
	Origin           string `json:"Origin"`
	Section          string `json:"Section"`
	Priority         string `json:"Priority"`
}

type aptRepositories struct {
	Files []aptRepositoryFile `json:"files"`
	Infos []aptRepositoryInfo `json:"infos"`
}

type aptRepositoryFile struct {
	Path         string               `json:"path"`
	Repositories []aptRepositoryEntry `json:"repositories"`
}

type aptRepositoryEntry struct {
	Types      []string `json:"types"`
	URIs       []string `json:"uris"`
	Suites     []string `json:"suites"`
	Components []string `json:"components"`
	Enabled    any      `json:"enabled"`
}

type aptRepositoryInfo struct {
	Path    string `json:"path"`
	Index   any    `json:"index"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type subscription struct {
	Status      string `json:"status"`
	Level       string `json:"level"`
	ProductName string `json:"productname"`
	ServerID    string `json:"serverid"`
	NextDueDate string `json:"nextduedate"`
	Message     string `json:"message"`
}

type guestSnapshot struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapTime    int64  `json:"snaptime"`
	Parent      string `json:"parent"`
	VMState     int    `json:"vmstate"`
}

type agentInfo struct {
	Result struct {
		Version           string `json:"version"`
		SupportedCommands []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"supported_commands"`
	} `json:"result"`
}

type agentHostname struct {
	Result struct {
		HostName string `json:"host-name"`
	} `json:"result"`
}

type agentOSInfo struct {
	Result struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PrettyName string `json:"pretty-name"`
		Version    string `json:"version"`
	} `json:"result"`
}

type agentNetworkInterfaces struct {
	Result []struct {
		Name            string `json:"name"`
		HardwareAddress string `json:"hardware-address"`
		IPAddresses     []struct {
			IPAddress     string `json:"ip-address"`
			IPAddressType string `json:"ip-address-type"`
			Prefix        int    `json:"prefix"`
		} `json:"ip-addresses"`
	} `json:"result"`
}

type agentFSInfo struct {
	Result []struct {
		Name       string `json:"name"`
		Mountpoint string `json:"mountpoint"`
		Type       string `json:"type"`
		UsedBytes  int64  `json:"used-bytes"`
		TotalBytes int64  `json:"total-bytes"`
		Disk       []struct {
			BusType string `json:"bus-type"`
		} `json:"disk"`
	} `json:"result"`
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

func applyClusterStatus(cluster *display.Cluster, statuses []clusterStatus) {
	cluster.NodesTotal = 0
	cluster.NodesOnline = 0
	for _, status := range statuses {
		switch status.Type {
		case "cluster":
			cluster.Name = firstNonEmpty(status.Name, cluster.Name)
			cluster.Quorate = status.Quorate != 0
			cluster.Version = status.Version
			cluster.NodesExpected = status.Expected
		case "node":
			cluster.NodesTotal++
			if status.Online != 0 {
				cluster.NodesOnline++
			}
		}
	}
	if cluster.NodesTotal == 0 {
		cluster.NodesTotal = cluster.NodesExpected
	}
	if cluster.NodesTotal > 0 && cluster.NodesOnline < cluster.NodesTotal {
		cluster.Health = maxHealth(cluster.Health, display.HealthWarning)
	}
}

func clusterOptionsDisplay(client *Client, options map[string]any) []display.ClusterOption {
	keys := make([]string, 0, len(options))
	for key := range options {
		if key == "digest" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]display.ClusterOption, 0, len(keys))
	for _, key := range keys {
		value := compactValue(options[key])
		out = append(out, display.ClusterOption{
			ID:       client.id + "/" + key,
			SourceID: client.id,
			Key:      key,
			Value:    value,
		})
	}
	return out
}

func certificateDisplay(client *Client, host *display.Host, cert nodeCertificate, now time.Time) display.Certificate {
	remaining := 0
	if cert.NotAfter > 0 {
		remaining = int(math.Floor(time.Unix(cert.NotAfter, 0).Sub(now).Hours() / 24))
	}
	health := display.HealthOK
	if cert.NotAfter > 0 {
		if cert.NotAfter <= now.Unix() {
			health = display.HealthCritical
		} else if remaining <= 30 {
			health = display.HealthWarning
		}
	}
	filename := firstNonEmpty(cert.Filename, cert.Fingerprint, "certificate")
	return display.Certificate{
		ID:            client.id + "/" + host.Node + "/" + filename,
		SourceID:      client.id,
		HostID:        host.ID,
		HostName:      host.Name,
		Node:          host.Node,
		Filename:      filename,
		Subject:       cert.Subject,
		Issuer:        cert.Issuer,
		SANs:          cert.SANs,
		Fingerprint:   cert.Fingerprint,
		PublicKeyType: cert.PublicKeyType,
		PublicKeyBits: cert.PublicKeyBits,
		NotBefore:     cert.NotBefore,
		NotAfter:      cert.NotAfter,
		DaysRemaining: remaining,
		Health:        health,
	}
}

func applyStorageConfig(storage *display.Storage, cfg storageConfig) {
	storage.PluginType = firstNonEmpty(storage.PluginType, cfg.Type)
	storage.Content = firstNonEmpty(storage.Content, cfg.Content)
	storage.Path = cfg.Path
	storage.Pool = cfg.Pool
	storage.Mountpoint = cfg.Mountpoint
	if !storage.Shared {
		storage.Shared = boolValue(cfg.Shared)
	}
}

func storageContentQueryable(storage display.Storage) bool {
	if storage.Name == "" {
		return false
	}
	status := strings.ToLower(storage.Status)
	if status != "" && status != "available" && status != "ok" {
		return false
	}
	return storage.Content != ""
}

func storageItemDisplay(client *Client, storage display.Storage, item storageContent) display.StorageItem {
	vmid := stringValue(item.VMID)
	verification := verificationState(item)
	health := display.HealthOK
	if strings.EqualFold(verification, "failed") || strings.EqualFold(verification, "error") {
		health = display.HealthWarning
	}
	return display.StorageItem{
		ID:                client.id + "/" + storage.Node + "/" + item.VolID,
		SourceID:          client.id,
		HostID:            storage.HostID,
		HostName:          storage.HostName,
		Node:              storage.Node,
		Storage:           storage.Name,
		Content:           item.Content,
		VolID:             item.VolID,
		VMID:              vmid,
		Format:            item.Format,
		SizeBytes:         item.Size,
		CreatedAt:         item.CTime,
		Notes:             item.Notes,
		Protected:         item.Protected != 0,
		VerificationState: verification,
		Health:            health,
	}
}

func applyStorageItemSummary(storage *display.Storage, item display.StorageItem) {
	storage.ContentItems++
	switch item.Content {
	case "backup":
		storage.BackupCount++
	case "iso":
		storage.ISOCount++
	case "vztmpl":
		storage.TemplateCount++
	case "images":
		storage.ImageCount++
	case "rootdir":
		storage.RootdirCount++
	}
}

func verificationState(item storageContent) string {
	if item.Verification != nil {
		if state := stringValue(item.Verification["state"]); state != "" {
			return state
		}
	}
	if item.Verified > 0 {
		return "verified"
	}
	return ""
}

func networkDisplay(client *Client, host *display.Host, network networkInterface) display.Network {
	health := display.HealthOK
	if network.Active == 0 && network.Autostart != 0 {
		health = display.HealthWarning
	}
	return display.Network{
		ID:          client.id + "/" + host.Node + "/" + network.Iface,
		SourceID:    client.id,
		HostID:      host.ID,
		HostName:    host.Name,
		Node:        host.Node,
		Iface:       network.Iface,
		Type:        network.Type,
		Active:      network.Active != 0,
		Autostart:   network.Autostart != 0,
		Method:      network.Method,
		Address:     network.Address,
		CIDR:        network.CIDR,
		Gateway:     network.Gateway,
		BridgePorts: network.BridgePorts,
		Slaves:      network.Slaves,
		VLANAware:   network.VLANAware != 0,
		Comments:    network.Comments,
		Health:      health,
	}
}

func serviceDisplay(client *Client, host *display.Host, service nodeService) display.Service {
	name := firstNonEmpty(service.Name, service.Service)
	health := display.HealthOK
	state := strings.ToLower(firstNonEmpty(service.State, service.UnitState))
	if strings.Contains(state, "failed") {
		health = display.HealthWarning
	}
	return display.Service{
		ID:          client.id + "/" + host.Node + "/" + name,
		SourceID:    client.id,
		HostID:      host.ID,
		HostName:    host.Name,
		Node:        host.Node,
		Name:        name,
		State:       service.State,
		UnitState:   service.UnitState,
		Description: service.Description,
		Health:      health,
	}
}

func zfsPoolDisplay(client *Client, host *display.Host, pool zfsPool) display.ZFSPool {
	name := firstNonEmpty(pool.Name, pool.Pool)
	health := healthFromText(firstNonEmpty(pool.Health, pool.Status))
	return display.ZFSPool{
		ID:               client.id + "/" + host.Node + "/" + name,
		SourceID:         client.id,
		HostID:           host.ID,
		HostName:         host.Name,
		Node:             host.Node,
		Name:             name,
		HealthText:       pool.Health,
		Status:           pool.Status,
		State:            pool.Health,
		SizeBytes:        pool.Size,
		AllocatedBytes:   pool.Alloc,
		FreeBytes:        pool.Free,
		FragmentationPct: pool.Frag,
		DedupRatio:       stringValue(pool.Dedup),
		Health:           health,
	}
}

func applyZFSPoolDetail(pool *display.ZFSPool, detail zfsPoolDetail) {
	pool.State = firstNonEmpty(detail.State, pool.State)
	pool.Status = firstNonEmpty(detail.Status, pool.Status)
	pool.Scan = detail.Scan
	pool.Errors = detail.Errors
	devices, issues := zfsDeviceCounts(detail)
	pool.DeviceCount = devices
	pool.IssueCount = issues
	if issues > 0 {
		pool.Health = maxHealth(pool.Health, display.HealthWarning)
	}
	if pool.State != "" {
		pool.Health = maxHealth(pool.Health, healthFromText(pool.State))
	}
}

func backupJobDisplay(client *Client, job backupJob) display.BackupJob {
	id := firstNonEmpty(job.ID, job.Storage+"/"+job.Schedule)
	enabled := job.Enabled != 0
	health := display.HealthOK
	if !enabled {
		health = display.HealthWarning
	}
	return display.BackupJob{
		ID:       client.id + "/" + id,
		SourceID: client.id,
		Storage:  job.Storage,
		Schedule: job.Schedule,
		Mode:     job.Mode,
		Enabled:  enabled,
		All:      job.All != 0,
		VMIDs:    job.VMID,
		Compress: job.Compress,
		MailTo:   job.MailTo,
		Health:   health,
	}
}

func haResourceDisplay(client *Client, resource haResource) display.HAResource {
	health := display.HealthOK
	state := strings.ToLower(firstNonEmpty(resource.State, resource.RequestState))
	if strings.Contains(state, "error") || strings.Contains(state, "fail") || strings.Contains(state, "fence") {
		health = display.HealthCritical
	} else if state != "" && !strings.Contains(state, "started") && !strings.Contains(state, "disabled") && !strings.Contains(state, "ignored") {
		health = display.HealthWarning
	}
	return display.HAResource{
		ID:           client.id + "/" + resource.SID,
		SourceID:     client.id,
		SID:          resource.SID,
		Type:         resource.Type,
		State:        resource.State,
		RequestState: resource.RequestState,
		Group:        resource.Group,
		Node:         resource.Node,
		MaxRestart:   resource.MaxRestart,
		MaxRelocate:  resource.MaxRelocate,
		Health:       health,
	}
}

func replicationDisplay(client *Client, replication replicationJob, guests map[string]display.Guest) display.Replication {
	vmid := strconv.Itoa(replication.Guest)
	if vmid == "0" {
		vmid = ""
	}
	guestID := ""
	guestName := ""
	if vmid != "" {
		guestID = client.id + "/" + vmid
		if guest, ok := guests[guestID]; ok {
			guestName = guest.Name
		}
	}
	health := display.HealthOK
	if replication.Error != "" || replication.FailCount > 0 {
		health = display.HealthWarning
	} else if text := strings.ToLower(firstNonEmpty(replication.Status, replication.State)); text != "" && text != "ok" {
		health = display.HealthWarning
	}
	return display.Replication{
		ID:             client.id + "/" + replication.ID,
		SourceID:       client.id,
		GuestID:        guestID,
		GuestName:      guestName,
		VMID:           vmid,
		SourceNode:     replication.Source,
		TargetNode:     replication.Target,
		Schedule:       replication.Schedule,
		Rate:           replication.Rate,
		Enabled:        replication.Disable == 0,
		Status:         replication.Status,
		State:          replication.State,
		DurationSec:    replication.Duration,
		FailCount:      replication.FailCount,
		LastSync:       replication.LastSync,
		NextSync:       replication.NextSync,
		LastSyncStatus: replication.LastSyncStatus,
		Error:          replication.Error,
		Health:         health,
	}
}

func applyReplicationStatus(replication *replicationJob, status replicationStatus) {
	replication.Status = firstNonEmpty(status.Status, replication.Status)
	replication.State = firstNonEmpty(status.State, replication.State)
	if status.Duration > 0 {
		replication.Duration = status.Duration
	}
	if status.FailCount > 0 {
		replication.FailCount = status.FailCount
	}
	if status.LastSync > 0 {
		replication.LastSync = status.LastSync
	}
	if status.NextSync > 0 {
		replication.NextSync = status.NextSync
	}
	replication.LastSyncStatus = firstNonEmpty(status.LastSyncStatus, replication.LastSyncStatus)
	replication.Error = firstNonEmpty(status.Error, replication.Error)
}

func updateDisplay(client *Client, host *display.Host, update aptUpdate) display.Update {
	pkg := firstNonEmpty(update.Package, update.Title)
	return display.Update{
		ID:               client.id + "/" + host.Node + "/" + pkg,
		SourceID:         client.id,
		HostID:           host.ID,
		HostName:         host.Name,
		Node:             host.Node,
		Package:          pkg,
		Title:            update.Title,
		CurrentVersion:   update.CurrentVersion,
		CandidateVersion: update.CandidateVersion,
		Origin:           update.Origin,
		Section:          update.Section,
		Priority:         update.Priority,
		Health:           display.HealthWarning,
	}
}

func repositoriesDisplay(client *Client, host *display.Host, repos aptRepositories) []display.Repository {
	infoByPathIndex := map[string]aptRepositoryInfo{}
	for _, info := range repos.Infos {
		infoByPathIndex[info.Path+"/"+stringValue(info.Index)] = info
	}
	out := []display.Repository{}
	for _, file := range repos.Files {
		for i, repo := range file.Repositories {
			index := strconv.Itoa(i)
			info := infoByPathIndex[file.Path+"/"+index]
			health := display.HealthOK
			if strings.EqualFold(info.Status, "warning") || strings.EqualFold(info.Status, "error") || info.Message != "" {
				health = display.HealthWarning
			}
			out = append(out, display.Repository{
				ID:         client.id + "/" + host.Node + "/" + file.Path + "/" + index,
				SourceID:   client.id,
				HostID:     host.ID,
				HostName:   host.Name,
				Node:       host.Node,
				File:       file.Path,
				Types:      strings.Join(repo.Types, ","),
				URIs:       strings.Join(repo.URIs, ","),
				Suites:     strings.Join(repo.Suites, ","),
				Components: strings.Join(repo.Components, ","),
				Enabled:    boolValue(repo.Enabled),
				Status:     info.Status,
				Warning:    info.Message,
				Health:     health,
			})
		}
	}
	return out
}

func subscriptionDisplay(client *Client, host *display.Host, sub subscription) display.Subscription {
	health := display.HealthOK
	if sub.Status != "" && sub.Status != "Active" && sub.Status != "active" {
		health = display.HealthWarning
	}
	return display.Subscription{
		ID:          client.id + "/" + host.Node,
		SourceID:    client.id,
		HostID:      host.ID,
		HostName:    host.Name,
		Node:        host.Node,
		Status:      sub.Status,
		Level:       sub.Level,
		ProductName: sub.ProductName,
		ServerID:    sub.ServerID,
		NextDueDate: sub.NextDueDate,
		Health:      health,
	}
}

func snapshotDisplay(client *Client, guest display.Guest, snapshot guestSnapshot) display.Snapshot {
	return display.Snapshot{
		ID:          client.id + "/" + guest.HostName + "/" + guest.VMID + "/" + snapshot.Name,
		SourceID:    client.id,
		HostID:      guest.HostID,
		HostName:    guest.HostName,
		GuestID:     guest.ID,
		GuestName:   guest.Name,
		VMID:        guest.VMID,
		Type:        guest.Type,
		Name:        snapshot.Name,
		Description: snapshot.Description,
		SnapTime:    snapshot.SnapTime,
		Parent:      snapshot.Parent,
		VMState:     snapshot.VMState != 0,
		Health:      display.HealthOK,
	}
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
	guest.CPUType = stringValue(cfg["cpu"])
	guest.BIOS = stringValue(cfg["bios"])
	guest.Machine = stringValue(cfg["machine"])
	guest.BootOrder = stringValue(cfg["boot"])
	guest.Startup = stringValue(cfg["startup"])
	guest.Nameserver = stringValue(cfg["nameserver"])
	guest.SearchDomain = stringValue(cfg["searchdomain"])
	guest.Features = stringValue(cfg["features"])
	guest.OnBoot = boolValue(cfg["onboot"])
	guest.Protection = boolValue(cfg["protection"])
	guest.Template = boolValue(cfg["template"])
	guest.Unprivileged = boolValue(cfg["unprivileged"])
	guest.AgentEnabled = agentEnabled(cfg["agent"])
	guest.IPAddress = guestIPAddress(cfg)
	guest.Disks = guestDisks(cfg)
	guest.NICs = guestNICs(cfg)
}

func applyGuestStatus(guest *display.Guest, status guestStatus) {
	guest.Status = firstNonEmpty(status.Status, guest.Status)
	if status.CPU > 0 {
		guest.CPUPct = pctFloat(status.CPU)
	}
	if status.CPUs > 0 {
		guest.MaxCPU = status.CPUs
	}
	if status.MaxMem > 0 {
		guest.MemoryPct = pctInt64(status.Mem, status.MaxMem)
		guest.MemoryUsedBytes = status.Mem
		guest.MemoryTotalBytes = status.MaxMem
	}
	if status.MemHost > 0 {
		guest.MemoryHostBytes = status.MemHost
	}
	if status.MaxSwap > 0 {
		guest.SwapPct = pctInt64(status.Swap, status.MaxSwap)
		guest.SwapUsedBytes = status.Swap
		guest.SwapTotalBytes = status.MaxSwap
	}
	if status.MaxDisk > 0 {
		guest.DiskPct = pctInt64(status.Disk, status.MaxDisk)
		guest.DiskUsedBytes = status.Disk
		guest.DiskTotalBytes = status.MaxDisk
	}
	if status.Uptime > 0 {
		guest.UptimeSec = status.Uptime
	}
	if status.NetIn > 0 {
		guest.NetInBytes = status.NetIn
	}
	if status.NetOut > 0 {
		guest.NetOutBytes = status.NetOut
	}
	if status.DiskRead > 0 {
		guest.DiskReadBytes = status.DiskRead
	}
	if status.DiskWrite > 0 {
		guest.DiskWriteBytes = status.DiskWrite
	}
	if status.Tags != "" && guest.Tags == "" {
		guest.Tags = status.Tags
	}
	guest.PID = status.PID
	guest.QMPStatus = status.QMPStatus
	guest.RunningQEMU = status.RunningQEMU
	guest.HAManaged = status.HA.Managed != 0
	guest.PressureCPUSomePct = pctAny(status.PressureCPUSome)
	guest.PressureCPUFullPct = pctAny(status.PressureCPUFull)
	guest.PressureIOSomePct = pctAny(status.PressureIOSome)
	guest.PressureIOFullPct = pctAny(status.PressureIOFull)
	guest.PressureMemorySomePct = pctAny(status.PressureMemorySome)
	guest.PressureMemoryFullPct = pctAny(status.PressureMemoryFull)
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

func placeholderIPAddress(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	return text == "" || text == "dhcp" || strings.HasPrefix(text, "dhcp/")
}

func guestDisks(cfg map[string]any) []display.GuestDisk {
	disks := []display.GuestDisk{}
	for key, value := range cfg {
		if !isGuestDiskKey(key) {
			continue
		}
		text := stringValue(value)
		if text == "" {
			continue
		}
		parts := strings.Split(text, ",")
		volume := strings.TrimSpace(parts[0])
		options := parseOptions(parts[1:])
		storage := ""
		if split := strings.Index(volume, ":"); split > 0 {
			storage = volume[:split]
		}
		disks = append(disks, display.GuestDisk{
			Name:      key,
			Storage:   storage,
			Volume:    volume,
			Size:      options["size"],
			Format:    options["format"],
			Backup:    boolValue(options["backup"]),
			Replicate: boolValue(options["replicate"]),
			SSD:       boolValue(options["ssd"]),
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Name < disks[j].Name })
	if len(disks) > maxGuestDisks {
		return disks[:maxGuestDisks]
	}
	return disks
}

func guestNICs(cfg map[string]any) []display.GuestNIC {
	nics := []display.GuestNIC{}
	for key, value := range cfg {
		if !strings.HasPrefix(key, "net") {
			continue
		}
		text := stringValue(value)
		if text == "" {
			continue
		}
		parts := strings.Split(text, ",")
		options := parseOptions(parts)
		model := ""
		mac := ""
		for _, part := range parts {
			pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(pair) == 2 && strings.Contains(pair[1], ":") && pair[0] != "bridge" {
				model = pair[0]
				mac = pair[1]
				break
			}
		}
		nics = append(nics, display.GuestNIC{
			Name:     key,
			Model:    model,
			MAC:      mac,
			Bridge:   options["bridge"],
			Firewall: boolValue(options["firewall"]),
			Tag:      options["tag"],
			Rate:     options["rate"],
		})
	}
	sort.Slice(nics, func(i, j int) bool { return nics[i].Name < nics[j].Name })
	if len(nics) > maxGuestNICs {
		return nics[:maxGuestNICs]
	}
	return nics
}

func isGuestDiskKey(key string) bool {
	for _, prefix := range []string{"scsi", "virtio", "sata", "ide", "mp", "unused"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return key == "rootfs"
}

func parseOptions(parts []string) map[string]string {
	options := map[string]string{}
	for _, part := range parts {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 {
			options[pair[0]] = pair[1]
		}
	}
	return options
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

func agentIPAddresses(networks agentNetworkInterfaces) []string {
	seen := map[string]bool{}
	addresses := []string{}
	for _, iface := range networks.Result {
		if iface.Name == "lo" {
			continue
		}
		for _, ip := range iface.IPAddresses {
			if ip.IPAddress == "" || strings.HasPrefix(ip.IPAddress, "127.") || ip.IPAddress == "::1" {
				continue
			}
			address := ip.IPAddress
			if ip.Prefix > 0 {
				address += "/" + strconv.Itoa(ip.Prefix)
			}
			if seen[address] {
				continue
			}
			seen[address] = true
			addresses = append(addresses, address)
			if len(addresses) >= maxGuestIPs {
				return addresses
			}
		}
	}
	return addresses
}

func agentFilesystems(info agentFSInfo) []display.GuestFilesystem {
	filesystems := []display.GuestFilesystem{}
	for _, fs := range info.Result {
		filesystems = append(filesystems, display.GuestFilesystem{
			Name:       fs.Name,
			Mountpoint: fs.Mountpoint,
			Type:       fs.Type,
			UsedBytes:  fs.UsedBytes,
			TotalBytes: fs.TotalBytes,
		})
		if len(filesystems) >= maxGuestFilesystems {
			return filesystems
		}
	}
	return filesystems
}

func containerIPAddresses(interfaces []containerInterface) []string {
	seen := map[string]bool{}
	addresses := []string{}
	add := func(address string) {
		address = strings.TrimSpace(address)
		if address == "" || strings.HasPrefix(address, "127.") || address == "::1" || strings.HasPrefix(address, "fe80:") {
			return
		}
		if seen[address] {
			return
		}
		seen[address] = true
		addresses = append(addresses, address)
	}
	for _, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}
		for _, ip := range iface.IPAddresses {
			if ip.IPAddress == "" {
				continue
			}
			address := ip.IPAddress
			if prefix := intValue(ip.Prefix); prefix > 0 {
				address += "/" + strconv.Itoa(prefix)
			}
			add(address)
			if len(addresses) >= maxGuestIPs {
				return addresses
			}
		}
		add(iface.Inet)
		if len(addresses) >= maxGuestIPs {
			return addresses
		}
		add(iface.Inet6)
		if len(addresses) >= maxGuestIPs {
			return addresses
		}
	}
	return addresses
}

func nodeMetricTrends(client *Client, host *display.Host, points []rrdPoint) []display.MetricTrend {
	base := metricTrendBase{
		sourceID:     client.id,
		hostID:       host.ID,
		resourceType: "host",
		resourceName: host.Name,
		timeframe:    "hour",
	}
	return []display.MetricTrend{
		trend(base, "cpu_pct", "%", points, func(p rrdPoint) int { return pctFloat(p.CPU) }),
		trend(base, "memory_pct", "%", points, func(p rrdPoint) int {
			used := firstPositiveFloat(p.MemUsed, p.Mem)
			total := firstPositiveFloat(p.MemTotal, p.MaxMem)
			return pctFloatRatio(used, total)
		}),
		trend(base, "root_pct", "%", points, func(p rrdPoint) int { return pctFloatRatio(p.RootUsed, p.RootTotal) }),
		trend(base, "net_in_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.NetIn) }),
		trend(base, "net_out_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.NetOut) }),
		trend(base, "iowait_pct", "%", points, func(p rrdPoint) int { return pctFloat(p.IOWait) }),
	}
}

func guestMetricTrends(client *Client, guest display.Guest, points []rrdPoint) []display.MetricTrend {
	base := metricTrendBase{
		sourceID:     client.id,
		hostID:       guest.HostID,
		guestID:      guest.ID,
		resourceType: guest.Type,
		resourceName: guest.Name,
		timeframe:    "hour",
	}
	return []display.MetricTrend{
		trend(base, "cpu_pct", "%", points, func(p rrdPoint) int { return pctFloat(p.CPU) }),
		trend(base, "memory_pct", "%", points, func(p rrdPoint) int { return pctFloatRatio(p.Mem, p.MaxMem) }),
		trend(base, "disk_pct", "%", points, func(p rrdPoint) int { return pctFloatRatio(p.Disk, p.MaxDisk) }),
		trend(base, "net_in_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.NetIn) }),
		trend(base, "net_out_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.NetOut) }),
		trend(base, "disk_read_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.DiskRead) }),
		trend(base, "disk_write_kib_s", "KiB/s", points, func(p rrdPoint) int { return kibRate(p.DiskWrite) }),
	}
}

type metricTrendBase struct {
	sourceID     string
	hostID       string
	guestID      string
	resourceType string
	resourceName string
	timeframe    string
}

func trend(base metricTrendBase, metric string, unit string, points []rrdPoint, value func(rrdPoint) int) display.MetricTrend {
	values := trendValues(points, value)
	last := 0
	if len(values) > 0 {
		last = values[len(values)-1]
	}
	idParts := []string{base.sourceID, base.resourceType, base.resourceName, metric}
	return display.MetricTrend{
		ID:           strings.Join(idParts, "/"),
		SourceID:     base.sourceID,
		HostID:       base.hostID,
		GuestID:      base.guestID,
		ResourceType: base.resourceType,
		ResourceName: base.resourceName,
		Metric:       metric,
		Unit:         unit,
		Timeframe:    base.timeframe,
		Last:         last,
		Values:       values,
	}
}

func trendValues(points []rrdPoint, value func(rrdPoint) int) []int {
	if len(points) == 0 {
		return []int{}
	}
	samples := maxTrendSamples
	if len(points) < samples {
		samples = len(points)
	}
	values := make([]int, 0, samples)
	for i := 0; i < samples; i++ {
		idx := 0
		if samples > 1 {
			idx = int(math.Round(float64(i) * float64(len(points)-1) / float64(samples-1)))
		}
		values = append(values, value(points[idx]))
	}
	return values
}

func zfsDeviceCounts(detail zfsPoolDetail) (int, int) {
	devices := 0
	issues := 0
	var walk func(zfsPoolDetail)
	walk = func(node zfsPoolDetail) {
		if node.Leaf != 0 || len(node.Children) == 0 {
			if node.Name != "" {
				devices++
			}
			state := strings.ToUpper(strings.TrimSpace(node.State))
			if state != "" && state != "ONLINE" && state != "AVAIL" {
				issues++
			}
			if node.Read > 0 || node.Write > 0 || node.Checksum > 0 {
				issues++
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, child := range detail.Children {
		walk(child)
	}
	return devices, issues
}

func healthFromText(value string) display.Health {
	text := strings.ToLower(strings.TrimSpace(value))
	switch {
	case text == "", text == "unknown":
		return display.HealthUnknown
	case strings.Contains(text, "online"), strings.Contains(text, "active"), strings.Contains(text, "ok"), strings.Contains(text, "pass"), strings.Contains(text, "healthy"):
		return display.HealthOK
	case strings.Contains(text, "degraded"), strings.Contains(text, "warn"):
		return display.HealthWarning
	case strings.Contains(text, "fault"), strings.Contains(text, "fail"), strings.Contains(text, "error"), strings.Contains(text, "critical"), strings.Contains(text, "offline"):
		return display.HealthCritical
	default:
		return display.HealthUnknown
	}
}

func addClusterWarning(cluster *display.Cluster, message string) {
	for _, existing := range cluster.DataWarnings {
		if existing == message {
			return
		}
	}
	cluster.DataWarnings = append(cluster.DataWarnings, message)
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

func pctFloatRatio(used, total float64) int {
	if used <= 0 || total <= 0 {
		return 0
	}
	return clampPct(int(math.Round(used / total * 100)))
}

func pctAny(value any) int {
	switch v := value.(type) {
	case float64:
		if v > 1 {
			return clampPct(int(math.Round(v)))
		}
		return pctFloat(v)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if parsed > 1 {
			return clampPct(int(math.Round(parsed)))
		}
		return pctFloat(parsed)
	default:
		return pctFloatRatio(float64(int64Value(value)), 100)
	}
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func kibRate(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(math.Round(value / 1024))
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

func compactValue(value any) string {
	switch value.(type) {
	case map[string]any, []any, []string:
		data, err := json.Marshal(value)
		if err == nil {
			return string(data)
		}
	}
	return stringValue(value)
}

func addCapability(state *display.State, sourceID string, hostID string, guestID string, name string, endpoint string, err error) {
	if len(state.Capabilities) >= maxCapabilities {
		return
	}
	status := "ok"
	httpStatus := 200
	message := ""
	if err != nil {
		status = "error"
		httpStatus = 0
		message = err.Error()
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			httpStatus = apiErr.StatusCode
			switch apiErr.StatusCode {
			case http.StatusForbidden, http.StatusUnauthorized:
				status = "forbidden"
			case http.StatusNotFound:
				status = "not_found"
			case http.StatusNotImplemented:
				status = "not_available"
			default:
				if apiErr.StatusCode >= 500 {
					status = "not_available"
				}
			}
			message = fmt.Sprintf("HTTP %d", apiErr.StatusCode)
		}
	}
	id := sourceID + "/" + name + "/" + strings.Trim(strings.ReplaceAll(endpoint, "/api2/json/", ""), "/")
	state.Capabilities = append(state.Capabilities, display.Capability{
		ID:         id,
		SourceID:   sourceID,
		HostID:     hostID,
		GuestID:    guestID,
		Name:       name,
		Endpoint:   endpoint,
		Status:     status,
		HTTPStatus: httpStatus,
		Message:    message,
	})
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
