package onvif

import (
	"net/http/httptest"
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
