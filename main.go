package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config structures
type Config struct {
	Cameras         []CameraConfig `yaml:"cameras"`
	RTSPHost        string         `yaml:"rtsp_host"`
	RTSPPort        int            `yaml:"rtsp_port"`
	EnableDiscovery bool           `yaml:"enable_discovery"`
}

type CameraConfig struct {
	Name             string `yaml:"name"`
	Manufacturer     string `yaml:"manufacturer"`
	Model            string `yaml:"model"`
	Serial           string `yaml:"serial"`
	HTTPPort         int    `yaml:"http_port"`
	RTSPStream       string `yaml:"rtsp_stream"`
	SubstreamEnabled bool   `yaml:"substream_enabled"`
	SubstreamPath    string `yaml:"substream_path"`
}

// SOAP Envelope structures
type SOAPEnvelope struct {
	XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
	SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
	TT      string      `xml:"xmlns:tt,attr"`
	TDS     string      `xml:"xmlns:tds,attr"`
	TRT     string      `xml:"xmlns:trt,attr"`
	Body    interface{} `xml:"SOAP-ENV:Body"`
}

type SOAPBody struct {
	Content interface{} `xml:",any"`
}

type SOAPFault struct {
	XMLName xml.Name `xml:"SOAP-ENV:Fault"`
	Code    struct {
		Value string `xml:"SOAP-ENV:Value"`
	} `xml:"SOAP-ENV:Code"`
	Reason struct {
		Text string `xml:"SOAP-ENV:Text"`
	} `xml:"SOAP-ENV:Reason"`
}

// ONVIF Device Service
type GetSystemDateAndTime struct {
	XMLName xml.Name `xml:"http://www.onvif.org/ver10/device/wsdl GetSystemDateAndTime"`
}

type GetSystemDateAndTimeResponse struct {
	XMLName           xml.Name          `xml:"tds:GetSystemDateAndTimeResponse"`
	SystemDateAndTime SystemDateAndTime `xml:"tds:SystemDateAndTime"`
}

type SetSystemDateAndTime struct {
	XMLName         xml.Name `xml:"http://www.onvif.org/ver10/device/wsdl SetSystemDateAndTime"`
	DateTimeType    string   `xml:"DateTimeType"`
	DaylightSavings bool     `xml:"DaylightSavings"`
	TimeZone        *TimeZone `xml:"TimeZone,omitempty"`
	UTCDateTime     *DateTime `xml:"UTCDateTime,omitempty"`
}

type SetSystemDateAndTimeResponse struct {
	XMLName xml.Name `xml:"tds:SetSystemDateAndTimeResponse"`
}

type SystemDateAndTime struct {
	DateTimeType    string   `xml:"tt:DateTimeType"`
	DaylightSavings bool     `xml:"tt:DaylightSavings"`
	TimeZone        TimeZone `xml:"tt:TimeZone"`
	UTCDateTime     DateTime `xml:"tt:UTCDateTime"`
	LocalDateTime   DateTime `xml:"tt:LocalDateTime"`
}

type TimeZone struct {
	TZ string `xml:"tt:TZ"`
}

type DateTime struct {
	Time Time `xml:"tt:Time"`
	Date Date `xml:"tt:Date"`
}

type Time struct {
	Hour   int `xml:"tt:Hour"`
	Minute int `xml:"tt:Minute"`
	Second int `xml:"tt:Second"`
}

type Date struct {
	Year  int `xml:"tt:Year"`
	Month int `xml:"tt:Month"`
	Day   int `xml:"tt:Day"`
}

type GetDeviceInformation struct {
	XMLName xml.Name `xml:"http://www.onvif.org/ver10/device/wsdl GetDeviceInformation"`
}

type GetDeviceInformationResponse struct {
	XMLName         xml.Name `xml:"tds:GetDeviceInformationResponse"`
	Manufacturer    string   `xml:"tds:Manufacturer"`
	Model           string   `xml:"tds:Model"`
	FirmwareVersion string   `xml:"tds:FirmwareVersion"`
	SerialNumber    string   `xml:"tds:SerialNumber"`
	HardwareId      string   `xml:"tds:HardwareId"`
}

type GetCapabilities struct {
	XMLName xml.Name `xml:"http://www.onvif.org/ver10/device/wsdl GetCapabilities"`
}

type GetCapabilitiesResponse struct {
	XMLName      xml.Name     `xml:"tds:GetCapabilitiesResponse"`
	Capabilities Capabilities `xml:"tds:Capabilities"`
}

type Capabilities struct {
	Media MediaCapabilities `xml:"tt:Media"`
	Events EventCapabilities `xml:"tt:Events"`
}

type MediaCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type EventCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type GetServices struct {
	XMLName xml.Name `xml:"http://www.onvif.org/ver10/device/wsdl GetServices"`
}

type GetServicesResponse struct {
	XMLName xml.Name  `xml:"tds:GetServicesResponse"`
	Service []Service `xml:"tds:Service"`
}

type Service struct {
	Namespace string  `xml:"tds:Namespace"`
	XAddr     string  `xml:"tds:XAddr"`
	Version   Version `xml:"tds:Version"`
}

type Version struct {
	Major int `xml:"tt:Major"`
	Minor int `xml:"tt:Minor"`
}

// ONVIF Media Service
type GetProfiles struct {
	XMLName xml.Name `xml:"http://www.onvif.org/ver10/media/wsdl GetProfiles"`
}

type GetProfilesResponse struct {
	XMLName  xml.Name  `xml:"trt:GetProfilesResponse"`
	Profiles []Profile `xml:"trt:Profiles"`
}

type GetProfile struct {
	XMLName      xml.Name `xml:"http://www.onvif.org/ver10/media/wsdl GetProfile"`
	ProfileToken string   `xml:"ProfileToken"`
}

type GetProfileResponse struct {
	XMLName xml.Name `xml:"trt:GetProfileResponse"`
	Profile Profile  `xml:"trt:Profile"`
}

type GetVideoEncoderConfiguration struct {
	XMLName            xml.Name `xml:"http://www.onvif.org/ver10/media/wsdl GetVideoEncoderConfiguration"`
	ConfigurationToken string   `xml:"ConfigurationToken"`
}

type GetVideoEncoderConfigurationResponse struct {
	XMLName       xml.Name                  `xml:"trt:GetVideoEncoderConfigurationResponse"`
	Configuration VideoEncoderConfiguration `xml:"trt:Configuration"`
}

type GetVideoEncoderConfigurationOptions struct {
	XMLName            xml.Name `xml:"http://www.onvif.org/ver10/media/wsdl GetVideoEncoderConfigurationOptions"`
	ConfigurationToken string   `xml:"ConfigurationToken"`
}

type GetVideoEncoderConfigurationOptionsResponse struct {
	XMLName xml.Name                         `xml:"trt:GetVideoEncoderConfigurationOptionsResponse"`
	Options VideoEncoderConfigurationOptions `xml:"trt:Options"`
}

