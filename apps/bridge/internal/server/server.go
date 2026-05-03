package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
	appruntime "github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/runtime"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/store"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/version"
)

type Server struct {
	cfg   config.Config
	cache *store.Cache
	admin *appruntime.Manager
	mock  bool
	mux   *http.ServeMux
}

func New(cfg config.Config, cache *store.Cache, mock bool, admin *appruntime.Manager) http.Handler {
	s := &Server{
		cfg:   cfg,
		cache: cache,
		admin: admin,
		mock:  mock,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.index)
	s.mux.Handle("/admin", s.adminAuth(http.HandlerFunc(s.adminIndex)))
	s.mux.Handle("/admin/settings", s.adminAuth(http.HandlerFunc(s.adminSettings)))
	s.mux.Handle("/admin/proxmox", s.adminAuth(http.HandlerFunc(s.adminProxmox)))
	s.mux.Handle("/admin/proxmox/delete", s.adminAuth(http.HandlerFunc(s.adminDeleteProxmox)))
	s.mux.Handle("/admin/refresh", s.adminAuth(http.HandlerFunc(s.adminRefresh)))
	s.mux.HandleFunc("/healthz", s.healthz)
	s.mux.Handle("/api/v1/display-state", s.auth(http.HandlerFunc(s.displayState)))
	s.mux.Handle("/api/v1/detail-state", s.auth(http.HandlerFunc(s.detailState)))
	s.mux.Handle("/api/v1/full-state", s.auth(http.HandlerFunc(s.fullState)))
	s.mux.Handle("/api/v1/debug", s.auth(http.HandlerFunc(s.debug)))
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": version.Version,
		"mock":    s.mock,
	})
}

func (s *Server) displayState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.cache.State()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, display.CompactForDisplay(state))
}

func (s *Server) detailState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.cache.State()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, display.DetailForDisplay(state))
}

