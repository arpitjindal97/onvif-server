package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Global debug flag
var debugMode bool

// logDebug logs messages only when debug mode is enabled
func logDebug(format string, args ...interface{}) {
	if debugMode {
		log.Printf(format, args...)
	}
}

// logInfo logs important informational messages (always shown)
func logInfo(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Config structures
type Config struct {
	Cameras         []CameraConfig `yaml:"cameras"`
	RTSPHost        string         `yaml:"rtsp_host"`
	RTSPPort        int            `yaml:"rtsp_port"`
	EnableDiscovery bool           `yaml:"enable_discovery"`
	Username        string         `yaml:"username"`
	Password        string         `yaml:"password"`
}

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

// StreamInfo holds detected stream properties
type StreamInfo struct {
	Width     int
	Height    int
	Codec     string
	FrameRate int
	BitRate   int // in kbps
	Profile   string
}

// WS-Security structures
type Security struct {
	WSSE          string        `xml:"xmlns:wsse,attr"`
	WSU           string        `xml:"xmlns:wsu,attr"`
	UsernameToken UsernameToken `xml:"wsse:UsernameToken"`
}

type UsernameToken struct {
	Username string   `xml:"wsse:Username"`
	Password Password `xml:"wsse:Password"`
	Nonce    Nonce    `xml:"wsse:Nonce"`
	Created  string   `xml:"wsu:Created"`
}

type Password struct {
	Type  string `xml:"Type,attr"`
	Value string `xml:",chardata"`
}

type Nonce struct {
	EncodingType string `xml:"EncodingType,attr"`
	Value        string `xml:",chardata"`
}

// SOAP Envelope with Security
type SOAPEnvelope struct {
	XMLName xml.Name     `xml:"SOAP-ENV:Envelope"`
	SOAPENV string       `xml:"xmlns:SOAP-ENV,attr"`
	Header  *SOAPHeader  `xml:"SOAP-ENV:Header,omitempty"`
	Body    interface{}  `xml:"SOAP-ENV:Body"`
}

type SOAPHeader struct {
	Security *Security `xml:"wsse:Security,omitempty"`
}

type SOAPBody struct {
	Content interface{} `xml:",any"`
}

type SOAPFault struct {
	XMLName xml.Name `xml:"SOAP-ENV:Fault"`
	Code    struct {
		Value   string `xml:"SOAP-ENV:Value"`
		Subcode *struct {
			Value string `xml:"SOAP-ENV:Value"`
		} `xml:"SOAP-ENV:Subcode,omitempty"`
	} `xml:"SOAP-ENV:Code"`
	Reason struct {
		Text string `xml:"SOAP-ENV:Text"`
	} `xml:"SOAP-ENV:Reason"`
	Detail *struct {
		Text string `xml:",chardata"`
	} `xml:"SOAP-ENV:Detail,omitempty"`
}

// Time structures
type SystemDateAndTime struct {
	DateTimeType    string   `xml:"tt:DateTimeType"`
	DaylightSavings bool     `xml:"tt:DaylightSavings"`
	TimeZone        TimeZone `xml:"tt:TimeZone,omitempty"`
	UTCDateTime     DateTime `xml:"tt:UTCDateTime"`
	LocalDateTime   DateTime `xml:"tt:LocalDateTime,omitempty"`
	Extension       *struct{} `xml:"tt:Extension,omitempty"`
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

// Device Service Responses
type GetSystemDateAndTimeResponse struct {
	XMLName           xml.Name          `xml:"tds:GetSystemDateAndTimeResponse"`
	SystemDateAndTime SystemDateAndTime `xml:"tds:SystemDateAndTime"`
}

type GetDeviceInformationResponse struct {
	XMLName         xml.Name `xml:"tds:GetDeviceInformationResponse"`
	Manufacturer    string   `xml:"tds:Manufacturer"`
	Model           string   `xml:"tds:Model"`
	FirmwareVersion string   `xml:"tds:FirmwareVersion"`
	SerialNumber    string   `xml:"tds:SerialNumber"`
	HardwareId      string   `xml:"tds:HardwareId"`
}

type GetCapabilitiesResponse struct {
	XMLName      xml.Name     `xml:"tds:GetCapabilitiesResponse"`
	Capabilities Capabilities `xml:"tds:Capabilities"`
}

type Capabilities struct {
	Analytics *AnalyticsCapabilities `xml:"tt:Analytics,omitempty"`
	Device    *DeviceCapabilities    `xml:"tt:Device,omitempty"`
	Events    *EventCapabilities     `xml:"tt:Events,omitempty"`
	Imaging   *ImagingCapabilities   `xml:"tt:Imaging,omitempty"`
	Media     *MediaCapabilities     `xml:"tt:Media,omitempty"`
	Media2    *Media2Capabilities    `xml:"tt:Media2,omitempty"`
	PTZ       *PTZCapabilities       `xml:"tt:PTZ,omitempty"`
	Extension *struct{}              `xml:"tt:Extension,omitempty"`
}

type AnalyticsCapabilities struct {
	XAddr              string    `xml:"tt:XAddr"`
	RuleSupport        bool      `xml:"tt:RuleSupport"`
	AnalyticsModuleSupport bool  `xml:"tt:AnalyticsModuleSupport"`
}

type DeviceCapabilities struct {
	XAddr   string                 `xml:"tt:XAddr"`
	Network *NetworkCapabilities   `xml:"tt:Network,omitempty"`
	System  *SystemCapabilities    `xml:"tt:System,omitempty"`
	IO      *IOCapabilities        `xml:"tt:IO,omitempty"`
	Security *SecurityCapabilities `xml:"tt:Security,omitempty"`
}

type NetworkCapabilities struct {
	IPFilter            bool `xml:"tt:IPFilter,omitempty"`
	ZeroConfiguration   bool `xml:"tt:ZeroConfiguration,omitempty"`
	IPVersion6          bool `xml:"tt:IPVersion6,omitempty"`
	DynDNS              bool `xml:"tt:DynDNS,omitempty"`
	Dot11Configuration  bool `xml:"tt:Dot11Configuration,omitempty"`
	Dot1XConfigurations int  `xml:"tt:Dot1XConfigurations,omitempty"`
	HostnameFromDHCP    bool `xml:"tt:HostnameFromDHCP,omitempty"`
	NTP                 int  `xml:"tt:NTP,omitempty"`
	DHCPv6              bool `xml:"tt:DHCPv6,omitempty"`
}

type SystemCapabilities struct {
	DiscoveryResolve    bool `xml:"tt:DiscoveryResolve"`
	DiscoveryBye        bool `xml:"tt:DiscoveryBye"`
	RemoteDiscovery     bool `xml:"tt:RemoteDiscovery"`
	SystemBackup        bool `xml:"tt:SystemBackup"`
	SystemLogging       bool `xml:"tt:SystemLogging"`
	FirmwareUpgrade     bool `xml:"tt:FirmwareUpgrade"`
	HttpFirmwareUpgrade bool `xml:"tt:HttpFirmwareUpgrade,omitempty"`
	HttpSystemBackup    bool `xml:"tt:HttpSystemBackup,omitempty"`
	HttpSystemLogging   bool `xml:"tt:HttpSystemLogging,omitempty"`
	HttpSupportInformation bool `xml:"tt:HttpSupportInformation,omitempty"`
}

type IOCapabilities struct {
	InputConnectors int `xml:"tt:InputConnectors,omitempty"`
	RelayOutputs    int `xml:"tt:RelayOutputs,omitempty"`
}

type SecurityCapabilities struct {
	TLS11           bool `xml:"tt:TLS1.1"`
	TLS12           bool `xml:"tt:TLS1.2"`
	OnboardKeyGeneration bool `xml:"tt:OnboardKeyGeneration"`
	AccessPolicyConfig   bool `xml:"tt:AccessPolicyConfig"`
	DefaultAccessPolicy  bool `xml:"tt:DefaultAccessPolicy,omitempty"`
	Dot1X           bool `xml:"tt:Dot1X"`
	RemoteUserHandling bool `xml:"tt:RemoteUserHandling"`
	X509Token       bool `xml:"tt:X.509Token"`
	SAMLToken       bool `xml:"tt:SAMLToken"`
	KerberosToken   bool `xml:"tt:KerberosToken"`
	UsernameToken   bool `xml:"tt:UsernameToken"`
	HttpDigest      bool `xml:"tt:HttpDigest"`
	RELToken        bool `xml:"tt:RELToken"`
}

type EventCapabilities struct {
	XAddr              string `xml:"tt:XAddr"`
	WSSubscriptionPolicySupport bool `xml:"tt:WSSubscriptionPolicySupport"`
	WSPullPointSupport bool `xml:"tt:WSPullPointSupport"`
	WSPausableSubscriptionManagerInterfaceSupport bool `xml:"tt:WSPausableSubscriptionManagerInterfaceSupport,omitempty"`
}

type ImagingCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type MediaCapabilities struct {
	XAddr                    string                      `xml:"tt:XAddr"`
	StreamingCapabilities    *StreamingCapabilities      `xml:"tt:StreamingCapabilities,omitempty"`
}

type StreamingCapabilities struct {
	RTPMulticast bool `xml:"tt:RTPMulticast,omitempty"`
	RTP_TCP      bool `xml:"tt:RTP_TCP"`
	RTP_RTSP_TCP bool `xml:"tt:RTP_RTSP_TCP"`
}

type PTZCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type Media2Capabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type GetServicesResponse struct {
	XMLName xml.Name  `xml:"tds:GetServicesResponse"`
	Service []Service `xml:"tds:Service"`
}

type Service struct {
	Namespace string       `xml:"tds:Namespace"`
	XAddr     string       `xml:"tds:XAddr"`
	Capabilities *ServiceCapabilities `xml:"tds:Capabilities,omitempty"`
	Version   Version      `xml:"tds:Version"`
}

type ServiceCapabilities struct {
	Network  bool `xml:"tds:Network,attr,omitempty"`
	Security bool `xml:"tds:Security,attr,omitempty"`
	System   bool `xml:"tds:System,attr,omitempty"`
}

type Version struct {
	Major int `xml:"tt:Major"`
	Minor int `xml:"tt:Minor"`
}

type GetScopesResponse struct {
	XMLName xml.Name `xml:"tds:GetScopesResponse"`
	Scopes  []Scope  `xml:"tds:Scopes"`
}

type Scope struct {
	ScopeDef      string `xml:"tds:ScopeDef"`
	ScopeItem     string `xml:"tds:ScopeItem"`
}

type GetHostnameResponse struct {
	XMLName      xml.Name     `xml:"tds:GetHostnameResponse"`
	HostnameInfo HostnameInfo `xml:"tds:HostnameInformation"`
}

type HostnameInfo struct {
	FromDHCP bool   `xml:"tt:FromDHCP"`
	Name     string `xml:"tt:Name,omitempty"`
}

type GetDNSResponse struct {
	XMLName xml.Name `xml:"tds:GetDNSResponse"`
	DNSInfo DNSInfo  `xml:"tds:DNSInformation"`
}

type DNSInfo struct {
	FromDHCP       bool          `xml:"tt:FromDHCP"`
	DNSFromDHCP    []DNSName     `xml:"tt:DNSFromDHCP,omitempty"`
	DNSManual      []DNSName     `xml:"tt:DNSManual,omitempty"`
}

type DNSName struct {
	Type        string `xml:"tt:Type"`
	IPv4Address string `xml:"tt:IPv4Address,omitempty"`
	IPv6Address string `xml:"tt:IPv6Address,omitempty"`
}

type GetNetworkInterfacesResponse struct {
	XMLName           xml.Name           `xml:"tds:GetNetworkInterfacesResponse"`
	NetworkInterfaces []NetworkInterface `xml:"tds:NetworkInterfaces"`
}

type NetworkInterface struct {
	Token   string       `xml:"token,attr"`
	Enabled bool         `xml:"tt:Enabled"`
	Info    NetworkInfo  `xml:"tt:Info,omitempty"`
	Link    *NetworkLink `xml:"tt:Link,omitempty"`
	IPv4    *IPv4Config  `xml:"tt:IPv4,omitempty"`
	IPv6    *IPv6Config  `xml:"tt:IPv6,omitempty"`
}

type NetworkInfo struct {
	Name      string `xml:"tt:Name,omitempty"`
	HwAddress string `xml:"tt:HwAddress"`
	MTU       int    `xml:"tt:MTU,omitempty"`
}

type NetworkLink struct {
	AdminSettings LinkSettings `xml:"tt:AdminSettings"`
	OperSettings  LinkSettings `xml:"tt:OperSettings"`
	InterfaceType int          `xml:"tt:InterfaceType"`
}

type LinkSettings struct {
	AutoNegotiation bool `xml:"tt:AutoNegotiation"`
	Speed           int  `xml:"tt:Speed"`
	Duplex          string `xml:"tt:Duplex"`
}

type IPv4Config struct {
	Enabled bool         `xml:"tt:Enabled"`
	Config  IPv4Network  `xml:"tt:Config"`
}

type IPv4Network struct {
	Manual      []PrefixedIPv4Address `xml:"tt:Manual,omitempty"`
	LinkLocal   *PrefixedIPv4Address  `xml:"tt:LinkLocal,omitempty"`
	FromDHCP    *PrefixedIPv4Address  `xml:"tt:FromDHCP,omitempty"`
	DHCP        bool                  `xml:"tt:DHCP"`
}

type PrefixedIPv4Address struct {
	Address      string `xml:"tt:Address"`
	PrefixLength int    `xml:"tt:PrefixLength"`
}

type IPv6Config struct {
	Enabled bool `xml:"tt:Enabled"`
}

type GetNetworkProtocolsResponse struct {
	XMLName          xml.Name           `xml:"tds:GetNetworkProtocolsResponse"`
	NetworkProtocols []NetworkProtocol  `xml:"tds:NetworkProtocols"`
}

type NetworkProtocol struct {
	Name    string `xml:"tt:Name"`
	Enabled bool   `xml:"tt:Enabled"`
	Port    []int  `xml:"tt:Port,omitempty"`
}

// Media Service
type GetProfilesResponse struct {
	XMLName  xml.Name  `xml:"trt:GetProfilesResponse"`
	Profiles []Profile `xml:"trt:Profiles"`
}

type Profile struct {
	Token                     string                     `xml:"token,attr"`
	Fixed                     bool                       `xml:"fixed,attr"`
	Name                      string                     `xml:"tt:Name"`
	VideoSourceConfiguration  *VideoSourceConfiguration  `xml:"tt:VideoSourceConfiguration,omitempty"`
	AudioSourceConfiguration  *AudioSourceConfiguration  `xml:"tt:AudioSourceConfiguration,omitempty"`
	VideoEncoderConfiguration *VideoEncoderConfiguration `xml:"tt:VideoEncoderConfiguration,omitempty"`
	AudioEncoderConfiguration *AudioEncoderConfiguration `xml:"tt:AudioEncoderConfiguration,omitempty"`
	VideoAnalyticsConfiguration *VideoAnalyticsConfiguration `xml:"tt:VideoAnalyticsConfiguration,omitempty"`
	PTZConfiguration          *PTZConfiguration          `xml:"tt:PTZConfiguration,omitempty"`
	MetadataConfiguration     *MetadataConfiguration     `xml:"tt:MetadataConfiguration,omitempty"`
}

type VideoSourceConfiguration struct {
	Token       string  `xml:"token,attr"`
	Name        string  `xml:"tt:Name"`
	UseCount    int     `xml:"tt:UseCount"`
	SourceToken string  `xml:"tt:SourceToken"`
	Bounds      Bounds  `xml:"tt:Bounds"`
}

type Bounds struct {
	X      int `xml:"x,attr"`
	Y      int `xml:"y,attr"`
	Width  int `xml:"width,attr"`
	Height int `xml:"height,attr"`
}

type VideoEncoderConfiguration struct {
	Token          string       `xml:"token,attr"`
	Name           string       `xml:"tt:Name"`
	UseCount       int          `xml:"tt:UseCount"`
	Encoding       string       `xml:"tt:Encoding"`
	Resolution     Resolution   `xml:"tt:Resolution"`
	Quality        float64      `xml:"tt:Quality"`
	RateControl    *RateControl `xml:"tt:RateControl,omitempty"`
	H264           *H264Config  `xml:"tt:H264,omitempty"`
	Multicast      Multicast    `xml:"tt:Multicast"`
	SessionTimeout string       `xml:"tt:SessionTimeout"`
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
	IPv6Address string `xml:"tt:IPv6Address,omitempty"`
}

type AudioSourceConfiguration struct {
	Token       string `xml:"token,attr"`
	Name        string `xml:"tt:Name"`
	UseCount    int    `xml:"tt:UseCount"`
	SourceToken string `xml:"tt:SourceToken"`
}

type AudioEncoderConfiguration struct {
	Token          string  `xml:"token,attr"`
	Name           string  `xml:"tt:Name"`
	UseCount       int     `xml:"tt:UseCount"`
	Encoding       string  `xml:"tt:Encoding"`
	Bitrate        int     `xml:"tt:Bitrate"`
	SampleRate     int     `xml:"tt:SampleRate"`
	Multicast      Multicast `xml:"tt:Multicast"`
	SessionTimeout string  `xml:"tt:SessionTimeout"`
}

type VideoAnalyticsConfiguration struct {
	Token             string `xml:"token,attr"`
	Name              string `xml:"tt:Name"`
	UseCount          int    `xml:"tt:UseCount"`
	AnalyticsEngineConfiguration struct{} `xml:"tt:AnalyticsEngineConfiguration"`
	RuleEngineConfiguration      struct{} `xml:"tt:RuleEngineConfiguration"`
}

type PTZConfiguration struct {
	Token    string `xml:"token,attr"`
	Name     string `xml:"tt:Name"`
	UseCount int    `xml:"tt:UseCount"`
	NodeToken string `xml:"tt:NodeToken"`
}

type MetadataConfiguration struct {
	Token          string `xml:"token,attr"`
	Name           string `xml:"tt:Name"`
	UseCount       int    `xml:"tt:UseCount"`
	SessionTimeout string `xml:"tt:SessionTimeout"`
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

type GetSnapshotUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetSnapshotUriResponse"`
	MediaUri MediaUri `xml:"trt:MediaUri"`
}

type GetVideoSourcesResponse struct {
	XMLName      xml.Name      `xml:"trt:GetVideoSourcesResponse"`
	VideoSources []VideoSource `xml:"trt:VideoSources"`
}

type VideoSource struct {
	Token      string     `xml:"token,attr"`
	Framerate  float64    `xml:"tt:Framerate"`
	Resolution Resolution `xml:"tt:Resolution"`
	Imaging    *struct{}  `xml:"tt:Imaging,omitempty"`
}

type GetAudioSourcesResponse struct {
	XMLName      xml.Name      `xml:"trt:GetAudioSourcesResponse"`
	AudioSources []AudioSource `xml:"trt:AudioSources"`
}

type AudioSource struct {
	Token    string `xml:"token,attr"`
	Channels int    `xml:"tt:Channels"`
}

type GetVideoEncoderConfigurationsResponse struct {
	XMLName        xml.Name                     `xml:"trt:GetVideoEncoderConfigurationsResponse"`
	Configurations []VideoEncoderConfiguration  `xml:"trt:Configurations"`
}

type GetVideoEncoderConfigurationResponse struct {
	XMLName       xml.Name                  `xml:"trt:GetVideoEncoderConfigurationResponse"`
	Configuration VideoEncoderConfiguration `xml:"trt:Configuration"`
}

type GetVideoEncoderConfigurationOptionsResponse struct {
	XMLName xml.Name                            `xml:"trt:GetVideoEncoderConfigurationOptionsResponse"`
	Options VideoEncoderConfigurationOptions    `xml:"trt:Options"`
}

type VideoEncoderConfigurationOptions struct {
	QualityRange       QualityRange         `xml:"tt:QualityRange"`
	JPEG               *JpegOptions         `xml:"tt:JPEG,omitempty"`
	MPEG4              *Mpeg4Options        `xml:"tt:MPEG4,omitempty"`
	H264               *H264Options         `xml:"tt:H264,omitempty"`
	Extension          *EncoderExtension    `xml:"tt:Extension,omitempty"`
}

type QualityRange struct {
	Min int `xml:"tt:Min"`
	Max int `xml:"tt:Max"`
}

type JpegOptions struct {
	ResolutionsAvailable []Resolution      `xml:"tt:ResolutionsAvailable"`
	FrameRateRange       IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange         `xml:"tt:EncodingIntervalRange"`
}

type Mpeg4Options struct {
	ResolutionsAvailable  []Resolution      `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        IntRange          `xml:"tt:GovLengthRange"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	Mpeg4ProfilesSupported []string         `xml:"tt:Mpeg4ProfilesSupported"`
}

type H264Options struct {
	ResolutionsAvailable  []Resolution      `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        IntRange          `xml:"tt:GovLengthRange"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	H264ProfilesSupported []string          `xml:"tt:H264ProfilesSupported"`
}

type IntRange struct {
	Min int `xml:"tt:Min"`
	Max int `xml:"tt:Max"`
}

type EncoderExtension struct {
	JPEG  *JpegOptions2  `xml:"tt:JPEG,omitempty"`
	MPEG4 *Mpeg4Options2 `xml:"tt:MPEG4,omitempty"`
	H264  *H264Options2  `xml:"tt:H264,omitempty"`
	Extension *struct{}   `xml:"tt:Extension,omitempty"`
}

type JpegOptions2 struct {
	ResolutionsAvailable  []Resolution      `xml:"tt:ResolutionsAvailable"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	BitrateRange          IntRange          `xml:"tt:BitrateRange"`
}

type Mpeg4Options2 struct {
	ResolutionsAvailable  []Resolution      `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        IntRange          `xml:"tt:GovLengthRange"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	Mpeg4ProfilesSupported []string         `xml:"tt:Mpeg4ProfilesSupported"`
	BitrateRange          IntRange          `xml:"tt:BitrateRange"`
}

type H264Options2 struct {
	ResolutionsAvailable  []Resolution      `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        IntRange          `xml:"tt:GovLengthRange"`
	FrameRateRange        IntRange          `xml:"tt:FrameRateRange"`
	EncodingIntervalRange IntRange          `xml:"tt:EncodingIntervalRange"`
	H264ProfilesSupported []string          `xml:"tt:H264ProfilesSupported"`
	BitrateRange          IntRange          `xml:"tt:BitrateRange"`
}

// Event Service
type SubscribeResponse struct {
	XMLName              xml.Name `xml:"wsnt:SubscribeResponse"`
	SubscriptionReference SubscriptionReference `xml:"wsnt:SubscriptionReference"`
	CurrentTime          string   `xml:"wsnt:CurrentTime,omitempty"`
	TerminationTime      string   `xml:"wsnt:TerminationTime,omitempty"`
}

type SubscriptionReference struct {
	Address string `xml:"wsa5:Address"`
}

type GetEventPropertiesResponse struct {
	XMLName                xml.Name `xml:"tev:GetEventPropertiesResponse"`
	TopicNamespaceLocation []string `xml:"tev:TopicNamespaceLocation,omitempty"`
	FixedTopicSet          bool     `xml:"wsnt:FixedTopicSet"`
	TopicSet               *struct {
		Any string `xml:",any"`
	} `xml:"tev:TopicSet,omitempty"`
}

type CreatePullPointSubscriptionResponse struct {
	XMLName              xml.Name              `xml:"tev:CreatePullPointSubscriptionResponse"`
	SubscriptionReference SubscriptionReference `xml:"tev:SubscriptionReference"`
	CurrentTime          string                `xml:"wsnt:CurrentTime,omitempty"`
	TerminationTime      string                `xml:"wsnt:TerminationTime,omitempty"`
}

// ClientTimeSettings stores time synchronization settings per client
type ClientTimeSettings struct {
	BaseTime       time.Time // Time that was set by NVR
	SystemBaseTime time.Time // System time when SetSystemDateAndTime was called
	DateTimeType   string    // NTP, Manual, etc.
	TimeZone       string    // Timezone string
}

// ONVIFServer represents the server
type ONVIFServer struct {
	config          CameraConfig
	rtspHost        string
	rtspPort        int
	username        string
	password        string
	timeSettings    *ClientTimeSettings // Stored time synchronization settings
	timeSettingsMu  sync.RWMutex        // Protects timeSettings
	mu              sync.RWMutex
	streamInfo      *StreamInfo  // Main stream information
	substreamInfo   *StreamInfo  // Substream information
	streamInfoMu    sync.RWMutex // Protects streamInfo and substreamInfo
	bitrateCache    map[string]int // Cache for bitrate limits keyed by V_ENC_CFG_* token
	bitrateMu       sync.RWMutex   // Protects bitrateCache
}

func NewONVIFServer(config CameraConfig, rtspHost string, rtspPort int, username, password string) *ONVIFServer {
	s := &ONVIFServer{
		config:       config,
		rtspHost:     rtspHost,
		rtspPort:     rtspPort,
		username:     username,
		password:     password,
		timeSettings: nil, // Will be set when NVR calls SetSystemDateAndTime
		streamInfo: &StreamInfo{
			// Defaults for main stream
			Width:     1920,
			Height:    1080,
			Codec:     "H264",
			FrameRate: 25,
			BitRate:   4096,
			Profile:   "High",
		},
		substreamInfo: &StreamInfo{
			// Defaults for substream (lower quality) - using standard D1 resolution
			Width:     640,
			Height:    480,
			Codec:     "H264",
			FrameRate: 15,
			BitRate:   512,
			Profile:   "Baseline",
		},
		bitrateCache: make(map[string]int),
	}

	// Detect stream info asynchronously (non-blocking)
	go s.detectStreamInfo(false) // Main stream

	// Detect substream info if enabled
	if config.SubstreamEnabled {
		go s.detectStreamInfo(true) // Substream
	}

	return s
}

// detectStreamInfo uses ffprobe to detect stream properties
func (s *ONVIFServer) detectStreamInfo(isSubstream bool) {
	streamPath := s.config.RTSPStream
	streamType := "main"
	var targetInfo *StreamInfo

	if isSubstream {
		streamPath = s.config.SubstreamPath
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub" // Default fallback
		}
		streamType = "sub"
	}

	// Lock to get the correct target
	s.streamInfoMu.RLock()
	if isSubstream {
		targetInfo = s.substreamInfo
	} else {
		targetInfo = s.streamInfo
	}
	s.streamInfoMu.RUnlock()

	rtspURL := fmt.Sprintf("rtsp://localhost:%d%s", s.rtspPort, streamPath)

	logDebug("[%s] 🔍 Detecting %s stream properties for '%s'...", s.config.Name, streamType, streamPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,bit_rate,profile",
		"-of", "json",
		rtspURL,
	)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logDebug("[%s] ⚠️  Stream detection timeout for %s stream '%s' (using defaults: %dx%d %s %dkbps)",
				s.config.Name, streamType, streamPath, targetInfo.Width, targetInfo.Height, targetInfo.Codec, targetInfo.BitRate)
		} else {
			logDebug("[%s] ⚠️  Failed to detect %s stream '%s': %v (using defaults: %dx%d %s %dkbps)",
				s.config.Name, streamType, streamPath, err, targetInfo.Width, targetInfo.Height, targetInfo.Codec, targetInfo.BitRate)
		}
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
		logDebug("[%s] ⚠️  Failed to parse ffprobe output for %s stream: %v", s.config.Name, streamType, err)
		return
	}

	if len(result.Streams) == 0 {
		logDebug("[%s] ⚠️  No video stream found for %s stream", s.config.Name, streamType)
		return
	}

	stream := result.Streams[0]

	// Update stream info with lock
	s.streamInfoMu.Lock()
	defer s.streamInfoMu.Unlock()

	if isSubstream {
		targetInfo = s.substreamInfo
	} else {
		targetInfo = s.streamInfo
	}

	// Update properties
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

	logInfo("[%s] ✅ Detected %s stream '%s': %dx%d %s %dfps %dkbps Profile:%s",
		s.config.Name,
		streamType,
		streamPath,
		targetInfo.Width,
		targetInfo.Height,
		targetInfo.Codec,
		targetInfo.FrameRate,
		targetInfo.BitRate,
		targetInfo.Profile,
	)
}

