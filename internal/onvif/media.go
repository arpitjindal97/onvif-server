package onvif

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/aragarwal/onvif-server/internal/logger"
)

// handleGetProfiles returns three hard-coded profiles (main + two substreams).
func (s *Server) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := []profile{
		buildProfile("Profile000", "V_ENC_CFG_000", "A_ENC_CFG_000", "V_ANALYTICS_CFG_000", "PTZ_CFG_000", "META_CFG_000",
			1920, 1080, 4096),
		buildProfile("Profile001", "V_ENC_CFG_001", "A_ENC_CFG_001", "V_ANALYTICS_CFG_001", "PTZ_CFG_001", "META_CFG_001",
			704, 576, 1024),
		buildProfile("Profile002", "V_ENC_CFG_002", "A_ENC_CFG_002", "V_ANALYTICS_CFG_002", "PTZ_CFG_002", "META_CFG_002",
			352, 288, 512),
	}

	response := getProfilesResponse{Profiles: profiles}
	s.sendSOAPResponse(w, response)
}

// buildProfile constructs a stock profile with the given encoder parameters.
func buildProfile(token, vEncToken, aEncToken, analyticsToken, ptzToken, metaToken string,
	width, height, bitrate int) profile {
	return profile{
		Token: token,
		Fixed: true,
		Name:  token,
		VideoSourceConfiguration: &videoSourceConfiguration{
			Token:       "V_SRC_CFG_000",
			Name:        "V_SRC_CFG_000",
			UseCount:    3,
			SourceToken: "V_SRC_000",
			Bounds:      bounds{X: 0, Y: 0, Width: 1920, Height: 1080},
		},
		AudioSourceConfiguration: &audioSourceConfiguration{
			Token:       "A_SRC_CFG_000",
			Name:        "A_SRC_CFG_000",
			UseCount:    3,
			SourceToken: "A_SRC_000",
		},
		VideoEncoderConfiguration: &videoEncoderConfiguration{
			Token:       vEncToken,
			Name:        vEncToken,
			UseCount:    1,
			Encoding:    "H264",
			Resolution:  resolution{Width: width, Height: height},
			Quality:     5.0,
			RateControl: &rateControl{FrameRateLimit: 25, EncodingInterval: 1, BitrateLimit: bitrate},
			H264:        &h264Config{GovLength: 50, H264Profile: "High"},
			Multicast: multicast{
				Address:   multicastAddress{Type: "IPv4", IPv4Address: "0.0.0.0"},
				Port:      0,
				TTL:       5,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		},
		AudioEncoderConfiguration: &audioEncoderConfiguration{
			Token:      aEncToken,
			Name:       aEncToken,
			UseCount:   1,
			Encoding:   "AAC",
			Bitrate:    32,
			SampleRate: 8,
			Multicast: multicast{
				Address:   multicastAddress{Type: "IPv4", IPv4Address: "0.0.0.0"},
				Port:      0,
				TTL:       5,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		},
		VideoAnalyticsConfiguration: &videoAnalyticsConfiguration{
			Token:    analyticsToken,
			Name:     analyticsToken,
			UseCount: 1,
		},
		PTZConfiguration: &ptzConfiguration{
			Token:     ptzToken,
			Name:      ptzToken,
			UseCount:  1,
			NodeToken: "PTZ_NODE_000",
		},
		MetadataConfiguration: &metadataConfiguration{
			Token:          metaToken,
			Name:           metaToken,
			UseCount:       1,
			SessionTimeout: "PT60S",
		},
	}
}

func (s *Server) handleGetStreamUri(w http.ResponseWriter, r *http.Request, bodyContent string) {
	hostIP := s.getHostIP(r)

	logger.Debug("[%s] 🎬 GetStreamUri REQUEST BODY:\n%s", s.config.Name, bodyContent)

	subtype := 0
	profileToken := "Profile000"

	if strings.Contains(bodyContent, "<StreamType>RTP-Unicast-Substream</StreamType>") ||
		strings.Contains(bodyContent, "<StreamType>RTP-Unicast-Sub</StreamType>") ||
		strings.Contains(bodyContent, "<tt:StreamType>RTP-Unicast-Substream</tt:StreamType>") ||
		strings.Contains(bodyContent, "<tt:StreamType>RTP-Unicast-Sub</tt:StreamType>") {
		subtype = 1
		logger.Debug("[%s] 🎬 GetStreamUri: StreamType indicates SUBSTREAM", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile001") {
		subtype = 1
		profileToken = "Profile001"
		logger.Debug("[%s] 🎬 GetStreamUri: Profile001 detected -> SUBSTREAM", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile002") {
		subtype = 2
		profileToken = "Profile002"
		logger.Debug("[%s] 🎬 GetStreamUri: Profile002 detected -> SUBSTREAM (low quality)", s.config.Name)
	} else if strings.Contains(bodyContent, "Profile000") {
		profileToken = "Profile000"
		logger.Debug("[%s] 🎬 GetStreamUri: Profile000 detected -> MAIN stream", s.config.Name)
	}

	streamPath := s.config.RTSPStream
	streamName := "MAIN"
	if subtype == 1 && s.config.SubstreamEnabled {
		streamPath = s.config.SubstreamPath
		streamName = "SUBSTREAM"
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub"
		}
	} else if subtype == 2 && s.config.SubstreamEnabled {
		streamPath = s.config.SubstreamPath
		streamName = "SUBSTREAM"
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub"
		}
	}

	rtspURL := fmt.Sprintf("rtsp://%s:%d%s", hostIP, s.rtspPort, streamPath)

	logger.Info("[%s] 🎬 GetStreamUri: Profile=%s, Subtype=%d, Stream=%s, Path='%s' -> %s",
		s.config.Name, profileToken, subtype, streamName, streamPath, rtspURL)

	response := getStreamUriResponse{
		MediaUri: mediaUri{
			Uri:                 rtspURL,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetSnapshotUri(w http.ResponseWriter, r *http.Request) {
	hostIP := s.getHostIP(r)
	snapshotURL := fmt.Sprintf("http://%s:%d/snapshot", hostIP, s.config.HTTPPort)

	response := getSnapshotUriResponse{
		MediaUri: mediaUri{
			Uri:                 snapshotURL,
			InvalidAfterConnect: false,
			InvalidAfterReboot:  false,
			Timeout:             "PT0S",
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetVideoSources(w http.ResponseWriter, r *http.Request) {
	response := getVideoSourcesResponse{
		VideoSources: []videoSource{
			{
				Token:      "VideoSource_1",
				Framerate:  25.0,
				Resolution: resolution{Width: 1920, Height: 1080},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetAudioSources(w http.ResponseWriter, r *http.Request) {
	response := getAudioSourcesResponse{
		AudioSources: []audioSource{
			{Token: "AudioSource_1", Channels: 1},
		},
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetVideoEncoderConfigurations(w http.ResponseWriter, r *http.Request) {
	configs := []videoEncoderConfiguration{
		{
			Token:      "VideoEncoderToken",
			Name:       "VideoEncoderConfig",
			UseCount:   1,
			Encoding:   "H264",
			Resolution: resolution{Width: 1920, Height: 1080},
			Quality:    5.0,
			RateControl: &rateControl{
				FrameRateLimit:   25,
				EncodingInterval: 1,
				BitrateLimit:     4096,
			},
			H264: &h264Config{GovLength: 50, H264Profile: "High"},
			Multicast: multicast{
				Address:   multicastAddress{Type: "IPv4", IPv4Address: "0.0.0.0"},
				Port:      0,
				TTL:       1,
				AutoStart: false,
			},
			SessionTimeout: "PT60S",
		},
	}

	response := getVideoEncoderConfigurationsResponse{Configurations: configs}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleGetVideoEncoderConfiguration(w http.ResponseWriter, r *http.Request, bodyContent string) {
	token := "V_ENC_CFG_000"
	tokenRegex := regexp.MustCompile(`<.*?:?ConfigurationToken>([^<]+)</`)
	if match := tokenRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		token = match[1]
	}

	streamInfo, isSubstream := s.getStreamInfoForToken(token)

	streamName := s.config.RTSPStream
	if isSubstream {
		if s.config.SubstreamEnabled && s.config.SubstreamPath != "" {
			streamName = s.config.SubstreamPath
		} else {
			streamName = s.config.RTSPStream + "_sub"
		}
	}

	logger.Debug("[%s] 📹 GetVideoEncoderConfiguration for stream '%s' (token %s): %dx%d %s %dfps %dkbps Profile:%s",
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

	cfg := videoEncoderConfiguration{
		Token:      token,
		Name:       token,
		UseCount:   1,
		Encoding:   streamInfo.Codec,
		Resolution: resolution{Width: streamInfo.Width, Height: streamInfo.Height},
		Quality:    5.0,
		RateControl: &rateControl{
			FrameRateLimit:   streamInfo.FrameRate,
			EncodingInterval: 1,
			BitrateLimit:     streamInfo.BitRate,
		},
		H264: &h264Config{GovLength: 50, H264Profile: streamInfo.Profile},
		Multicast: multicast{
			Address:   multicastAddress{Type: "IPv4", IPv4Address: "0.0.0.0"},
			Port:      0,
			TTL:       1,
			AutoStart: false,
		},
		SessionTimeout: "PT60S",
	}

	response := getVideoEncoderConfigurationResponse{Configuration: cfg}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleSetVideoEncoderConfiguration(w http.ResponseWriter, r *http.Request, bodyContent string) {
	logger.Debug("[%s] 🔧 SetVideoEncoderConfiguration REQUEST:\n%s", s.config.Name, bodyContent)

	tokenRegex := regexp.MustCompile(`<.*?:?Configuration.*?token="([^"]+)"`)
	token := ""
	if match := tokenRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		token = match[1]
		logger.Debug("[%s] 🔧 SetVideoEncoderConfiguration: Token=%s", s.config.Name, token)
	}

	bitrateRegex := regexp.MustCompile(`<.*?:?BitrateLimit>(\d+)</`)
	if match := bitrateRegex.FindStringSubmatch(bodyContent); len(match) > 1 {
		bitrateStr := match[1]
		if bitrate, err := strconv.Atoi(bitrateStr); err == nil {
			s.bitrateMu.Lock()
			s.bitrateCache[token] = bitrate
			s.bitrateMu.Unlock()
			logger.Debug("[%s] 🔧 SetVideoEncoderConfiguration: BitrateLimit stored -> %d kbps for token %s", s.config.Name, bitrate, token)
		} else {
			logger.Debug("[%s] ⚠️ SetVideoEncoderConfiguration: Failed to parse BitrateLimit: %s", s.config.Name, bitrateStr)
		}
	}

	responseXML := `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope"
                   xmlns:tr2="http://www.onvif.org/ver20/media/wsdl">
  <SOAP-ENV:Body>
    <tr2:SetVideoEncoderConfigurationResponse></tr2:SetVideoEncoderConfigurationResponse>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

	logger.Debug("[%s] ✅ SetVideoEncoderConfiguration: Acknowledging configuration change", s.config.Name)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseXML))
}

func (s *Server) handleGetVideoEncoderConfigurationOptions(w http.ResponseWriter, r *http.Request) {
	jpegResolutions := []resolution{
		{Width: 1920, Height: 1080},
		{Width: 1280, Height: 960},
		{Width: 1280, Height: 720},
		{Width: 704, Height: 576},
		{Width: 640, Height: 480},
		{Width: 352, Height: 288},
		{Width: 320, Height: 240},
		{Width: 176, Height: 144},
	}

	h264Resolutions := []resolution{
		{Width: 1920, Height: 1080},
		{Width: 1280, Height: 960},
		{Width: 1280, Height: 720},
		{Width: 704, Height: 576},
		{Width: 640, Height: 480},
		{Width: 352, Height: 288},
	}

	response := getVideoEncoderConfigurationOptionsResponse{
		Options: videoEncoderConfigurationOptions{
			QualityRange: qualityRange{Min: 1, Max: 6},
			JPEG: &jpegOptions{
				ResolutionsAvailable:  jpegResolutions,
				FrameRateRange:        intRange{Min: 1, Max: 25},
				EncodingIntervalRange: intRange{Min: 1, Max: 1},
			},
			H264: &h264Options{
				ResolutionsAvailable:  h264Resolutions,
				GovLengthRange:        intRange{Min: 1, Max: 400},
				FrameRateRange:        intRange{Min: 1, Max: 25},
				EncodingIntervalRange: intRange{Min: 1, Max: 1},
				H264ProfilesSupported: []string{"Baseline", "Main", "High"},
			},
			Extension: &encoderExtension{
				JPEG: &jpegOptions2{
					ResolutionsAvailable:  jpegResolutions,
					FrameRateRange:        intRange{Min: 1, Max: 25},
					EncodingIntervalRange: intRange{Min: 1, Max: 1},
					BitrateRange:          intRange{Min: 2, Max: 20480},
				},
				H264: &h264Options2{
					ResolutionsAvailable:  h264Resolutions,
					GovLengthRange:        intRange{Min: 1, Max: 400},
					FrameRateRange:        intRange{Min: 1, Max: 25},
					EncodingIntervalRange: intRange{Min: 1, Max: 1},
					H264ProfilesSupported: []string{"Baseline", "Main", "High"},
					BitrateRange:          intRange{Min: 2, Max: 20480},
				},
			},
		},
	}
	s.sendSOAPResponse(w, response)
}
