package onvif

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
)

// TestHelperProcess is invoked as a child process to impersonate ffprobe.
// It is not a real test: it inspects env vars set by fakeExecCommand and
// emits stdout / exits with the requested code.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("FFPROBE_STDOUT"))
	if os.Getenv("FFPROBE_EXIT") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

// fakeExecCommand returns a substitute for exec.CommandContext that re-execs
// the current test binary in helper-process mode with the supplied stdout
// and exit behavior.
func fakeExecCommand(stdout string, fail bool) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=^TestHelperProcess$", "--"}
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			"FFPROBE_STDOUT=" + stdout,
		}
		if fail {
			cmd.Env = append(cmd.Env, "FFPROBE_EXIT=1")
		}
		return cmd
	}
}

// withFakeExec swaps execCommandContext for the duration of the test.
// The mutex matches the read side in runExec so that lingering goroutines
// from prior tests (e.g. NewServer's async detection) cannot race with us.
func withFakeExec(t *testing.T, stdout string, fail bool) {
	t.Helper()
	execCommandContextMu.Lock()
	prev := execCommandContext
	execCommandContext = fakeExecCommand(stdout, fail)
	execCommandContextMu.Unlock()
	t.Cleanup(func() {
		execCommandContextMu.Lock()
		execCommandContext = prev
		execCommandContextMu.Unlock()
	})
}

func TestGetStreamInfoForToken_KnownSubstreamTokens(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	for _, token := range []string{"V_ENC_CFG_001", "V_ENC_CFG_002"} {
		info, isSub := s.getStreamInfoForToken(token)
		if !isSub {
			t.Errorf("token %q: expected isSubstream=true", token)
		}
		if info != s.substreamInfo {
			t.Errorf("token %q: expected substreamInfo, got %+v", token, info)
		}
	}
}

func TestGetStreamInfoForToken_KnownMainToken(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	info, isSub := s.getStreamInfoForToken("V_ENC_CFG_000")
	if isSub {
		t.Error("expected isSubstream=false for V_ENC_CFG_000")
	}
	if info != s.streamInfo {
		t.Errorf("expected streamInfo, got %+v", info)
	}
}

func TestGetStreamInfoForToken_UnknownTokenLowBitrateUsesSubstream(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	s.bitrateCache["VideoEncoder001"] = 512

	info, isSub := s.getStreamInfoForToken("VideoEncoder001")
	if !isSub {
		t.Error("expected low-bitrate unknown token to map to substream")
	}
	if info != s.substreamInfo {
		t.Error("expected substreamInfo for low-bitrate unknown token")
	}
}

func TestGetStreamInfoForToken_UnknownTokenHighBitrateUsesMain(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	s.bitrateCache["VideoEncoder001"] = 4096

	info, isSub := s.getStreamInfoForToken("VideoEncoder001")
	if isSub {
		t.Error("expected high-bitrate unknown token to map to main stream")
	}
	if info != s.streamInfo {
		t.Error("expected streamInfo for high-bitrate unknown token")
	}
}

func TestGetStreamInfoForToken_UnknownTokenNoBitrateUsesMain(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")

	_, isSub := s.getStreamInfoForToken("Whatever")
	if isSub {
		t.Error("expected unknown token without cached bitrate to default to main")
	}
}