type VideoEncoderConfigurationOptions struct {
	QualityRange IntRange                 `xml:"tt:QualityRange"`
	H264         H264ConfigurationOptions `xml:"tt:H264,omitempty"`
}

type IntRange struct {
	Min int `xml:"tt:Min"`
	Max int `xml:"tt:Max"`
}

type H264ConfigurationOptions struct {
	ResolutionsAvailable  []VideoResolution `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        IntRange          `xml:"tt:GovLengthRange"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	H264ProfilesSupported []string          `xml:"tt:H264ProfilesSupported"`
}

type VideoResolution struct {
	Width  int `xml:"tt:Width"`
	Height int `xml:"tt:Height"`
}

type Profile struct {
	Token                     string                    `xml:"token,attr"`
	Fixed                     bool                      `xml:"fixed,attr"`
	Name                      string                    `xml:"tt:Name"`
	VideoSourceConfiguration  VideoSourceConfiguration  `xml:"tt:VideoSourceConfiguration"`
	VideoEncoderConfiguration VideoEncoderConfiguration `xml:"tt:VideoEncoderConfiguration"`
}

type VideoSourceConfiguration struct {
	Token       string `xml:"token,attr"`
	Name        string `xml:"tt:Name"`
	UseCount    int    `xml:"tt:UseCount"`
	SourceToken string `xml:"tt:SourceToken"`
	Bounds      Bounds `xml:"tt:Bounds"`
}

type Bounds struct {
	X      int `xml:"x,attr"`
	Y      int `xml:"y,attr"`
	Width  int `xml:"width,attr"`
	Height int `xml:"height,attr"`
}

type VideoEncoderConfiguration struct {
	Token          string      `xml:"token,attr"`
	Name           string      `xml:"tt:Name"`
	UseCount       int         `xml:"tt:UseCount"`
	Encoding       string      `xml:"tt:Encoding"`
	Resolution     Resolution  `xml:"tt:Resolution"`
	Quality        int         `xml:"tt:Quality"`
	RateControl    RateControl `xml:"tt:RateControl"`
	H264           *H264Config `xml:"tt:H264,omitempty"`
	Multicast      Multicast   `xml:"tt:Multicast"`
	SessionTimeout string      `xml:"tt:SessionTimeout"`
}

type H264Config struct {
	GovLength   int    `xml:"tt:GovLength"`
	H264Profile string `xml:"tt:H264Profile"`
}

type Multicast struct {
	Address   MulticastAddress `xml:"tt:Address"`
	Port      int              `xml:"tt:Port"`
	TTL       int              `xml:"tt:TTL"`
	AutoStart bool             `xml:"tt:AutoStart"`
}

type MulticastAddress struct {
	Type        string `xml:"tt:Type"`
	IPv4Address string `xml:"tt:IPv4Address,omitempty"`
}

type Resolution struct {
	Width  int `xml:"tt:Width"`
	Height int `xml:"tt:Height"`
}

type RateControl struct {
	FrameRateLimit   int `xml:"tt:FrameRateLimit"`
	EncodingInterval int `xml:"tt:EncodingInterval"`
	BitrateLimit     int `xml:"tt:BitrateLimit"`
}

type GetStreamUri struct {
	XMLName      xml.Name    `xml:"http://www.onvif.org/ver10/media/wsdl GetStreamUri"`
	ProfileToken string      `xml:"ProfileToken"`
	StreamSetup  StreamSetup `xml:"StreamSetup"`
}

type StreamSetup struct {
	Stream    string    `xml:"Stream"`
	Transport Transport `xml:"Transport"`
}

type Transport struct {
	Protocol string `xml:"Protocol"`
}

type GetStreamUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetStreamUriResponse"`
	MediaUri MediaUri `xml:"trt:MediaUri"`
}

type MediaUri struct {
	Uri                 string `xml:"tt:Uri"`
	InvalidAfterConnect bool   `xml:"tt:InvalidAfterConnect"`
	InvalidAfterReboot  bool   `xml:"tt:InvalidAfterReboot"`
	Timeout             string `xml:"tt:Timeout"`
}

type GetSnapshotUri struct {
	XMLName      xml.Name `xml:"http://www.onvif.org/ver10/media/wsdl GetSnapshotUri"`
	ProfileToken string   `xml:"ProfileToken"`
}

type GetSnapshotUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetSnapshotUriResponse"`
	MediaUri MediaUri `xml:"trt:MediaUri"`
}

// Network Interfaces
type GetNetworkInterfacesResponse struct {
	XMLName           xml.Name           `xml:"tds:GetNetworkInterfacesResponse"`
	NetworkInterfaces []NetworkInterface `xml:"tds:NetworkInterfaces"`
}

type NetworkInterface struct {
	Token   string          `xml:"token,attr"`
	Enabled bool            `xml:"tt:Enabled"`
	Info    NetworkInfo     `xml:"tt:Info,omitempty"`
	IPv4    *IPv4Config     `xml:"tt:IPv4,omitempty"`
	IPv6    *IPv6Config     `xml:"tt:IPv6,omitempty"`
}

type NetworkInfo struct {
	Name       string `xml:"tt:Name,omitempty"`
	HwAddress  string `xml:"tt:HwAddress"`
	MTU        int    `xml:"tt:MTU,omitempty"`
}

type IPv4Config struct {
	Enabled bool        `xml:"tt:Enabled"`
	Config  IPv4Network `xml:"tt:Config"`
}

type IPv4Network struct {
	Manual []IPv4Address `xml:"tt:Manual,omitempty"`
	DHCP   bool          `xml:"tt:DHCP"`
}

type IPv4Address struct {
	Address      string `xml:"tt:Address"`
	PrefixLength int    `xml:"tt:PrefixLength"`
}

type IPv6Config struct {
	Enabled bool `xml:"tt:Enabled"`
}

// SetSynchronizationPoint
type SetSynchronizationPointResponse struct {
	XMLName xml.Name `xml:"trt:SetSynchronizationPointResponse"`
}

// OSD Options
type GetOSDOptionsResponse struct {
	XMLName    xml.Name   `xml:"trt:GetOSDOptionsResponse"`
	OSDOptions OSDOptions `xml:"trt:OSDOptions"`
}

type OSDOptions struct {
	MaximumNumberOfOSDs      int              `xml:"tt:MaximumNumberOfOSDs,omitempty"`
	Type                     []string         `xml:"tt:Type,omitempty"`
	PositionOption           []string         `xml:"tt:PositionOption,omitempty"`
	TextOption               *TextOptions     `xml:"tt:TextOption,omitempty"`
}

type TextOptions struct {
	Type                []string `xml:"tt:Type,omitempty"`
	FontSizeRange       *IntRange `xml:"tt:FontSizeRange,omitempty"`
	FontColor           []string `xml:"tt:FontColor,omitempty"`
}

