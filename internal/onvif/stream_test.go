package onvif

import (
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestGetStreamInfoForToken_KnownSubstreamTokens(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	for _, token := range []string{"V_ENC_CFG_001", "V_ENC_CFG_002"} {
		info, isSub := s.getStreamInfoForToken(token)
		if !isSub {
			t.Errorf("token %q: expected isSubstream=true", token)
		}
		if info != s.substreamInfo {
			t.Errorf("token %q: expected substreamInfo, got %+v", token, info)
		}
	}
}

func TestGetStreamInfoForToken_KnownMainToken(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	info, isSub := s.getStreamInfoForToken("V_ENC_CFG_000")
	if isSub {
		t.Error("expected isSubstream=false for V_ENC_CFG_000")
	}
	if info != s.streamInfo {
		t.Errorf("expected streamInfo, got %+v", info)
	}
}

func TestGetStreamInfoForToken_UnknownTokenLowBitrateUsesSubstream(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	s.bitrateCache["VideoEncoder001"] = 512

	info, isSub := s.getStreamInfoForToken("VideoEncoder001")
	if !isSub {
		t.Error("expected low-bitrate unknown token to map to substream")
	}
	if info != s.substreamInfo {
		t.Error("expected substreamInfo for low-bitrate unknown token")
	}
}

func TestGetStreamInfoForToken_UnknownTokenHighBitrateUsesMain(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	s.bitrateCache["VideoEncoder001"] = 4096

	info, isSub := s.getStreamInfoForToken("VideoEncoder001")
	if isSub {
		t.Error("expected high-bitrate unknown token to map to main stream")
	}
	if info != s.streamInfo {
		t.Error("expected streamInfo for high-bitrate unknown token")
	}
}

func TestGetStreamInfoForToken_UnknownTokenNoBitrateUsesMain(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	_, isSub := s.getStreamInfoForToken("Whatever")
	if isSub {
		t.Error("expected unknown token without cached bitrate to default to main")
	}
}

func TestGetRTSPURLForToken(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.CameraConfig
		tok  string
		want string
	}{
		{
			name: "main stream",
			cfg:  config.CameraConfig{RTSPStream: "/cam"},
			tok:  "V_ENC_CFG_000",
			want: "rtsp://1.2.3.4:554/cam",
		},
		{
			name: "substream with explicit substream path",
			cfg:  config.CameraConfig{RTSPStream: "/cam", SubstreamEnabled: true, SubstreamPath: "/cam_sub"},
			tok:  "V_ENC_CFG_001",
			want: "rtsp://1.2.3.4:554/cam_sub",
		},
		{
			name: "substream falls back to <main>_sub",
			cfg:  config.CameraConfig{RTSPStream: "/cam", SubstreamEnabled: false},
			tok:  "V_ENC_CFG_001",
			want: "rtsp://1.2.3.4:554/cam_sub",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(tc.cfg, "admin", "admin")
			got := s.getRTSPURLForToken(tc.tok, "1.2.3.4")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
