package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_AppliesDefaultCredentials(t *testing.T) {
	path := writeTemp(t, `
cameras:
  - name: cam1
    http_port: 8081
    rtsp_stream: /cam1
rtsp_port: 554
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Username != "admin" || cfg.Password != "admin" {
		t.Errorf("expected default admin/admin credentials, got %q/%q", cfg.Username, cfg.Password)
	}
	if len(cfg.Cameras) != 1 || cfg.Cameras[0].Name != "cam1" {
		t.Errorf("unexpected cameras parsed: %+v", cfg.Cameras)
	}
	if cfg.RTSPPort != 554 {
		t.Errorf("expected RTSPPort 554, got %d", cfg.RTSPPort)
	}
}

func TestLoad_PreservesExplicitCredentials(t *testing.T) {
	path := writeTemp(t, `
username: alice
password: s3cret
cameras: []
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Username != "alice" || cfg.Password != "s3cret" {
		t.Errorf("expected alice/s3cret, got %q/%q", cfg.Username, cfg.Password)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "this: is: not: valid: yaml: [")
	if _, err := Load(path); err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
}

func TestLoad_AppliesDefaultMetrics(t *testing.T) {
	path := writeTemp(t, `cameras: []`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Metrics.Enabled {
		t.Error("expected Metrics.Enabled=false by default")
	}
	if cfg.Metrics.OTLPEndpoint != "" {
		t.Errorf("OTLPEndpoint = %q, want empty (no default)", cfg.Metrics.OTLPEndpoint)
	}
	if cfg.Metrics.ServiceName != "onvif-server" {
		t.Errorf("ServiceName = %q, want onvif-server", cfg.Metrics.ServiceName)
	}
}

func TestLoad_RejectsEnabledMetricsWithoutEndpoint(t *testing.T) {
	path := writeTemp(t, `
cameras: []
metrics:
  enabled: true
`)
	_, err := Load(path)
	if !errors.Is(err, ErrMetricsEndpointRequired) {
		t.Fatalf("expected ErrMetricsEndpointRequired, got %v", err)
	}
}

func TestLoad_PreservesExplicitMetrics(t *testing.T) {
	path := writeTemp(t, `
cameras: []
metrics:
  enabled: true
  otlp_endpoint: collector.example.com:4317
  insecure: false
  service_name: my-onvif
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Metrics.Enabled {
		t.Error("expected Metrics.Enabled=true")
	}
	if cfg.Metrics.OTLPEndpoint != "collector.example.com:4317" {
		t.Errorf("OTLPEndpoint = %q", cfg.Metrics.OTLPEndpoint)
	}
	if cfg.Metrics.Insecure {
		t.Error("expected Insecure=false")
	}
	if cfg.Metrics.ServiceName != "my-onvif" {
		t.Errorf("ServiceName = %q", cfg.Metrics.ServiceName)
	}
}
