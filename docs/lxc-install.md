# Proxmox LXC Install

The first stable release should provide a small installer script. Until then, use Docker Compose inside a Debian LXC or run the Go binary directly.

Recommended LXC sizing:

- 1 vCPU
- 256-512 MB RAM
- 1-2 GB disk
- Debian 12

Manual binary flow:

```bash
useradd --system --home /opt/pve-desk-display --create-home pve-desk
install -m 0755 pve-desk-display /usr/local/bin/pve-desk-display
install -m 0640 config.yaml /etc/pve-desk-display.yaml
```

Create `/etc/systemd/system/pve-desk-display.service`:

```ini
[Unit]
Description=Proxmox Desk Display Bridge
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/pve-desk-display.env
ExecStart=/usr/local/bin/pve-desk-display --config /etc/pve-desk-display.yaml
Restart=on-failure
User=pve-desk
Group=pve-desk

[Install]
WantedBy=multi-user.target
```

Then:

```bash
systemctl daemon-reload
systemctl enable --now pve-desk-display
```
