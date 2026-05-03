# Hardware

## Tier 1

The first officially supported board is the LILYGO T-Display-S3.

Important traits:

- ESP32-S3;
- 1.9 inch ST7789 display;
- 170x320 resolution;
- USB-C;
- BOOT button and IO14 user button;
- 16 MB flash and 8 MB PSRAM on common variants.

The firmware uses the TFT_eSPI pin mapping equivalent to `Setup206_LilyGo_T_Display_S3`.

## Future Boards

Future hardware should use explicit board profiles:

- ESP32-2432S028 Cheap Yellow Display;
- M5Stack Core2;
- WT32-SC01 Plus;
- ESP32 e-paper boards.

Each board profile must define display driver, resolution, pins, input method, orientation, and a layout density.
