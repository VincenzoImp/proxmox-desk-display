# Proxmox Desk Display

Physical status display for Proxmox homelabs, starting with the LILYGO T-Display-S3.

The project turns a small ESP32-S3 display into an always-on appliance that shows whether your Proxmox lab is healthy without opening a browser dashboard. It is intentionally not a Grafana, Pulse, Netdata, or Zabbix replacement. It is a glanceable physical companion for the state that matters most.

## Status

Early MVP scaffold. The repository contains:

- a Go bridge that reads Proxmox APIs and exposes a compact display JSON API;
- a bridge admin UI for adding/removing Proxmox sources, tokens, polling, and alert thresholds;
- PlatformIO firmware for the LILYGO T-Display-S3;
- Docker Compose examples and setup documentation;
- mock mode for development without Proxmox.

## Architecture

```text
Proxmox host/cluster A     Proxmox host/cluster B
          |                         |
          | HTTPS API :8006         |
          v                         v
+------------------------------------------------+
| Bridge                                         |
| Go binary / Docker / LXC                       |
| auth, TLS, cache, alerts, normalization        |
+------------------------------------------------+
                    |
                    | HTTP JSON
                    v
+------------------------------------------------+
| LILYGO T-Display-S3                            |
| Wi-Fi, captive portal, UI, buttons             |
+------------------------------------------------+
```

The firmware never stores a Proxmox API token. It only knows the bridge URL and a display token.

## Quick Start

### 1. Start the Bridge

The public Docker image stores editable configuration in a persistent volume:

```bash
docker compose -f examples/docker-compose.yaml up -d
```

For local development before the Docker Hub image is published:

```bash
docker compose -f examples/docker-compose.local.yaml up -d --build
```

Open:

```text
http://localhost:8765/admin
```

On first setup the admin page is open. Set a display token, add one or more
Proxmox sources, then save. After a display token exists, the admin page accepts
HTTP basic auth with user `admin` and the admin token if configured, otherwise
the display token.

Create one read-only token per Proxmox install. See [docs/proxmox-token.md](docs/proxmox-token.md).
See [docs/admin-ui.md](docs/admin-ui.md) for the admin UI details and
[docs/docker-hub.md](docs/docker-hub.md) for publishing.

For a demo without Proxmox:

```bash
docker compose -f examples/docker-compose.yaml run --rm --service-ports bridge --mock
```

### 3. Flash the Firmware

Build and upload from `firmware/t-display-s3` with PlatformIO:

```bash
pio run -t upload --upload-port /dev/cu.usbmodem1101
```

On first boot the device creates the Wi-Fi network `Proxmox-Desk-Setup`. Join it, open the captive portal, and enter Wi-Fi plus bridge URL.

## Hardware Support

Tier 1:

- LILYGO T-Display-S3

Future board support should be added through explicit board profiles. This project does not promise generic support for every ESP32 display.

## Repository Layout

```text
apps/bridge/          Go bridge and Proxmox provider
firmware/t-display-s3 PlatformIO firmware
docs/                 Setup and troubleshooting
examples/             Config and Docker Compose examples
site/                 Web flasher placeholder
```

## API Contract

The firmware reads:

```http
GET /api/v1/display-state
Authorization: Bearer <display_token>
```

It also reads `/api/v1/detail-state` for bounded detail pages. The response
schema is versioned as `proxmox-desk-display.v1`. See [docs/api.md](docs/api.md).

## License

MIT. See [LICENSE](LICENSE).
