package proxmox

import (
	"encoding/json"
	"testing"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
)

func TestSummarizeGPUs(t *testing.T) {
	count, summary := summarizeGPUs([]pciDevice{
		{Class: "0x020000", VendorName: "Intel", DeviceName: "Ethernet Controller"},
		{Class: "0x030000", VendorName: "Intel", DeviceName: "UHD Graphics 600"},
		{Class: "0x030200", VendorName: "NVIDIA", DeviceName: "Tesla P4"},
	})
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if summary != "Intel UHD Graphics 600 +1" {
		t.Fatalf("summary = %q", summary)
	}
}

func TestGPUNameAvoidsDuplicateVendor(t *testing.T) {
	got := gpuName(pciDevice{VendorName: "Intel", DeviceName: "Intel Corporation Alder Lake-N Graphics"})
	if got != "Intel Corporation Alder Lake-N Graphics" {
		t.Fatalf("gpuName = %q", got)
	}
}

func TestSummarizeNetworkSkipsLoopback(t *testing.T) {
	total, active, primary := summarizeNetwork([]networkInterface{
		{Iface: "lo", Active: 1, Address: "127.0.0.1"},
		{Iface: "vmbr0", Active: 1, Address: "192.168.1.55"},
		{Iface: "eno1", Active: 0},
	})
	if total != 2 || active != 1 || primary != "192.168.1.55" {
		t.Fatalf("network summary = (%d, %d, %q)", total, active, primary)
	}
}

func TestApplyGuestConfig(t *testing.T) {
	guest := display.Guest{}
	applyGuestConfig(&guest, map[string]any{
		"cores":      float64(4),
		"memory":     "8192",
		"ostype":     "l26",
		"agent":      "enabled=1",
		"onboot":     "1",
		"protection": float64(1),
		"ipconfig0":  "ip=192.168.1.50/24,gw=192.168.1.1",
		"scsi0":      "local-lvm:vm-100-disk-0,size=50G,ssd=1,backup=1",
		"net0":       "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1,tag=20",
	})
	if guest.MaxCPU != 4 || guest.MemoryTotalBytes != 8192*1024*1024 {
		t.Fatalf("guest resources not applied: %#v", guest)
	}
	if !guest.AgentEnabled || !guest.OnBoot || !guest.Protection {
		t.Fatalf("guest flags not applied: %#v", guest)
	}
	if guest.OSType != "l26" || guest.IPAddress != "192.168.1.50/24" {
		t.Fatalf("guest identity config not applied: %#v", guest)
	}
	if len(guest.Disks) != 1 || guest.Disks[0].Storage != "local-lvm" || !guest.Disks[0].SSD || !guest.Disks[0].Backup {
		t.Fatalf("guest disks not applied: %#v", guest.Disks)
	}
	if len(guest.NICs) != 1 || guest.NICs[0].Bridge != "vmbr0" || guest.NICs[0].MAC != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("guest NICs not applied: %#v", guest.NICs)
	}
}

func TestAgentIPAddresses(t *testing.T) {
	got := agentIPAddresses(agentNetworkInterfaces{Result: []struct {
		Name            string `json:"name"`
		HardwareAddress string `json:"hardware-address"`
		IPAddresses     []struct {
			IPAddress     string `json:"ip-address"`
			IPAddressType string `json:"ip-address-type"`
			Prefix        int    `json:"prefix"`
		} `json:"ip-addresses"`
	}{
		{
			Name: "lo",
			IPAddresses: []struct {
				IPAddress     string `json:"ip-address"`
				IPAddressType string `json:"ip-address-type"`
				Prefix        int    `json:"prefix"`
			}{{IPAddress: "127.0.0.1", Prefix: 8}},
		},
		{
			Name: "eth0",
			IPAddresses: []struct {
				IPAddress     string `json:"ip-address"`
				IPAddressType string `json:"ip-address-type"`
				Prefix        int    `json:"prefix"`
			}{{IPAddress: "192.168.1.50", Prefix: 24}},
		},
	}})
	if len(got) != 1 || got[0] != "192.168.1.50/24" {
		t.Fatalf("agentIPAddresses = %#v", got)
	}
}

func TestZFSPoolDisplayAcceptsNumericDedup(t *testing.T) {
	var pools []zfsPool
	if err := json.Unmarshal([]byte(`[{"name":"datapool","health":"ONLINE","size":1000,"alloc":200,"free":800,"frag":3,"dedup":1}]`), &pools); err != nil {
		t.Fatalf("unmarshal zfs pools: %v", err)
	}
	host := display.Host{ID: "pve-1/pve", Name: "pve", Node: "pve"}
	got := zfsPoolDisplay(&Client{id: "pve-1"}, &host, pools[0])
	if got.Name != "datapool" || got.DedupRatio != "1" || got.Health != display.HealthOK {
		t.Fatalf("zfs pool display = %#v", got)
	}
}

func TestStorageItemSummary(t *testing.T) {
	storage := display.Storage{Name: "local", Node: "pve", HostID: "pve-1/pve", HostName: "pve"}
	item := storageItemDisplay(&Client{id: "pve-1"}, storage, storageContent{
		VolID:     "local:backup/vzdump-qemu-100.vma.zst",
		Content:   "backup",
		Format:    "vma.zst",
		Size:      1024,
		CTime:     1770000000,
		VMID:      float64(100),
		Protected: 1,
		Verification: map[string]any{
			"state": "ok",
		},
	})
	applyStorageItemSummary(&storage, item)
	if item.VMID != "100" || !item.Protected || item.VerificationState != "ok" {
		t.Fatalf("storage item = %#v", item)
	}
	if storage.ContentItems != 1 || storage.BackupCount != 1 {
		t.Fatalf("storage summary = %#v", storage)
	}
}

func TestAddCapabilityClassifiesForbidden(t *testing.T) {
	state := display.NewState()
	addCapability(&state, "pve-1", "pve-1/pve", "", "apt_updates", "/api2/json/nodes/pve/apt/update", &APIError{SourceID: "pve-1", StatusCode: 403})
	if len(state.Capabilities) != 1 {
		t.Fatalf("capabilities = %#v", state.Capabilities)
	}
	got := state.Capabilities[0]
	if got.Status != "forbidden" || got.HTTPStatus != 403 {
		t.Fatalf("capability = %#v", got)
	}
}

func TestContainerIPAddresses(t *testing.T) {
	got := containerIPAddresses([]containerInterface{
		{
			Name: "eth0",
			IPAddresses: []struct {
				IPAddress     string `json:"ip-address"`
				IPAddressType string `json:"ip-address-type"`
				Prefix        any    `json:"prefix"`
			}{
				{IPAddress: "192.168.1.20", Prefix: "24"},
				{IPAddress: "fe80::1", Prefix: "64"},
			},
		},
	})
	if len(got) != 1 || got[0] != "192.168.1.20/24" {
		t.Fatalf("containerIPAddresses = %#v", got)
	}
}
