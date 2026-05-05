package display

import (
	"sort"
	"time"
)

const Schema = "proxmox-desk-display.v1"

type Health string

const (
	HealthOK       Health = "ok"
	HealthWarning  Health = "warning"
	HealthCritical Health = "critical"
	HealthUnknown  Health = "unknown"
)

type State struct {
	Schema         string          `json:"schema"`
	GeneratedAt    time.Time       `json:"generated_at"`
	Stale          bool            `json:"stale"`
	Summary        Summary         `json:"summary"`
	Clusters       []Cluster       `json:"clusters"`
	Hosts          []Host          `json:"hosts"`
	Storages       []Storage       `json:"storages"`
	Disks          []Disk          `json:"disks"`
	Networks       []Network       `json:"networks"`
	Services       []Service       `json:"services"`
	ZFSPools       []ZFSPool       `json:"zfs_pools"`
	Guests         []Guest         `json:"guests"`
	Snapshots      []Snapshot      `json:"snapshots"`
	Tasks          []Task          `json:"tasks"`
	BackupJobs     []BackupJob     `json:"backup_jobs"`
	Replications   []Replication   `json:"replications"`
	HAResources    []HAResource    `json:"ha_resources"`
	Certificates   []Certificate   `json:"certificates"`
	StorageItems   []StorageItem   `json:"storage_items"`
	MetricTrends   []MetricTrend   `json:"metric_trends"`
	ClusterOptions []ClusterOption `json:"cluster_options"`
	CephClusters   []CephCluster   `json:"ceph_clusters"`
	Capabilities   []Capability    `json:"capabilities"`
	Updates        []Update        `json:"updates"`
	Repositories   []Repository    `json:"repositories"`
	Subscriptions  []Subscription  `json:"subscriptions"`
	Alerts         []Alert         `json:"alerts"`
}

const (
	MaxDetailZFSPools       = 24
	MaxDetailCertificates   = 24
	MaxDetailStorageItems   = 48
	MaxDetailMetricTrends   = 64
	MaxDetailClusterOptions = 32
	MaxDetailCephClusters   = 8
	MaxDetailCapabilities   = 64
)

type DetailState struct {
	Schema         string          `json:"schema"`
	GeneratedAt    time.Time       `json:"generated_at"`
	Stale          bool            `json:"stale"`
	ZFSPools       []ZFSPool       `json:"zfs_pools"`
	Certificates   []Certificate   `json:"certificates"`
	StorageItems   []StorageItem   `json:"storage_items"`
	MetricTrends   []MetricTrend   `json:"metric_trends"`
	ClusterOptions []ClusterOption `json:"cluster_options"`
	CephClusters   []CephCluster   `json:"ceph_clusters"`
	Capabilities   []Capability    `json:"capabilities"`
}

type Summary struct {
	Health        Health `json:"health"`
	HostsOnline   int    `json:"hosts_online"`
	HostsTotal    int    `json:"hosts_total"`
	GuestsRunning int    `json:"guests_running"`
	GuestsStopped int    `json:"guests_stopped"`
	Alerts        int    `json:"alerts"`
	Updates       int    `json:"updates"`
}

type Cluster struct {
	ID            string   `json:"id"`
	SourceID      string   `json:"source_id"`
	Name          string   `json:"name,omitempty"`
	Version       int      `json:"version,omitempty"`
	Quorate       bool     `json:"quorate"`
	NodesOnline   int      `json:"nodes_online"`
	NodesTotal    int      `json:"nodes_total"`
	NodesExpected int      `json:"nodes_expected,omitempty"`
	Health        Health   `json:"health"`
	DataWarnings  []string `json:"data_warnings,omitempty"`
}

