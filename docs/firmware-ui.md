# Firmware UI

The LILYGO T-Display-S3 UI is organized as drill-down screens.

Button A changes screen. Long press A goes backward.

Button B changes the selected item on overview/list/detail screens:

- `PROXMOX`, `HOSTS`, `HOST`, and `SYSTEM`: next host;
- `STORAGE`: next storage;
- `GUESTS` and `DETAIL`: next VM/LXC;
- `ALERTS`: next alert;
- other screens: refresh now.

Long press B cycles brightness.

## Screens

1. `PROXMOX`: global status counts, selected host focus metrics, selected host storage pressure, load, and the top active alert.
2. `HOSTS`: paged compact host list with CPU/RAM/root storage.
3. `HOST`: selected host metric detail with CPU, RAM, rootfs, uptime, load, and guest count.
4. `SYSTEM`: selected host system detail with CPU model, GPU summary when Proxmox exposes it, PVE version, and kernel.
5. `STORAGE`: selected storage detail with plugin type, status, content, shared flag, used/total and alert color.
6. `GUESTS`: paged compact VM/LXC list with VMID, CPU/RAM/disk.
7. `DETAIL`: selected VM/LXC detail with VMID, CPU cores, RAM, disk, uptime in the context line, network, disk IO, and first tag when present.
8. `ALERTS`: paged active warnings and critical issues.
9. `DEVICE`: Wi-Fi, device IP, bridge status and firmware version.

The UI is intentionally glance-first. Global alerts appear as compact counters outside the alert screen; individual detail screens use the selected item's own health. Full raw data belongs in the bridge debug/API, while the device shows prioritized status and drill-down details.
