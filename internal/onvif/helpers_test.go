package onvif

import "github.com/aragarwal/onvif-server/internal/config"

// newTestServer builds a Server directly without invoking NewServer, which
// would otherwise spawn ffprobe goroutines.
func newTestServer(cfg config.CameraConfig, username, password string) *Server {
	return &Server{
		config:   cfg,
		rtspHost: "10.0.0.1",
		rtspPort: 554,
		username: username,
		password: password,
		streamInfo: &StreamInfo{
			Width: 1920, Height: 1080, Codec: "H264",
			FrameRate: 25, BitRate: 4096, Profile: "High",
		},
		substreamInfo: &StreamInfo{
			Width: 640, Height: 480, Codec: "H264",
			FrameRate: 15, BitRate: 512, Profile: "Baseline",
		},
		bitrateCache: make(map[string]int),
	}
}