type Host struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	SourceID             string   `json:"source_id"`
	Node                 string   `json:"node"`
	Online               bool     `json:"online"`
	CPUPct               int      `json:"cpu_pct"`
	MaxCPU               int      `json:"max_cpu"`
	CPUModel             string   `json:"cpu_model,omitempty"`
	GPUCount             int      `json:"gpu_count,omitempty"`
	GPUSummary           string   `json:"gpu_summary,omitempty"`
	MemoryPct            int      `json:"memory_pct"`
	MemoryUsedBytes      int64    `json:"memory_used_bytes"`
	MemoryTotalBytes     int64    `json:"memory_total_bytes"`
	MemoryAvailableBytes int64    `json:"memory_available_bytes,omitempty"`
	SwapPct              int      `json:"swap_pct,omitempty"`
	SwapUsedBytes        int64    `json:"swap_used_bytes,omitempty"`
	SwapTotalBytes       int64    `json:"swap_total_bytes,omitempty"`
	IOWaitPct            int      `json:"iowait_pct,omitempty"`
	KSMSharedBytes       int64    `json:"ksm_shared_bytes,omitempty"`
	StoragePct           int      `json:"storage_pct"`
	StorageUsedBytes     int64    `json:"storage_used_bytes"`
	StorageTotalBytes    int64    `json:"storage_total_bytes"`
	StorageMaxPct        int      `json:"storage_max_pct"`
	StorageMaxName       string   `json:"storage_max_name,omitempty"`
	UptimeSec            int64    `json:"uptime_sec"`
	LoadAvg              []string `json:"load_avg,omitempty"`
	PVEVersion           string   `json:"pve_version,omitempty"`
	KernelVersion        string   `json:"kernel_version,omitempty"`
	PrimaryAddress       string   `json:"primary_address,omitempty"`
	NetworkActive        int      `json:"network_active"`
	NetworkTotal         int      `json:"network_total"`
	ServicesRunning      int      `json:"services_running"`
	ServicesFailed       int      `json:"services_failed"`
	ServicesTotal        int      `json:"services_total"`
	UpdatesAvailable     int      `json:"updates_available"`
	SubscriptionStatus   string   `json:"subscription_status,omitempty"`
	DiskCount            int      `json:"disk_count"`
	DiskIssues           int      `json:"disk_issues"`
	FailedTasks24h       int      `json:"failed_tasks_24h"`
	LastBackupStatus     string   `json:"last_backup_status,omitempty"`
	LastBackupAgeSec     int64    `json:"last_backup_age_sec,omitempty"`
	DataWarnings         []string `json:"data_warnings,omitempty"`
	GuestsRunning        int      `json:"guests_running"`
	GuestsStopped        int      `json:"guests_stopped"`
	Health               Health   `json:"health"`
	Error                *string  `json:"error"`
}

type Storage struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SourceID       string `json:"source_id"`
	HostID         string `json:"host_id"`
	HostName       string `json:"host_name"`
	Node           string `json:"node"`
	Status         string `json:"status"`
	PluginType     string `json:"plugin_type"`
	Content        string `json:"content"`
	Path           string `json:"path,omitempty"`
	Pool           string `json:"pool,omitempty"`
	Mountpoint     string `json:"mountpoint,omitempty"`
	Shared         bool   `json:"shared"`
	DiskPct        int    `json:"disk_pct"`
	DiskUsedBytes  int64  `json:"disk_used_bytes"`
	DiskTotalBytes int64  `json:"disk_total_bytes"`
	ContentItems   int    `json:"content_items,omitempty"`
	BackupCount    int    `json:"backup_count,omitempty"`
	ISOCount       int    `json:"iso_count,omitempty"`
	TemplateCount  int    `json:"template_count,omitempty"`
	ImageCount     int    `json:"image_count,omitempty"`
	RootdirCount   int    `json:"rootdir_count,omitempty"`
	Health         Health `json:"health"`
}

type Disk struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostID      string `json:"host_id"`
	HostName    string `json:"host_name"`
	Node        string `json:"node"`
	Name        string `json:"name"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	Type        string `json:"type,omitempty"`
	UsedBy      string `json:"used_by,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	SMARTHealth string `json:"smart_health,omitempty"`
	WearoutPct  int    `json:"wearout_pct,omitempty"`
	Health      Health `json:"health"`
}

