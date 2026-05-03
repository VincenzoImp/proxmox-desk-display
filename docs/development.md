# Development

## Bridge

The bridge is a Go module in `apps/bridge`.

Run tests:

```bash
cd apps/bridge
go test ./...
```

Run mock mode:

```bash
DISPLAY_TOKEN=dev-token go run ./cmd/pve-desk-display --mock
```

Open:

```text
http://localhost:8765
```

## Firmware

The firmware is a PlatformIO project in `firmware/t-display-s3`.

Build:

```bash
cd firmware/t-display-s3
pio run
```

Upload:

```bash
pio run -t upload --upload-port /dev/cu.usbmodem1101
```

## Design Rule

Do not add Proxmox-specific code to the firmware. Add new data handling in the bridge, keep `/api/v1/display-state` stable, and expose bounded heavy display data through `/api/v1/detail-state` instead of making the ESP32 parse `/api/v1/full-state`.
