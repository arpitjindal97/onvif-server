# ONVIF Server

[![Test](https://github.com/arpitjindal97/onvif-server/actions/workflows/test.yml/badge.svg)](https://github.com/arpitjindal97/onvif-server/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/arpitjindal97/onvif-server/branch/main/graph/badge.svg)](https://codecov.io/gh/arpitjindal97/onvif-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/arpitjindal97/onvif-server)](https://goreportcard.com/report/github.com/arpitjindal97/onvif-server)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight, dependency-free **ONVIF Profile S** server written in Go. It puts a virtual ONVIF interface in front of any RTSP source so that NVRs (e.g. Synology Surveillance Station, Reolink NVR, Frigate, Blue Iris, Hikvision, Dahua, UniFi Protect) can discover and ingest cameras that don't natively speak ONVIF — for example Tapo, Wyze with RTSP firmware, MediaMTX/RTSP-Simple-Server relays, or any FFmpeg-fed RTSP endpoint.

> **Why?** Many cheap IP cameras expose RTSP but not ONVIF, so NVRs can't auto-discover or auto-configure them. This server bridges that gap by impersonating a fully ONVIF-compliant camera.

---

## Features

- **ONVIF Profile S** — Device, Media, and Events services
- **Multi-camera** — one virtual ONVIF endpoint per camera, each on its own HTTP port
- **WS-Discovery** — NVRs auto-detect cameras on the LAN (multicast on `239.255.255.250:3702`)
- **WS-Security UsernameToken (Digest)** — username/password authentication on every SOAP request
- **Multiple profiles per camera** — main stream + two substream profiles for low-bandwidth use
- **Auto-detection** — uses `ffprobe` to populate resolution, codec, bitrate, framerate, and H.264 profile from the live RTSP stream every 10 minutes
- **Snapshot endpoint** — returns a placeholder JPEG (real cameras typically expose a `/snapshot` URL; this is a stub)
- **Time sync** — accepts `SetSystemDateAndTime` from NVRs and reports back the synced clock
- **Zero hardware dependencies** — runs anywhere Go does (Raspberry Pi, NAS, Docker, x86)

---

## Requirements

- **Go 1.21+** (build only)
- **`ffprobe`** (from FFmpeg) on `PATH` — used to detect stream parameters. Without it, sane defaults are used (1920×1080 / H.264 / 4 Mbps).
- An RTSP source reachable from the machine running the server (e.g. an [go2rtc](https://github.com/AlexxIT/go2rtc) instance or a real camera's RTSP URL).

---

## Quick start

```bash
git clone https://github.com/arpitjindal97/onvif-server.git
cd onvif-server
go build -o onvif-server ./cmd/onvif-server
# edit config.yaml to taste
./onvif-server config.yaml
```

The default `config.yaml` (sample below) wires up a single camera on port `8081`. Point your NVR at `<host>:8081` over ONVIF and it should appear immediately — or wait a few seconds for WS-Discovery to advertise it.

---

## Configuration

A YAML file (passed as the only CLI argument; defaults to `config.yaml`) declares one or more cameras and global settings.

```yaml
# Global settings
rtsp_host: ""              # Leave empty to use the IP from each incoming SOAP request.
                           # Set explicitly (e.g. "192.168.1.50") if behind NAT or on a fixed IP.
rtsp_port: 554
enable_discovery: true     # Enable WS-Discovery (multicast advertisement)

# WS-Security credentials (used for NVR <-> ONVIF auth on every SOAP call)
username: "admin"
password: "admin"          # default if omitted

cameras:
  - name: "Front Door"
    manufacturer: "TP-Link"
    model: "Tapo C200"
    serial: "TAPO-FRONT-001"
    http_port: 8081                # ONVIF HTTP port (must be unique per camera)
    rtsp_stream: "/tapo_front"     # Main stream path -> rtsp://<rtsp_host>:554/tapo_front
    h264_profile: "High"           # Optional: Baseline | Main | High (default: High)
    substream_enabled: true        # Expose a low-resolution substream as Profile001/002
    substream_path: "/tapo_front_sub"
    substream_h264_profile: "Main" # Optional

  - name: "Back Yard"
    manufacturer: "Generic"
    model: "IP Camera"
    serial: "CAM002"
    http_port: 8082
    rtsp_stream: "/backyard"
    substream_enabled: false
```

### Field reference

| Field | Type | Notes |
|---|---|---|
| `cameras[].name` | string | Friendly name (becomes ONVIF scope `name`) |
| `cameras[].manufacturer` / `model` / `serial` | string | Returned by `GetDeviceInformation` |
| `cameras[].http_port` | int | ONVIF HTTP port; **must be unique** across cameras |
| `cameras[].rtsp_stream` | string | Path component of the main RTSP URL |
| `cameras[].h264_profile` | string | `Baseline`, `Main`, or `High`. Auto-detected from `ffprobe` if omitted. |
| `cameras[].substream_enabled` | bool | Adds Profile001/Profile002 (low-res variants) |
| `cameras[].substream_path` | string | If empty, defaults to `<rtsp_stream>_sub` |
| `cameras[].substream_h264_profile` | string | Same options as `h264_profile` |
| `rtsp_host` | string | Empty = derive from incoming request `Host` header |
| `rtsp_port` | int | RTSP port on `rtsp_host` (typically 554) |
| `enable_discovery` | bool | Toggle WS-Discovery multicast |
| `username` / `password` | string | WS-Security credentials (default `admin` / `admin`) |

---

## Endpoints

Each camera exposes the following on its `http_port`:

| Path | Description |
|---|---|
| `/onvif/device_service` | Device service (info, capabilities, network, system) |
| `/onvif/media_service` | Media service (profiles, stream URI, encoder config) |
| `/onvif/media2_service` | ONVIF Media2 (encoder configuration set) |
| `/onvif/event_service` | Events (Subscribe / GetEventProperties / PullPoint) |
| `/snapshot` | Static JPEG (placeholder) |

The RTSP source is **not** proxied — your NVR connects directly to `rtsp://<rtsp_host>:<rtsp_port>/<rtsp_stream>`. This server only advertises the URL.

---

## Adding to an NVR

### Auto-discovery

If `enable_discovery: true`, the server replies to WS-Discovery probes on `239.255.255.250:3702`. Most NVRs running on the same L2 segment will list the cameras automatically when you scan for ONVIF devices.

### Manual add

1. NVR → Add Camera → **ONVIF**.
2. **IP**: the host running this server.
3. **Port**: `http_port` from the config (e.g. `8081`).
4. **Username / Password**: as set in the config (default `admin` / `admin`).
5. Save. The NVR will fetch profiles and start recording the RTSP stream.

---

## Supported ONVIF operations

### Device service

`GetSystemDateAndTime`, `SetSystemDateAndTime`, `GetDeviceInformation`, `GetCapabilities`, `GetServices`, `GetScopes`, `GetHostname`, `GetDNS`, `GetNetworkInterfaces`, `GetNetworkProtocols`, `SystemReboot`

### Media / Media2 service

`GetProfiles`, `GetStreamUri`, `GetSnapshotUri`, `GetVideoSources`, `GetAudioSources`, `GetVideoEncoderConfigurations`, `GetVideoEncoderConfiguration`, `GetVideoEncoderConfigurationOptions`, `SetVideoEncoderConfiguration`

### Event service

`Subscribe`, `GetEventProperties`, `CreatePullPointSubscription`

---

## Manual testing

```bash
# Get device info
curl -X POST http://localhost:8081/onvif/device_service \
  -H "Content-Type: application/soap+xml" \
  -d '<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body><GetDeviceInformation xmlns="http://www.onvif.org/ver10/device/wsdl"/></s:Body>
</s:Envelope>'

# Fetch a snapshot
curl -o snap.jpg http://localhost:8081/snapshot
```

You can also point [`onvif-cli`](https://github.com/quatanium/python-onvif) or [ONVIF Device Manager](https://sourceforge.net/projects/onvifdm/) (Windows) at the server.

---

## Development

```bash
# Run tests with race detector and coverage
make coverage

# Just run tests
go test ./...

# Build
go build -o onvif-server ./cmd/onvif-server
```

### Project layout

```text
cmd/onvif-server/    # main entrypoint
internal/
  config/            # YAML config loader
  discovery/         # WS-Discovery multicast listener
  logger/            # debug/info logging
  netutil/           # outbound IP detection
  onvif/             # all SOAP handlers + server core
```

---

## Limitations

- **No PTZ** — the PTZ service URL is advertised but `GetConfigurations` etc. are not implemented (NVRs gracefully fall back to "no PTZ").
- **No real event source** — event subscriptions are accepted but no events are ever published.
- **Snapshot is a placeholder** — returns a static JPEG; if you need real snapshots, point your NVR at the camera's native snapshot URL.
- **No audio decoding** — the audio source is advertised but not wired to anything.
- **TLS not implemented** — SOAP is plain HTTP. Run on a trusted network or behind a reverse proxy with TLS termination.

---

## License

[MIT](LICENSE)
