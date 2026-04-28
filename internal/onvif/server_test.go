package onvif

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestGetHostIP_FromHostHeaderWithPort(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "192.168.1.50:8081"

	if got := s.getHostIP(r); got != "192.168.1.50" {
		t.Errorf("got %q, want 192.168.1.50", got)
	}
}

func TestGetHostIP_FromHostHeaderNoPort(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "camera.local"

	if got := s.getHostIP(r); got != "camera.local" {
		t.Errorf("got %q, want camera.local", got)
	}
}

func TestGetBaseURL(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "10.0.0.5:8081"

	got := s.getBaseURL(r)
	want := "http://10.0.0.5:8081"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCameraName(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "front-door"}, "admin", "admin")
	if s.CameraName() != "front-door" {
		t.Errorf("CameraName: got %q, want front-door", s.CameraName())
	}
}

func TestHandleSnapshot_ReturnsJPEG(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("GET", "/snapshot", nil)
	w := httptest.NewRecorder()

	s.handleSnapshot(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) < 4 {
		t.Fatalf("body too short: %d bytes", len(body))
	}
	// JPEG SOI 0xFFD8 + EOI 0xFFD9.
	if body[0] != 0xFF || body[1] != 0xD8 {
		t.Errorf("missing JPEG SOI marker, got % x", body[:2])
	}
	if body[len(body)-2] != 0xFF || body[len(body)-1] != 0xD9 {
		t.Errorf("missing JPEG EOI marker, got % x", body[len(body)-2:])
	}
}

func TestGetHostIP_FallsBackToRTSPHost(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	s.rtspHost = "10.99.99.99"
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = ""

	if got := s.getHostIP(r); got != "10.99.99.99" {
		t.Errorf("got %q, want 10.99.99.99 (rtspHost fallback)", got)
	}
}

func TestSubstreamEnabled(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", SubstreamEnabled: true}, "admin", "admin")
	if !s.SubstreamEnabled() {
		t.Error("SubstreamEnabled() = false, want true")
	}

	s2 := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	if s2.SubstreamEnabled() {
		t.Error("SubstreamEnabled() = true, want false")
	}
}

func TestMinInt(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 5, -1},
	}
	for _, c := range cases {
		if got := minInt(c.a, c.b); got != c.want {
			t.Errorf("minInt(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSendSOAPFault(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	w := httptest.NewRecorder()

	s.sendSOAPFault(w, "s:Sender", "ter:NotAuthorized", "auth failed")

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	for _, want := range []string{"SOAP-ENV:Fault", "s:Sender", "ter:NotAuthorized", "auth failed"} {
		if !strings.Contains(bs, want) {
			t.Errorf("missing %q in fault body:\n%s", want, bs)
		}
	}
}

// soapEnvelope wraps a body fragment in a complete SOAP envelope for handleRequest.
func wrapSOAP(bodyXML string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope">
<SOAP-ENV:Body>` + bodyXML + `</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`
}

func TestHandleRequest_RoutesGetDeviceInformation(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", Manufacturer: "AcmeCo"}, "admin", "admin")
	body := wrapSOAP(`<GetDeviceInformation/>`)

	r := httptest.NewRequest("POST", "/onvif/device_service", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleRequest(w, r)

	if !strings.Contains(w.Body.String(), "GetDeviceInformationResponse") {
		t.Errorf("expected device info response:\n%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "AcmeCo") {
		t.Errorf("expected manufacturer in response:\n%s", w.Body.String())
	}
}

func TestHandleRequest_InvalidSOAP(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", strings.NewReader("not-xml"))
	w := httptest.NewRecorder()

	s.handleRequest(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "InvalidArgVal") {
		t.Errorf("expected InvalidArgVal fault:\n%s", w.Body.String())
	}
}

func TestRouteRequest_UnsupportedOperation(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.routeRequest(w, r, `<TotallyUnknownOperation/>`)

	if !strings.Contains(w.Body.String(), "ActionNotSupported") {
		t.Errorf("expected ActionNotSupported fault:\n%s", w.Body.String())
	}
}

func TestRouteRequest_DispatchesEachOperation(t *testing.T) {
	cases := []struct {
		name        string
		bodyContent string
		expectInOut string
	}{
		{"GetSystemDateAndTime", `<GetSystemDateAndTime/>`, "GetSystemDateAndTimeResponse"},
		{"GetCapabilities", `<GetCapabilities/>`, "GetCapabilitiesResponse"},
		{"GetServices", `<GetServices/>`, "GetServicesResponse"},
		{"GetScopes", `<GetScopes/>`, "GetScopesResponse"},
		{"GetHostname", `<GetHostname/>`, "GetHostnameResponse"},
		{"GetDNS", `<GetDNS/>`, "GetDNSResponse"},
		{"GetNetworkInterfaces", `<GetNetworkInterfaces/>`, "GetNetworkInterfacesResponse"},
		{"GetNetworkProtocols", `<GetNetworkProtocols/>`, "GetNetworkProtocolsResponse"},
		{"SystemReboot", `<SystemReboot/>`, "SystemRebootResponse"},
		{"GetProfiles", `<GetProfiles/>`, "GetProfilesResponse"},
		{"GetSnapshotUri", `<GetSnapshotUri/>`, "GetSnapshotUriResponse"},
		{"GetVideoSources", `<GetVideoSources/>`, "GetVideoSourcesResponse"},
		{"GetAudioSources", `<GetAudioSources/>`, "GetAudioSourcesResponse"},
		{"GetVideoEncoderConfigurationOptions", `<GetVideoEncoderConfigurationOptions/>`, "GetVideoEncoderConfigurationOptionsResponse"},
		{"GetVideoEncoderConfigurations", `<GetVideoEncoderConfigurations/>`, "GetVideoEncoderConfigurationsResponse"},
		{"GetEventProperties", `<GetEventProperties/>`, "GetEventPropertiesResponse"},
		{"CreatePullPointSubscription", `<CreatePullPointSubscription/>`, "CreatePullPointSubscriptionResponse"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
			r := httptest.NewRequest("POST", "/onvif/device_service", nil)
			r.Host = "10.0.0.5:8081"
			w := httptest.NewRecorder()

			s.routeRequest(w, r, tc.bodyContent)
			if !strings.Contains(w.Body.String(), tc.expectInOut) {
				t.Errorf("missing %q in response:\n%s", tc.expectInOut, w.Body.String())
			}
		})
	}
}
