# Proxmox Read-Only Token

Create a dedicated read-only user and token on each Proxmox install.

Run on the Proxmox host:

```bash
pveum user add monitor@pve --comment "Proxmox Desk Display"
pveum acl modify / -user monitor@pve -role PVEAuditor -propagate 1
pveum user token add monitor@pve desk -privsep 1
pveum acl modify / -token 'monitor@pve!desk' -role PVEAuditor -propagate 1
```

Both ACLs matter when `-privsep 1` is used. A privilege-separated token can only use the intersection of the user's permissions and the token's permissions. If either side is missing `PVEAuditor`, Proxmox can return:

```text
403 Permission check failed (/nodes/pve, Sys.Audit)
```

Verify:

```bash
pveum user permissions monitor@pve
pveum user token permissions monitor@pve desk
```

The token permissions output should include `Sys.Audit`.

Save the token value in `.env`:

```bash
PVE_A_TOKEN='monitor@pve!desk=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
```

The bridge sends it as:

```http
Authorization: PVEAPIToken=monitor@pve!desk=...
```

Do not put Proxmox tokens in the firmware.
