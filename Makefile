.PHONY: bridge-test bridge-run bridge-mock firmware-build firmware-upload

bridge-test:
	cd apps/bridge && go test ./...

bridge-run:
	cd apps/bridge && go run ./cmd/proxmox-desk-display --config ../../config.yaml

bridge-mock:
	cd apps/bridge && DISPLAY_TOKEN=dev-token go run ./cmd/proxmox-desk-display --mock

firmware-build:
	cd firmware/t-display-s3 && pio run

firmware-upload:
	cd firmware/t-display-s3 && pio run -t upload --upload-port /dev/cu.usbmodem1101
