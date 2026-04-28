package onvif

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aragarwal/onvif-server/internal/logger"
)

// handleGetSystemDateAndTime returns the current device time, optionally
// adjusted by a previous SetSystemDateAndTime call.
func (s *Server) handleGetSystemDateAndTime(w http.ResponseWriter, r *http.Request) {
	s.timeSettingsMu.RLock()
	settings := s.timeSettings
	s.timeSettingsMu.RUnlock()

	var now time.Time
	dateTimeType := "NTP"
	tz := "EAST-4"

	if settings != nil && !settings.BaseTime.IsZero() {
		elapsed := time.Now().UTC().Sub(settings.SystemBaseTime)
		now = settings.BaseTime.Add(elapsed)
		dateTimeType = settings.DateTimeType
		if settings.TimeZone != "" {
			tz = settings.TimeZone
		}
		logger.Debug("[%s] GetSystemDateAndTime: Returning synced time: %v (Type: %s, TZ: %s)",
			s.config.Name, now.Format(time.RFC3339), dateTimeType, tz)
	} else {
		now = time.Now().UTC()
		logger.Debug("[%s] GetSystemDateAndTime: Returning system time: %v (no sync yet)",
			s.config.Name, now.Format(time.RFC3339))
	}

	localTime := now

	response := getSystemDateAndTimeResponse{
		SystemDateAndTime: systemDateAndTime{
			DateTimeType:    dateTimeType,
			DaylightSavings: false,
			TimeZone:        timeZone{TZ: tz},
			UTCDateTime: dateTime{
				Time: timeOfDay{Hour: now.Hour(), Minute: now.Minute(), Second: now.Second()},
				Date: date{Year: now.Year(), Month: int(now.Month()), Day: now.Day()},
			},
			LocalDateTime: dateTime{
				Time: timeOfDay{Hour: localTime.Hour(), Minute: localTime.Minute(), Second: localTime.Second()},
				Date: date{Year: localTime.Year(), Month: int(localTime.Month()), Day: localTime.Day()},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetDeviceInformation(w http.ResponseWriter, r *http.Request) {
	response := getDeviceInformationResponse{
		Manufacturer:    s.config.Manufacturer,
		Model:           s.config.Model,
		FirmwareVersion: "5.7.10",
		SerialNumber:    s.config.Serial,
		HardwareId:      "1.0",
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)

	response := getCapabilitiesResponse{
		Capabilities: capabilities{
			Analytics: &analyticsCapabilities{
				XAddr:                  baseURL + "/onvif/analytics_service",
				RuleSupport:            true,
				AnalyticsModuleSupport: true,
			},
			Device: &deviceCapabilities{
				XAddr: baseURL + "/onvif/device_service",
				Network: &networkCapabilities{
					IPFilter:          true,
					ZeroConfiguration: true,
					IPVersion6:        false,
					DynDNS:            true,
					NTP:               1,
					DHCPv6:            false,
				},
				System: &systemCapabilities{
					DiscoveryResolve:       true,
					DiscoveryBye:           true,
					RemoteDiscovery:        true,
					SystemBackup:           true,
					SystemLogging:          true,
					FirmwareUpgrade:        true,
					HttpFirmwareUpgrade:    true,
					HttpSystemBackup:       true,
					HttpSystemLogging:      true,
					HttpSupportInformation: true,
				},
				IO: &ioCapabilities{InputConnectors: 1, RelayOutputs: 1},
				Security: &securityCapabilities{
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
			Events: &eventCapabilities{
				XAddr:                       baseURL + "/onvif/event_service",
				WSSubscriptionPolicySupport: true,
				WSPullPointSupport:          true,
				WSPausableSubscriptionManagerInterfaceSupport: false,
			},
			Imaging: &imagingCapabilities{XAddr: baseURL + "/onvif/imaging_service"},
			Media: &mediaCapabilities{
				XAddr: baseURL + "/onvif/media_service",
				StreamingCapabilities: &streamingCapabilities{
					RTPMulticast: false,
					RTP_TCP:      true,
					RTP_RTSP_TCP: true,
				},
			},
			Media2: &media2Capabilities{XAddr: baseURL + "/onvif/media2_service"},
			PTZ:    &ptzCapabilities{XAddr: baseURL + "/onvif/ptz_service"},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)

	response := getServicesResponse{
		Service: []service{
			{Namespace: "http://www.onvif.org/ver10/device/wsdl", XAddr: baseURL + "/onvif/device_service", Version: version{Major: 16, Minor: 12}},
			{Namespace: "http://www.onvif.org/ver10/media/wsdl", XAddr: baseURL + "/onvif/media_service", Version: version{Major: 2, Minor: 6}},
			{Namespace: "http://www.onvif.org/ver20/media/wsdl", XAddr: baseURL + "/onvif/media2_service", Version: version{Major: 2, Minor: 6}},
			{Namespace: "http://www.onvif.org/ver10/events/wsdl", XAddr: baseURL + "/onvif/event_service", Version: version{Major: 2, Minor: 6}},
			{Namespace: "http://www.onvif.org/ver20/imaging/wsdl", XAddr: baseURL + "/onvif/imaging_service", Version: version{Major: 2, Minor: 6}},
			{Namespace: "http://www.onvif.org/ver20/ptz/wsdl", XAddr: baseURL + "/onvif/ptz_service", Version: version{Major: 2, Minor: 6}},
			{Namespace: "http://www.onvif.org/ver20/analytics/wsdl", XAddr: baseURL + "/onvif/analytics_service", Version: version{Major: 2, Minor: 6}},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetScopes(w http.ResponseWriter, r *http.Request) {
	response := getScopesResponse{
		Scopes: []scope{
			{ScopeDef: "Fixed", ScopeItem: "onvif://www.onvif.org/type/video_encoder"},
			{ScopeDef: "Fixed", ScopeItem: "onvif://www.onvif.org/type/audio_encoder"},
			{ScopeDef: "Fixed", ScopeItem: "onvif://www.onvif.org/hardware/" + s.config.Model},
			{ScopeDef: "Fixed", ScopeItem: "onvif://www.onvif.org/name/" + s.config.Name},
			{ScopeDef: "Fixed", ScopeItem: "onvif://www.onvif.org/location/"},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetHostname(w http.ResponseWriter, r *http.Request) {
	response := getHostnameResponse{
		HostnameInfo: hostnameInfo{FromDHCP: false, Name: s.config.Name},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetDNS(w http.ResponseWriter, r *http.Request) {
	response := getDNSResponse{
		DNSInfo: dnsInfo{
			FromDHCP: true,
			DNSFromDHCP: []dnsName{
				{Type: "IPv4", IPv4Address: "8.8.8.8"},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	hostIP := s.getHostIP(r)

	response := getNetworkInterfacesResponse{
		NetworkInterfaces: []networkInterface{
			{
				Token:   "eth0",
				Enabled: true,
				Info:    networkInfo{Name: "eth0", HwAddress: "00:11:22:33:44:55", MTU: 1500},
				Link: &networkLink{
					AdminSettings: linkSettings{AutoNegotiation: true, Speed: 1000, Duplex: "Full"},
					OperSettings:  linkSettings{AutoNegotiation: true, Speed: 1000, Duplex: "Full"},
					InterfaceType: 6,
				},
				IPv4: &ipv4Config{
					Enabled: true,
					Config: ipv4Network{
						DHCP:     true,
						FromDHCP: &prefixedIPv4Address{Address: hostIP, PrefixLength: 24},
					},
				},
				IPv6: &ipv6Config{Enabled: false},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetNetworkProtocols(w http.ResponseWriter, r *http.Request) {
	response := getNetworkProtocolsResponse{
		NetworkProtocols: []networkProtocol{
			{Name: "HTTP", Enabled: true, Port: []int{s.config.HTTPPort}},
			{Name: "HTTPS", Enabled: false},
			{Name: "RTSP", Enabled: true, Port: []int{s.rtspPort}},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleSystemReboot(w http.ResponseWriter, r *http.Request) {
	response := struct {
		XMLName xml.Name `xml:"tds:SystemRebootResponse"`
		Message string   `xml:"tds:Message"`
	}{Message: "Device is rebooting"}
	s.sendSOAPResponse(w, response)
}

// handleSetSystemDateAndTime parses an NVR time-set request and stores the
// offset so subsequent GetSystemDateAndTime calls return synced time.
func (s *Server) handleSetSystemDateAndTime(w http.ResponseWriter, r *http.Request, requestBody []byte) {
	bodyStr := string(requestBody)
	logger.Debug("[%s] SetSystemDateAndTime request:\n%s", s.config.Name, bodyStr)

	dateTimeType := "Manual"
	tz := "UTC"
	var year, month, day, hour, minute, second int

	if strings.Contains(bodyStr, "<DateTimeType>") {
		start := strings.Index(bodyStr, "<DateTimeType>") + len("<DateTimeType>")
		end := strings.Index(bodyStr[start:], "</DateTimeType>")
		if end > 0 {
			dateTimeType = strings.TrimSpace(bodyStr[start : start+end])
		}
	}

	if strings.Contains(bodyStr, "<TZ>") {
		start := strings.Index(bodyStr, "<TZ>") + len("<TZ>")
		end := strings.Index(bodyStr[start:], "</TZ>")
		if end > 0 {
			tz = strings.TrimSpace(bodyStr[start : start+end])
		}
	}

	hasDateTime := strings.Contains(bodyStr, "<UTCDateTime>")
	if hasDateTime {
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

			settings := &clientTimeSettings{
				BaseTime:       requestedTime,
				SystemBaseTime: systemBaseTime,
				DateTimeType:   dateTimeType,
				TimeZone:       tz,
			}

			s.timeSettingsMu.Lock()
			s.timeSettings = settings
			s.timeSettingsMu.Unlock()

			logger.Info("[%s] ✓ Time synchronized by NVR:", s.config.Name)
			logger.Debug("  - DateTimeType: %s", dateTimeType)
			logger.Debug("  - TimeZone: %s", tz)
			logger.Debug("  - NVR time: %v", requestedTime.Format(time.RFC3339))
			logger.Debug("  - System time: %v", systemBaseTime.Format(time.RFC3339))
			logger.Debug("  - Time offset: %v", requestedTime.Sub(systemBaseTime))
		} else {
			logger.Debug("[%s] SetSystemDateAndTime: Invalid date/time values (Y:%d M:%d D:%d)",
				s.config.Name, year, month, day)
		}
	} else {
		logger.Debug("[%s] SetSystemDateAndTime: No UTCDateTime provided", s.config.Name)
	}

	response := struct {
		XMLName xml.Name `xml:"tds:SetSystemDateAndTimeResponse"`
	}{}
	s.sendSOAPResponse(w, response)
}
