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
  "storages": [],
  "guests": [],
  "alerts": []
}
```

`stale` means the bridge is serving cached data older than the configured freshness window.

Host objects include node-level detail for the Host Detail screen:

```json
{
  "id": "pve-56/pve",
  "name": "zimablade / pve",
  "online": true,
  "cpu_pct": 3,
  "max_cpu": 4,
  "cpu_model": "Intel(R) Celeron(R) CPU N3450 @ 1.10GHz",
  "gpu_count": 1,
  "gpu_summary": "Intel UHD Graphics 500",
  "memory_pct": 27,
  "memory_used_bytes": 4435075072,
  "memory_total_bytes": 16609353728,
  "storage_pct": 20,
  "storage_used_bytes": 5787279360,
  "storage_total_bytes": 28971167744,
  "uptime_sec": 5890516,
  "load_avg": ["0.40", "0.48", "0.49"],
  "pve_version": "pve-manager/9.1.2/...",
  "kernel_version": "Linux 6.17.2-2-pve ..."
}
```

Storage objects power the Storage screen:

```json
{
  "id": "pve-55/pve/datapool",
  "name": "datapool",
  "host_name": "jonsboN4 / pve",
  "status": "available",
  "plugin_type": "zfspool",
  "content": "images",
  "shared": false,
  "disk_pct": 99,
  "disk_used_bytes": 7759663476736,
  "disk_total_bytes": 7834020347904,
  "health": "critical"
}
```

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