// startStreamDetectionRoutine runs a background goroutine that refreshes stream detection every 10 minutes for all servers
func startStreamDetectionRoutine(servers []*ONVIFServer) {
	logDebug("🔄 Global stream detection routine started (runs immediately then every 10 minutes for all cameras)")

	// Run first iteration immediately
	for _, server := range servers {
		go server.detectStreamInfo(false) // Main stream
		if server.config.SubstreamEnabled {
			go server.detectStreamInfo(true) // Substream
		}
	}

	// Then continue with periodic refresh
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		logDebug("🔄 Refreshing stream detection for all cameras...")
		for _, server := range servers {
			go server.detectStreamInfo(false) // Main stream
			if server.config.SubstreamEnabled {
				go server.detectStreamInfo(true) // Substream
			}
		}
	}
}

func (s *ONVIFServer) Start() error {
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
	logInfo("Starting ONVIF server for '%s' on %s", s.config.Name, addr)

	return http.ListenAndServe(addr, mux)
}

func (s *ONVIFServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	logDebug("[%s] Request to %s:\n%s", s.config.Name, r.URL.Path, string(body))

	// Parse SOAP envelope
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Header  struct {
			Security *Security `xml:"wsse:Security"`
		} `xml:"Header"`
		Body struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		logDebug("[%s] Failed to parse SOAP: %v", s.config.Name, err)
		s.sendSOAPFault(w, "s:Sender", "ter:InvalidArgVal", "Invalid SOAP request")
		return
	}

	// Validate WS-Security if present
	if envelope.Header.Security != nil {
		if !s.validateSecurity(envelope.Header.Security) {
			logDebug("[%s] Authentication failed", s.config.Name)
			s.sendSOAPFault(w, "s:Sender", "ter:NotAuthorized", "Authentication failed")
			return
		}
	}

	bodyContent := string(envelope.Body.Content)

	// Route to appropriate handler
	s.routeRequest(w, r, bodyContent)
}

