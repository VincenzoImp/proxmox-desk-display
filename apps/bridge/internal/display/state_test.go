package display

import "testing"

func TestFinalizeSummary(t *testing.T) {
	state := NewState()
	state.Hosts = []Host{
		{ID: "a", Online: true},
		{ID: "b", Online: true},
	}
	state.Guests = []Guest{
		{ID: "100", Status: "running"},
		{ID: "101", Status: "stopped"},
	}
	state.Alerts = []Alert{
		{ID: "a", Severity: HealthWarning},
	}

	got := Finalize(state)
	if got.Summary.HostsTotal != 2 || got.Summary.HostsOnline != 2 {
		t.Fatalf("unexpected host summary: %#v", got.Summary)
	}
	if got.Summary.GuestsRunning != 1 || got.Summary.GuestsStopped != 1 {
		t.Fatalf("unexpected guest summary: %#v", got.Summary)
	}
	if got.Summary.Health != HealthWarning {
		t.Fatalf("health = %q, want warning", got.Summary.Health)
	}
}
