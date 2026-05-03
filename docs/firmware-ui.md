# Firmware UI

The LILYGO T-Display-S3 UI is organized as drill-down screens.

Button A changes screen. Long press A goes backward.

Button B changes the selected item on overview/list/detail screens:

- `PROXMOX`, `HOSTS`, `HOST`, and `SYSTEM`: next host;
- `OPS`: next host;
- `DISKS`: next physical disk;
- `STORAGE`: next storage;
- `CONTENT`: next storage content item;
- `ZFS`: next ZFS pool;
- `CEPH`: next Ceph cluster;
- `GUESTS`, `DETAIL`, and `GCFG`: next VM/LXC;
- `TREND`: next RRD trend;
- `TASKS`: next recent Proxmox task;
- `CERTS`: next certificate;
- `CAPS`: next endpoint capability diagnostic;
- `ALERTS`: next alert;
- other screens: refresh now.

Long press B cycles brightness.

## Screens

1. `PROXMOX`: global status counts, selected host focus metrics, selected host storage pressure, load, and the top active alert.
2. `HOSTS`: paged compact host list with CPU/RAM/max storage pressure.
3. `HOST`: selected host metric detail with CPU, RAM, max storage pressure, uptime, load, and guest count.
4. `SYSTEM`: selected host system detail with CPU model, GPU summary when Proxmox exposes it, PVE version, and kernel.
5. `OPS`: selected host operational detail with network count/IP, service count, disk issues, failed tasks, last backup task, and collector warnings.
6. `DISKS`: selected physical disk inventory with model, host, size, usage, SMART health, serial, and wearout value when Proxmox exposes it.
7. `STORAGE`: selected storage detail with plugin type, status, content item counts, path/pool, used/total and alert color.
8. `CONTENT`: selected storage item with volume ID, host, size, format, VMID/protection, and verification state when Proxmox exposes it.
9. `ZFS`: selected ZFS pool with used/total, fragmentation, device count, issue count, scan and error text.
10. `CEPH`: selected Ceph summary with usage, available capacity, OSD in/up counts, PG count, and health text when Ceph is configured.
11. `GUESTS`: paged compact VM/LXC list with VMID, CPU/RAM/disk.
12. `DETAIL`: selected VM/LXC runtime detail with VMID, CPU cores, RAM, disk, uptime in the context line, network, disk IO, and first tag when present.
13. `GCFG`: selected VM/LXC configuration detail with OS/IP, agent availability/version, QMP/QEMU/HA state, pressure metrics, pinned expectation, tag, or config warning.
14. `TREND`: selected one-hour RRD trend with last value and sparkline.
15. `TASKS`: selected recent Proxmox task with status, type, target, host, user, age, duration, and full status text.
16. `CERTS`: selected node certificate with filename, host, subject, issuer, days remaining, and expiry health.
17. `CAPS`: selected endpoint capability diagnostic, sorted so permission and availability problems appear before healthy endpoints.
18. `ALERTS`: paged active warnings and critical issues.
19. `DEVICE`: Wi-Fi, device IP, bridge status, detail endpoint status, and firmware version.

The UI is intentionally glance-first. Global alerts appear as compact counters outside the alert screen; individual detail screens use the selected item's own health. Raw, unbounded data belongs in the bridge debug/API, while the device consumes `/api/v1/display-state` plus bounded `/api/v1/detail-state` and shows prioritized drill-down details.
