package onvif

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aragarwal/onvif-server/internal/logger"
)

// StreamInfo holds detected stream properties for a main or sub stream.
type StreamInfo struct {
	Width     int
	Height    int
	Codec     string
	FrameRate int
	BitRate   int // kbps
	Profile   string
}

// detectStreamInfo uses ffprobe to detect stream properties.
func (s *Server) detectStreamInfo(isSubstream bool) {
	streamPath := s.config.RTSPStream
	streamType := "main"
	var targetInfo *StreamInfo

	if isSubstream {
		streamPath = s.config.SubstreamPath
		if streamPath == "" {
			streamPath = s.config.RTSPStream + "_sub"
		}
		streamType = "sub"
	}

	s.streamInfoMu.RLock()
	if isSubstream {
		targetInfo = s.substreamInfo
	} else {
		targetInfo = s.streamInfo
	}
	s.streamInfoMu.RUnlock()

	rtspURL := fmt.Sprintf("rtsp://localhost:%d%s", s.rtspPort, streamPath)

	logger.Debug("[%s] 🔍 Detecting %s stream properties for '%s'...", s.config.Name, streamType, streamPath)

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
			logger.Debug("[%s] ⚠️  Stream detection timeout for %s stream '%s' (using defaults: %dx%d %s %dkbps)",
				s.config.Name, streamType, streamPath, targetInfo.Width, targetInfo.Height, targetInfo.Codec, targetInfo.BitRate)
		} else {
			logger.Debug("[%s] ⚠️  Failed to detect %s stream '%s': %v (using defaults: %dx%d %s %dkbps)",
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
		logger.Debug("[%s] ⚠️  Failed to parse ffprobe output for %s stream: %v", s.config.Name, streamType, err)
		return
	}

	if len(result.Streams) == 0 {
		logger.Debug("[%s] ⚠️  No video stream found for %s stream", s.config.Name, streamType)
		return
	}

	stream := result.Streams[0]

	s.streamInfoMu.Lock()
	defer s.streamInfoMu.Unlock()

	if isSubstream {
		targetInfo = s.substreamInfo
	} else {
		targetInfo = s.streamInfo
	}

	if stream.Width > 0 {
		targetInfo.Width = stream.Width
	}
	if stream.Height > 0 {
		targetInfo.Height = stream.Height
	}
	if stream.CodecName != "" {
		codec := strings.ToUpper(stream.CodecName)
		if codec == "HEVC" {
			codec = "H265"
		}
		targetInfo.Codec = codec
	}
	if stream.Profile != "" {
		targetInfo.Profile = stream.Profile
	}

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

	if stream.BitRate != "" {
		bitRate, _ := strconv.Atoi(stream.BitRate)
		if bitRate > 0 {
			targetInfo.BitRate = bitRate / 1024
		}
	}

	logger.Info("[%s] ✅ Detected %s stream '%s': %dx%d %s %dfps %dkbps Profile:%s",
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

// StartDetectionRoutine runs a background loop that refreshes stream
// detection every 10 minutes for all servers. It blocks; run in a goroutine.
func StartDetectionRoutine(servers []*Server) {
	logger.Debug("🔄 Global stream detection routine started (runs immediately then every 10 minutes for all cameras)")

	for _, server := range servers {
		go server.detectStreamInfo(false)
		if server.config.SubstreamEnabled {
			go server.detectStreamInfo(true)
		}
	}

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("🔄 Refreshing stream detection for all cameras...")
		for _, server := range servers {
			go server.detectStreamInfo(false)
			if server.config.SubstreamEnabled {
				go server.detectStreamInfo(true)
			}
		}
	}
}

// getStreamInfoForToken returns the appropriate StreamInfo based on token.
// Returns (streamInfo, isSubstream).
func (s *Server) getStreamInfoForToken(token string) (*StreamInfo, bool) {
	isSubstream := false
	if token == "V_ENC_CFG_001" || token == "V_ENC_CFG_002" {
		isSubstream = true
	} else {
		s.bitrateMu.RLock()
		bitrate, bitrateSet := s.bitrateCache[token]
		s.bitrateMu.RUnlock()
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

// getRTSPURLForToken returns the RTSP URL for a given encoder token.
func (s *Server) getRTSPURLForToken(token string, hostIP string) string {
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
