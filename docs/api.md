# Bridge API

The firmware-facing API is intentionally small and stable.

## Authentication

Display endpoints require:

```http
Authorization: Bearer <display_token>
```

The token is configured in the bridge through `server.display_token_env`.

## Health

```http
GET /healthz
```

Response:

```json
{
  "ok": true,
  "version": "0.1.0"
}
```

## Display State

```http
GET /api/v1/display-state
Authorization: Bearer <display_token>
```

Response:

```json
{
  "schema": "pve-desk-display.v1",
  "generated_at": "2026-05-03T12:00:00Z",
  "stale": false,
  "summary": {
    "health": "ok",
    "hosts_online": 2,
    "hosts_total": 2,
    "guests_running": 12,
    "guests_stopped": 3,
    "alerts": 0
  },
  "hosts": [],
  "guests": [],
  "alerts": []
}
```

`stale` means the bridge is serving cached data older than the configured freshness window.

Guest objects include both utilization and allocated resources:

```json
{
  "id": "pve-56/100",
  "vmid": "100",
  "name": "crafty-controller",
  "type": "lxc",
  "host_id": "pve-56/pve",
  "host_name": "zimablade / pve",
  "status": "running",
  "cpu_pct": 0,
  "max_cpu": 4,
  "memory_pct": 13,
  "memory_used_bytes": 1105723392,
  "memory_total_bytes": 8589934592,
  "disk_pct": 32,
  "disk_used_bytes": 10565300224,
  "disk_total_bytes": 33501757440,
  "uptime_sec": 5889692,
  "net_in_bytes": 4243614872,
  "net_out_bytes": 6101255841,
  "disk_read_bytes": 50119139328,
  "disk_write_bytes": 215348330496
}
```

## Debug

```http
GET /api/v1/debug
Authorization: Bearer <display_token>
```

Returns the same display state plus bridge metadata. It is intended for local troubleshooting.
