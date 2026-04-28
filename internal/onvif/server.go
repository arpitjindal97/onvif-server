// Package onvif implements a virtual ONVIF server that fronts an existing
// RTSP stream so that ONVIF clients (NVRs) can discover and consume it.
package onvif

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/aragarwal/onvif-server/internal/config"
	"github.com/aragarwal/onvif-server/internal/logger"
)

// Server represents a single virtual ONVIF camera.
type Server struct {
	config         config.CameraConfig
	rtspHost       string
	rtspPort       int
	username       string
	password       string
	timeSettings   *clientTimeSettings
	timeSettingsMu sync.RWMutex
	mu             sync.RWMutex
	streamInfo     *StreamInfo
	substreamInfo  *StreamInfo
	streamInfoMu   sync.RWMutex
	bitrateCache   map[string]int
	bitrateMu      sync.RWMutex
}

// NewServer constructs a Server with default stream metadata and kicks off
// asynchronous stream-property detection.
func NewServer(cfg config.CameraConfig, rtspHost string, rtspPort int, username, password string) *Server {
	s := &Server{
		config:       cfg,
		rtspHost:     rtspHost,
		rtspPort:     rtspPort,
		username:     username,
		password:     password,
		timeSettings: nil,
		streamInfo: &StreamInfo{
			Width:     1920,
			Height:    1080,
			Codec:     "H264",
			FrameRate: 25,
			BitRate:   4096,
			Profile:   "High",
		},
		substreamInfo: &StreamInfo{
			Width:     640,
			Height:    480,
			Codec:     "H264",
			FrameRate: 15,
			BitRate:   512,
			Profile:   "Baseline",
		},
		bitrateCache: make(map[string]int),
	}

	go s.detectStreamInfo(false)
	if cfg.SubstreamEnabled {
		go s.detectStreamInfo(true)
	}

	return s
}

// CameraName returns the camera's configured name (used by external callers
// such as the stream-detection routine for logging).
func (s *Server) CameraName() string { return s.config.Name }

// SubstreamEnabled returns whether the substream is enabled for this camera.
func (s *Server) SubstreamEnabled() bool { return s.config.SubstreamEnabled }

// Start begins serving HTTP on the configured port. It blocks.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// ONVIF endpoints
	mux.HandleFunc("/onvif/device_service", s.handleRequest)
	mux.HandleFunc("/onvif/media_service", s.handleRequest)
	mux.HandleFunc("/onvif/media2_service", s.handleRequest)
	mux.HandleFunc("/onvif/event_service", s.handleRequest)
	mux.HandleFunc("/onvif/imaging_service", s.handleRequest)
	mux.HandleFunc("/onvif/ptz_service", s.handleRequest)
	mux.HandleFunc("/onvif/analytics_service", s.handleRequest)
	mux.HandleFunc("/onvif/service", s.handleRequest)

	// Snapshot endpoint
	mux.HandleFunc("/snapshot", s.handleSnapshot)
	mux.HandleFunc("/onvif/snapshot", s.handleSnapshot)

	addr := fmt.Sprintf(":%d", s.config.HTTPPort)
	logger.Info("Starting ONVIF server for '%s' on %s", s.config.Name, addr)

	return http.ListenAndServe(addr, mux)
}

// handleRequest is the common HTTP handler for all ONVIF service endpoints.
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	logger.Debug("[%s] Request to %s:\n%s", s.config.Name, r.URL.Path, string(body))

	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Header  struct {
			Security *security `xml:"wsse:Security"`
		} `xml:"Header"`
		Body struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		logger.Debug("[%s] Failed to parse SOAP: %v", s.config.Name, err)
		s.sendSOAPFault(w, "s:Sender", "ter:InvalidArgVal", "Invalid SOAP request")
		return
	}

	if envelope.Header.Security != nil {
		if !s.validateSecurity(envelope.Header.Security) {
			logger.Debug("[%s] Authentication failed", s.config.Name)
			s.sendSOAPFault(w, "s:Sender", "ter:NotAuthorized", "Authentication failed")
			return
		}
	}

	bodyContent := string(envelope.Body.Content)
	s.routeRequest(w, r, bodyContent)
}

