package onvif

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

// extractStreamURI pulls the <tt:Uri>...</tt:Uri> value out of a SOAP body.
func extractStreamURI(t *testing.T, body string) string {
	t.Helper()
	const open, close_ = "<tt:Uri>", "</tt:Uri>"
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatalf("response missing <tt:Uri>:\n%s", body)
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, close_)
	if j < 0 {
		t.Fatalf("response missing </tt:Uri>:\n%s", body)
	}
	return rest[:j]
}

func TestHandleGetStreamUri_MainProfileNoQueryParams(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:       "cam",
		HTTPPort:   8081,
		RTSPStream: "/tapo",
	}, "admin", "admin")
	s.rtspPort = 554

	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	r.Host = "192.168.1.50:8081"
	w := httptest.NewRecorder()

	body := `<GetStreamUri><ProfileToken>Profile000</ProfileToken></GetStreamUri>`
	s.handleGetStreamUri(w, r, body)

	uri := extractStreamURI(t, w.Body.String())
	want := "rtsp://192.168.1.50:554/tapo"
	if uri != want {
		t.Errorf("uri = %q, want %q", uri, want)
	}
	if strings.ContainsAny(uri, "?&") {
		t.Errorf("expected clean URI without query params, got %q", uri)
	}
}

func TestHandleGetStreamUri_Substream(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:             "cam",
		HTTPPort:         8081,
		RTSPStream:       "/tapo",
		SubstreamEnabled: true,
		SubstreamPath:    "/tapo_sub",
	}, "admin", "admin")
	s.rtspPort = 554

	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	r.Host = "192.168.1.50:8081"
	w := httptest.NewRecorder()

	body := `<GetStreamUri><ProfileToken>Profile001</ProfileToken></GetStreamUri>`
	s.handleGetStreamUri(w, r, body)

	uri := extractStreamURI(t, w.Body.String())
	want := "rtsp://192.168.1.50:554/tapo_sub"
	if uri != want {
		t.Errorf("uri = %q, want %q", uri, want)
	}
}

func TestHandleGetStreamUri_StreamTypeSubstream(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:             "cam",
		HTTPPort:         8081,
		RTSPStream:       "/tapo",
		SubstreamEnabled: true,
		SubstreamPath:    "/tapo_sub",
	}, "admin", "admin")
	s.rtspPort = 554

	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	r.Host = "192.168.1.50:8081"
	w := httptest.NewRecorder()

	body := `<GetStreamUri><StreamType>RTP-Unicast-Substream</StreamType></GetStreamUri>`
	s.handleGetStreamUri(w, r, body)

	uri := extractStreamURI(t, w.Body.String())
	want := "rtsp://192.168.1.50:554/tapo_sub"
	if uri != want {
		t.Errorf("uri = %q, want %q", uri, want)
	}
}

func TestHandleGetSnapshotUri(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:     "cam",
		HTTPPort: 8081,
	}, "admin", "admin")

	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	r.Host = "192.168.1.50:8081"
	w := httptest.NewRecorder()

	s.handleGetSnapshotUri(w, r)

	uri := extractStreamURI(t, w.Body.String())
	want := "http://192.168.1.50:8081/snapshot"
	if uri != want {
		t.Errorf("uri = %q, want %q", uri, want)
	}
}

func TestHandleSetVideoEncoderConfiguration_StoresBitrate(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")

	r := httptest.NewRequest("POST", "/onvif/media2_service", nil)
	w := httptest.NewRecorder()

	body := `<SetVideoEncoderConfiguration>
<Configuration token="V_ENC_CFG_001">
<BitrateLimit>768</BitrateLimit>
</Configuration>
</SetVideoEncoderConfiguration>`

	s.handleSetVideoEncoderConfiguration(w, r, body)

	s.bitrateMu.RLock()
	got, ok := s.bitrateCache["V_ENC_CFG_001"]
	s.bitrateMu.RUnlock()
	if !ok {
		t.Fatal("expected bitrate to be cached for token")
	}
	if got != 768 {
		t.Errorf("bitrate = %d, want 768", got)
	}
}