func (s *Server) fullState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.cache.State()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) debug(w http.ResponseWriter, _ *http.Request) {
	state, err := s.cache.State()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": err.Error(),
			"cache": s.cache.Metadata(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": version.Version,
		"mock":    s.mock,
		"cache":   s.cache.Metadata(),
		"state":   state,
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.currentConfig().Server.DisplayToken()
		if s.mock && token == "" {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.TrimPrefix(header, "Bearer ") != token {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.currentConfig().Server.AdminToken()
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") &&
			strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == token {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if ok && user == "admin" && pass == token {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="Proxmox Desk Display Admin"`)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin authentication required"})
	})
}

func (s *Server) currentConfig() config.Config {
	if s.admin != nil {
		return s.admin.Config()
	}
	return s.cfg
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	state, _ := s.cache.State()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = indexTemplate.Execute(w, map[string]any{
		"Version": version.Version,
		"Mock":    s.mock,
		"State":   state,
		"Cache":   s.cache.Metadata(),
	})
}

func (s *Server) adminIndex(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		http.NotFound(w, r)
		return
	}
	state, _ := s.cache.State()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminTemplate.Execute(w, map[string]any{
		"Version": version.Version,
		"State":   state,
		"Cache":   s.cache.Metadata(),
		"Admin":   s.admin.Snapshot(),
		"Message": r.URL.Query().Get("message"),
		"Error":   r.URL.Query().Get("error"),
	})
}

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectAdmin(w, r, "", err)
		return
	}
	err := s.admin.UpdateServer(r.Context(), appruntime.ServerUpdate{
		PollIntervalSeconds: atoiDefault(r.FormValue("poll_interval_seconds"), 10),
		StaleAfterSeconds:   atoiDefault(r.FormValue("stale_after_seconds"), 45),
		DisplayToken:        r.FormValue("display_token"),
		AdminToken:          r.FormValue("admin_token"),
		MemoryWarningPct:    atoiDefault(r.FormValue("memory_warning_pct"), 85),
		MemoryCriticalPct:   atoiDefault(r.FormValue("memory_critical_pct"), 95),
		StorageWarningPct:   atoiDefault(r.FormValue("storage_warning_pct"), 80),
		StorageCriticalPct:  atoiDefault(r.FormValue("storage_critical_pct"), 90),
	})
	redirectAdmin(w, r, "settings saved", err)
}

func (s *Server) adminProxmox(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectAdmin(w, r, "", err)
		return
	}
	err := s.admin.UpsertSource(r.Context(), appruntime.SourceUpdate{
		ID:          r.FormValue("id"),
		Name:        r.FormValue("name"),
		BaseURL:     r.FormValue("base_url"),
		Token:       r.FormValue("token"),
		TLSMode:     r.FormValue("tls_mode"),
		Fingerprint: r.FormValue("fingerprint"),
		CAFile:      r.FormValue("ca_file"),
	})
	redirectAdmin(w, r, "source saved", err)
}

func (s *Server) adminDeleteProxmox(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectAdmin(w, r, "", err)
		return
	}
	err := s.admin.DeleteSource(r.Context(), r.FormValue("id"))
	redirectAdmin(w, r, "source deleted", err)
}

func (s *Server) adminRefresh(w http.ResponseWriter, r *http.Request) {
	if s.admin == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.admin.Refresh(r.Context())
	redirectAdmin(w, r, "refresh requested", err)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func redirectAdmin(w http.ResponseWriter, r *http.Request, message string, err error) {
	target := "/admin"
	if err != nil {
		target += "?error=" + urlQuery(err.Error())
	} else if message != "" {
		target += "?message=" + urlQuery(message)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func atoiDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func urlQuery(value string) string {
	return url.QueryEscape(value)
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Proxmox Desk Display Bridge</title>
  <style>
    body { font-family: system-ui, -apple-system, Segoe UI, sans-serif; margin: 2rem; max-width: 980px; color: #17202a; }
    code, pre { background: #f4f6f8; border-radius: 6px; }
    pre { padding: 1rem; overflow: auto; }
    .ok { color: #0f7b45; }
    .warning { color: #9a6700; }
    .critical { color: #b42318; }
    table { border-collapse: collapse; width: 100%; margin-top: 1rem; }
    th, td { border-bottom: 1px solid #d9dee5; padding: .55rem; text-align: left; }
  </style>
</head>
<body>
  <h1>Proxmox Desk Display Bridge</h1>
  <p>Version {{.Version}}{{if .Mock}} · mock mode{{end}}</p>
  <p>Display endpoint: <code>/api/v1/display-state</code> · detail endpoint: <code>/api/v1/detail-state</code> · full inventory: <code>/api/v1/full-state</code></p>
  <h2>Summary</h2>
  <p class="{{.State.Summary.Health}}">Health: {{.State.Summary.Health}}</p>
  <p>Hosts: {{.State.Summary.HostsOnline}}/{{.State.Summary.HostsTotal}} · Guests running: {{.State.Summary.GuestsRunning}} · Alerts: {{.State.Summary.Alerts}}</p>
  <h2>Hosts</h2>
  <table>
    <thead><tr><th>Name</th><th>Online</th><th>CPU</th><th>Memory</th><th>Storage</th><th>Guests</th></tr></thead>
    <tbody>
    {{range .State.Hosts}}
      <tr><td>{{.Name}}</td><td>{{.Online}}</td><td>{{.CPUPct}}%</td><td>{{.MemoryPct}}%</td><td>{{.StoragePct}}%</td><td>{{.GuestsRunning}} run / {{.GuestsStopped}} stop</td></tr>
    {{else}}
      <tr><td colspan="6">No hosts collected yet.</td></tr>
    {{end}}
    </tbody>
  </table>
  <h2>Cache</h2>
  <pre>{{printf "%#v" .Cache}}</pre>
</body>
</html>`))

var adminTemplate = template.Must(template.New("admin").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Proxmox Desk Display Admin</title>
  <style>
    :root { color-scheme: light dark; --bg:#f6f7f9; --panel:#fff; --text:#17202a; --muted:#667085; --line:#d9dee5; --accent:#1455d9; --danger:#b42318; --ok:#0f7b45; }
    @media (prefers-color-scheme: dark) { :root { --bg:#111418; --panel:#171b21; --text:#eef2f6; --muted:#9aa4b2; --line:#303741; --accent:#7aa7ff; --danger:#ff7b72; --ok:#56d68a; } }
    * { box-sizing: border-box; }
    body { margin:0; background:var(--bg); color:var(--text); font-family:system-ui,-apple-system,Segoe UI,sans-serif; }
    header { padding:18px 24px; border-bottom:1px solid var(--line); background:var(--panel); display:flex; gap:16px; align-items:center; justify-content:space-between; }
    main { max-width:1120px; margin:0 auto; padding:24px; display:grid; gap:18px; }
    h1 { font-size:20px; margin:0; }
    h2 { font-size:16px; margin:0 0 14px; }
    .meta { color:var(--muted); font-size:13px; }
    .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(260px,1fr)); gap:18px; }
    section { background:var(--panel); border:1px solid var(--line); border-radius:8px; padding:18px; }
    label { display:block; font-size:13px; font-weight:650; margin:12px 0 5px; color:var(--text); }
    input, select { width:100%; padding:10px 11px; border:1px solid var(--line); border-radius:6px; background:transparent; color:var(--text); font:inherit; }
    button { margin-top:14px; padding:10px 13px; border:0; border-radius:6px; background:var(--accent); color:white; font-weight:700; cursor:pointer; }
    button.secondary { background:transparent; color:var(--accent); border:1px solid var(--line); }
    button.danger { background:var(--danger); }
    table { width:100%; border-collapse:collapse; }
    th, td { text-align:left; padding:9px 7px; border-bottom:1px solid var(--line); font-size:14px; vertical-align:top; }
    th { color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:.04em; }
    code { background:rgba(127,127,127,.12); padding:2px 5px; border-radius:4px; }
    .pill { display:inline-block; padding:3px 7px; border-radius:999px; border:1px solid var(--line); color:var(--muted); font-size:12px; }
    .ok { color:var(--ok); }
    .danger-text { color:var(--danger); }
    .notice { border-left:4px solid var(--accent); }
    .error { border-left:4px solid var(--danger); }
    .actions { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>Proxmox Desk Display Admin</h1>
      <div class="meta">Version {{.Version}} · config <code>{{.Admin.ConfigPath}}</code></div>
    </div>
    <form method="post" action="/admin/refresh"><button class="secondary" type="submit">Refresh Now</button></form>
  </header>
  <main>
    {{if .Message}}<section class="notice">{{.Message}}</section>{{end}}
    {{if .Error}}<section class="error danger-text">{{.Error}}</section>{{end}}

    <div class="grid">
      <section>
        <h2>Runtime</h2>
        <table>
          <tbody>
            <tr><th>Health</th><td class="{{.State.Summary.Health}}">{{.State.Summary.Health}}</td></tr>
            <tr><th>Hosts</th><td>{{.State.Summary.HostsOnline}}/{{.State.Summary.HostsTotal}}</td></tr>
            <tr><th>Guests</th><td>{{.State.Summary.GuestsRunning}} running · {{.State.Summary.GuestsStopped}} stopped</td></tr>
            <tr><th>Alerts</th><td>{{.State.Summary.Alerts}}</td></tr>
            <tr><th>Poll</th><td>{{index .Cache "poll_interval"}} · stale after {{index .Cache "stale_after"}}</td></tr>
            <tr><th>Tokens</th><td>display {{if .Admin.DisplayTokenSet}}<span class="ok">set</span>{{else}}<span class="danger-text">missing</span>{{end}} · admin {{if .Admin.AdminConfigured}}<span class="ok">set</span>{{else}}<span class="danger-text">open setup</span>{{end}}</td></tr>
          </tbody>
        </table>
      </section>

      <section>
        <h2>Bridge Settings</h2>
        <form method="post" action="/admin/settings">
          <label>Refresh interval seconds</label>
          <input name="poll_interval_seconds" type="number" min="5" value="{{.Admin.Server.PollIntervalSeconds}}">
          <label>Stale after seconds</label>
          <input name="stale_after_seconds" type="number" min="5" value="{{.Admin.Server.StaleAfterSeconds}}">
          <label>Display token</label>
          <input name="display_token" type="password" placeholder="{{if .Admin.DisplayTokenSet}}saved, leave blank to keep{{else}}required for displays{{end}}">
          <label>Admin token</label>
          <input name="admin_token" type="password" placeholder="{{if .Admin.AdminConfigured}}saved, leave blank to keep{{else}}optional; display token will be used if blank{{end}}">
          <label>Memory warning / critical</label>
          <div class="actions">
            <input name="memory_warning_pct" type="number" min="1" max="100" value="{{.Admin.Alerts.MemoryWarningPct}}">
            <input name="memory_critical_pct" type="number" min="1" max="100" value="{{.Admin.Alerts.MemoryCriticalPct}}">
          </div>
          <label>Storage warning / critical</label>
          <div class="actions">
            <input name="storage_warning_pct" type="number" min="1" max="100" value="{{.Admin.Alerts.StorageWarningPct}}">
            <input name="storage_critical_pct" type="number" min="1" max="100" value="{{.Admin.Alerts.StorageCriticalPct}}">
          </div>
          <button type="submit">Save Settings</button>
        </form>
      </section>
    </div>

    <section>
      <h2>Proxmox Sources</h2>
      <table>
        <thead><tr><th>ID</th><th>Name</th><th>URL</th><th>TLS</th><th>Token</th><th></th></tr></thead>
        <tbody>
        {{range .Admin.Sources}}
          <tr>
            <td><code>{{.ID}}</code></td>
            <td>{{.Name}}</td>
            <td>{{.BaseURL}}</td>
            <td>{{.TLSMode}}</td>
            <td>{{if .TokenSet}}<span class="ok">saved</span>{{else}}<span class="danger-text">missing</span>{{end}}</td>
            <td>
              <form method="post" action="/admin/proxmox/delete">
                <input type="hidden" name="id" value="{{.ID}}">
                <button class="danger" type="submit">Delete</button>
              </form>
            </td>
          </tr>
        {{else}}
          <tr><td colspan="6">No Proxmox source configured.</td></tr>
        {{end}}
        </tbody>
      </table>
    </section>

    <section>
      <h2>Add Or Update Proxmox</h2>
      <form method="post" action="/admin/proxmox">
        <div class="grid">
          <div>
            <label>ID</label>
            <input name="id" placeholder="zimablade">
            <label>Name</label>
            <input name="name" placeholder="Zimablade">
            <label>Base URL</label>
            <input name="base_url" placeholder="https://192.168.1.56:8006">
          </div>
          <div>
            <label>API token</label>
            <input name="token" type="password" placeholder="PVEAPIToken=monitor@pve!desk=...">
            <label>TLS mode</label>
            <select name="tls_mode">
              <option value="fingerprint">fingerprint</option>
              <option value="system">system</option>
              <option value="insecure">insecure</option>
              <option value="ca_file">ca_file</option>
            </select>
            <label>SHA256 fingerprint</label>
            <input name="fingerprint" placeholder="SHA256:...">
            <label>CA file</label>
            <input name="ca_file" placeholder="/data/ca.pem">
          </div>
        </div>
        <button type="submit">Save Source</button>
      </form>
    </section>
  </main>
</body>
</html>`))