// routeRequest dispatches a SOAP request body to the appropriate handler.
func (s *Server) routeRequest(w http.ResponseWriter, r *http.Request, bodyContent string) {
	var fullBody []byte
	if strings.Contains(bodyContent, "SetSystemDateAndTime") {
		r.Body = io.NopCloser(strings.NewReader(bodyContent))
		fullBody, _ = io.ReadAll(r.Body)
	}

	switch {
	// Device Service
	case strings.Contains(bodyContent, "GetSystemDateAndTime"):
		s.handleGetSystemDateAndTime(w, r)
	case strings.Contains(bodyContent, "GetDeviceInformation"):
		s.handleGetDeviceInformation(w, r)
	case strings.Contains(bodyContent, "GetCapabilities"):
		s.handleGetCapabilities(w, r)
	case strings.Contains(bodyContent, "GetServices"):
		s.handleGetServices(w, r)
	case strings.Contains(bodyContent, "GetScopes"):
		s.handleGetScopes(w, r)
	case strings.Contains(bodyContent, "GetHostname"):
		s.handleGetHostname(w, r)
	case strings.Contains(bodyContent, "GetDNS"):
		s.handleGetDNS(w, r)
	case strings.Contains(bodyContent, "GetNetworkInterfaces"):
		s.handleGetNetworkInterfaces(w, r)
	case strings.Contains(bodyContent, "GetNetworkProtocols"):
		s.handleGetNetworkProtocols(w, r)
	case strings.Contains(bodyContent, "SystemReboot"):
		s.handleSystemReboot(w, r)
	case strings.Contains(bodyContent, "SetSystemDateAndTime"):
		s.handleSetSystemDateAndTime(w, r, fullBody)

	// Media Service
	case strings.Contains(bodyContent, "GetProfiles"):
		s.handleGetProfiles(w, r)
	case strings.Contains(bodyContent, "GetStreamUri"):
		s.handleGetStreamUri(w, r, bodyContent)
	case strings.Contains(bodyContent, "GetSnapshotUri"):
		s.handleGetSnapshotUri(w, r)
	case strings.Contains(bodyContent, "GetVideoSources"):
		s.handleGetVideoSources(w, r)
	case strings.Contains(bodyContent, "GetAudioSources"):
		s.handleGetAudioSources(w, r)
	case strings.Contains(bodyContent, "GetVideoEncoderConfigurationOptions"):
		s.handleGetVideoEncoderConfigurationOptions(w, r)
	case strings.Contains(bodyContent, "GetVideoEncoderConfigurations"):
		s.handleGetVideoEncoderConfigurations(w, r)
	case strings.Contains(bodyContent, "GetVideoEncoderConfiguration"):
		s.handleGetVideoEncoderConfiguration(w, r, bodyContent)
	case strings.Contains(bodyContent, "SetVideoEncoderConfiguration"):
		s.handleSetVideoEncoderConfiguration(w, r, bodyContent)

	// Event Service
	case strings.Contains(bodyContent, "Subscribe"):
		s.handleSubscribe(w, r)
	case strings.Contains(bodyContent, "GetEventProperties"):
		s.handleGetEventProperties(w, r)
	case strings.Contains(bodyContent, "CreatePullPointSubscription"):
		s.handleCreatePullPointSubscription(w, r)

	default:
		log.Printf("[%s] Unsupported operation: %s", s.config.Name, bodyContent[:minInt(200, len(bodyContent))])
		s.sendSOAPFault(w, "s:Sender", "ter:ActionNotSupported", "Operation not supported")
	}
}

// getBaseURL returns the base URL (scheme+host+port) for this server.
func (s *Server) getBaseURL(r *http.Request) string {
	hostIP := s.getHostIP(r)
	return fmt.Sprintf("http://%s:%d", hostIP, s.config.HTTPPort)
}

// getHostIP returns the host IP, preferring the request Host header.
func (s *Server) getHostIP(r *http.Request) string {
	host := r.Host
	if host != "" {
		if idx := strings.Index(host, ":"); idx != -1 {
			return host[:idx]
		}
		return host
	}
	return s.rtspHost
}

// handleSnapshot returns a placeholder 1x1 JPEG image.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	jpeg := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x03, 0x02, 0x02, 0x02, 0x02, 0x02, 0x03, 0x02, 0x02, 0x02, 0x03,
		0x03, 0x03, 0x03, 0x04, 0x06, 0x04, 0x04, 0x04, 0x04, 0x04, 0x08, 0x06,
		0x06, 0x05, 0x06, 0x09, 0x08, 0x0A, 0x0A, 0x09, 0x08, 0x09, 0x09, 0x0A,
		0x0C, 0x0F, 0x0C, 0x0A, 0x0B, 0x0E, 0x0B, 0x09, 0x09, 0x0D, 0x11, 0x0D,
		0x0E, 0x0F, 0x10, 0x10, 0x11, 0x10, 0x0A, 0x0C, 0x12, 0x13, 0x12, 0x10,
		0x13, 0x0F, 0x10, 0x10, 0x10, 0xFF, 0xC9, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xCC, 0x00, 0x06, 0x00, 0x10,
		0x10, 0x05, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00,
		0xD2, 0xCF, 0x20, 0xFF, 0xD9,
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(jpeg)))
	w.WriteHeader(http.StatusOK)
	w.Write(jpeg)
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
