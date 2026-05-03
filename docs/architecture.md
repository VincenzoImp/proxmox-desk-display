# Architecture

Proxmox Desk Display is intentionally split into two products that share one protocol.

## Bridge

The bridge is the data authority. It reads Proxmox APIs, applies auth/TLS/cache/alert rules, and emits a compact schema designed for small displays.

The bridge is also where future data providers belong. Pulse, Prometheus, Uptime Kuma, Docker, or NAS integrations should normalize into the same display state instead of changing the firmware protocol.

The bridge exposes two state views:

- `/api/v1/display-state`: compact firmware-safe payload.
- `/api/v1/full-state`: full Proxmox inventory with heavier details such as certificates, storage contents, RRD trends, Ceph summaries, and endpoint capability diagnostics.

## Firmware

The firmware is a display appliance. It handles Wi-Fi setup, bridge polling, local cache, buttons, and rendering.

It must not contain Proxmox-specific logic or Proxmox credentials.

## Contract

The contract between bridge and firmware is:

```http
GET /api/v1/display-state
```

This lets the firmware remain stable while the bridge evolves.

When a screen needs heavy details, the preferred next step is to add a
page-specific endpoint or query shape instead of making the ESP32 parse the full
inventory on every refresh.

## Why Not Direct LILYGO To Proxmox?

Direct mode is technically possible, but worse for a public-quality project:

- Proxmox tokens would live on the device;
- TLS and certificate rotation are harder on embedded firmware;
- firmware updates would be needed for API changes;
- debugging is much harder than opening a local bridge page.

The bridge-first design is a product-quality choice, not a technical limitation.
