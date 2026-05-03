# Proxmox Read-Only Token

Create a dedicated read-only user and token on each Proxmox install.

Run on the Proxmox host:

```bash
pveum user add monitor@pve --comment "Proxmox Desk Display"
pveum acl modify / -user monitor@pve -role PVEAuditor
pveum user token add monitor@pve desk -privsep 1
pveum acl modify / -token 'monitor@pve!desk' -role PVEAuditor
```

Save the token value in `.env`:

```bash
PVE_A_TOKEN='monitor@pve!desk=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
```

The bridge sends it as:

```http
Authorization: PVEAPIToken=monitor@pve!desk=...
```

Do not put Proxmox tokens in the firmware.
