# Bridge Admin UI

Open:

```text
http://localhost:8765/admin
```

The admin UI owns bridge-side configuration:

- display token used by the LILYGO firmware;
- optional admin token for the web UI;
- Proxmox sources and API tokens;
- Proxmox polling interval and stale timeout;
- memory and storage alert thresholds.

The firmware owns physical display settings such as Wi-Fi credentials and
brightness.

## Authentication

On first run, when no display/admin token exists, `/admin` is open so the bridge
can be configured.

After a token exists, use HTTP basic auth:

```text
user: admin
password: <admin token, or display token when no admin token is configured>
```

## Storage

The Docker image writes persistent data under `/data`:

```text
/data/config.yaml
/data/secrets.yaml
```

`config.yaml` contains non-secret settings. `secrets.yaml` contains the display
token, admin token, and Proxmox API tokens and is written with owner-only
permissions inside the container.

## Proxmox Sources

Each source needs:

- stable ID, for example `zimablade`;
- display name;
- base URL, for example `https://192.168.1.56:8006`;
- API token;
- TLS mode.

Recommended TLS mode is `fingerprint`, using the SHA256 fingerprint of the
Proxmox certificate. `system`, `ca_file`, and `insecure` are available for
specific deployments.

Use `Detect Fingerprint` to read the certificate fingerprint directly from the
Proxmox URL. Use `Test Connection` to verify URL, TLS, and API token before or
after saving. The test calls the Proxmox `/api2/json/version` endpoint and does
not store changes unless you press `Save Source`.

To update an existing source, submit the same ID again. Leave the token field
blank to keep the saved token.

## Safety

Admin form posts reject cross-origin `Origin` or `Referer` headers. For public
or remote access, still put the bridge behind a trusted HTTPS reverse proxy and
use a dedicated admin token that differs from the display token.
