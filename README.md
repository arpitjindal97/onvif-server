# ONVIF Server

[![Test](https://github.com/arpitjindal97/onvif-server/actions/workflows/test.yml/badge.svg)](https://github.com/arpitjindal97/onvif-server/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/arpitjindal97/onvif-server/branch/main/graph/badge.svg)](https://codecov.io/gh/arpitjindal97/onvif-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/arpitjindal97/onvif-server)](https://goreportcard.com/report/github.com/arpitjindal97/onvif-server)

A lightweight ONVIF server implementation in Go that wraps RTSP streams and makes them discoverable by NVRs (Network Video Recorders).

## Features

- ✅ Minimal ONVIF Device Service implementation
- ✅ ONVIF Media Service for RTSP stream URLs
- ✅ WS-Discovery for NVR auto-detection
- ✅ Multi-camera support (each on separate HTTP port)
- ✅ Configurable via YAML
- ✅ No dependencies on actual camera hardware supporting ONVIF

## Requirements

- Go 1.21 or later
- RTSP streams accessible at `rtsp://your-ip:554/<path>`

## Installation

```bash
go mod download
go build -o onvif-server
```

## Configuration

Edit `config.yaml`:

```yaml
cameras:
  - name: "Camera 1"
    manufacturer: "Generic"
    model: "IP Camera"
    serial: "CAM001"
    http_port: 8081              # ONVIF HTTP port
    rtsp_stream: "/camera1"      # RTSP path

rtsp_host: ""                    # Auto-detect or set explicitly
rtsp_port: 554
enable_discovery: true
```

## Usage

```bash
./onvif-server [config.yaml]
```

Each camera will be accessible at:
- ONVIF Device Service: `http://your-ip:<http_port>/onvif/device_service`
- ONVIF Media Service: `http://your-ip:<http_port>/onvif/media_service`
- RTSP Stream: `rtsp://your-ip:554/<rtsp_stream>`

## Adding to NVR

### Manual Add:
1. Add camera manually in your NVR
2. Protocol: ONVIF
3. IP: Your server IP
4. Port: HTTP port from config (e.g., 8081)
5. No authentication required

### Auto-Discovery:
If `enable_discovery: true`, NVRs should automatically detect cameras on the network.

## Supported ONVIF Operations

### Device Service
- `GetDeviceInformation` - Returns camera info
- `GetCapabilities` - Returns available services
- `GetServices` - Lists ONVIF services

### Media Service
- `GetProfiles` - Returns video profiles
- `GetStreamUri` - Returns RTSP stream URL

## Testing

Test with `onvif-cli` or any ONVIF client:

```bash
# Test device info
curl -X POST http://localhost:8081/onvif/device_service \
  -H "Content-Type: application/soap+xml" \
  -d '<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <GetDeviceInformation xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>'
```

## Limitations

- No authentication
- No PTZ support
- Basic ONVIF Profile S implementation
- Single video profile per camera

## License

MIT
