package onvif

import (
	"encoding/xml"
	"time"
)

// Time structures used by the Device service.
type systemDateAndTime struct {
	DateTimeType    string    `xml:"tt:DateTimeType"`
	DaylightSavings bool      `xml:"tt:DaylightSavings"`
	TimeZone        timeZone  `xml:"tt:TimeZone,omitempty"`
	UTCDateTime     dateTime  `xml:"tt:UTCDateTime"`
	LocalDateTime   dateTime  `xml:"tt:LocalDateTime,omitempty"`
	Extension       *struct{} `xml:"tt:Extension,omitempty"`
}

type timeZone struct {
	TZ string `xml:"tt:TZ"`
}

type dateTime struct {
	Time timeOfDay `xml:"tt:Time"`
	Date date      `xml:"tt:Date"`
}

type timeOfDay struct {
	Hour   int `xml:"tt:Hour"`
	Minute int `xml:"tt:Minute"`
	Second int `xml:"tt:Second"`
}

type date struct {
	Year  int `xml:"tt:Year"`
	Month int `xml:"tt:Month"`
	Day   int `xml:"tt:Day"`
}

// Device service responses.
type getSystemDateAndTimeResponse struct {
	XMLName           xml.Name          `xml:"tds:GetSystemDateAndTimeResponse"`
	SystemDateAndTime systemDateAndTime `xml:"tds:SystemDateAndTime"`
}

type getDeviceInformationResponse struct {
	XMLName         xml.Name `xml:"tds:GetDeviceInformationResponse"`
	Manufacturer    string   `xml:"tds:Manufacturer"`
	Model           string   `xml:"tds:Model"`
	FirmwareVersion string   `xml:"tds:FirmwareVersion"`
	SerialNumber    string   `xml:"tds:SerialNumber"`
	HardwareId      string   `xml:"tds:HardwareId"`
}

type getCapabilitiesResponse struct {
	XMLName      xml.Name     `xml:"tds:GetCapabilitiesResponse"`
	Capabilities capabilities `xml:"tds:Capabilities"`
}

type capabilities struct {
	Analytics *analyticsCapabilities `xml:"tt:Analytics,omitempty"`
	Device    *deviceCapabilities    `xml:"tt:Device,omitempty"`
	Events    *eventCapabilities     `xml:"tt:Events,omitempty"`
	Imaging   *imagingCapabilities   `xml:"tt:Imaging,omitempty"`
	Media     *mediaCapabilities     `xml:"tt:Media,omitempty"`
	Media2    *media2Capabilities    `xml:"tt:Media2,omitempty"`
	PTZ       *ptzCapabilities       `xml:"tt:PTZ,omitempty"`
	Extension *struct{}              `xml:"tt:Extension,omitempty"`
}

type analyticsCapabilities struct {
	XAddr                  string `xml:"tt:XAddr"`
	RuleSupport            bool   `xml:"tt:RuleSupport"`
	AnalyticsModuleSupport bool   `xml:"tt:AnalyticsModuleSupport"`
}

type deviceCapabilities struct {
	XAddr    string                `xml:"tt:XAddr"`
	Network  *networkCapabilities  `xml:"tt:Network,omitempty"`
	System   *systemCapabilities   `xml:"tt:System,omitempty"`
	IO       *ioCapabilities       `xml:"tt:IO,omitempty"`
	Security *securityCapabilities `xml:"tt:Security,omitempty"`
}

