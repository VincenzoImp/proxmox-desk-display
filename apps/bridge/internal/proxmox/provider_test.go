package proxmox

import "testing"

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
