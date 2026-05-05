# Proxmox LXC Install

The first stable release should provide a small installer script. Until then, use Docker Compose inside a Debian LXC or run the Go binary directly.

Recommended LXC sizing:

- 1 vCPU
- 256-512 MB RAM
- 1-2 GB disk
- Debian 12

Manual binary flow:

```bash
useradd --system --home /opt/proxmox-desk-display-bridge --create-home proxmox-desk
install -m 0755 proxmox-desk-display-bridge /usr/local/bin/proxmox-desk-display-bridge
install -m 0640 config.yaml /etc/proxmox-desk-display-bridge.yaml
```

Create `/etc/systemd/system/proxmox-desk-display-bridge.service`:

```ini
[Unit]
Description=Proxmox Desk Display Bridge
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/proxmox-desk-display-bridge.env
ExecStart=/usr/local/bin/proxmox-desk-display-bridge --config /etc/proxmox-desk-display-bridge.yaml
Restart=on-failure
User=proxmox-desk
Group=proxmox-desk

[Install]
WantedBy=multi-user.target
```

Then:

```bash
systemctl daemon-reload
systemctl enable --now proxmox-desk-display-bridge
```