type Network struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostID      string `json:"host_id"`
	HostName    string `json:"host_name"`
	Node        string `json:"node"`
	Iface       string `json:"iface"`
	Type        string `json:"type,omitempty"`
	Active      bool   `json:"active"`
	Autostart   bool   `json:"autostart"`
	Method      string `json:"method,omitempty"`
	Address     string `json:"address,omitempty"`
	CIDR        string `json:"cidr,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	BridgePorts string `json:"bridge_ports,omitempty"`
	Slaves      string `json:"slaves,omitempty"`
	VLANAware   bool   `json:"vlan_aware,omitempty"`
	Comments    string `json:"comments,omitempty"`
	Health      Health `json:"health"`
}

type Service struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostID      string `json:"host_id"`
	HostName    string `json:"host_name"`
	Node        string `json:"node"`
	Name        string `json:"name"`
	State       string `json:"state,omitempty"`
	UnitState   string `json:"unit_state,omitempty"`
	Description string `json:"description,omitempty"`
	Health      Health `json:"health"`
}

type ZFSPool struct {
	ID               string `json:"id"`
	SourceID         string `json:"source_id"`
	HostID           string `json:"host_id"`
	HostName         string `json:"host_name"`
	Node             string `json:"node"`
	Name             string `json:"name"`
	HealthText       string `json:"health_text,omitempty"`
	Status           string `json:"status,omitempty"`
	State            string `json:"state,omitempty"`
	Scan             string `json:"scan,omitempty"`
	Errors           string `json:"errors,omitempty"`
	DeviceCount      int    `json:"device_count,omitempty"`
	IssueCount       int    `json:"issue_count,omitempty"`
	SizeBytes        int64  `json:"size_bytes"`
	AllocatedBytes   int64  `json:"allocated_bytes"`
	FreeBytes        int64  `json:"free_bytes"`
	FragmentationPct int    `json:"fragmentation_pct,omitempty"`
	DedupRatio       string `json:"dedup_ratio,omitempty"`
	Health           Health `json:"health"`
}

type Guest struct {
	ID                    string            `json:"id"`
	VMID                  string            `json:"vmid"`
	Name                  string            `json:"name"`
	Type                  string            `json:"type"`
	HostID                string            `json:"host_id"`
	HostName              string            `json:"host_name"`
	SourceID              string            `json:"source_id"`
	Status                string            `json:"status"`
	CPUPct                int               `json:"cpu_pct"`
	MaxCPU                int               `json:"max_cpu"`
	MemoryPct             int               `json:"memory_pct"`
	MemoryUsedBytes       int64             `json:"memory_used_bytes"`
	MemoryTotalBytes      int64             `json:"memory_total_bytes"`
	MemoryHostBytes       int64             `json:"memory_host_bytes,omitempty"`
	SwapPct               int               `json:"swap_pct,omitempty"`
	SwapUsedBytes         int64             `json:"swap_used_bytes,omitempty"`
	SwapTotalBytes        int64             `json:"swap_total_bytes,omitempty"`
	DiskPct               int               `json:"disk_pct"`
	DiskUsedBytes         int64             `json:"disk_used_bytes"`
	DiskTotalBytes        int64             `json:"disk_total_bytes"`
	UptimeSec             int64             `json:"uptime_sec"`
	NetInBytes            int64             `json:"net_in_bytes"`
	NetOutBytes           int64             `json:"net_out_bytes"`
	DiskReadBytes         int64             `json:"disk_read_bytes"`
	DiskWriteBytes        int64             `json:"disk_write_bytes"`
	Tags                  string            `json:"tags,omitempty"`
	OSType                string            `json:"os_type,omitempty"`
	IPAddress             string            `json:"ip_address,omitempty"`
	IPAddresses           []string          `json:"ip_addresses,omitempty"`
	AgentEnabled          bool              `json:"agent_enabled"`
	AgentAvailable        bool              `json:"agent_available"`
	AgentHostname         string            `json:"agent_hostname,omitempty"`
	AgentOS               string            `json:"agent_os,omitempty"`
	AgentVersion          string            `json:"agent_version,omitempty"`
	AgentCommandCount     int               `json:"agent_command_count,omitempty"`
	AgentWarning          string            `json:"agent_warning,omitempty"`
	PID                   int               `json:"pid,omitempty"`
	QMPStatus             string            `json:"qmp_status,omitempty"`
	RunningQEMU           string            `json:"running_qemu,omitempty"`
	HAManaged             bool              `json:"ha_managed,omitempty"`
	PressureCPUSomePct    int               `json:"pressure_cpu_some_pct,omitempty"`
	PressureCPUFullPct    int               `json:"pressure_cpu_full_pct,omitempty"`
	PressureIOSomePct     int               `json:"pressure_io_some_pct,omitempty"`
	PressureIOFullPct     int               `json:"pressure_io_full_pct,omitempty"`
	PressureMemorySomePct int               `json:"pressure_memory_some_pct,omitempty"`
	PressureMemoryFullPct int               `json:"pressure_memory_full_pct,omitempty"`
	OnBoot                bool              `json:"onboot"`
	Protection            bool              `json:"protection"`
	Template              bool              `json:"template"`
	Unprivileged          bool              `json:"unprivileged"`
	CPUType               string            `json:"cpu_type,omitempty"`
	BIOS                  string            `json:"bios,omitempty"`
	Machine               string            `json:"machine,omitempty"`
	BootOrder             string            `json:"boot_order,omitempty"`
	Startup               string            `json:"startup,omitempty"`
	Nameserver            string            `json:"nameserver,omitempty"`
	SearchDomain          string            `json:"search_domain,omitempty"`
	Features              string            `json:"features,omitempty"`
	Disks                 []GuestDisk       `json:"disks,omitempty"`
	NICs                  []GuestNIC        `json:"nics,omitempty"`
	Filesystems           []GuestFilesystem `json:"filesystems,omitempty"`
	ConfigWarning         string            `json:"config_warning,omitempty"`
	Pinned                bool              `json:"pinned"`
	Expected              string            `json:"expected,omitempty"`
	Health                Health            `json:"health"`
}

type GuestDisk struct {
	Name      string `json:"name"`
	Storage   string `json:"storage,omitempty"`
	Volume    string `json:"volume,omitempty"`
	Size      string `json:"size,omitempty"`
	Format    string `json:"format,omitempty"`
	Backup    bool   `json:"backup,omitempty"`
	Replicate bool   `json:"replicate,omitempty"`
	SSD       bool   `json:"ssd,omitempty"`
}

type GuestNIC struct {
	Name     string `json:"name"`
	Model    string `json:"model,omitempty"`
	MAC      string `json:"mac,omitempty"`
	Bridge   string `json:"bridge,omitempty"`
	Firewall bool   `json:"firewall,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Rate     string `json:"rate,omitempty"`
}

