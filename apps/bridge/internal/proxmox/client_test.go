package proxmox

import "testing"

func TestParseFingerprint(t *testing.T) {
	got, err := parseFingerprint("SHA256:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseFingerprintRejectsInvalid(t *testing.T) {
	if _, err := parseFingerprint("SHA256:bad"); err == nil {
		t.Fatal("expected invalid fingerprint error")
	}
}
