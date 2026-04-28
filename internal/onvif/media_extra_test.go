package onvif

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestHandleGetProfiles(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	s.handleGetProfiles(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"GetProfilesResponse",
		`token="Profile000"`,
		`token="Profile001"`,
		`token="Profile002"`,
		`token="V_ENC_CFG_000"`,
		`token="V_ENC_CFG_001"`,
		`token="V_ENC_CFG_002"`,
		"<tt:BitrateLimit>4096</tt:BitrateLimit>",
		"<tt:BitrateLimit>1024</tt:BitrateLimit>",
		"<tt:BitrateLimit>512</tt:BitrateLimit>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in profiles response", want)
		}
	}
}

func TestBuildProfile_Resolution(t *testing.T) {
	p := buildProfile("ProfileX", "V_ENC_X", "A_ENC_X", "VA_X", "PTZ_X", "META_X", 1280, 720, 2048)
	if p.Token != "ProfileX" || p.Name != "ProfileX" {
		t.Errorf("token/name = %q/%q, want ProfileX/ProfileX", p.Token, p.Name)
	}
	if p.VideoEncoderConfiguration.Resolution.Width != 1280 ||
		p.VideoEncoderConfiguration.Resolution.Height != 720 {
		t.Errorf("resolution = %+v, want 1280x720", p.VideoEncoderConfiguration.Resolution)
	}
	if p.VideoEncoderConfiguration.RateControl.BitrateLimit != 2048 {
		t.Errorf("bitrate = %d, want 2048", p.VideoEncoderConfiguration.RateControl.BitrateLimit)
	}
	if p.AudioEncoderConfiguration.Token != "A_ENC_X" {
		t.Errorf("audio encoder token = %q, want A_ENC_X", p.AudioEncoderConfiguration.Token)
	}
}

func TestHandleGetVideoSources(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	s.handleGetVideoSources(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"GetVideoSourcesResponse",
		`token="VideoSource_1"`,
		"<tt:Width>1920</tt:Width>",
		"<tt:Height>1080</tt:Height>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in video sources response", want)
		}
	}
}

func TestHandleGetAudioSources(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	s.handleGetAudioSources(w, r)
	body := w.Body.String()

	if !strings.Contains(body, "GetAudioSourcesResponse") ||
		!strings.Contains(body, `token="AudioSource_1"`) {
		t.Errorf("unexpected audio sources response:\n%s", body)
	}
}

func TestHandleGetVideoEncoderConfigurations(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	s.handleGetVideoEncoderConfigurations(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"GetVideoEncoderConfigurationsResponse",
		`token="VideoEncoderToken"`,
		"<tt:Encoding>H264</tt:Encoding>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in encoder configs response", want)
		}
	}
}

func TestHandleGetVideoEncoderConfiguration_DefaultToken(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	// No token specified -> default V_ENC_CFG_000 (main stream).
	s.handleGetVideoEncoderConfiguration(w, r, `<GetVideoEncoderConfiguration></GetVideoEncoderConfiguration>`)
	body := w.Body.String()

	if !strings.Contains(body, `token="V_ENC_CFG_000"`) {
		t.Errorf("expected default token V_ENC_CFG_000:\n%s", body)
	}
	if !strings.Contains(body, "<tt:Width>1920</tt:Width>") {
		t.Errorf("expected main-stream width 1920:\n%s", body)
	}
}

func TestHandleGetVideoEncoderConfiguration_SubstreamToken(t *testing.T) {
	s := newTestServer(config.CameraConfig{
		Name:             "cam",
		RTSPStream:       "/cam",
		SubstreamEnabled: true,
		SubstreamPath:    "/cam_sub",
	}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	body := `<GetVideoEncoderConfiguration><ConfigurationToken>V_ENC_CFG_001</ConfigurationToken></GetVideoEncoderConfiguration>`
	s.handleGetVideoEncoderConfiguration(w, r, body)
	out := w.Body.String()

	if !strings.Contains(out, `token="V_ENC_CFG_001"`) {
		t.Errorf("expected token V_ENC_CFG_001:\n%s", out)
	}
	if !strings.Contains(out, "<tt:Width>640</tt:Width>") {
		t.Errorf("expected substream width 640:\n%s", out)
	}
}

func TestHandleGetVideoEncoderConfigurationOptions(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	w := httptest.NewRecorder()

	s.handleGetVideoEncoderConfigurationOptions(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"GetVideoEncoderConfigurationOptionsResponse",
		"Baseline", "Main", "High",
		"<tt:Width>1920</tt:Width>",
		"<tt:Width>176</tt:Width>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in encoder options:\n%s", want, body)
		}
	}
}

func TestHandleGetStreamUri_Profile002Substream(t *testing.T) {
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

	s.handleGetStreamUri(w, r, `<GetStreamUri><ProfileToken>Profile002</ProfileToken></GetStreamUri>`)
	uri := extractStreamURI(t, w.Body.String())
	if uri != "rtsp://192.168.1.50:554/tapo_sub" {
		t.Errorf("uri = %q, want rtsp://192.168.1.50:554/tapo_sub", uri)
	}
}

func TestHandleGetStreamUri_SubstreamFallbackPath(t *testing.T) {
	// SubstreamEnabled=true but no SubstreamPath -> falls back to RTSPStream + "_sub".
	s := newTestServer(config.CameraConfig{
		Name:             "cam",
		HTTPPort:         8081,
		RTSPStream:       "/cam",
		SubstreamEnabled: true,
	}, "admin", "admin")
	s.rtspPort = 554

	r := httptest.NewRequest("POST", "/onvif/media_service", nil)
	r.Host = "192.168.1.50:8081"
	w := httptest.NewRecorder()

	s.handleGetStreamUri(w, r, `<GetStreamUri><ProfileToken>Profile001</ProfileToken></GetStreamUri>`)
	uri := extractStreamURI(t, w.Body.String())
	if uri != "rtsp://192.168.1.50:554/cam_sub" {
		t.Errorf("uri = %q, want rtsp://192.168.1.50:554/cam_sub", uri)
	}
}

func TestHandleSetVideoEncoderConfiguration_NoBitrateField(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/media2_service", nil)
	w := httptest.NewRecorder()

	// No BitrateLimit element: handler should still respond OK and not panic.
	body := `<SetVideoEncoderConfiguration><Configuration token="V_ENC_CFG_000"></Configuration></SetVideoEncoderConfiguration>`
	s.handleSetVideoEncoderConfiguration(w, r, body)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "SetVideoEncoderConfigurationResponse") {
		t.Errorf("missing response body:\n%s", w.Body.String())
	}
}