func (s *ONVIFServer) validateSecurity(security *Security) bool {
	if security == nil {
		return true // No auth required for some operations
	}

	username := security.UsernameToken.Username
	password := security.UsernameToken.Password.Value
	nonce := security.UsernameToken.Nonce.Value
	created := security.UsernameToken.Created

	logDebug("[%s] Auth check - Username: '%s', Nonce: '%s', Created: '%s', ReceivedDigest: '%s'",
		s.config.Name, username, nonce, created, password)

	// Validate username
	if username != s.username {
		logDebug("[%s] Username mismatch: got '%s', expected '%s'", s.config.Name, username, s.username)
		return false
	}

	// Decode nonce
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		logDebug("[%s] Failed to decode nonce: %v", s.config.Name, err)
		return false
	}

	// Calculate expected password digest
	// PasswordDigest = Base64( SHA-1( nonce + created + password ) )
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(s.password))
	expectedDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	logDebug("[%s] Expected digest: '%s', Received digest: '%s'", s.config.Name, expectedDigest, password)

	return password == expectedDigest
}

func (s *ONVIFServer) routeRequest(w http.ResponseWriter, r *http.Request, bodyContent string) {
	// Get the original body for SetSystemDateAndTime
	var fullBody []byte
	if strings.Contains(bodyContent, "SetSystemDateAndTime") {
		// Re-read body for this specific operation
		r.Body = io.NopCloser(strings.NewReader(bodyContent))
		fullBody, _ = io.ReadAll(r.Body)
	}

	// Device Service
	if strings.Contains(bodyContent, "GetSystemDateAndTime") {
		s.handleGetSystemDateAndTime(w, r)
	} else if strings.Contains(bodyContent, "GetDeviceInformation") {
		s.handleGetDeviceInformation(w, r)
	} else if strings.Contains(bodyContent, "GetCapabilities") {
		s.handleGetCapabilities(w, r)
	} else if strings.Contains(bodyContent, "GetServices") {
		s.handleGetServices(w, r)
	} else if strings.Contains(bodyContent, "GetScopes") {
		s.handleGetScopes(w, r)
	} else if strings.Contains(bodyContent, "GetHostname") {
		s.handleGetHostname(w, r)
	} else if strings.Contains(bodyContent, "GetDNS") {
		s.handleGetDNS(w, r)
	} else if strings.Contains(bodyContent, "GetNetworkInterfaces") {
		s.handleGetNetworkInterfaces(w, r)
	} else if strings.Contains(bodyContent, "GetNetworkProtocols") {
		s.handleGetNetworkProtocols(w, r)
	} else if strings.Contains(bodyContent, "SystemReboot") {
		s.handleSystemReboot(w, r)
	} else if strings.Contains(bodyContent, "SetSystemDateAndTime") {
		s.handleSetSystemDateAndTime(w, r, fullBody)
	// Media Service
	} else if strings.Contains(bodyContent, "GetProfiles") {
		s.handleGetProfiles(w, r)
	} else if strings.Contains(bodyContent, "GetStreamUri") {
		s.handleGetStreamUri(w, r, bodyContent)
	} else if strings.Contains(bodyContent, "GetSnapshotUri") {
		s.handleGetSnapshotUri(w, r)
	} else if strings.Contains(bodyContent, "GetVideoSources") {
		s.handleGetVideoSources(w, r)
	} else if strings.Contains(bodyContent, "GetAudioSources") {
		s.handleGetAudioSources(w, r)
	} else if strings.Contains(bodyContent, "GetVideoEncoderConfigurationOptions") {
		s.handleGetVideoEncoderConfigurationOptions(w, r)
	} else if strings.Contains(bodyContent, "GetVideoEncoderConfigurations") {
		s.handleGetVideoEncoderConfigurations(w, r)
	} else if strings.Contains(bodyContent, "GetVideoEncoderConfiguration") {
		s.handleGetVideoEncoderConfiguration(w, r, bodyContent)
	} else if strings.Contains(bodyContent, "SetVideoEncoderConfiguration") {
		s.handleSetVideoEncoderConfiguration(w, r, bodyContent)
	// Event Service
	} else if strings.Contains(bodyContent, "Subscribe") {
		s.handleSubscribe(w, r)
	} else if strings.Contains(bodyContent, "GetEventProperties") {
		s.handleGetEventProperties(w, r)
	} else if strings.Contains(bodyContent, "CreatePullPointSubscription") {
		s.handleCreatePullPointSubscription(w, r)
	} else {
		log.Printf("[%s] Unsupported operation: %s", s.config.Name, bodyContent[:min(200, len(bodyContent))])
		s.sendSOAPFault(w, "s:Sender", "ter:ActionNotSupported", "Operation not supported")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Device Service Handlers
func (s *ONVIFServer) handleGetSystemDateAndTime(w http.ResponseWriter, r *http.Request) {
	s.timeSettingsMu.RLock()
	settings := s.timeSettings
	s.timeSettingsMu.RUnlock()

	var now time.Time
	dateTimeType := "NTP" // Default to NTP
	timeZone := "EAST-4"  // Default timezone

	if settings != nil && !settings.BaseTime.IsZero() {
		// Calculate elapsed time since SetSystemDateAndTime was called
		elapsed := time.Now().UTC().Sub(settings.SystemBaseTime)
		// Add elapsed time to the base time that was set by NVR
		now = settings.BaseTime.Add(elapsed)
		dateTimeType = settings.DateTimeType
		if settings.TimeZone != "" {
			timeZone = settings.TimeZone
		}
		logDebug("[%s] GetSystemDateAndTime: Returning synced time: %v (Type: %s, TZ: %s)",
			s.config.Name, now.Format(time.RFC3339), dateTimeType, timeZone)
	} else {
		// No time sync set by NVR yet, return actual system time
		now = time.Now().UTC()
		logDebug("[%s] GetSystemDateAndTime: Returning system time: %v (no sync yet)",
			s.config.Name, now.Format(time.RFC3339))
	}

	// Calculate local time (for simplicity, using UTC - can be enhanced with actual timezone)
	localTime := now

	response := GetSystemDateAndTimeResponse{
		SystemDateAndTime: SystemDateAndTime{
			DateTimeType:    dateTimeType,
			DaylightSavings: false,
			TimeZone: TimeZone{
				TZ: timeZone,
			},
			UTCDateTime: DateTime{
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
			LocalDateTime: DateTime{
				Time: Time{
					Hour:   localTime.Hour(),
					Minute: localTime.Minute(),
					Second: localTime.Second(),
				},
				Date: Date{
					Year:  localTime.Year(),
					Month: int(localTime.Month()),
					Day:   localTime.Day(),
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetDeviceInformation(w http.ResponseWriter, r *http.Request) {
	response := GetDeviceInformationResponse{
		Manufacturer:    s.config.Manufacturer,
		Model:           s.config.Model,
		FirmwareVersion: "5.7.10",
		SerialNumber:    s.config.Serial,
		HardwareId:      "1.0",
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)

	response := GetCapabilitiesResponse{
		Capabilities: Capabilities{
			Analytics: &AnalyticsCapabilities{
				XAddr:              baseURL + "/onvif/analytics_service",
				RuleSupport:        true,
				AnalyticsModuleSupport: true,
			},
			Device: &DeviceCapabilities{
				XAddr: baseURL + "/onvif/device_service",
				Network: &NetworkCapabilities{
					IPFilter:          true,
					ZeroConfiguration: true,
					IPVersion6:        false,
					DynDNS:            true,
					NTP:               1,
					DHCPv6:            false,
				},
				System: &SystemCapabilities{
					DiscoveryResolve:    true,
					DiscoveryBye:        true,
					RemoteDiscovery:     true,
					SystemBackup:        true,
					SystemLogging:       true,
					FirmwareUpgrade:     true,
					HttpFirmwareUpgrade: true,
					HttpSystemBackup:    true,
					HttpSystemLogging:   true,
					HttpSupportInformation: true,
				},
				IO: &IOCapabilities{
					InputConnectors: 1,
					RelayOutputs:    1,
				},
				Security: &SecurityCapabilities{
					TLS11:                true,
					TLS12:                true,
					OnboardKeyGeneration: true,
					AccessPolicyConfig:   true,
					DefaultAccessPolicy:  true,
					Dot1X:                false,
					RemoteUserHandling:   true,
					X509Token:            false,
					SAMLToken:            false,
					KerberosToken:        false,
					UsernameToken:        true,
					HttpDigest:           true,
					RELToken:             false,
				},
			},
			Events: &EventCapabilities{
				XAddr:              baseURL + "/onvif/event_service",
				WSSubscriptionPolicySupport: true,
				WSPullPointSupport: true,
				WSPausableSubscriptionManagerInterfaceSupport: false,
			},
			Imaging: &ImagingCapabilities{
				XAddr: baseURL + "/onvif/imaging_service",
			},
			Media: &MediaCapabilities{
				XAddr: baseURL + "/onvif/media_service",
				StreamingCapabilities: &StreamingCapabilities{
					RTPMulticast: false,
					RTP_TCP:      true,
					RTP_RTSP_TCP: true,
				},
			},
			Media2: &Media2Capabilities{
				XAddr: baseURL + "/onvif/media2_service",
			},
			PTZ: &PTZCapabilities{
				XAddr: baseURL + "/onvif/ptz_service",
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetServices(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)

	response := GetServicesResponse{
		Service: []Service{
			{
				Namespace: "http://www.onvif.org/ver10/device/wsdl",
				XAddr:     baseURL + "/onvif/device_service",
				Version:   Version{Major: 16, Minor: 12},
			},
			{
				Namespace: "http://www.onvif.org/ver10/media/wsdl",
				XAddr:     baseURL + "/onvif/media_service",
				Version:   Version{Major: 2, Minor: 6},
			},
			{
				Namespace: "http://www.onvif.org/ver20/media/wsdl",
				XAddr:     baseURL + "/onvif/media2_service",
				Version:   Version{Major: 2, Minor: 6},
			},
			{
				Namespace: "http://www.onvif.org/ver10/events/wsdl",
				XAddr:     baseURL + "/onvif/event_service",
				Version:   Version{Major: 2, Minor: 6},
			},
			{
				Namespace: "http://www.onvif.org/ver20/imaging/wsdl",
				XAddr:     baseURL + "/onvif/imaging_service",
				Version:   Version{Major: 2, Minor: 6},
			},
			{
				Namespace: "http://www.onvif.org/ver20/ptz/wsdl",
				XAddr:     baseURL + "/onvif/ptz_service",
				Version:   Version{Major: 2, Minor: 6},
			},
			{
				Namespace: "http://www.onvif.org/ver20/analytics/wsdl",
				XAddr:     baseURL + "/onvif/analytics_service",
				Version:   Version{Major: 2, Minor: 6},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetScopes(w http.ResponseWriter, r *http.Request) {
	response := GetScopesResponse{
		Scopes: []Scope{
			{
				ScopeDef:  "Fixed",
				ScopeItem: "onvif://www.onvif.org/type/video_encoder",
			},
			{
				ScopeDef:  "Fixed",
				ScopeItem: "onvif://www.onvif.org/type/audio_encoder",
			},
			{
				ScopeDef:  "Fixed",
				ScopeItem: "onvif://www.onvif.org/hardware/" + s.config.Model,
			},
			{
				ScopeDef:  "Fixed",
				ScopeItem: "onvif://www.onvif.org/name/" + s.config.Name,
			},
			{
				ScopeDef:  "Fixed",
				ScopeItem: "onvif://www.onvif.org/location/",
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetHostname(w http.ResponseWriter, r *http.Request) {
	response := GetHostnameResponse{
		HostnameInfo: HostnameInfo{
			FromDHCP: false,
			Name:     s.config.Name,
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetDNS(w http.ResponseWriter, r *http.Request) {
	response := GetDNSResponse{
		DNSInfo: DNSInfo{
			FromDHCP: true,
			DNSFromDHCP: []DNSName{
				{
					Type:        "IPv4",
					IPv4Address: "8.8.8.8",
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	hostIP := s.getHostIP(r)

	response := GetNetworkInterfacesResponse{
		NetworkInterfaces: []NetworkInterface{
			{
				Token:   "eth0",
				Enabled: true,
				Info: NetworkInfo{
					Name:      "eth0",
					HwAddress: "00:11:22:33:44:55",
					MTU:       1500,
				},
				Link: &NetworkLink{
					AdminSettings: LinkSettings{
						AutoNegotiation: true,
						Speed:           1000,
						Duplex:          "Full",
					},
					OperSettings: LinkSettings{
						AutoNegotiation: true,
						Speed:           1000,
						Duplex:          "Full",
					},
					InterfaceType: 6,
				},
				IPv4: &IPv4Config{
					Enabled: true,
					Config: IPv4Network{
						DHCP: true,
						FromDHCP: &PrefixedIPv4Address{
							Address:      hostIP,
							PrefixLength: 24,
						},
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

func (s *ONVIFServer) handleGetNetworkProtocols(w http.ResponseWriter, r *http.Request) {
	response := GetNetworkProtocolsResponse{
		NetworkProtocols: []NetworkProtocol{
			{Name: "HTTP", Enabled: true, Port: []int{s.config.HTTPPort}},
			{Name: "HTTPS", Enabled: false},
			{Name: "RTSP", Enabled: true, Port: []int{s.rtspPort}},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSystemReboot(w http.ResponseWriter, r *http.Request) {
	response := struct {
		XMLName xml.Name `xml:"tds:SystemRebootResponse"`
		Message string   `xml:"tds:Message"`
	}{
		Message: "Device is rebooting",
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSetSystemDateAndTime(w http.ResponseWriter, r *http.Request, requestBody []byte) {
	bodyStr := string(requestBody)
	logDebug("[%s] SetSystemDateAndTime request:\n%s", s.config.Name, bodyStr)

	// Extract time values using string parsing (more flexible than XML unmarshaling with namespaces)
	var dateTimeType string = "Manual"
	var timeZone string = "UTC"
	var year, month, day, hour, minute, second int

	// Parse DateTimeType
	if strings.Contains(bodyStr, "<DateTimeType>") {
		start := strings.Index(bodyStr, "<DateTimeType>") + len("<DateTimeType>")
		end := strings.Index(bodyStr[start:], "</DateTimeType>")
		if end > 0 {
			dateTimeType = strings.TrimSpace(bodyStr[start : start+end])
		}
	}

	// Parse TimeZone
	if strings.Contains(bodyStr, "<TZ>") {
		start := strings.Index(bodyStr, "<TZ>") + len("<TZ>")
		end := strings.Index(bodyStr[start:], "</TZ>")
		if end > 0 {
			timeZone = strings.TrimSpace(bodyStr[start : start+end])
		}
	}

	// Parse UTC DateTime if present
	hasDateTime := strings.Contains(bodyStr, "<UTCDateTime>")
	if hasDateTime {
		// Helper function to parse integer fields
		parseIntField := func(fieldName string) int {
			tag := "<" + fieldName + ">"
			endTag := "</" + fieldName + ">"
			start := strings.Index(bodyStr, tag)
			if start == -1 {
				return 0
			}
			start += len(tag)
			end := strings.Index(bodyStr[start:], endTag)
			if end == -1 {
				return 0
			}
			valueStr := strings.TrimSpace(bodyStr[start : start+end])
			value := 0
			fmt.Sscanf(valueStr, "%d", &value)
			return value
		}

		year = parseIntField("Year")
		month = parseIntField("Month")
		day = parseIntField("Day")
		hour = parseIntField("Hour")
		minute = parseIntField("Minute")
		second = parseIntField("Second")

		if year > 0 && month > 0 && day > 0 {
			requestedTime := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
			systemBaseTime := time.Now().UTC()

			// Store time synchronization settings
			settings := &ClientTimeSettings{
				BaseTime:       requestedTime,
				SystemBaseTime: systemBaseTime,
				DateTimeType:   dateTimeType,
				TimeZone:       timeZone,
			}

			s.timeSettingsMu.Lock()
			s.timeSettings = settings
			s.timeSettingsMu.Unlock()

			logInfo("[%s] ✓ Time synchronized by NVR:", s.config.Name)
			logDebug("  - DateTimeType: %s", dateTimeType)
			logDebug("  - TimeZone: %s", timeZone)
			logDebug("  - NVR time: %v", requestedTime.Format(time.RFC3339))
			logDebug("  - System time: %v", systemBaseTime.Format(time.RFC3339))
			logDebug("  - Time offset: %v", requestedTime.Sub(systemBaseTime))
		} else {
			logDebug("[%s] SetSystemDateAndTime: Invalid date/time values (Y:%d M:%d D:%d)",
				s.config.Name, year, month, day)
		}
	} else {
		logDebug("[%s] SetSystemDateAndTime: No UTCDateTime provided", s.config.Name)
	}

	response := struct {
		XMLName xml.Name `xml:"tds:SetSystemDateAndTimeResponse"`
	}{}
	s.sendSOAPResponse(w, response)
}

// Media Service Handlers
func (s *ONVIFServer) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := []Profile{
		{
			Token: "Profile000",
			Fixed: true,
			Name:  "Profile000",
			VideoSourceConfiguration: &VideoSourceConfiguration{
				Token:       "V_SRC_CFG_000",
				Name:        "V_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "V_SRC_000",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  1920,
					Height: 1080,
				},
			},
			AudioSourceConfiguration: &AudioSourceConfiguration{
				Token:       "A_SRC_CFG_000",
				Name:        "A_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "A_SRC_000",
			},
			VideoEncoderConfiguration: &VideoEncoderConfiguration{
				Token:    "V_ENC_CFG_000",
				Name:     "V_ENC_CFG_000",
				UseCount: 1,
				Encoding: "H264",
				Resolution: Resolution{
					Width:  1920,
					Height: 1080,
				},
				Quality: 5.0,
				RateControl: &RateControl{
					FrameRateLimit:   25,
					EncodingInterval: 1,
					BitrateLimit:     4096,
				},
				H264: &H264Config{
					GovLength:   50,
					H264Profile: "High",
				},
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			AudioEncoderConfiguration: &AudioEncoderConfiguration{
				Token:      "A_ENC_CFG_000",
				Name:       "A_ENC_CFG_000",
				UseCount:   1,
				Encoding:   "AAC",
				Bitrate:    32,
				SampleRate: 8,
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			VideoAnalyticsConfiguration: &VideoAnalyticsConfiguration{
				Token:    "V_ANALYTICS_CFG_000",
				Name:     "V_ANALYTICS_CFG_000",
				UseCount: 1,
			},
			PTZConfiguration: &PTZConfiguration{
				Token:     "PTZ_CFG_000",
				Name:      "PTZ_CFG_000",
				UseCount:  1,
				NodeToken: "PTZ_NODE_000",
			},
			MetadataConfiguration: &MetadataConfiguration{
				Token:          "META_CFG_000",
				Name:           "META_CFG_000",
				UseCount:       1,
				SessionTimeout: "PT60S",
			},
		},
		{
			Token: "Profile001",
			Fixed: true,
			Name:  "Profile001",
			VideoSourceConfiguration: &VideoSourceConfiguration{
				Token:       "V_SRC_CFG_000",
				Name:        "V_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "V_SRC_000",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  1920,
					Height: 1080,
				},
			},
			AudioSourceConfiguration: &AudioSourceConfiguration{
				Token:       "A_SRC_CFG_000",
				Name:        "A_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "A_SRC_000",
			},
			VideoEncoderConfiguration: &VideoEncoderConfiguration{
				Token:    "V_ENC_CFG_001",
				Name:     "V_ENC_CFG_001",
				UseCount: 1,
				Encoding: "H264",
				Resolution: Resolution{
					Width:  704,
					Height: 576,
				},
				Quality: 5.0,
				RateControl: &RateControl{
					FrameRateLimit:   25,
					EncodingInterval: 1,
					BitrateLimit:     1024,
				},
				H264: &H264Config{
					GovLength:   50,
					H264Profile: "High",
				},
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			AudioEncoderConfiguration: &AudioEncoderConfiguration{
				Token:      "A_ENC_CFG_001",
				Name:       "A_ENC_CFG_001",
				UseCount:   1,
				Encoding:   "AAC",
				Bitrate:    32,
				SampleRate: 8,
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			VideoAnalyticsConfiguration: &VideoAnalyticsConfiguration{
				Token:    "V_ANALYTICS_CFG_001",
				Name:     "V_ANALYTICS_CFG_001",
				UseCount: 1,
			},
			PTZConfiguration: &PTZConfiguration{
				Token:     "PTZ_CFG_001",
				Name:      "PTZ_CFG_001",
				UseCount:  1,
				NodeToken: "PTZ_NODE_000",
			},
			MetadataConfiguration: &MetadataConfiguration{
				Token:          "META_CFG_001",
				Name:           "META_CFG_001",
				UseCount:       1,
				SessionTimeout: "PT60S",
			},
		},
		{
			Token: "Profile002",
			Fixed: true,
			Name:  "Profile002",
			VideoSourceConfiguration: &VideoSourceConfiguration{
				Token:       "V_SRC_CFG_000",
				Name:        "V_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "V_SRC_000",
				Bounds: Bounds{
					X:      0,
					Y:      0,
					Width:  1920,
					Height: 1080,
				},
			},
			AudioSourceConfiguration: &AudioSourceConfiguration{
				Token:       "A_SRC_CFG_000",
				Name:        "A_SRC_CFG_000",
				UseCount:    3,
				SourceToken: "A_SRC_000",
			},
			VideoEncoderConfiguration: &VideoEncoderConfiguration{
				Token:    "V_ENC_CFG_002",
				Name:     "V_ENC_CFG_002",
				UseCount: 1,
				Encoding: "H264",
				Resolution: Resolution{
					Width:  352,
					Height: 288,
				},
				Quality: 5.0,
				RateControl: &RateControl{
					FrameRateLimit:   25,
					EncodingInterval: 1,
					BitrateLimit:     512,
				},
				H264: &H264Config{
					GovLength:   50,
					H264Profile: "High",
				},
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			AudioEncoderConfiguration: &AudioEncoderConfiguration{
				Token:      "A_ENC_CFG_002",
				Name:       "A_ENC_CFG_002",
				UseCount:   1,
				Encoding:   "AAC",
				Bitrate:    32,
				SampleRate: 8,
				Multicast: Multicast{
					Address: MulticastAddress{
						Type:        "IPv4",
						IPv4Address: "0.0.0.0",
					},
					Port:      0,
					TTL:       5,
					AutoStart: false,
				},
				SessionTimeout: "PT60S",
			},
			VideoAnalyticsConfiguration: &VideoAnalyticsConfiguration{
				Token:    "V_ANALYTICS_CFG_002",
				Name:     "V_ANALYTICS_CFG_002",
				UseCount: 1,
			},
			PTZConfiguration: &PTZConfiguration{
				Token:     "PTZ_CFG_002",
				Name:      "PTZ_CFG_002",
				UseCount:  1,
				NodeToken: "PTZ_NODE_000",
			},
			MetadataConfiguration: &MetadataConfiguration{
				Token:          "META_CFG_002",
				Name:           "META_CFG_002",
				UseCount:       1,
				SessionTimeout: "PT60S",
			},
		},
	}

	response := GetProfilesResponse{
		Profiles: profiles,
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetStreamUri(w http.ResponseWriter, r *http.Request, bodyContent string) {
	hostIP := s.getHostIP(r)

	// Log the full request for debugging
	logDebug("[%s] 🎬 GetStreamUri REQUEST BODY:\n%s", s.config.Name, bodyContent)

	// Determine stream subtype based on StreamType or profile
	subtype := 0 // Default main stream
	profileToken := "Profile000"

	// Check for StreamType parameter (Hikvision uses this)
	if strings.Contains(bodyContent, "<StreamType>RTP-Unicast-Substream</StreamType>") ||
		strings.Contains(bodyContent, "<StreamType>RTP-Unicast-Sub</StreamType>") ||
		strings.Contains(bodyContent, "<tt:StreamType>RTP-Unicast-Substream</tt:StreamType>") ||
		strings.Contains(bodyContent, "<tt:StreamType>RTP-Unicast-Sub</tt:StreamType>") {
		subtype = 1
		logDebug("[%s] 🎬 GetStreamUri: StreamType indicates SUBSTREAM", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile001") {
		subtype = 1
		profileToken = "Profile001"
		logDebug("[%s] 🎬 GetStreamUri: Profile001 detected -> SUBSTREAM", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile002") {
		subtype = 2
		profileToken = "Profile002"
		logDebug("[%s] 🎬 GetStreamUri: Profile002 detected -> SUBSTREAM (low quality)", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile000") {
		profileToken = "Profile000"
		logDebug("[%s] 🎬 GetStreamUri: Profile000 detected -> MAIN stream", s.config.Name)
	}

	// Determine stream path - use different paths for different profiles
	streamPath := s.config.RTSPStream
	streamName := "MAIN"
	if subtype == 1 && s.config.SubstreamEnabled {
		streamPath = s.config.SubstreamPath
		streamName = "SUBSTREAM"
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub"
		}
	} else if subtype == 2 && s.config.SubstreamEnabled {
		// For Profile002 (lowest quality), append _sub2 or use substream path
		streamPath = s.config.SubstreamPath
		streamName = "SUBSTREAM"
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub"
		}
	}

	// Build RTSP URL - if rtspPort is 8554 (go2rtc), use simple paths
	// Otherwise use query parameters for real camera compatibility
	var rtspURL string
	if s.rtspPort == 8554 {
		// go2rtc format: simple paths without query parameters
		rtspURL = fmt.Sprintf("rtsp://%s:%d%s", hostIP, s.rtspPort, streamPath)
	} else {
		// Real camera format: with query parameters
		rtspURL = fmt.Sprintf("rtsp://%s:%d%s?channel=1&subtype=%d&unicast=true&proto=Onvif",
			hostIP, s.rtspPort, streamPath, subtype)
	}

	logInfo("[%s] 🎬 GetStreamUri: Profile=%s, Subtype=%d, Stream=%s, Path='%s' -> %s",
		s.config.Name, profileToken, subtype, streamName, streamPath, rtspURL)

	response := GetStreamUriResponse{
		MediaUri: MediaUri{
			Uri:                 rtspURL,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetSnapshotUri(w http.ResponseWriter, r *http.Request) {
	hostIP := s.getHostIP(r)
	snapshotURL := fmt.Sprintf("http://%s:%d/snapshot", hostIP, s.config.HTTPPort)

	response := GetSnapshotUriResponse{
		MediaUri: MediaUri{
			Uri:                 snapshotURL,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetVideoSources(w http.ResponseWriter, r *http.Request) {
	response := GetVideoSourcesResponse{
		VideoSources: []VideoSource{
			{
				Token:     "VideoSource_1",
				Framerate: 25.0,
				Resolution: Resolution{
					Width:  1920,
					Height: 1080,
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleGetAudioSources(w http.ResponseWriter, r *http.Request) {
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

func (s *ONVIFServer) handleGetVideoEncoderConfigurations(w http.ResponseWriter, r *http.Request) {
	configs := []VideoEncoderConfiguration{
		{
			Token:    "VideoEncoderToken",
			Name:     "VideoEncoderConfig",
			UseCount: 1,
			Encoding: "H264",
			Resolution: Resolution{
				Width:  1920,
				Height: 1080,
			},
			Quality: 5.0,
			RateControl: &RateControl{
				FrameRateLimit:   25,
				EncodingInterval: 1,
				BitrateLimit:     4096,
			},
			H264: &H264Config{
				GovLength:   50,
				H264Profile: "High",
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
		},
	}

	response := GetVideoEncoderConfigurationsResponse{
		Configurations: configs,
	}
	s.sendSOAPResponse(w, response)
}

// probeRTSPResolution uses ffprobe to detect actual stream resolution
// getStreamInfoForToken returns the appropriate StreamInfo based on token
// Returns (streamInfo, isSubstream)
func (s *ONVIFServer) getStreamInfoForToken(token string) (*StreamInfo, bool) {
	// Check if this is a known substream token
	isSubstream := false
	if token == "V_ENC_CFG_001" || token == "V_ENC_CFG_002" {
		isSubstream = true
	} else {
		// For unknown tokens (e.g., VideoEncoder001), check bitrate to determine stream type
		s.bitrateMu.RLock()
		bitrate, bitrateSet := s.bitrateCache[token]
		s.bitrateMu.RUnlock()

		// If bitrate is set and it's low (<=1024), use substream
		if bitrateSet && bitrate <= 1024 {
			isSubstream = true
		}
	}

	s.streamInfoMu.RLock()
	defer s.streamInfoMu.RUnlock()

	if isSubstream {
		return s.substreamInfo, true
	}
	return s.streamInfo, false
}

// getRTSPURLForToken returns the RTSP URL for a given encoder token
func (s *ONVIFServer) getRTSPURLForToken(token string, hostIP string) string {
	_, isSubstream := s.getStreamInfoForToken(token)

	streamPath := s.config.RTSPStream
	if isSubstream {
		if s.config.SubstreamEnabled && s.config.SubstreamPath != "" {
			streamPath = s.config.SubstreamPath
		} else {
			streamPath = s.config.RTSPStream + "_sub"
		}
	}

	return fmt.Sprintf("rtsp://%s:%d%s", hostIP, s.rtspPort, streamPath)
}

func (s *ONVIFServer) handleGetVideoEncoderConfiguration(w http.ResponseWriter, r *http.Request, bodyContent string) {
	// Extract token from request - look for ConfigurationToken in XML
	token := "V_ENC_CFG_000" // Default to main stream
	tokenRegex := regexp.MustCompile(`<.*?:?ConfigurationToken>([^<]+)</`)
	if match := tokenRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		token = match[1]
	}

	// Get stream info for this token
	streamInfo, isSubstream := s.getStreamInfoForToken(token)

	// Determine stream name for logging
	streamName := s.config.RTSPStream
	if isSubstream {
		if s.config.SubstreamEnabled && s.config.SubstreamPath != "" {
			streamName = s.config.SubstreamPath
		} else {
			streamName = s.config.RTSPStream + "_sub"
		}
	}

	logDebug("[%s] 📹 GetVideoEncoderConfiguration for stream '%s' (token %s): %dx%d %s %dfps %dkbps Profile:%s",
		s.config.Name,
		streamName,
		token,
		streamInfo.Width,
		streamInfo.Height,
		streamInfo.Codec,
		streamInfo.FrameRate,
		streamInfo.BitRate,
		streamInfo.Profile,
	)

	config := VideoEncoderConfiguration{
		Token:    token,
		Name:     token,
		UseCount: 1,
		Encoding: streamInfo.Codec,
		Resolution: Resolution{
			Width:  streamInfo.Width,
			Height: streamInfo.Height,
		},
		Quality: 5.0,
		RateControl: &RateControl{
			FrameRateLimit:   streamInfo.FrameRate,
			EncodingInterval: 1,
			BitrateLimit:     streamInfo.BitRate,
		},
		H264: &H264Config{
			GovLength:   50,
			H264Profile: streamInfo.Profile,
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

	response := GetVideoEncoderConfigurationResponse{
		Configuration: config,
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSetVideoEncoderConfiguration(w http.ResponseWriter, r *http.Request, bodyContent string) {
	// Log the request to understand what the NVR is trying to change
	logDebug("[%s] 🔧 SetVideoEncoderConfiguration REQUEST:\n%s", s.config.Name, bodyContent)

	// Extract token if present
	tokenRegex := regexp.MustCompile(`<.*?:?Configuration.*?token="([^"]+)"`)
	token := ""
	if match := tokenRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		token = match[1]
		logDebug("[%s] 🔧 SetVideoEncoderConfiguration: Token=%s", s.config.Name, token)
	}

	// Extract BitrateLimit from request (this is the key parameter for stream switching)
	bitrateRegex := regexp.MustCompile(`<.*?:?BitrateLimit>(\d+)</`)
	if match := bitrateRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		bitrateStr := match[1]
		if bitrate, err := strconv.Atoi(bitrateStr); err == nil {
			// Store the bitrate in cache
			s.bitrateMu.Lock()
			s.bitrateCache[token] = bitrate
			s.bitrateMu.Unlock()
			logDebug("[%s] 🔧 SetVideoEncoderConfiguration: BitrateLimit stored -> %d kbps for token %s", s.config.Name, bitrate, token)
		} else {
			logDebug("[%s] ⚠️ SetVideoEncoderConfiguration: Failed to parse BitrateLimit: %s", s.config.Name, bitrateStr)
		}
	}

	// Return empty success response (matching real camera behavior from logs)
	// The response uses Media2 namespace (tr2)
	responseXML := `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope"
                   xmlns:tr2="http://www.onvif.org/ver20/media/wsdl">
  <SOAP-ENV:Body>
    <tr2:SetVideoEncoderConfigurationResponse></tr2:SetVideoEncoderConfigurationResponse>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

	logDebug("[%s] ✅ SetVideoEncoderConfiguration: Acknowledging configuration change", s.config.Name)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseXML))
}

func (s *ONVIFServer) handleGetVideoEncoderConfigurationOptions(w http.ResponseWriter, r *http.Request) {
	// Define available resolutions for JPEG
	jpegResolutions := []Resolution{
		{Width: 1920, Height: 1080},
		{Width: 1280, Height: 960},
		{Width: 1280, Height: 720},
		{Width: 704, Height: 576},
		{Width: 640, Height: 480},
		{Width: 352, Height: 288},
		{Width: 320, Height: 240},
		{Width: 176, Height: 144},
	}

	// Define available resolutions for H264
	h264Resolutions := []Resolution{
		{Width: 1920, Height: 1080},
		{Width: 1280, Height: 960},
		{Width: 1280, Height: 720},
		{Width: 704, Height: 576},
		{Width: 640, Height: 480},
		{Width: 352, Height: 288},
	}

	response := GetVideoEncoderConfigurationOptionsResponse{
		Options: VideoEncoderConfigurationOptions{
			QualityRange: QualityRange{
				Min: 1,
				Max: 6,
			},
			JPEG: &JpegOptions{
				ResolutionsAvailable: jpegResolutions,
				FrameRateRange: IntRange{
					Min: 1,
					Max: 25,
				},
				EncodingIntervalRange: IntRange{
					Min: 1,
					Max: 1,
				},
			},
			H264: &H264Options{
				ResolutionsAvailable: h264Resolutions,
				GovLengthRange: IntRange{
					Min: 1,
					Max: 400,
				},
				FrameRateRange: IntRange{
					Min: 1,
					Max: 25,
				},
				EncodingIntervalRange: IntRange{
					Min: 1,
					Max: 1,
				},
				H264ProfilesSupported: []string{"Baseline", "Main", "High"},
			},
			Extension: &EncoderExtension{
				JPEG: &JpegOptions2{
					ResolutionsAvailable: jpegResolutions,
					FrameRateRange: IntRange{
						Min: 1,
						Max: 25,
					},
					EncodingIntervalRange: IntRange{
						Min: 1,
						Max: 1,
					},
					BitrateRange: IntRange{
						Min: 2,
						Max: 20480,
					},
				},
				H264: &H264Options2{
					ResolutionsAvailable: h264Resolutions,
					GovLengthRange: IntRange{
						Min: 1,
						Max: 400,
					},
					FrameRateRange: IntRange{
						Min: 1,
						Max: 25,
					},
					EncodingIntervalRange: IntRange{
						Min: 1,
						Max: 1,
					},
					H264ProfilesSupported: []string{"Baseline", "Main", "High"},
					BitrateRange: IntRange{
						Min: 2,
						Max: 20480,
					},
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

// Event Service Handlers
func (s *ONVIFServer) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)
	subscriptionAddr := baseURL + "/onvif/subscription/events"

	now := time.Now().UTC()
	terminationTime := now.Add(1 * time.Hour)

	response := SubscribeResponse{
		SubscriptionReference: SubscriptionReference{
			Address: subscriptionAddr,
		},
		CurrentTime:     now.Format(time.RFC3339),
		TerminationTime: terminationTime.Format(time.RFC3339),
	}

	// Build SOAP envelope with proper namespaces
	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		WSNT    string      `xml:"xmlns:wsnt,attr"`
		WSA5    string      `xml:"xmlns:wsa5,attr"`
		Body    struct {
			XMLName xml.Name    `xml:"SOAP-ENV:Body"`
			Content interface{} `xml:",any"`
		} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		WSNT:    "http://docs.oasis-open.org/wsn/b-2",
		WSA5:    "http://www.w3.org/2005/08/addressing",
	}
	envelope.Body.Content = response

	output, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		logInfo("[%s] Failed to marshal response: %v", s.config.Name, err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseStr := xml.Header + string(output)
	logDebug("[%s] Sending Response:\n%s", s.config.Name, responseStr)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseStr))
}

func (s *ONVIFServer) handleGetEventProperties(w http.ResponseWriter, r *http.Request) {
	response := GetEventPropertiesResponse{
		TopicNamespaceLocation: []string{
			"http://www.onvif.org/onvif/ver10/topics/topicns.xml",
		},
		FixedTopicSet: true,
	}
	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleCreatePullPointSubscription(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)
	subscriptionAddr := baseURL + "/onvif/subscription/pullpoint"

	now := time.Now().UTC()
	terminationTime := now.Add(1 * time.Hour)

	response := CreatePullPointSubscriptionResponse{
		SubscriptionReference: SubscriptionReference{
			Address: subscriptionAddr,
		},
		CurrentTime:     now.Format(time.RFC3339),
		TerminationTime: terminationTime.Format(time.RFC3339),
	}

	s.sendSOAPResponse(w, response)
}

func (s *ONVIFServer) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	// Return a simple 1x1 pixel JPEG
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
		TEV     string      `xml:"xmlns:tev,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		TT:      "http://www.onvif.org/ver10/schema",
		TDS:     "http://www.onvif.org/ver10/device/wsdl",
		TRT:     "http://www.onvif.org/ver10/media/wsdl",
		TEV:     "http://www.onvif.org/ver10/events/wsdl",
		Body:    body,
	}

	output, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		logInfo("[%s] Failed to marshal response: %v", s.config.Name, err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseStr := xml.Header + string(output)
	logDebug("[%s] Sending Response:\n%s", s.config.Name, responseStr)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseStr))
}

func (s *ONVIFServer) sendSOAPFault(w http.ResponseWriter, code, subcode, reason string) {
	fault := SOAPFault{}
	fault.Code.Value = code
	if subcode != "" {
		fault.Code.Subcode = &struct {
			Value string `xml:"SOAP-ENV:Value"`
		}{Value: subcode}
	}
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
		TER     string      `xml:"xmlns:ter,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		TER:     "http://www.onvif.org/ver10/error",
		Body:    body,
	}

	output, _ := xml.MarshalIndent(envelope, "", "  ")

	logDebug("[%s] Sending SOAP Fault: %s - %s", s.config.Name, code, reason)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(xml.Header))
	w.Write(output)
}

func (s *ONVIFServer) getBaseURL(r *http.Request) string {
	hostIP := s.getHostIP(r)
	return fmt.Sprintf("http://%s:%d", hostIP, s.config.HTTPPort)
}

func (s *ONVIFServer) getHostIP(r *http.Request) string {
	// Get from Host header
	host := r.Host
	if host != "" {
		if idx := strings.Index(host, ":"); idx != -1 {
			return host[:idx]
		}
		return host
	}

	// Fallback to configured host
	return s.rtspHost
}

// WS-Discovery
func startDiscoveryService() {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		logInfo("Failed to resolve discovery address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		logInfo("Failed to listen on multicast: %v", err)
		return
	}
	defer conn.Close()

	logInfo("WS-Discovery service started on 239.255.255.250:3702")

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			logDebug("Discovery read error: %v", err)
			continue
		}

		message := string(buf[:n])
		if strings.Contains(message, "Probe") && strings.Contains(message, "onvif") {
			logDebug("Received ONVIF discovery probe from %s", remoteAddr)
			// In production, respond with ProbeMatches
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

func generateNonce() string {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	return base64.StdEncoding.EncodeToString(nonce)
}

func generatePasswordDigest(nonce, created, password string) string {
	nonceBytes, _ := base64.StdEncoding.DecodeString(nonce)
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(password))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func calculateMD5(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
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

	// Set default credentials if not specified
	if config.Username == "" {
		config.Username = "admin"
	}
	if config.Password == "" {
		config.Password = "admin"
	}

	return &config, nil
}

func main() {
	// Parse command line flags
	debug := flag.Bool("debug", false, "Enable debug logging (verbose output)")
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	debugMode = *debug

	if debugMode {
		logInfo("Debug mode enabled - verbose logging active")
	}

	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Auto-detect IP if not configured
	rtspHost := config.RTSPHost
	if rtspHost == "" {
		rtspHost = getOutboundIP()
		logInfo("Auto-detected IP: %s", rtspHost)
	}

	// Start WS-Discovery if enabled
	if config.EnableDiscovery {
		go startDiscoveryService()
	}

	// Create all servers first
	var servers []*ONVIFServer
	for _, camConfig := range config.Cameras {
		server := NewONVIFServer(camConfig, rtspHost, config.RTSPPort, config.Username, config.Password)
		servers = append(servers, server)
	}

	// Start single global stream detection routine for all cameras
	go startStreamDetectionRoutine(servers)

	// Give the routine a moment to start first iteration
	time.Sleep(100 * time.Millisecond)

	// Start ONVIF server for each camera
	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s *ONVIFServer) {
			defer wg.Done()
			if err := s.Start(); err != nil {
				logInfo("Server for '%s' failed: %v", s.config.Name, err)
			}
		}(server)

		time.Sleep(100 * time.Millisecond)
	}

	logInfo("All ONVIF servers started successfully")
	wg.Wait()
}
