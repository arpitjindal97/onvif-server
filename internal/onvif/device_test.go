package onvif

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
)

func newDeviceRequest(host string) *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func TestHandleGetDeviceInformation(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:         "cam",
		Manufacturer: "AcmeCo",
		Model:        "X-100",
		Serial:       "SN-1234",
		HTTPPort:     8081,
	}, "admin", "admin")

	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()
	s.handleGetDeviceInformation(w, r)

	body := w.Body.String()
	for _, want := range []string{"AcmeCo", "X-100", "SN-1234", "GetDeviceInformationResponse"} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q:\n%s", want, body)
		}
	}
}

func TestHandleGetCapabilities(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "10.0.0.5:8081"
	w := httptest.NewRecorder()

	s.handleGetCapabilities(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"http://10.0.0.5:8081/onvif/device_service",
		"http://10.0.0.5:8081/onvif/media_service",
		"http://10.0.0.5:8081/onvif/event_service",
		"http://10.0.0.5:8081/onvif/ptz_service",
		"GetCapabilitiesResponse",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q:\n%s", want, body)
		}
	}
}

func TestHandleGetServices(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "10.0.0.5:8081"
	w := httptest.NewRecorder()

	s.handleGetServices(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"http://www.onvif.org/ver10/device/wsdl",
		"http://www.onvif.org/ver10/media/wsdl",
		"http://www.onvif.org/ver20/media/wsdl",
		"http://www.onvif.org/ver10/events/wsdl",
		"http://www.onvif.org/ver20/imaging/wsdl",
		"http://www.onvif.org/ver20/ptz/wsdl",
		"http://www.onvif.org/ver20/analytics/wsdl",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing namespace %q", want)
		}
	}
}

func TestHandleGetScopes(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "front", Model: "X100"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.handleGetScopes(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"onvif://www.onvif.org/hardware/X100",
		"onvif://www.onvif.org/name/front",
		"onvif://www.onvif.org/type/video_encoder",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing scope %q", want)
		}
	}
}

func TestHandleGetHostname(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "front-door"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.handleGetHostname(w, r)
	body := w.Body.String()

	if !strings.Contains(body, "<tt:Name>front-door</tt:Name>") {
		t.Errorf("hostname missing or wrong:\n%s", body)
	}
}

func TestHandleGetDNS(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.handleGetDNS(w, r)
	body := w.Body.String()

	if !strings.Contains(body, "8.8.8.8") {
		t.Errorf("DNS response missing 8.8.8.8:\n%s", body)
	}
}

func TestHandleGetNetworkInterfaces(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	r.Host = "10.0.0.5:8081"
	w := httptest.NewRecorder()

	s.handleGetNetworkInterfaces(w, r)
	body := w.Body.String()

	for _, want := range []string{"eth0", "00:11:22:33:44:55", "10.0.0.5"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in response", want)
		}
	}
}

func TestHandleGetNetworkProtocols(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.handleGetNetworkProtocols(w, r)
	body := w.Body.String()

	for _, want := range []string{"HTTP", "HTTPS", "RTSP", "<tt:Port>8081</tt:Port>", "<tt:Port>554</tt:Port>"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in response", want)
		}
	}
}

func TestHandleSystemReboot(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	s.handleSystemReboot(w, r)
	body := w.Body.String()

	if !strings.Contains(body, "SystemRebootResponse") || !strings.Contains(body, "Device is rebooting") {
		t.Errorf("unexpected reboot response:\n%s", body)
	}
}

func TestHandleGetSystemDateAndTime_NoSyncReturnsSystemTime(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	before := time.Now().UTC()
	s.handleGetSystemDateAndTime(w, r)
	after := time.Now().UTC()

	body := w.Body.String()
	if !strings.Contains(body, "GetSystemDateAndTimeResponse") {
		t.Fatalf("missing response wrapper:\n%s", body)
	}
	if !strings.Contains(body, "<tt:DateTimeType>NTP</tt:DateTimeType>") {
		t.Errorf("expected default DateTimeType=NTP:\n%s", body)
	}
	// Year should be current year.
	year := before.Format("2006")
	yearLater := after.Format("2006")
	if !strings.Contains(body, "<tt:Year>"+year+"</tt:Year>") &&
		!strings.Contains(body, "<tt:Year>"+yearLater+"</tt:Year>") {
		t.Errorf("expected current year in response:\n%s", body)
	}
}

