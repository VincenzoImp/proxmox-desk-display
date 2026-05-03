# Firmware UI

The LILYGO T-Display-S3 UI is organized as drill-down screens.

Button A changes screen. Long press A goes backward.

Button B changes the selected item on list/detail screens:

- `HOSTS` and `HOST`: next host;
- `STORAGE`: next storage;
- `GUESTS` and `DETAIL`: next VM/LXC;
- other screens: refresh now.

Long press B cycles brightness.

## Screens

1. `PROXMOX`: global status, host count, guest count, top host cards.
2. `HOSTS`: compact host list with CPU/RAM/root storage.
3. `HOST`: selected host detail with CPU model, cores, RAM, rootfs, uptime, load, PVE/kernel versions.
4. `STORAGE`: selected storage detail with plugin type, status, content, shared flag, used/total and alert color.
5. `GUESTS`: compact VM/LXC list with CPU/RAM/disk.
6. `DETAIL`: selected VM/LXC detail with CPU cores, RAM, disk, uptime, network and disk IO.
7. `ALERTS`: active warnings and critical issues.
8. `DEVICE`: Wi-Fi, device IP, bridge status and firmware version.

The UI is intentionally glance-first. Full raw data belongs in the bridge debug/API, while the device shows prioritized status and drill-down details.
