# ONVIF Server - Changes Made

## Problem
NVR was giving "response parse failed" error when querying the ONVIF server.

## Root Cause
The server was generating SOAP responses in a different XML format than expected by NVRs. After inspecting a real camera's responses at `192.168.1.10:2020`, the issues were identified:

### Format Differences

**Before (incorrect):**
```xml
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Body xmlns="http://www.w3.org/2003/05/soap-envelope">
    <GetSystemDateAndTimeResponse xmlns="http://www.onvif.org/ver10/device/wsdl">
      <SystemDateAndTime>
        <DateTimeType>NTP</DateTimeType>
```

**After (correct):**
```xml
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope" xmlns:tt="http://www.onvif.org/ver10/schema" xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <SOAP-ENV:Body>
    <tds:GetSystemDateAndTimeResponse>
      <tds:SystemDateAndTime>
        <tt:DateTimeType>NTP</tt:DateTimeType>
```

## Changes Made

### 1. SOAP Envelope Structure
- Changed from default namespace to `SOAP-ENV:` prefix
- Added proper namespace declarations: `tt:`, `tds:`, `trt:`
- Fixed Body element to use `SOAP-ENV:Body` with proper nesting

### 2. Response Element Prefixes
All response elements now use proper prefixes:
- Device service responses: `tds:` prefix
- Media service responses: `trt:` prefix
- Schema types: `tt:` prefix

### 3. Added GetSystemDateAndTime Support
- This is typically the first request NVRs make
- Returns current UTC and local time
- Proper DateTime structure with `tt:` prefixes

### 4. Updated All Type Definitions
Updated XML tags for all structs:
- `GetSystemDateAndTimeResponse` → uses `tds:` prefix
- `GetDeviceInformationResponse` → uses `tds:` prefix
- `GetCapabilitiesResponse` → uses `tds:` and `tt:` prefixes
- `GetProfilesResponse` → uses `trt:` and `tt:` prefixes
- `GetStreamUriResponse` → uses `trt:` and `tt:` prefixes

### 5. Proper Body Wrapping
Fixed both response and fault handlers to properly wrap content in `SOAP-ENV:Body` element.

## Testing

The server now generates responses that match real ONVIF cameras exactly. Test with:

```bash
./onvif-server

# In another terminal:
curl -X POST http://localhost:8081/onvif/service \
  -H "Content-Type: application/soap+xml; charset=utf-8" \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>'
```

Response will match the format expected by NVRs.

## Next Steps

1. Update `config.yaml` with your camera details
2. Run `./onvif-server`
3. Add cameras to NVR manually or via auto-discovery
4. Each camera will be at its configured `http_port` with RTSP at `rtsp://your-ip:554/<rtsp_stream>`
