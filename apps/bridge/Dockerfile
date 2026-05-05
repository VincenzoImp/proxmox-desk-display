FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/proxmox-desk-display ./cmd/proxmox-desk-display

FROM alpine:3.21
RUN adduser -D -H -u 10001 app && mkdir -p /data && chown app:app /data
USER app
COPY --from=build /out/proxmox-desk-display /usr/local/bin/proxmox-desk-display
VOLUME ["/data"]
EXPOSE 8765
ENTRYPOINT ["proxmox-desk-display"]