type GuestFilesystem struct {
	Name       string `json:"name"`
	Mountpoint string `json:"mountpoint,omitempty"`
	Type       string `json:"type,omitempty"`
	UsedBytes  int64  `json:"used_bytes"`
	TotalBytes int64  `json:"total_bytes"`
}

type Snapshot struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostID      string `json:"host_id"`
	HostName    string `json:"host_name"`
	GuestID     string `json:"guest_id"`
	GuestName   string `json:"guest_name"`
	VMID        string `json:"vmid"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SnapTime    int64  `json:"snap_time,omitempty"`
	Parent      string `json:"parent,omitempty"`
	VMState     bool   `json:"vmstate"`
	Health      Health `json:"health"`
}

type Task struct {
	ID            string `json:"id"`
	SourceID      string `json:"source_id"`
	HostID        string `json:"host_id"`
	HostName      string `json:"host_name"`
	Node          string `json:"node"`
	Type          string `json:"type"`
	User          string `json:"user,omitempty"`
	Status        string `json:"status"`
	Target        string `json:"target,omitempty"`
	VMID          string `json:"vmid,omitempty"`
	GuestName     string `json:"guest_name,omitempty"`
	StartedAt     int64  `json:"started_at"`
	StartedAgeSec int64  `json:"started_age_sec,omitempty"`
	EndedAt       int64  `json:"ended_at,omitempty"`
	DurationSec   int64  `json:"duration_sec,omitempty"`
	Health        Health `json:"health"`
}

type BackupJob struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Storage  string `json:"storage,omitempty"`
	Schedule string `json:"schedule,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Enabled  bool   `json:"enabled"`
	All      bool   `json:"all"`
	VMIDs    string `json:"vmids,omitempty"`
	Compress string `json:"compress,omitempty"`
	MailTo   string `json:"mailto,omitempty"`
	Health   Health `json:"health"`
}

