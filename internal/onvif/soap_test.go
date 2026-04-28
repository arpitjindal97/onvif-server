package onvif

import (
	"crypto/sha1"
	"encoding/base64"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func makeDigest(nonceB64, created, password string) string {
	nonceBytes, _ := base64.StdEncoding.DecodeString(nonceB64)
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(password))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestValidateSecurity_NilHeader(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	if !s.validateSecurity(nil) {
		t.Error("expected nil security to validate (no auth required)")
	}
}

func TestValidateSecurity_ValidDigest(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "secret")

	nonceB64 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	created := "2024-01-01T00:00:00Z"
	digest := makeDigest(nonceB64, created, "secret")

	sec := &security{
		UsernameToken: usernameToken{
			Username: "admin",
			Password: password{Value: digest},
			Nonce:    nonce{Value: nonceB64},
			Created:  created,
		},
	}
	if !s.validateSecurity(sec) {
		t.Error("expected valid digest to pass validation")
	}
}

func TestValidateSecurity_WrongPassword(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "secret")

	nonceB64 := base64.StdEncoding.EncodeToString([]byte("nonce-bytes-here"))
	created := "2024-01-01T00:00:00Z"
	digest := makeDigest(nonceB64, created, "wrong-password")

	sec := &security{
		UsernameToken: usernameToken{
			Username: "admin",
			Password: password{Value: digest},
			Nonce:    nonce{Value: nonceB64},
			Created:  created,
		},
	}
	if s.validateSecurity(sec) {
		t.Error("expected mismatched-password digest to be rejected")
	}
}

func TestValidateSecurity_WrongUsername(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "secret")

	nonceB64 := base64.StdEncoding.EncodeToString([]byte("nonce-bytes-here"))
	created := "2024-01-01T00:00:00Z"
	digest := makeDigest(nonceB64, created, "secret")

	sec := &security{
		UsernameToken: usernameToken{
			Username: "intruder",
			Password: password{Value: digest},
			Nonce:    nonce{Value: nonceB64},
			Created:  created,
		},
	}
	if s.validateSecurity(sec) {
		t.Error("expected wrong username to be rejected")
	}
}

func TestValidateSecurity_InvalidNonceBase64(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "secret")
	sec := &security{
		UsernameToken: usernameToken{
			Username: "admin",
			Password: password{Value: "anything"},
			Nonce:    nonce{Value: "!!!not-base64!!!"},
			Created:  "2024-01-01T00:00:00Z",
		},
	}
	if s.validateSecurity(sec) {
		t.Error("expected invalid base64 nonce to be rejected")
	}
}

func TestGeneratePasswordDigest_MatchesReference(t *testing.T) {
	nonceB64 := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789"))
	created := "2024-06-01T12:00:00Z"
	got := generatePasswordDigest(nonceB64, created, "hunter2")
	want := makeDigest(nonceB64, created, "hunter2")
	if got != want {
		t.Errorf("digest mismatch: got %q want %q", got, want)
	}
}
