package onvif

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestGenerateNonce_RandomAndBase64(t *testing.T) {
	a := generateNonce()
	b := generateNonce()
	if a == "" || b == "" {
		t.Fatal("generateNonce returned empty string")
	}
	if a == b {
		t.Error("generateNonce returned identical values across calls")
	}
	if _, err := base64.StdEncoding.DecodeString(a); err != nil {
		t.Errorf("nonce is not valid base64: %v", err)
	}
}

func TestCalculateMD5(t *testing.T) {
	got := calculateMD5("hello")
	want := func() string {
		sum := md5.Sum([]byte("hello"))
		return hex.EncodeToString(sum[:])
	}()
	if got != want {
		t.Errorf("calculateMD5(%q) = %q, want %q", "hello", got, want)
	}
}

// TestNewServer exercises the constructor (which also spawns background
// detection goroutines that will harmlessly fail because the RTSP port is
// unreachable). We only verify struct initialization here.
func TestNewServer_InitializesDefaults(t *testing.T) {
	cfg := config.CameraConfig{
		Name:       "cam",
		RTSPStream: "/cam",
		HTTPPort:   8081,
	}
	s := NewServer(cfg, "10.0.0.1", 1, "user", "pw")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.CameraName() != "cam" {
		t.Errorf("CameraName = %q", s.CameraName())
	}
	if s.streamInfo == nil || s.streamInfo.Width != 1920 {
		t.Errorf("default main stream width = %v, want 1920", s.streamInfo)
	}
	if s.substreamInfo == nil || s.substreamInfo.Width != 640 {
		t.Errorf("default substream width = %v, want 640", s.substreamInfo)
	}
	if s.bitrateCache == nil {
		t.Error("bitrateCache not initialized")
	}
}

func TestNewServer_WithSubstream(t *testing.T) {
	cfg := config.CameraConfig{
		Name:             "cam",
		RTSPStream:       "/cam",
		SubstreamEnabled: true,
		SubstreamPath:    "/cam_sub",
		HTTPPort:         8081,
	}
	s := NewServer(cfg, "10.0.0.1", 1, "u", "p")
	if !s.SubstreamEnabled() {
		t.Error("SubstreamEnabled() = false, want true")
	}
}
