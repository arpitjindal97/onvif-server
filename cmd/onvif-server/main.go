// Command onvif-server runs one virtual ONVIF server per camera defined in
// the configuration file, fronting an existing RTSP server.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
	"github.com/aragarwal/onvif-server/internal/discovery"
	"github.com/aragarwal/onvif-server/internal/logger"
	"github.com/aragarwal/onvif-server/internal/metrics"
	"github.com/aragarwal/onvif-server/internal/netutil"
	"github.com/aragarwal/onvif-server/internal/onvif"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging (verbose output)")
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	logger.SetDebug(*debug)
	if *debug {
		logger.Info("Debug mode enabled - verbose logging active")
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rtspHost := cfg.RTSPHost
	if rtspHost == "" {
		rtspHost = netutil.GetOutboundIP()
		logger.Info("Auto-detected IP: %s", rtspHost)
	}

	// Initialize OpenTelemetry metrics (no-op if disabled in config).
	shutdownMetrics, err := metrics.Init(context.Background(), cfg.Metrics, version)
	if err != nil {
		logger.Info("Metrics: init failed: %v (continuing without metrics)", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownMetrics(ctx); err != nil {
			logger.Info("Metrics: shutdown error: %v", err)
		}
	}()

	if cfg.EnableDiscovery {
		go discovery.Start()
	}

	servers := make([]*onvif.Server, 0, len(cfg.Cameras))
	for _, camCfg := range cfg.Cameras {
		servers = append(servers, onvif.NewServer(camCfg, rtspHost, cfg.RTSPPort, cfg.Username, cfg.Password))
	}

	go onvif.StartDetectionRoutine(context.Background(), servers)
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s *onvif.Server) {
			defer wg.Done()
			if err := s.Start(); err != nil {
				logger.Info("Server for '%s' failed: %v", s.CameraName(), err)
			}
		}(server)

		time.Sleep(100 * time.Millisecond)
	}

	logger.Info("All ONVIF servers started successfully")

	// Block until SIGINT/SIGTERM, then return so deferred metrics flush runs.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Shutdown signal received, exiting...")

	// Note: wg is intentionally not waited on — http.ListenAndServe blocks
	// forever in each goroutine; we exit the process after flushing metrics.
}