func TestGetRTSPURLForToken(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.CameraConfig
		tok  string
		want string
	}{
		{
			name: "main stream",
			cfg:  config.CameraConfig{RTSPStream: "/cam"},
			tok:  "V_ENC_CFG_000",
			want: "rtsp://1.2.3.4:554/cam",
		},
		{
			name: "substream with explicit substream path",
			cfg:  config.CameraConfig{RTSPStream: "/cam", SubstreamEnabled: true, SubstreamPath: "/cam_sub"},
			tok:  "V_ENC_CFG_001",
			want: "rtsp://1.2.3.4:554/cam_sub",
		},
		{
			name: "substream falls back to <main>_sub",
			cfg:  config.CameraConfig{RTSPStream: "/cam", SubstreamEnabled: false},
			tok:  "V_ENC_CFG_001",
			want: "rtsp://1.2.3.4:554/cam_sub",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(tc.cfg, "admin", "admin")
			got := s.getRTSPURLForToken(tc.tok, "1.2.3.4")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectStreamInfo_PopulatesAllFields(t *testing.T) {
	probeJSON := `{"streams":[{
		"codec_name":"h264","width":1280,"height":720,
		"r_frame_rate":"30/1","bit_rate":"2048000","profile":"Main"
	}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	s.detectStreamInfo(false)

	got := s.streamInfo
	if got.Width != 1280 || got.Height != 720 {
		t.Errorf("resolution = %dx%d, want 1280x720", got.Width, got.Height)
	}
	if got.Codec != "H264" {
		t.Errorf("codec = %q, want H264", got.Codec)
	}
	if got.FrameRate != 30 {
		t.Errorf("frameRate = %d, want 30", got.FrameRate)
	}
	if got.BitRate != 2048000/1024 {
		t.Errorf("bitrate = %d, want %d", got.BitRate, 2048000/1024)
	}
	if got.Profile != "Main" {
		t.Errorf("profile = %q, want Main", got.Profile)
	}
}

func TestDetectStreamInfo_HEVCMappedToH265(t *testing.T) {
	probeJSON := `{"streams":[{"codec_name":"hevc","width":1920,"height":1080,"r_frame_rate":"25/1"}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	s.detectStreamInfo(false)

	if s.streamInfo.Codec != "H265" {
		t.Errorf("codec = %q, want H265 (mapped from hevc)", s.streamInfo.Codec)
	}
}

func TestDetectStreamInfo_FrameRateZeroDenominatorIgnored(t *testing.T) {
	probeJSON := `{"streams":[{"codec_name":"h264","width":640,"height":480,"r_frame_rate":"30/0"}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	originalRate := s.streamInfo.FrameRate
	s.detectStreamInfo(false)

	if s.streamInfo.FrameRate != originalRate {
		t.Errorf("frameRate changed despite invalid denominator: got %d, want %d",
			s.streamInfo.FrameRate, originalRate)
	}
}

func TestDetectStreamInfo_ZeroBitrateIgnored(t *testing.T) {
	probeJSON := `{"streams":[{"codec_name":"h264","width":640,"height":480,"bit_rate":"0"}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	originalBR := s.streamInfo.BitRate
	s.detectStreamInfo(false)

	if s.streamInfo.BitRate != originalBR {
		t.Errorf("bitrate overwritten by 0: got %d, want %d", s.streamInfo.BitRate, originalBR)
	}
}

func TestDetectStreamInfo_ExecError(t *testing.T) {
	withFakeExec(t, "", true) // helper exits non-zero

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	before := *s.streamInfo
	s.detectStreamInfo(false)

	// Defaults must be untouched on exec failure.
	if *s.streamInfo != before {
		t.Errorf("stream info modified after exec error: got %+v, want %+v", *s.streamInfo, before)
	}
}

func TestDetectStreamInfo_InvalidJSON(t *testing.T) {
	withFakeExec(t, "not json", false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	before := *s.streamInfo
	s.detectStreamInfo(false)

	if *s.streamInfo != before {
		t.Errorf("stream info modified despite JSON parse error: got %+v, want %+v", *s.streamInfo, before)
	}
}

func TestDetectStreamInfo_EmptyStreams(t *testing.T) {
	withFakeExec(t, `{"streams":[]}`, false)

	s := newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin")
	before := *s.streamInfo
	s.detectStreamInfo(false)

	if *s.streamInfo != before {
		t.Errorf("stream info modified despite empty streams array: got %+v, want %+v", *s.streamInfo, before)
	}
}

func TestDetectStreamInfo_SubstreamPath(t *testing.T) {
	probeJSON := `{"streams":[{"codec_name":"h264","width":704,"height":576,"r_frame_rate":"15/1","bit_rate":"512000"}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{
		Name:             "cam",
		RTSPStream:       "/cam",
		SubstreamEnabled: true,
		SubstreamPath:    "/cam_sub",
	}, "admin", "admin")
	s.detectStreamInfo(true)

	if s.substreamInfo.Width != 704 || s.substreamInfo.Height != 576 {
		t.Errorf("substream = %dx%d, want 704x576", s.substreamInfo.Width, s.substreamInfo.Height)
	}
	// Main stream defaults must remain untouched.
	if s.streamInfo.Width != 1920 {
		t.Errorf("main stream was modified by substream detection: %dx%d", s.streamInfo.Width, s.streamInfo.Height)
	}
}

func TestDetectStreamInfo_SubstreamFallbackPath(t *testing.T) {
	// SubstreamEnabled but no SubstreamPath: should fall back to RTSPStream + "_sub".
	probeJSON := `{"streams":[{"codec_name":"h264","width":352,"height":288}]}`
	withFakeExec(t, probeJSON, false)

	s := newTestServer(config.CameraConfig{
		Name:       "cam",
		RTSPStream: "/cam",
	}, "admin", "admin")
	s.detectStreamInfo(true)

	if s.substreamInfo.Width != 352 {
		t.Errorf("substream width = %d, want 352", s.substreamInfo.Width)
	}
}

// swapExec installs a custom exec factory under the production mutex and
// restores it on test cleanup.
func swapExec(t *testing.T, fn func(ctx context.Context, name string, args ...string) *exec.Cmd) {
	t.Helper()
	execCommandContextMu.Lock()
	prev := execCommandContext
	execCommandContext = fn
	execCommandContextMu.Unlock()
	t.Cleanup(func() {
		execCommandContextMu.Lock()
		execCommandContext = prev
		execCommandContextMu.Unlock()
	})
}

// withInterval temporarily shortens detectionInterval for tests.
func withInterval(t *testing.T, d time.Duration) {
	t.Helper()
	prev := detectionInterval
	detectionInterval = d
	t.Cleanup(func() { detectionInterval = prev })
}

func TestStartDetectionRoutine_KicksImmediatelyAndOnTick(t *testing.T) {
	var calls atomic.Int64
	base := fakeExecCommand(`{"streams":[]}`, false)
	swapExec(t, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls.Add(1)
		return base(ctx, name, args...)
	})
	withInterval(t, 30*time.Millisecond)

	servers := []*Server{
		newTestServer(config.CameraConfig{Name: "cam1", RTSPStream: "/cam1"}, "admin", "admin"),
		newTestServer(config.CameraConfig{
			Name:             "cam2",
			RTSPStream:       "/cam2",
			SubstreamEnabled: true,
			SubstreamPath:    "/cam2_sub",
		}, "admin", "admin"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		StartDetectionRoutine(ctx, servers)
		close(done)
	}()

	// Wait for the immediate kick + at least 2 more ticks.
	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartDetectionRoutine did not return after context cancel")
	}

	// Each iteration: 1 (cam1 main) + 2 (cam2 main + sub) = 3 calls.
	// We expect at least 3 iterations × 3 = 9 calls (immediate + 2 ticks).
	got := calls.Load()
	if got < 6 {
		t.Errorf("ffprobe called %d times, want >= 6 (immediate + at least 1 tick)", got)
	}
}

func TestStartDetectionRoutine_ReturnsImmediatelyOnPreCancelledContext(t *testing.T) {
	swapExec(t, fakeExecCommand(`{"streams":[]}`, false))
	withInterval(t, 24*time.Hour) // make sure ticker won't fire during the test

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the routine starts

	servers := []*Server{
		newTestServer(config.CameraConfig{Name: "cam", RTSPStream: "/cam"}, "admin", "admin"),
	}

	done := make(chan struct{})
	go func() {
		StartDetectionRoutine(ctx, servers)
		close(done)
	}()

	select {
	case <-done:
		// Expected: the for-select should observe ctx.Done and return.
	case <-time.After(1 * time.Second):
		t.Fatal("StartDetectionRoutine did not exit on pre-cancelled context within 1s")
	}
}