func TestHandleSetSystemDateAndTime_StoresSettingsAndAffectsGet(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	setBody := []byte(`<SetSystemDateAndTime>
<DateTimeType>Manual</DateTimeType>
<TimeZone><TZ>EST5EDT</TZ></TimeZone>
<UTCDateTime>
  <Time><Hour>10</Hour><Minute>30</Minute><Second>15</Second></Time>
  <Date><Year>2030</Year><Month>6</Month><Day>15</Day></Date>
</UTCDateTime>
</SetSystemDateAndTime>`)

	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()
	s.handleSetSystemDateAndTime(w, r, setBody)

	if !strings.Contains(w.Body.String(), "SetSystemDateAndTimeResponse") {
		t.Fatalf("expected SetSystemDateAndTimeResponse:\n%s", w.Body.String())
	}

	s.timeSettingsMu.RLock()
	settings := s.timeSettings
	s.timeSettingsMu.RUnlock()
	if settings == nil {
		t.Fatal("expected timeSettings to be stored")
	}
	if settings.DateTimeType != "Manual" {
		t.Errorf("DateTimeType = %q, want Manual", settings.DateTimeType)
	}
	if settings.TimeZone != "EST5EDT" {
		t.Errorf("TimeZone = %q, want EST5EDT", settings.TimeZone)
	}
	if settings.BaseTime.Year() != 2030 || settings.BaseTime.Month() != 6 || settings.BaseTime.Day() != 15 {
		t.Errorf("BaseTime = %v, want 2030-06-15", settings.BaseTime)
	}

	// Now GetSystemDateAndTime should return the synced year.
	r2 := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w2 := httptest.NewRecorder()
	s.handleGetSystemDateAndTime(w2, r2)

	body := w2.Body.String()
	if !strings.Contains(body, "<tt:Year>2030</tt:Year>") {
		t.Errorf("expected synced year 2030 in response:\n%s", body)
	}
	if !strings.Contains(body, "<tt:DateTimeType>Manual</tt:DateTimeType>") {
		t.Errorf("expected DateTimeType=Manual after sync:\n%s", body)
	}
	if !strings.Contains(body, "<tt:TZ>EST5EDT</tt:TZ>") {
		t.Errorf("expected TZ=EST5EDT after sync:\n%s", body)
	}
}

func TestHandleSetSystemDateAndTime_NoUTCDateTime(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	// No UTCDateTime block: settings should remain unset.
	s.handleSetSystemDateAndTime(w, r, []byte(`<SetSystemDateAndTime></SetSystemDateAndTime>`))

	s.timeSettingsMu.RLock()
	defer s.timeSettingsMu.RUnlock()
	if s.timeSettings != nil {
		t.Errorf("expected no settings stored, got %+v", s.timeSettings)
	}
}

func TestHandleSetSystemDateAndTime_InvalidDateValues(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/device_service", nil)
	w := httptest.NewRecorder()

	// UTCDateTime present but Year=0 etc. - should not store.
	body := []byte(`<SetSystemDateAndTime>
<UTCDateTime>
  <Time><Hour>0</Hour><Minute>0</Minute><Second>0</Second></Time>
  <Date><Year>0</Year><Month>0</Month><Day>0</Day></Date>
</UTCDateTime>
</SetSystemDateAndTime>`)
	s.handleSetSystemDateAndTime(w, r, body)

	s.timeSettingsMu.RLock()
	defer s.timeSettingsMu.RUnlock()
	if s.timeSettings != nil {
		t.Errorf("expected no settings on invalid date, got %+v", s.timeSettings)
	}
}