// Audio Encoder Configuration
type GetAudioEncoderConfigurationResponse struct {
	XMLName       xml.Name                  `xml:"trt:GetAudioEncoderConfigurationResponse"`
	Configuration AudioEncoderConfiguration `xml:"trt:Configuration"`
}

type AudioEncoderConfiguration struct {
	Token          string `xml:"token,attr"`
	Name           string `xml:"tt:Name"`
	UseCount       int    `xml:"tt:UseCount"`
	Encoding       string `xml:"tt:Encoding"`
	Bitrate        int    `xml:"tt:Bitrate"`
	SampleRate     int    `xml:"tt:SampleRate"`
	SessionTimeout string `xml:"tt:SessionTimeout"`
}

type GetAudioEncoderConfigurationOptionsResponse struct {
	XMLName xml.Name                         `xml:"trt:GetAudioEncoderConfigurationOptionsResponse"`
	Options AudioEncoderConfigurationOptions `xml:"trt:Options"`
}

type AudioEncoderConfigurationOptions struct {
	Options []AudioEncoderOption `xml:"tt:Options"`
}

type AudioEncoderOption struct {
	Encoding          string    `xml:"tt:Encoding"`
	BitrateList       *IntList  `xml:"tt:BitrateList,omitempty"`
	SampleRateList    *IntList  `xml:"tt:SampleRateList,omitempty"`
}

// Audio Sources
type GetAudioSourcesResponse struct {
	XMLName      xml.Name      `xml:"trt:GetAudioSourcesResponse"`
	AudioSources []AudioSource `xml:"trt:AudioSources"`
}

type AudioSource struct {
	Token    string `xml:"token,attr"`
	Channels int    `xml:"tt:Channels"`
}

type IntList struct {
	Items []int `xml:"tt:Items"`
}

// WS-Notification structures for event subscriptions
type Subscribe struct {
	XMLName           xml.Name `xml:"http://docs.oasis-open.org/wsn/b-2 Subscribe"`
	ConsumerReference struct {
		Address string `xml:"Address"`
	} `xml:"ConsumerReference"`
	Filter *struct {
		TopicExpression string `xml:"TopicExpression,omitempty"`
	} `xml:"Filter,omitempty"`
	InitialTerminationTime *string `xml:"InitialTerminationTime,omitempty"`
}

type SubscribeResponse struct {
	XMLName              xml.Name `xml:"wsnt:SubscribeResponse"`
	SubscriptionReference struct {
		Address string `xml:"wsa5:Address"`
	} `xml:"wsnt:SubscriptionReference"`
	CurrentTime    string `xml:"wsnt:CurrentTime,omitempty"`
	TerminationTime string `xml:"wsnt:TerminationTime,omitempty"`
}

// ONVIFServer represents an ONVIF server for a single camera
type ONVIFServer struct {
	config        CameraConfig
	rtspHost      string
	rtspPort      int
	streamInfo    *StreamInfo
	substreamInfo *StreamInfo
	timeOffsets   map[string]time.Duration // Per-client time offsets (keyed by WS-Security username)
	timeOffsetsMu sync.RWMutex             // Protects timeOffsets map
}

// StreamInfo holds detected stream properties
type StreamInfo struct {
	Width     int
	Height    int
	Codec     string
	FrameRate int
	BitRate   int
	Profile   string
}

func NewONVIFServer(config CameraConfig, rtspHost string, rtspPort int) *ONVIFServer {
	server := &ONVIFServer{
		config:      config,
		rtspHost:    rtspHost,
		rtspPort:    rtspPort,
		timeOffsets: make(map[string]time.Duration),
		streamInfo: &StreamInfo{
			// Defaults for main stream
			Width:     1920,
			Height:    1080,
			Codec:     "H264",
			FrameRate: 25,
			BitRate:   2048,
			Profile:   "Main",
		},
		substreamInfo: &StreamInfo{
			// Defaults for substream (lower quality)
			Width:     640,
			Height:    480,
			Codec:     "H264",
			FrameRate: 15,
			BitRate:   512,
			Profile:   "Baseline",
		},
	}

	// Detect stream info asynchronously (non-blocking)
	go server.detectStreamInfo(false) // Main stream

	// Detect substream info if enabled
	if config.SubstreamEnabled {
		go server.detectStreamInfo(true) // Substream
	}

	return server
}