type Replication struct {
	ID             string `json:"id"`
	SourceID       string `json:"source_id"`
	GuestID        string `json:"guest_id,omitempty"`
	GuestName      string `json:"guest_name,omitempty"`
	VMID           string `json:"vmid,omitempty"`
	SourceNode     string `json:"source_node,omitempty"`
	TargetNode     string `json:"target_node,omitempty"`
	Schedule       string `json:"schedule,omitempty"`
	Rate           int64  `json:"rate,omitempty"`
	Enabled        bool   `json:"enabled"`
	Status         string `json:"status,omitempty"`
	State          string `json:"state,omitempty"`
	DurationSec    int64  `json:"duration_sec,omitempty"`
	FailCount      int    `json:"fail_count,omitempty"`
	LastSync       int64  `json:"last_sync,omitempty"`
	NextSync       int64  `json:"next_sync,omitempty"`
	LastSyncStatus string `json:"last_sync_status,omitempty"`
	Error          string `json:"error,omitempty"`
	Health         Health `json:"health"`
}

type HAResource struct {
	ID           string `json:"id"`
	SourceID     string `json:"source_id"`
	SID          string `json:"sid"`
	Type         string `json:"type,omitempty"`
	State        string `json:"state,omitempty"`
	RequestState string `json:"request_state,omitempty"`
	Group        string `json:"group,omitempty"`
	Node         string `json:"node,omitempty"`
	MaxRestart   int    `json:"max_restart,omitempty"`
	MaxRelocate  int    `json:"max_relocate,omitempty"`
	Health       Health `json:"health"`
}

type Certificate struct {
	ID            string   `json:"id"`
	SourceID      string   `json:"source_id"`
	HostID        string   `json:"host_id"`
	HostName      string   `json:"host_name"`
	Node          string   `json:"node"`
	Filename      string   `json:"filename"`
	Subject       string   `json:"subject,omitempty"`
	Issuer        string   `json:"issuer,omitempty"`
	SANs          []string `json:"sans,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	PublicKeyType string   `json:"public_key_type,omitempty"`
	PublicKeyBits int      `json:"public_key_bits,omitempty"`
	NotBefore     int64    `json:"not_before,omitempty"`
	NotAfter      int64    `json:"not_after,omitempty"`
	DaysRemaining int      `json:"days_remaining,omitempty"`
	Health        Health   `json:"health"`
}

type StorageItem struct {
	ID                string `json:"id"`
	SourceID          string `json:"source_id"`
	HostID            string `json:"host_id"`
	HostName          string `json:"host_name"`
	Node              string `json:"node"`
	Storage           string `json:"storage"`
	Content           string `json:"content"`
	VolID             string `json:"volid"`
	VMID              string `json:"vmid,omitempty"`
	Format            string `json:"format,omitempty"`
	SizeBytes         int64  `json:"size_bytes,omitempty"`
	CreatedAt         int64  `json:"created_at,omitempty"`
	Notes             string `json:"notes,omitempty"`
	Protected         bool   `json:"protected,omitempty"`
	VerificationState string `json:"verification_state,omitempty"`
	Health            Health `json:"health"`
}

type MetricTrend struct {
	ID           string `json:"id"`
	SourceID     string `json:"source_id"`
	HostID       string `json:"host_id,omitempty"`
	GuestID      string `json:"guest_id,omitempty"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Metric       string `json:"metric"`
	Unit         string `json:"unit"`
	Timeframe    string `json:"timeframe"`
	Last         int    `json:"last"`
	Values       []int  `json:"values"`
}

