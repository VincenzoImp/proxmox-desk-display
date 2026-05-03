# Troubleshooting

## Bridge Shows Proxmox TLS Error

For the recommended `fingerprint` TLS mode, make sure the configured SHA256 fingerprint matches the Proxmox certificate currently served on port `8006`.

For first setup only, you can temporarily use:

```yaml
tls:
  mode: insecure
```

Switch back to `fingerprint`, `ca_file`, or `system` after validating connectivity.

## Firmware Shows Bridge Offline

Check:

- the LILYGO is on the same network as the bridge;
- the bridge URL includes `http://` and port, for example `http://192.168.1.20:8765`;
- `DISPLAY_TOKEN` matches the token configured in the captive portal;
- `GET /healthz` works from a browser.

## Display Is Blank

The firmware uses the TFT_eSPI pin setup for LILYGO T-Display-S3, matching the official `Setup206_LilyGo_T_Display_S3` configuration. If you use a different board, it needs its own profile.
