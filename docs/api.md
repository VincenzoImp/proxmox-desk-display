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
  "clusters": [],
  "hosts": [],
  "storages": [],
  "disks": [],
  "networks": [],
  "services": [],
  "zfs_pools": [],
  "guests": [],
  "snapshots": [],
  "tasks": [],
  "backup_jobs": [],
  "replications": [],
  "ha_resources": [],
  "updates": [],
  "repositories": [],
  "subscriptions": [],
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
  "storage_max_pct": 99,
  "storage_max_name": "datapool",
  "uptime_sec": 5890516,
  "load_avg": ["0.40", "0.48", "0.49"],
  "pve_version": "pve-manager/9.1.2/...",
  "kernel_version": "Linux 6.17.2-2-pve ...",
  "primary_address": "192.168.1.55",
  "network_active": 2,
  "network_total": 2,
  "services_running": 20,
  "services_failed": 0,
  "services_total": 23,
  "disk_count": 8,
  "disk_issues": 0,
  "failed_tasks_24h": 0,
  "last_backup_status": "OK",
  "last_backup_age_sec": 7200
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

Disk objects are collected from Proxmox node disk inventory:

```json
{
  "id": "pve-55/pve/nvme0n1",
  "name": "nvme0n1",
  "host_name": "jonsboN4 / pve",
  "model": "CT1000P310SSD8",
  "serial": "2534524DCF93",
  "type": "nvme",
  "used_by": "BIOS boot",
  "size_bytes": 1000204886016,
  "smart_health": "PASSED",
  "wearout_pct": 100,
  "health": "ok"
}
```

Network, service, ZFS, repository, update, subscription, backup, replication, HA, and snapshot collections are also emitted when Proxmox exposes them to the configured API token. If an endpoint is unavailable because of permissions, missing subsystem configuration, or Proxmox edition/version differences, the affected host or cluster includes a `data_warnings` entry instead of failing the whole refresh.

Guest objects include both utilization, allocated resources, parsed config devices, and QEMU Guest Agent data when the agent is enabled and reachable:

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
  "disk_write_bytes": 215348330496,
  "os_type": "l26",
  "ip_address": "192.168.1.50/24",
  "ip_addresses": ["192.168.1.50/24"],
  "agent_enabled": true,
  "agent_available": true,
  "agent_hostname": "docker",
  "agent_os": "Debian GNU/Linux",
  "onboot": true,
  "protection": false,
  "template": false,
  "unprivileged": false,
  "disks": [
    {
      "name": "scsi0",
      "storage": "datapool",
      "volume": "datapool:vm-100-disk-0",
      "size": "50G",
      "backup": true
    }
  ],
  "nics": [
    {
      "name": "net0",
      "model": "virtio",
      "bridge": "vmbr0",
      "mac": "AA:BB:CC:DD:EE:FF",
      "firewall": true
    }
  ],
  "filesystems": [
    {
      "name": "sda1",
      "mountpoint": "/",
      "type": "ext4",
      "used_bytes": 1234,
      "total_bytes": 5678
    }
  ]
}
```

Task objects represent recent Proxmox node tasks, newest first. The bridge keeps the payload bounded for small displays.

```json
{
  "id": "UPID:pve:...",
  "host_name": "zimablade / pve",
  "type": "vncshell",
  "user": "root@pam",
  "status": "OK",
  "target": "100 crafty-controller",
  "started_at": 1777818485,
  "started_age_sec": 14147,
  "ended_at": 1777819880,
  "duration_sec": 1395,
  "health": "ok"
}
```

## Debug

```http
GET /api/v1/debug
Authorization: Bearer <display_token>
```

Returns the same display state plus bridge metadata. It is intended for local troubleshooting.
