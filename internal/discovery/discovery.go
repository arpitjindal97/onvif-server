// Package discovery implements a minimal WS-Discovery listener for ONVIF.
package discovery

import (
	"net"
	"strings"

	"github.com/aragarwal/onvif-server/internal/logger"
)

// Start begins listening for WS-Discovery probe messages on the standard
// multicast group 239.255.255.250:3702. It blocks; run it in a goroutine.
func Start() {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		logger.Info("Failed to resolve discovery address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		logger.Info("Failed to listen on multicast: %v", err)
		return
	}
	defer conn.Close()

	logger.Info("WS-Discovery service started on 239.255.255.250:3702")

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			logger.Debug("Discovery read error: %v", err)
			continue
		}

		message := string(buf[:n])
		if strings.Contains(message, "Probe") && strings.Contains(message, "onvif") {
			logger.Debug("Received ONVIF discovery probe from %s", remoteAddr)
			// In production, respond with ProbeMatches
		}
	}
}
