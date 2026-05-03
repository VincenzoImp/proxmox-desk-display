package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/config"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/display"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/store"
	"github.com/proxmox-desk-display/proxmox-desk-display/apps/bridge/internal/version"
)

type Server struct {
	cfg   config.Config
	cache *store.Cache
	mock  bool
	mux   *http.ServeMux
}

func New(cfg config.Config, cache *store.Cache, mock bool) http.Handler {
	s := &Server{
		cfg:   cfg,
		cache: cache,
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
		token := s.cfg.Server.DisplayToken()
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
