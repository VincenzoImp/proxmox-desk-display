# Firmware UI

The LILYGO T-Display-S3 UI is organized as drill-down screens.

Button A changes screen. Long press A goes backward.

Button B changes the selected item on overview/list/detail screens:

- `PROXMOX`, `HOSTS`, `HOST`, and `SYSTEM`: next host;
- `OPS`: next host;
- `DISKS`: next physical disk;
- `STORAGE`: next storage;
- `GUESTS`, `DETAIL`, and `GCFG`: next VM/LXC;
- `TASKS`: next recent Proxmox task;
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
7. `STORAGE`: selected storage detail with plugin type, status, content, shared flag, used/total and alert color.
8. `GUESTS`: paged compact VM/LXC list with VMID, CPU/RAM/disk.
9. `DETAIL`: selected VM/LXC runtime detail with VMID, CPU cores, RAM, disk, uptime in the context line, network, disk IO, and first tag when present.
10. `GCFG`: selected VM/LXC configuration detail with OS type, configured IP, agent/onboot/protection/template/unprivileged flags, pinned expectation, tag, or config warning.
11. `TASKS`: selected recent Proxmox task with status, type, target, host, user, age, duration, and full status text.
12. `ALERTS`: paged active warnings and critical issues.
13. `DEVICE`: Wi-Fi, device IP, bridge status and firmware version.

The UI is intentionally glance-first. Global alerts appear as compact counters outside the alert screen; individual detail screens use the selected item's own health. Full raw data belongs in the bridge debug/API, while the device shows prioritized status and drill-down details.