type ClusterOption struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

type CephCluster struct {
	ID             string `json:"id"`
	SourceID       string `json:"source_id"`
	FSID           string `json:"fsid,omitempty"`
	HealthText     string `json:"health_text,omitempty"`
	TotalBytes     int64  `json:"total_bytes,omitempty"`
	UsedBytes      int64  `json:"used_bytes,omitempty"`
	AvailableBytes int64  `json:"available_bytes,omitempty"`
	UsagePct       int    `json:"usage_pct,omitempty"`
	OSDs           int    `json:"osds,omitempty"`
	OSDsUp         int    `json:"osds_up,omitempty"`
	OSDsIn         int    `json:"osds_in,omitempty"`
	PGs            int    `json:"pgs,omitempty"`
	Health         Health `json:"health"`
}

type Capability struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	HostID     string `json:"host_id,omitempty"`
	GuestID    string `json:"guest_id,omitempty"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Message    string `json:"message,omitempty"`
}

type Update struct {
	ID               string `json:"id"`
	SourceID         string `json:"source_id"`
	HostID           string `json:"host_id"`
	HostName         string `json:"host_name"`
	Node             string `json:"node"`
	Package          string `json:"package"`
	Title            string `json:"title,omitempty"`
	CurrentVersion   string `json:"current_version,omitempty"`
	CandidateVersion string `json:"candidate_version,omitempty"`
	Origin           string `json:"origin,omitempty"`
	Section          string `json:"section,omitempty"`
	Priority         string `json:"priority,omitempty"`
	Health           Health `json:"health"`
}

type Repository struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	HostID     string `json:"host_id"`
	HostName   string `json:"host_name"`
	Node       string `json:"node"`
	File       string `json:"file,omitempty"`
	Types      string `json:"types,omitempty"`
	URIs       string `json:"uris,omitempty"`
	Suites     string `json:"suites,omitempty"`
	Components string `json:"components,omitempty"`
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status,omitempty"`
	Warning    string `json:"warning,omitempty"`
	Health     Health `json:"health"`
}

type Subscription struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostID      string `json:"host_id"`
	HostName    string `json:"host_name"`
	Node        string `json:"node"`
	Status      string `json:"status,omitempty"`
	Level       string `json:"level,omitempty"`
	ProductName string `json:"product_name,omitempty"`
	ServerID    string `json:"server_id,omitempty"`
	NextDueDate string `json:"next_due_date,omitempty"`
	Health      Health `json:"health"`
}

type Alert struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id,omitempty"`
	HostID   string `json:"host_id,omitempty"`
	GuestID  string `json:"guest_id,omitempty"`
	Severity Health `json:"severity"`
	Title    string `json:"title"`
	Message  string `json:"message"`
}

func NewState() State {
	return State{
		Schema:      Schema,
		GeneratedAt: time.Now().UTC(),
		Summary: Summary{
			Health: HealthOK,
		},
	}
}

