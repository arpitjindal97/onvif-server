// Package config loads the YAML configuration for the ONVIF server.
package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	Cameras         []CameraConfig `yaml:"cameras"`
	RTSPHost        string         `yaml:"rtsp_host"`
	RTSPPort        int            `yaml:"rtsp_port"`
	EnableDiscovery bool           `yaml:"enable_discovery"`
	Username        string         `yaml:"username"`
	Password        string         `yaml:"password"`
	Metrics         MetricsConfig  `yaml:"metrics"`
}

// MetricsConfig configures OpenTelemetry metric export over OTLP gRPC.
type MetricsConfig struct {
	Enabled      bool   `yaml:"enabled"`       // master switch
	OTLPEndpoint string `yaml:"otlp_endpoint"` // host:port of the OTLP gRPC collector (required when enabled)
	Insecure     bool   `yaml:"insecure"`      // disable TLS for the OTLP connection
	ServiceName  string `yaml:"service_name"`  // resource attribute service.name (default: onvif-server)
}

// ErrMetricsEndpointRequired is returned by Load when metrics are enabled
// but no OTLP endpoint is configured.
var ErrMetricsEndpointRequired = errors.New(
	"config: metrics.otlp_endpoint is required when metrics.enabled is true",
)

// CameraConfig describes a single virtual ONVIF camera.
type CameraConfig struct {
	Name                 string `yaml:"name"`
	Manufacturer         string `yaml:"manufacturer"`
	Model                string `yaml:"model"`
	Serial               string `yaml:"serial"`
	HTTPPort             int    `yaml:"http_port"`
	RTSPStream           string `yaml:"rtsp_stream"`
	H264Profile          string `yaml:"h264_profile"`
	SubstreamEnabled     bool   `yaml:"substream_enabled"`
	SubstreamPath        string `yaml:"substream_path"`
	SubstreamH264Profile string `yaml:"substream_h264_profile"`
}

// Load reads and parses the configuration file at filename.
// Default credentials (admin/admin) are applied if not provided.
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	if c.Username == "" {
		c.Username = "admin"
	}
	if c.Password == "" {
		c.Password = "admin"
	}
	if c.Metrics.ServiceName == "" {
		c.Metrics.ServiceName = "onvif-server"
	}
	if c.Metrics.Enabled && c.Metrics.OTLPEndpoint == "" {
		return nil, ErrMetricsEndpointRequired
	}

	return &c, nil
}