// detectStreamInfo uses ffprobe to detect stream properties
func (s *ONVIFServer) detectStreamInfo(isSubstream bool) {
	streamPath := s.config.RTSPStream
	streamType := "main"
	targetInfo := s.streamInfo

	if isSubstream {
		streamPath = s.config.SubstreamPath
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub" // Default fallback
		}
		streamType = "sub"
		targetInfo = s.substreamInfo
	}

	rtspURL := fmt.Sprintf("rtsp://localhost:%d%s", s.rtspPort, streamPath)

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,bit_rate,profile",
		"-of", "json",
		rtspURL,
	)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("[%s] Failed to detect %s stream info: %v (using defaults)", s.config.Name, streamType, err)
		return
	}

	var result struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			BitRate    string `json:"bit_rate"`
			Profile    string `json:"profile"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("[%s] Failed to parse ffprobe output for %s stream: %v", s.config.Name, streamType, err)
		return
	}

	if len(result.Streams) == 0 {
		log.Printf("[%s] No video stream found for %s stream", s.config.Name, streamType)
		return
	}

	stream := result.Streams[0]

	// Update stream info
	if stream.Width > 0 {
		targetInfo.Width = stream.Width
	}
	if stream.Height > 0 {
		targetInfo.Height = stream.Height
	}
	if stream.CodecName != "" {
		codec := strings.ToUpper(stream.CodecName)
		// Map HEVC to H265 for NVR compatibility
		if codec == "HEVC" {
			codec = "H265"
		}
		targetInfo.Codec = codec
	}
	if stream.Profile != "" {
		targetInfo.Profile = stream.Profile
	}

	// Parse frame rate (e.g., "25/1" -> 25)
	if stream.RFrameRate != "" {
		parts := strings.Split(stream.RFrameRate, "/")
		if len(parts) == 2 {
			num, _ := strconv.Atoi(parts[0])
			den, _ := strconv.Atoi(parts[1])
			if den > 0 {
				targetInfo.FrameRate = num / den
			}
		}
	}

	// Parse bit rate (convert from bps to kbps)
	if stream.BitRate != "" {
		bitRate, _ := strconv.Atoi(stream.BitRate)
		if bitRate > 0 {
			targetInfo.BitRate = bitRate / 1024
		}
	}

	log.Printf("[%s] Detected %s stream: %dx%d %s %dfps %dkbps Profile:%s",
		s.config.Name,
		streamType,
		targetInfo.Width,
		targetInfo.Height,
		targetInfo.Codec,
		targetInfo.FrameRate,
		targetInfo.BitRate,
		targetInfo.Profile,
	)
}

// getClientIdentifier extracts the WS-Security username from the SOAP request body
// This is used to uniquely identify different NVRs even if they share the same IP (e.g., via Tailscale)
func (s *ONVIFServer) getClientIdentifier(body []byte) string {
	// Parse WS-Security username from SOAP envelope
	var envelope struct {
		Header struct {
			Security struct {
				UsernameToken struct {
					Username string `xml:"Username"`
				} `xml:"UsernameToken"`
			} `xml:"Security"`
		} `xml:"Header"`
	}

	if err := xml.Unmarshal(body, &envelope); err == nil {
		if envelope.Header.Security.UsernameToken.Username != "" {
			return envelope.Header.Security.UsernameToken.Username
		}
	}

	// Fallback to "default" if no username found
	return "default"
}

// getONVIFEncoding returns ONVIF-compatible encoding value
// H265/HEVC is reported as H264 for maximum NVR compatibility
// since ONVIF Profile S only officially supports H264
func (s *ONVIFServer) getONVIFEncoding(codec string) string {
	// Map H265 to H264 for ONVIF Profile S compatibility
	// Most NVRs will accept the actual H265 RTSP stream regardless
	if codec == "H265" || codec == "HEVC" {
		return "H264"
	}
	return codec
}

// startStreamInfoRefresh periodically refreshes stream info
func (s *ONVIFServer) startStreamInfoRefresh(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			s.detectStreamInfo(false) // Main stream
			if s.config.SubstreamEnabled {
				s.detectStreamInfo(true) // Substream
			}
		}
	}()
}

func (s *ONVIFServer) Start() error {
	mux := http.NewServeMux()

	// Common endpoint used by many NVRs - routes based on SOAP body
	mux.HandleFunc("/onvif/service", s.handleUnifiedService)

	// Standard ONVIF endpoints
	mux.HandleFunc("/onvif/device_service", s.handleDeviceService)
	mux.HandleFunc("/onvif/media_service", s.handleMediaService)
	mux.HandleFunc("/onvif/event_service", s.handleEventService)

	addr := fmt.Sprintf(":%d", s.config.HTTPPort)
	log.Printf("Starting ONVIF server for '%s' on %s (RTSP: rtsp://%s:%d%s)",
		s.config.Name, addr, s.rtspHost, s.rtspPort, s.config.RTSPStream)

	return http.ListenAndServe(addr, mux)
}

// handleUnifiedService routes requests to the appropriate service based on SOAP body
func (s *ONVIFServer) handleUnifiedService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	bodyContent := string(body)
	log.Printf("[%s] Unified Service Request [%s]:\n%s", s.config.Name, r.RequestURI, bodyContent)

	// Check for specific operations first (more specific than namespace checks)
	// Media Service operations
	if strings.Contains(bodyContent, "GetProfiles") ||
		strings.Contains(bodyContent, "GetProfile") ||
		strings.Contains(bodyContent, "GetStreamUri") ||
		strings.Contains(bodyContent, "GetSnapshotUri") ||
		strings.Contains(bodyContent, "GetVideoEncoderConfiguration") ||
		strings.Contains(bodyContent, "GetAudioSources") ||
		strings.Contains(bodyContent, "GetAudioEncoderConfiguration") {
		log.Printf("[%s] Routing to Media Service [%s]", s.config.Name, r.RequestURI)
		s.processMediaRequest(w, r, body)
	} else if strings.Contains(bodyContent, "Subscribe") {
		log.Printf("[%s] Routing to Event Service [%s]", s.config.Name, r.RequestURI)
		s.processEventRequest(w, r, body)
	} else if strings.Contains(bodyContent, "GetDeviceInformation") ||
		strings.Contains(bodyContent, "GetSystemDateAndTime") ||
		strings.Contains(bodyContent, "SetSystemDateAndTime") ||
		strings.Contains(bodyContent, "GetCapabilities") ||
		strings.Contains(bodyContent, "GetServices") {
		log.Printf("[%s] Routing to Device Service [%s]", s.config.Name, r.RequestURI)
		s.processDeviceRequest(w, r, body)
	} else {
		log.Printf("[%s] Unknown ONVIF request [%s]: %s", s.config.Name, r.RequestURI, bodyContent[:min(200, len(bodyContent))])
		s.sendSOAPFault(w, "Client", "Unsupported operation")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *ONVIFServer) handleDeviceService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Check if this is a Subscribe request (should be routed to event service)
	bodyContent := string(body)
	if strings.Contains(bodyContent, "Subscribe") {
		log.Printf("[%s] Routing Subscribe from Device Service to Event Service", s.config.Name)
		s.processEventRequest(w, r, body)
		return
	}

	s.processDeviceRequest(w, r, body)
}

func (s *ONVIFServer) processDeviceRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	// Parse SOAP envelope
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Printf("[%s] Device Service - Failed to parse SOAP: %v", s.config.Name, err)
		s.sendSOAPFault(w, "Client", "Invalid SOAP request")
		return
	}

	bodyContent := string(envelope.Body.Content)

	// Route based on SOAP action
	if strings.Contains(bodyContent, "GetSystemDateAndTime") {
		log.Printf("[%s] Device Service - GetSystemDateAndTime", s.config.Name)
		s.handleGetSystemDateAndTime(w, body)
	} else if strings.Contains(bodyContent, "SetSystemDateAndTime") {
		log.Printf("[%s] Device Service - SetSystemDateAndTime", s.config.Name)
		s.handleSetSystemDateAndTime(w, body)
	} else if strings.Contains(bodyContent, "GetDeviceInformation") {
		log.Printf("[%s] Device Service - GetDeviceInformation", s.config.Name)
		s.handleGetDeviceInformation(w)
	} else if strings.Contains(bodyContent, "GetCapabilities") {
		log.Printf("[%s] Device Service - GetCapabilities", s.config.Name)
		s.handleGetCapabilities(w, r)
	} else if strings.Contains(bodyContent, "GetServices") {
		log.Printf("[%s] Device Service - GetServices", s.config.Name)
		s.handleGetServices(w, r)
	} else if strings.Contains(bodyContent, "GetNetworkInterfaces") {
		log.Printf("[%s] Device Service - GetNetworkInterfaces", s.config.Name)
		s.handleGetNetworkInterfaces(w, r)
	} else {
		log.Printf("[%s] Device Service - Unsupported operation: %s", s.config.Name, bodyContent[:min(100, len(bodyContent))])
		s.sendSOAPFault(w, "Client", "Unsupported operation")
	}
}

func (s *ONVIFServer) handleGetSystemDateAndTime(w http.ResponseWriter, body []byte) {
	username := s.getClientIdentifier(body)

	// Get client-specific time offset
	s.timeOffsetsMu.RLock()
	offset := s.timeOffsets[username]
	s.timeOffsetsMu.RUnlock()

	now := time.Now().Add(offset)
	utc := now.UTC()

	response := GetSystemDateAndTimeResponse{
		SystemDateAndTime: SystemDateAndTime{
			DateTimeType:    "NTP",
			DaylightSavings: false,
			TimeZone: TimeZone{
				TZ: "UTC",
			},
			UTCDateTime: DateTime{
				Time: Time{
					Hour:   utc.Hour(),
					Minute: utc.Minute(),
					Second: utc.Second(),
				},
				Date: Date{
					Year:  utc.Year(),
					Month: int(utc.Month()),
					Day:   utc.Day(),
				},
			},
			LocalDateTime: DateTime{
				Time: Time{
					Hour:   now.Hour(),
					Minute: now.Minute(),
					Second: now.Second(),
				},
				Date: Date{
					Year:  now.Year(),
					Month: int(now.Month()),
					Day:   now.Day(),
				},
			},
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSetSystemDateAndTime(w http.ResponseWriter, body []byte) {
	username := s.getClientIdentifier(body)
	log.Printf("[%s] SetSystemDateAndTime requested from user '%s'", s.config.Name, username)

	// Parse the incoming request
	var envelope struct {
		Body struct {
			SetSystemDateAndTime SetSystemDateAndTime
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err == nil {
		req := envelope.Body.SetSystemDateAndTime

		// If UTCDateTime is provided, calculate offset
		if req.UTCDateTime != nil {
			requestedTime := time.Date(
				req.UTCDateTime.Date.Year,
				time.Month(req.UTCDateTime.Date.Month),
				req.UTCDateTime.Date.Day,
				req.UTCDateTime.Time.Hour,
				req.UTCDateTime.Time.Minute,
				req.UTCDateTime.Time.Second,
				0,
				time.UTC,
			)

			currentTime := time.Now().UTC()
			offset := requestedTime.Sub(currentTime)

			// Store offset for this username
			s.timeOffsetsMu.Lock()
			s.timeOffsets[username] = offset
			s.timeOffsetsMu.Unlock()

			log.Printf("[%s] Time offset for user '%s' set to %v (NVR time: %v, System time: %v)",
				s.config.Name, username, offset, requestedTime.Format(time.RFC3339), currentTime.Format(time.RFC3339))
		}
	}

	response := SetSystemDateAndTimeResponse{}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetDeviceInformation(w http.ResponseWriter) {
	response := GetDeviceInformationResponse{
		Manufacturer:    s.config.Manufacturer,
		Model:           s.config.Model,
		FirmwareVersion: "1.0.0",
		SerialNumber:    s.config.Serial,
		HardwareId:      "ONVIF-1.0",
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	baseURL := fmt.Sprintf("http://%s:%d", s.getHostIP(r), s.config.HTTPPort)

	response := GetCapabilitiesResponse{
		Capabilities: Capabilities{
			Media: MediaCapabilities{
				XAddr: baseURL + "/onvif/service",
			},
			Events: EventCapabilities{
				XAddr: baseURL + "/onvif/service",
			},
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetServices(w http.ResponseWriter, r *http.Request) {
	baseURL := fmt.Sprintf("http://%s:%d", s.getHostIP(r), s.config.HTTPPort)

	response := GetServicesResponse{
		Service: []Service{
			{
				Namespace: "http://www.onvif.org/ver10/device/wsdl",
				XAddr:     baseURL + "/onvif/service",
				Version:   Version{Major: 2, Minor: 5},
			},
			{
				Namespace: "http://www.onvif.org/ver10/media/wsdl",
				XAddr:     baseURL + "/onvif/service",
				Version:   Version{Major: 2, Minor: 5},
			},
			{
				Namespace: "http://www.onvif.org/ver10/events/wsdl",
				XAddr:     baseURL + "/onvif/service",
				Version:   Version{Major: 2, Minor: 5},
			},
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	// Return a basic network interface configuration
	response := GetNetworkInterfacesResponse{
		NetworkInterfaces: []NetworkInterface{
			{
				Token:   "eth0",
				Enabled: true,
				Info: NetworkInfo{
					Name:      "eth0",
					HwAddress: "00:00:00:00:00:00",
					MTU:       1500,
				},
				IPv4: &IPv4Config{
					Enabled: true,
					Config: IPv4Network{
						DHCP: true,
						Manual: []IPv4Address{},
					},
				},
				IPv6: &IPv6Config{
					Enabled: false,
				},
			},
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleEventService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.processEventRequest(w, r, body)
}

func (s *ONVIFServer) processEventRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Printf("[%s] Event Service - Failed to parse SOAP: %v", s.config.Name, err)
		s.sendSOAPFault(w, "Client", "Invalid SOAP request")
		return
	}

	bodyContent := string(envelope.Body.Content)

	if strings.Contains(bodyContent, "Subscribe") {
		log.Printf("[%s] Event Service - Subscribe", s.config.Name)
		s.handleSubscribe(w, r)
	} else {
		log.Printf("[%s] Event Service - Unsupported operation: %s", s.config.Name, bodyContent[:min(100, len(bodyContent))])
		s.sendSOAPFault(w, "Client", "Unsupported operation")
	}
}

func (s *ONVIFServer) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	// Create a basic subscription response
	// The subscription endpoint is the event service itself
	baseURL := fmt.Sprintf("http://%s:%d", s.getHostIP(r), s.config.HTTPPort)
	subscriptionAddr := baseURL + "/onvif/subscription/events"

	// Set termination time to 1 hour from now
	now := time.Now().UTC()
	terminationTime := now.Add(1 * time.Hour)

	response := SubscribeResponse{}
	response.SubscriptionReference.Address = subscriptionAddr
	response.CurrentTime = now.Format(time.RFC3339)
	response.TerminationTime = terminationTime.Format(time.RFC3339)

	// Build SOAP envelope with proper namespaces
	body := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Body"`
		Content interface{} `xml:",any"`
	}{
		Content: response,
	}

	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		WSNT    string      `xml:"xmlns:wsnt,attr"`
		WSA5    string      `xml:"xmlns:wsa5,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		WSNT:    "http://docs.oasis-open.org/wsn/b-2",
		WSA5:    "http://www.w3.org/2005/08/addressing",
		Body:    body,
	}

	output, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		log.Printf("[%s] Failed to marshal subscription response: %v", s.config.Name, err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseStr := xml.Header + string(output)
	log.Printf("[%s] Sending Subscription Response:\n%s", s.config.Name, responseStr)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseStr))
}

func (s *ONVIFServer) handleMediaService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.processMediaRequest(w, r, body)
}

func (s *ONVIFServer) processMediaRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Printf("[%s] Media Service - Failed to parse SOAP: %v", s.config.Name, err)
		s.sendSOAPFault(w, "Client", "Invalid SOAP request")
		return
	}

	bodyContent := string(envelope.Body.Content)

	if strings.Contains(bodyContent, "GetProfiles") {
		log.Printf("[%s] Media Service - GetProfiles", s.config.Name)
		s.handleGetProfiles(w)
	} else if strings.Contains(bodyContent, "GetProfile") {
		log.Printf("[%s] Media Service - GetProfile", s.config.Name)
		s.handleGetProfile(w, bodyContent)
	} else if strings.Contains(bodyContent, "GetStreamUri") {
		log.Printf("[%s] Media Service - GetStreamUri", s.config.Name)
		s.handleGetStreamUri(w, r, bodyContent)
	} else if strings.Contains(bodyContent, "GetSnapshotUri") {
		log.Printf("[%s] Media Service - GetSnapshotUri", s.config.Name)
		s.handleGetSnapshotUri(w, r)
	} else if strings.Contains(bodyContent, "GetVideoEncoderConfigurationOptions") {
		log.Printf("[%s] Media Service - GetVideoEncoderConfigurationOptions", s.config.Name)
		s.handleGetVideoEncoderConfigurationOptions(w)
	} else if strings.Contains(bodyContent, "GetVideoEncoderConfiguration") {
		log.Printf("[%s] Media Service - GetVideoEncoderConfiguration", s.config.Name)
		s.handleGetVideoEncoderConfiguration(w, bodyContent)
	} else if strings.Contains(bodyContent, "SetSynchronizationPoint") {
		log.Printf("[%s] Media Service - SetSynchronizationPoint", s.config.Name)
		s.handleSetSynchronizationPoint(w, bodyContent)
	} else if strings.Contains(bodyContent, "GetOSDOptions") {
		log.Printf("[%s] Media Service - GetOSDOptions", s.config.Name)
		s.handleGetOSDOptions(w)
	} else if strings.Contains(bodyContent, "GetAudioEncoderConfiguration") {
		log.Printf("[%s] Media Service - GetAudioEncoderConfiguration", s.config.Name)
		s.handleGetAudioEncoderConfiguration(w)
	} else if strings.Contains(bodyContent, "GetAudioEncoderConfigurationOptions") {
		log.Printf("[%s] Media Service - GetAudioEncoderConfigurationOptions", s.config.Name)
		s.handleGetAudioEncoderConfigurationOptions(w)
	} else if strings.Contains(bodyContent, "GetAudioSources") {
		log.Printf("[%s] Media Service - GetAudioSources", s.config.Name)
		s.handleGetAudioSources(w)
	} else {
		log.Printf("[%s] Media Service - Unsupported operation: %s", s.config.Name, bodyContent[:min(100, len(bodyContent))])
		s.sendSOAPFault(w, "Client", "Unsupported operation")
	}
}

func (s *ONVIFServer) handleGetProfiles(w http.ResponseWriter) {
	// Build main stream encoder config
	mainEncoder := VideoEncoderConfiguration{
		Token:    "VideoEncoder_1",
		Name:     "VideoEncoderConfig",
		UseCount: 1,
		Encoding: s.getONVIFEncoding(s.streamInfo.Codec),
		Resolution: Resolution{
			Width:  s.streamInfo.Width,
			Height: s.streamInfo.Height,
		},
		Quality: 5,
		RateControl: RateControl{
			FrameRateLimit:   s.streamInfo.FrameRate,
			EncodingInterval: 1,
			BitrateLimit:     s.streamInfo.BitRate,
		},
		Multicast: Multicast{
			Address: MulticastAddress{
				Type:        "IPv4",
				IPv4Address: "0.0.0.0",
			},
			Port:      0,
			TTL:       1,
			AutoStart: false,
		},
		SessionTimeout: "PT60S",
	}

	// Always add H264 config block for ONVIF compatibility
	// Even for H265 streams (which we report as H264)
	mainEncoder.H264 = &H264Config{
		GovLength:   50,
		H264Profile: s.streamInfo.Profile,
	}
	if mainEncoder.H264.H264Profile == "" {
		mainEncoder.H264.H264Profile = "Main"
	}

	profiles := []Profile{
		{
			Token: "profile_1",
			Fixed: true,
			Name:  "MainStream",
			VideoSourceConfiguration: VideoSourceConfiguration{
				Token:       "VideoSource_1",
				Name:        "VideoSourceConfig",
				UseCount:    1,
				SourceToken: "VideoSource_1",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  s.streamInfo.Width,
					Height: s.streamInfo.Height,
				},
			},
			VideoEncoderConfiguration: mainEncoder,
		},
	}

	// Add substream profile if enabled
	if s.config.SubstreamEnabled {
		subEncoder := VideoEncoderConfiguration{
			Token:    "VideoEncoder_2",
			Name:     "VideoEncoderConfig_Sub",
			UseCount: 1,
			Encoding: s.getONVIFEncoding(s.substreamInfo.Codec),
			Resolution: Resolution{
				Width:  s.substreamInfo.Width,
				Height: s.substreamInfo.Height,
			},
			Quality: 3,
			RateControl: RateControl{
				FrameRateLimit:   s.substreamInfo.FrameRate,
				EncodingInterval: 1,
				BitrateLimit:     s.substreamInfo.BitRate,
			},
			Multicast: Multicast{
				Address: MulticastAddress{
					Type:        "IPv4",
					IPv4Address: "0.0.0.0",
				},
				Port:      0,
				TTL:       1,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		}

		// Always add H264 config block for ONVIF compatibility
		// Even for H265 streams (which we report as H264)
		subEncoder.H264 = &H264Config{
			GovLength:   25,
			H264Profile: s.substreamInfo.Profile,
		}
		if subEncoder.H264.H264Profile == "" {
			subEncoder.H264.H264Profile = "Main"
		}

		profiles = append(profiles, Profile{
			Token: "profile_2",
			Fixed: true,
			Name:  "SubStream",
			VideoSourceConfiguration: VideoSourceConfiguration{
				Token:       "VideoSource_2",
				Name:        "VideoSourceConfig_Sub",
				UseCount:    1,
				SourceToken: "VideoSource_2",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  s.substreamInfo.Width,
					Height: s.substreamInfo.Height,
				},
			},
			VideoEncoderConfiguration: subEncoder,
		})
	}

	response := GetProfilesResponse{
		Profiles: profiles,
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetProfile(w http.ResponseWriter, bodyContent string) {
	// Check if substream (profile_2) is requested
	if s.config.SubstreamEnabled && (strings.Contains(bodyContent, "profile_2") || strings.Contains(bodyContent, "SubStream")) {
		// Build substream encoder config
		subEncoder := VideoEncoderConfiguration{
			Token:    "VideoEncoder_2",
			Name:     "VideoEncoderConfig_Sub",
			UseCount: 1,
			Encoding: s.getONVIFEncoding(s.substreamInfo.Codec),
			Resolution: Resolution{
				Width:  s.substreamInfo.Width,
				Height: s.substreamInfo.Height,
			},
			Quality: 3,
			RateControl: RateControl{
				FrameRateLimit:   s.substreamInfo.FrameRate,
				EncodingInterval: 1,
				BitrateLimit:     s.substreamInfo.BitRate,
			},
			Multicast: Multicast{
				Address: MulticastAddress{
					Type:        "IPv4",
					IPv4Address: "0.0.0.0",
				},
				Port:      0,
				TTL:       1,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		}

		// Always add H264 config block for ONVIF compatibility
		// Even for H265 streams (which we report as H264)
		subEncoder.H264 = &H264Config{
			GovLength:   25,
			H264Profile: s.substreamInfo.Profile,
		}
		if subEncoder.H264.H264Profile == "" {
			subEncoder.H264.H264Profile = "Main"
		}

		profile := Profile{
			Token: "profile_2",
			Fixed: true,
			Name:  "SubStream",
			VideoSourceConfiguration: VideoSourceConfiguration{
				Token:       "VideoSource_2",
				Name:        "VideoSourceConfig_Sub",
				UseCount:    1,
				SourceToken: "VideoSource_2",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  s.substreamInfo.Width,
					Height: s.substreamInfo.Height,
				},
			},
			VideoEncoderConfiguration: subEncoder,
		}

		response := GetProfileResponse{
			Profile: profile,
		}
		s.sendSOAPResponse(w, response)
		return
	}

	// Default to MainStream (profile_1)
	mainEncoder := VideoEncoderConfiguration{
		Token:    "VideoEncoder_1",
		Name:     "VideoEncoderConfig",
		UseCount: 1,
		Encoding: s.getONVIFEncoding(s.streamInfo.Codec),
		Resolution: Resolution{
			Width:  s.streamInfo.Width,
			Height: s.streamInfo.Height,
		},
		Quality: 5,
		RateControl: RateControl{
			FrameRateLimit:   s.streamInfo.FrameRate,
			EncodingInterval: 1,
			BitrateLimit:     s.streamInfo.BitRate,
		},
		Multicast: Multicast{
			Address: MulticastAddress{
				Type:        "IPv4",
				IPv4Address: "0.0.0.0",
			},
			Port:      0,
			TTL:       1,
			AutoStart: false,
		},
		SessionTimeout: "PT60S",
	}

	// Always add H264 config block for ONVIF compatibility
	// Even for H265 streams (which we report as H264)
	mainEncoder.H264 = &H264Config{
		GovLength:   50,
		H264Profile: s.streamInfo.Profile,
	}
	if mainEncoder.H264.H264Profile == "" {
		mainEncoder.H264.H264Profile = "Main"
	}

	profile := Profile{
		Token: "profile_1",
		Fixed: true,
		Name:  "MainStream",
		VideoSourceConfiguration: VideoSourceConfiguration{
			Token:       "VideoSource_1",
			Name:        "VideoSourceConfig",
			UseCount:    1,
			SourceToken: "VideoSource_1",
			Bounds: Bounds{
				X:      0,
				Y:      0,
				Width:  s.streamInfo.Width,
				Height: s.streamInfo.Height,
			},
		},
		VideoEncoderConfiguration: mainEncoder,
	}

	response := GetProfileResponse{
		Profile: profile,
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetVideoEncoderConfiguration(w http.ResponseWriter, bodyContent string) {
	// Check if substream encoder (VideoEncoder_2) is requested
	if s.config.SubstreamEnabled && (strings.Contains(bodyContent, "VideoEncoder_2") || strings.Contains(bodyContent, "VideoEncoderConfig_Sub")) {
		subEncoder := VideoEncoderConfiguration{
			Token:    "VideoEncoder_2",
			Name:     "VideoEncoderConfig_Sub",
			UseCount: 1,
			Encoding: s.getONVIFEncoding(s.substreamInfo.Codec),
			Resolution: Resolution{
				Width:  s.substreamInfo.Width,
				Height: s.substreamInfo.Height,
			},
			Quality: 3,
			RateControl: RateControl{
				FrameRateLimit:   s.substreamInfo.FrameRate,
				EncodingInterval: 1,
				BitrateLimit:     s.substreamInfo.BitRate,
			},
			Multicast: Multicast{
				Address: MulticastAddress{
					Type:        "IPv4",
					IPv4Address: "0.0.0.0",
				},
				Port:      0,
				TTL:       1,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		}

		// Always add H264 config block for ONVIF compatibility
		// Even for H265 streams (which we report as H264)
		subEncoder.H264 = &H264Config{
			GovLength:   25,
			H264Profile: s.substreamInfo.Profile,
		}
		if subEncoder.H264.H264Profile == "" {
			subEncoder.H264.H264Profile = "Main"
		}

		response := GetVideoEncoderConfigurationResponse{
			Configuration: subEncoder,
		}
		s.sendSOAPResponse(w, response)
		return
	}

	// Default to main stream encoder (VideoEncoder_1)
	mainEncoder := VideoEncoderConfiguration{
		Token:    "VideoEncoder_1",
		Name:     "VideoEncoderConfig",
		UseCount: 1,
		Encoding: s.getONVIFEncoding(s.streamInfo.Codec),
		Resolution: Resolution{
			Width:  s.streamInfo.Width,
			Height: s.streamInfo.Height,
		},
		Quality: 5,
		RateControl: RateControl{
			FrameRateLimit:   s.streamInfo.FrameRate,
			EncodingInterval: 1,
			BitrateLimit:     s.streamInfo.BitRate,
		},
		Multicast: Multicast{
			Address: MulticastAddress{
				Type:        "IPv4",
				IPv4Address: "0.0.0.0",
			},
			Port:      0,
			TTL:       1,
			AutoStart: false,
		},
		SessionTimeout: "PT60S",
	}

	// Always add H264 config block for ONVIF compatibility
	// Even for H265 streams (which we report as H264)
	mainEncoder.H264 = &H264Config{
		GovLength:   50,
		H264Profile: s.streamInfo.Profile,
	}
	if mainEncoder.H264.H264Profile == "" {
		mainEncoder.H264.H264Profile = "Main"
	}

	response := GetVideoEncoderConfigurationResponse{
		Configuration: mainEncoder,
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetVideoEncoderConfigurationOptions(w http.ResponseWriter) {
	// Build available resolutions list based on detected resolution
	resolutions := []VideoResolution{
		{Width: s.streamInfo.Width, Height: s.streamInfo.Height},
	}

	// Add common lower resolutions
	if s.streamInfo.Width > 1280 {
		resolutions = append(resolutions, VideoResolution{Width: 1280, Height: 720})
	}
	if s.streamInfo.Width > 640 {
		resolutions = append(resolutions, VideoResolution{Width: 640, Height: 480})
	}

	response := GetVideoEncoderConfigurationOptionsResponse{
		Options: VideoEncoderConfigurationOptions{
			QualityRange: IntRange{
				Min: 1,
				Max: 10,
			},
			H264: H264ConfigurationOptions{
				ResolutionsAvailable: resolutions,
				GovLengthRange: IntRange{
					Min: 1,
					Max: 100,
				},
				FrameRateRange: IntRange{
					Min: 1,
					Max: s.streamInfo.FrameRate,
				},
				EncodingIntervalRange: IntRange{
					Min: 1,
					Max: 1,
				},
				H264ProfilesSupported: []string{"Baseline", "Main", "High"},
			},
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetStreamUri(w http.ResponseWriter, r *http.Request, bodyContent string) {
	// Determine stream path based on profile token
	rtspPath := s.config.RTSPStream
	if strings.Contains(bodyContent, "profile_2") || strings.Contains(bodyContent, "SubStream") {
		// Use substream path
		rtspPath = s.config.SubstreamPath
		if rtspPath == "" {
			rtspPath = s.config.RTSPStream + "_sub"
		}
	}

	// Use dynamic host IP based on incoming request
	hostIP := s.getHostIP(r)
	rtspURI := fmt.Sprintf("rtsp://%s:%d%s", hostIP, s.rtspPort, rtspPath)

	response := GetStreamUriResponse{
		MediaUri: MediaUri{
			Uri:                 rtspURI,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetSnapshotUri(w http.ResponseWriter, r *http.Request) {
	snapshotURI := fmt.Sprintf("http://%s:%d/snapshot", s.getHostIP(r), s.config.HTTPPort)

	response := GetSnapshotUriResponse{
		MediaUri: MediaUri{
			Uri:                 snapshotURI,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSetSynchronizationPoint(w http.ResponseWriter, bodyContent string) {
	// Extract profile token from request if needed (optional)
	// For now, just return success response
	response := SetSynchronizationPointResponse{}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetOSDOptions(w http.ResponseWriter) {
	// Return basic OSD options - indicating no OSD support
	response := GetOSDOptionsResponse{
		OSDOptions: OSDOptions{
			MaximumNumberOfOSDs: 0,
			Type:                []string{},
			PositionOption:      []string{},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetAudioEncoderConfiguration(w http.ResponseWriter) {
	// Return basic audio encoder configuration - indicating no audio
	response := GetAudioEncoderConfigurationResponse{
		Configuration: AudioEncoderConfiguration{
			Token:          "AudioEncoder_1",
			Name:           "AudioEncoderConfig",
			UseCount:       0,
			Encoding:       "G711",
			Bitrate:        64,
			SampleRate:     8,
			SessionTimeout: "PT60S",
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetAudioEncoderConfigurationOptions(w http.ResponseWriter) {
	// Return basic audio encoder options - indicating limited audio support
	response := GetAudioEncoderConfigurationOptionsResponse{
		Options: AudioEncoderConfigurationOptions{
			Options: []AudioEncoderOption{
				{
					Encoding: "G711",
					BitrateList: &IntList{
						Items: []int{64},
					},
					SampleRateList: &IntList{
						Items: []int{8},
					},
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetAudioSources(w http.ResponseWriter) {
	// Return a single audio source - most NVRs just need to see at least one
	response := GetAudioSourcesResponse{
		AudioSources: []AudioSource{
			{
				Token:    "AudioSource_1",
				Channels: 1,
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) sendSOAPResponse(w http.ResponseWriter, response interface{}) {
	body := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Body"`
		Content interface{} `xml:",any"`
	}{
		Content: response,
	}

	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		TT      string      `xml:"xmlns:tt,attr"`
		TDS     string      `xml:"xmlns:tds,attr"`
		TRT     string      `xml:"xmlns:trt,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		TT:      "http://www.onvif.org/ver10/schema",
		TDS:     "http://www.onvif.org/ver10/device/wsdl",
		TRT:     "http://www.onvif.org/ver10/media/wsdl",
		Body:    body,
	}

	output, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		log.Printf("[%s] Failed to marshal response: %v", s.config.Name, err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseStr := xml.Header + string(output)
	log.Printf("[%s] Sending Response:\n%s", s.config.Name, responseStr)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseStr))
}

func (s *ONVIFServer) sendSOAPFault(w http.ResponseWriter, code, reason string) {
	log.Printf("[%s] Sending SOAP Fault - Code: %s, Reason: %s", s.config.Name, code, reason)

	fault := SOAPFault{}
	fault.Code.Value = code
	fault.Reason.Text = reason

	body := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Body"`
		Content interface{} `xml:",any"`
	}{
		Content: fault,
	}

	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		Body:    body,
	}

	output, _ := xml.MarshalIndent(envelope, "", "  ")

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(xml.Header))
	w.Write(output)
}