type networkCapabilities struct {
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

type systemCapabilities struct {
	DiscoveryResolve       bool `xml:"tt:DiscoveryResolve"`
	DiscoveryBye           bool `xml:"tt:DiscoveryBye"`
	RemoteDiscovery        bool `xml:"tt:RemoteDiscovery"`
	SystemBackup           bool `xml:"tt:SystemBackup"`
	SystemLogging          bool `xml:"tt:SystemLogging"`
	FirmwareUpgrade        bool `xml:"tt:FirmwareUpgrade"`
	HttpFirmwareUpgrade    bool `xml:"tt:HttpFirmwareUpgrade,omitempty"`
	HttpSystemBackup       bool `xml:"tt:HttpSystemBackup,omitempty"`
	HttpSystemLogging      bool `xml:"tt:HttpSystemLogging,omitempty"`
	HttpSupportInformation bool `xml:"tt:HttpSupportInformation,omitempty"`
}

type ioCapabilities struct {
	InputConnectors int `xml:"tt:InputConnectors,omitempty"`
	RelayOutputs    int `xml:"tt:RelayOutputs,omitempty"`
}

type securityCapabilities struct {
	TLS11                bool `xml:"tt:TLS1.1"`
	TLS12                bool `xml:"tt:TLS1.2"`
	OnboardKeyGeneration bool `xml:"tt:OnboardKeyGeneration"`
	AccessPolicyConfig   bool `xml:"tt:AccessPolicyConfig"`
	DefaultAccessPolicy  bool `xml:"tt:DefaultAccessPolicy,omitempty"`
	Dot1X                bool `xml:"tt:Dot1X"`
	RemoteUserHandling   bool `xml:"tt:RemoteUserHandling"`
	X509Token            bool `xml:"tt:X.509Token"`
	SAMLToken            bool `xml:"tt:SAMLToken"`
	KerberosToken        bool `xml:"tt:KerberosToken"`
	UsernameToken        bool `xml:"tt:UsernameToken"`
	HttpDigest           bool `xml:"tt:HttpDigest"`
	RELToken             bool `xml:"tt:RELToken"`
}

type eventCapabilities struct {
	XAddr                                         string `xml:"tt:XAddr"`
	WSSubscriptionPolicySupport                   bool   `xml:"tt:WSSubscriptionPolicySupport"`
	WSPullPointSupport                            bool   `xml:"tt:WSPullPointSupport"`
	WSPausableSubscriptionManagerInterfaceSupport bool   `xml:"tt:WSPausableSubscriptionManagerInterfaceSupport,omitempty"`
}

type imagingCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type mediaCapabilities struct {
	XAddr                 string                 `xml:"tt:XAddr"`
	StreamingCapabilities *streamingCapabilities `xml:"tt:StreamingCapabilities,omitempty"`
}

type streamingCapabilities struct {
	RTPMulticast bool `xml:"tt:RTPMulticast,omitempty"`
	RTP_TCP      bool `xml:"tt:RTP_TCP"`
	RTP_RTSP_TCP bool `xml:"tt:RTP_RTSP_TCP"`
}

type ptzCapabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type media2Capabilities struct {
	XAddr string `xml:"tt:XAddr"`
}

type getServicesResponse struct {
	XMLName xml.Name  `xml:"tds:GetServicesResponse"`
	Service []service `xml:"tds:Service"`
}

type service struct {
	Namespace    string               `xml:"tds:Namespace"`
	XAddr        string               `xml:"tds:XAddr"`
	Capabilities *serviceCapabilities `xml:"tds:Capabilities,omitempty"`
	Version      version              `xml:"tds:Version"`
}

type serviceCapabilities struct {
	Network  bool `xml:"tds:Network,attr,omitempty"`
	Security bool `xml:"tds:Security,attr,omitempty"`
	System   bool `xml:"tds:System,attr,omitempty"`
}

type version struct {
	Major int `xml:"tt:Major"`
	Minor int `xml:"tt:Minor"`
}

type getScopesResponse struct {
	XMLName xml.Name `xml:"tds:GetScopesResponse"`
	Scopes  []scope  `xml:"tds:Scopes"`
}

type scope struct {
	ScopeDef  string `xml:"tds:ScopeDef"`
	ScopeItem string `xml:"tds:ScopeItem"`
}

type getHostnameResponse struct {
	XMLName      xml.Name     `xml:"tds:GetHostnameResponse"`
	HostnameInfo hostnameInfo `xml:"tds:HostnameInformation"`
}

type hostnameInfo struct {
	FromDHCP bool   `xml:"tt:FromDHCP"`
	Name     string `xml:"tt:Name,omitempty"`
}

type getDNSResponse struct {
	XMLName xml.Name `xml:"tds:GetDNSResponse"`
	DNSInfo dnsInfo  `xml:"tds:DNSInformation"`
}

type dnsInfo struct {
	FromDHCP    bool      `xml:"tt:FromDHCP"`
	DNSFromDHCP []dnsName `xml:"tt:DNSFromDHCP,omitempty"`
	DNSManual   []dnsName `xml:"tt:DNSManual,omitempty"`
}

type dnsName struct {
	Type        string `xml:"tt:Type"`
	IPv4Address string `xml:"tt:IPv4Address,omitempty"`
	IPv6Address string `xml:"tt:IPv6Address,omitempty"`
}

type getNetworkInterfacesResponse struct {
	XMLName           xml.Name           `xml:"tds:GetNetworkInterfacesResponse"`
	NetworkInterfaces []networkInterface `xml:"tds:NetworkInterfaces"`
}

type networkInterface struct {
	Token   string       `xml:"token,attr"`
	Enabled bool         `xml:"tt:Enabled"`
	Info    networkInfo  `xml:"tt:Info,omitempty"`
	Link    *networkLink `xml:"tt:Link,omitempty"`
	IPv4    *ipv4Config  `xml:"tt:IPv4,omitempty"`
	IPv6    *ipv6Config  `xml:"tt:IPv6,omitempty"`
}

type networkInfo struct {
	Name      string `xml:"tt:Name,omitempty"`
	HwAddress string `xml:"tt:HwAddress"`
	MTU       int    `xml:"tt:MTU,omitempty"`
}

type networkLink struct {
	AdminSettings linkSettings `xml:"tt:AdminSettings"`
	OperSettings  linkSettings `xml:"tt:OperSettings"`
	InterfaceType int          `xml:"tt:InterfaceType"`
}

type linkSettings struct {
	AutoNegotiation bool   `xml:"tt:AutoNegotiation"`
	Speed           int    `xml:"tt:Speed"`
	Duplex          string `xml:"tt:Duplex"`
}

type ipv4Config struct {
	Enabled bool        `xml:"tt:Enabled"`
	Config  ipv4Network `xml:"tt:Config"`
}

type ipv4Network struct {
	Manual    []prefixedIPv4Address `xml:"tt:Manual,omitempty"`
	LinkLocal *prefixedIPv4Address  `xml:"tt:LinkLocal,omitempty"`
	FromDHCP  *prefixedIPv4Address  `xml:"tt:FromDHCP,omitempty"`
	DHCP      bool                  `xml:"tt:DHCP"`
}

type prefixedIPv4Address struct {
	Address      string `xml:"tt:Address"`
	PrefixLength int    `xml:"tt:PrefixLength"`
}

type ipv6Config struct {
	Enabled bool `xml:"tt:Enabled"`
}

type getNetworkProtocolsResponse struct {
	XMLName          xml.Name          `xml:"tds:GetNetworkProtocolsResponse"`
	NetworkProtocols []networkProtocol `xml:"tds:NetworkProtocols"`
}

type networkProtocol struct {
	Name    string `xml:"tt:Name"`
	Enabled bool   `xml:"tt:Enabled"`
	Port    []int  `xml:"tt:Port,omitempty"`
}

// clientTimeSettings stores time synchronization settings per client.
type clientTimeSettings struct {
	BaseTime       time.Time
	SystemBaseTime time.Time
	DateTimeType   string
	TimeZone       string
}
