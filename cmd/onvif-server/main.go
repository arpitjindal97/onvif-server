// Command onvif-server runs one virtual ONVIF server per camera defined in
// the configuration file, fronting an existing RTSP server.
package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
	"github.com/aragarwal/onvif-server/internal/discovery"
	"github.com/aragarwal/onvif-server/internal/logger"
	"github.com/aragarwal/onvif-server/internal/netutil"
	"github.com/aragarwal/onvif-server/internal/onvif"
)

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

	if cfg.EnableDiscovery {
		go discovery.Start()
	}

	servers := make([]*onvif.Server, 0, len(cfg.Cameras))
	for _, camCfg := range cfg.Cameras {
		servers = append(servers, onvif.NewServer(camCfg, rtspHost, cfg.RTSPPort, cfg.Username, cfg.Password))
	}

	go onvif.StartDetectionRoutine(servers)
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
	wg.Wait()
}