func Finalize(s State) State {
	s.Schema = Schema
	if s.Hosts == nil {
		s.Hosts = []Host{}
	}
	if s.Clusters == nil {
		s.Clusters = []Cluster{}
	}
	if s.Storages == nil {
		s.Storages = []Storage{}
	}
	if s.Disks == nil {
		s.Disks = []Disk{}
	}
	if s.Networks == nil {
		s.Networks = []Network{}
	}
	if s.Services == nil {
		s.Services = []Service{}
	}
	if s.ZFSPools == nil {
		s.ZFSPools = []ZFSPool{}
	}
	if s.Guests == nil {
		s.Guests = []Guest{}
	}
	if s.Snapshots == nil {
		s.Snapshots = []Snapshot{}
	}
	if s.Tasks == nil {
		s.Tasks = []Task{}
	}
	if s.BackupJobs == nil {
		s.BackupJobs = []BackupJob{}
	}
	if s.Replications == nil {
		s.Replications = []Replication{}
	}
	if s.HAResources == nil {
		s.HAResources = []HAResource{}
	}
	if s.Certificates == nil {
		s.Certificates = []Certificate{}
	}
	if s.StorageItems == nil {
		s.StorageItems = []StorageItem{}
	}
	if s.MetricTrends == nil {
		s.MetricTrends = []MetricTrend{}
	}
	if s.ClusterOptions == nil {
		s.ClusterOptions = []ClusterOption{}
	}
	if s.CephClusters == nil {
		s.CephClusters = []CephCluster{}
	}
	if s.Capabilities == nil {
		s.Capabilities = []Capability{}
	}
	if s.Updates == nil {
		s.Updates = []Update{}
	}
	if s.Repositories == nil {
		s.Repositories = []Repository{}
	}
	if s.Subscriptions == nil {
		s.Subscriptions = []Subscription{}
	}
	if s.Alerts == nil {
		s.Alerts = []Alert{}
	}
	s.Summary = Summary{
		Health:     HealthOK,
		HostsTotal: len(s.Hosts),
		Alerts:     len(s.Alerts),
	}
	for _, h := range s.Hosts {
		if h.Online {
			s.Summary.HostsOnline++
		}
	}
	for _, g := range s.Guests {
		if g.Status == "running" {
			s.Summary.GuestsRunning++
		} else {
			s.Summary.GuestsStopped++
		}
	}
	s.Summary.Updates = len(s.Updates)
	for _, a := range s.Alerts {
		if a.Severity == HealthCritical {
			s.Summary.Health = HealthCritical
			return s
		}
		if a.Severity == HealthWarning {
			s.Summary.Health = HealthWarning
		}
	}
	if s.Summary.HostsTotal > 0 && s.Summary.HostsOnline < s.Summary.HostsTotal {
		s.Summary.Health = HealthCritical
	}
	return s
}

func CompactForDisplay(s State) State {
	s.Certificates = []Certificate{}
	s.StorageItems = []StorageItem{}
	s.MetricTrends = []MetricTrend{}
	s.ClusterOptions = []ClusterOption{}
	s.CephClusters = []CephCluster{}
	s.Capabilities = []Capability{}
	return Finalize(s)
}

func DetailForDisplay(s State) DetailState {
	return DetailState{
		Schema:         s.Schema,
		GeneratedAt:    s.GeneratedAt,
		Stale:          s.Stale,
		ZFSPools:       firstN(s.ZFSPools, MaxDetailZFSPools),
		Certificates:   firstN(s.Certificates, MaxDetailCertificates),
		StorageItems:   firstN(s.StorageItems, MaxDetailStorageItems),
		MetricTrends:   firstN(s.MetricTrends, MaxDetailMetricTrends),
		ClusterOptions: firstN(s.ClusterOptions, MaxDetailClusterOptions),
		CephClusters:   firstN(s.CephClusters, MaxDetailCephClusters),
		Capabilities:   prioritizedCapabilities(s.Capabilities, MaxDetailCapabilities),
	}
}

func firstN[T any](values []T, max int) []T {
	if values == nil {
		return []T{}
	}
	if len(values) <= max {
		return values
	}
	return values[:max]
}

func prioritizedCapabilities(values []Capability, max int) []Capability {
	if values == nil {
		return []Capability{}
	}
	caps := append([]Capability(nil), values...)
	sort.SliceStable(caps, func(i, j int) bool {
		left := capabilityRank(caps[i].Status)
		right := capabilityRank(caps[j].Status)
		if left != right {
			return left > right
		}
		if caps[i].SourceID != caps[j].SourceID {
			return caps[i].SourceID < caps[j].SourceID
		}
		if caps[i].Name != caps[j].Name {
			return caps[i].Name < caps[j].Name
		}
		return caps[i].Endpoint < caps[j].Endpoint
	})
	return firstN(caps, max)
}

func capabilityRank(status string) int {
	switch status {
	case "error":
		return 5
	case "forbidden", "unauthorized":
		return 4
	case "not_available", "not_found":
		return 3
	case "ok":
		return 1
	default:
		return 2
	}
}