func (s *ONVIFServer) getHostIP(r *http.Request) string {
	// Try to get from Host header first
	host := r.Host
	if host != "" {
		// Remove port if present
		if idx := strings.Index(host, ":"); idx != -1 {
			return host[:idx]
		}
		return host
	}

	// Fallback to configured RTSP host
	return s.rtspHost
}

// WS-Discovery for NVR auto-detection
func startDiscoveryService() {
	// Listen on multicast address for WS-Discovery
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		log.Printf("Failed to resolve discovery address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("Failed to listen on multicast: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WS-Discovery service started on 239.255.255.250:3702")

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Discovery read error: %v", err)
			continue
		}

		message := string(buf[:n])
		if strings.Contains(message, "Probe") && strings.Contains(message, "onvif") {
			log.Printf("Received ONVIF discovery probe from %s", remoteAddr)
			// In a full implementation, we'd respond with ProbeMatches
			// For now, just log it
		}
	}
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Auto-detect IP if not configured
	rtspHost := config.RTSPHost
	if rtspHost == "" {
		rtspHost = getOutboundIP()
		log.Printf("Auto-detected IP: %s", rtspHost)
	}

	// Start WS-Discovery if enabled
	if config.EnableDiscovery {
		go startDiscoveryService()
	}

	// Start ONVIF server for each camera
	var wg sync.WaitGroup
	for _, camConfig := range config.Cameras {
		wg.Add(1)
		go func(cfg CameraConfig) {
			defer wg.Done()
			server := NewONVIFServer(cfg, rtspHost, config.RTSPPort)

			// Start periodic stream info refresh (every 5 minutes)
			server.startStreamInfoRefresh(5 * time.Minute)

			if err := server.Start(); err != nil {
				log.Printf("Server for '%s' failed: %v", cfg.Name, err)
			}
		}(camConfig)

		// Small delay to avoid port conflicts
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("All ONVIF servers started successfully")
	wg.Wait()
}
