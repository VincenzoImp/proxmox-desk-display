# Flashing The LILYGO T-Display-S3

The first supported board is the LILYGO T-Display-S3.

## PlatformIO

Install PlatformIO, then run:

```bash
cd firmware/t-display-s3
pio run -t upload --upload-port /dev/cu.usbmodem1101
```

If upload fails, enter bootloader mode manually:

1. Hold `BOOT`.
2. Press and release `RST`.
3. Release `BOOT`.
4. Upload again.
5. Press `RST` after upload.

## First Boot

If no config is saved, the device starts an access point:

```text
Proxmox-Desk-Setup
```

Join it and open:

```text
http://192.168.4.1
```

Enter Wi-Fi credentials, bridge URL, and display token.
