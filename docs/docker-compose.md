# Docker Compose

From the repository root:

```bash
docker compose -f examples/docker-compose.yaml up -d
```

Open:

```text
http://localhost:8765/admin
```

The compose file uses the public image:

```yaml
image: vincenzoimp/proxmox-desk-display-bridge:latest
```

Before the Docker Hub image exists, build the same `/data`-based setup locally:

```bash
docker compose -f examples/docker-compose.local.yaml up -d --build
```

Configuration and secrets live in the Docker volume `proxmox-desk-display-data`. Updating is
therefore just:

```bash
docker compose -f examples/docker-compose.yaml pull
docker compose -f examples/docker-compose.yaml up -d
```

Early pre-release examples used the volume name `pve-desk-data`. Keep that
volume name in your local compose file or migrate it before switching names;
otherwise Docker will start with an empty configuration.

The first admin visit is open when no token exists. Set a display token and add
Proxmox sources from the browser. After that, log in with user `admin` and the
admin token if you configured one, otherwise the display token.

Check health:

```bash
curl http://localhost:8765/healthz
```

Read display state:

```bash
curl -H "Authorization: Bearer $DISPLAY_TOKEN" \
  http://localhost:8765/api/v1/display-state
```

## Legacy File Config

`config.yaml` plus `.env` is still supported for development and manual installs:

```bash
cp examples/config.example.yaml config.yaml
cp examples/.env.example .env
docker compose -f examples/docker-compose.legacy.yaml up -d --build
```

The admin UI can only persist changes when the container has a writable
`/data` volume.
