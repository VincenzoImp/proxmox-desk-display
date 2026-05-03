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

## Debug

```http
GET /api/v1/debug
Authorization: Bearer <display_token>
```

Returns the same display state plus bridge metadata. It is intended for local troubleshooting.
