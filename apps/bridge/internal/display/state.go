package display

import "time"

const Schema = "pve-desk-display.v1"

type Health string

const (
	HealthOK       Health = "ok"
	HealthWarning  Health = "warning"
	HealthCritical Health = "critical"
	HealthUnknown  Health = "unknown"
)

type State struct {
	Schema      string    `json:"schema"`
	GeneratedAt time.Time `json:"generated_at"`
	Stale       bool      `json:"stale"`
	Summary     Summary   `json:"summary"`
	Hosts       []Host    `json:"hosts"`
	Guests      []Guest   `json:"guests"`
	Alerts      []Alert   `json:"alerts"`
}

type Summary struct {
	Health        Health `json:"health"`
	HostsOnline   int    `json:"hosts_online"`
	HostsTotal    int    `json:"hosts_total"`
	GuestsRunning int    `json:"guests_running"`
	GuestsStopped int    `json:"guests_stopped"`
	Alerts        int    `json:"alerts"`
}

type Host struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	SourceID      string  `json:"source_id"`
	Node          string  `json:"node"`
	Online        bool    `json:"online"`
	CPUPct        int     `json:"cpu_pct"`
	MemoryPct     int     `json:"memory_pct"`
	StoragePct    int     `json:"storage_pct"`
	UptimeSec     int64   `json:"uptime_sec"`
	GuestsRunning int     `json:"guests_running"`
	GuestsStopped int     `json:"guests_stopped"`
	Health        Health  `json:"health"`
	Error         *string `json:"error"`
}

type Guest struct {
	ID        string `json:"id"`
	VMID      string `json:"vmid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	HostID    string `json:"host_id"`
	SourceID  string `json:"source_id"`
	Status    string `json:"status"`
	CPUPct    int    `json:"cpu_pct"`
	MemoryPct int    `json:"memory_pct"`
	Pinned    bool   `json:"pinned"`
	Expected  string `json:"expected,omitempty"`
	Health    Health `json:"health"`
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
	if s.Guests == nil {
		s.Guests = []Guest{}
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
