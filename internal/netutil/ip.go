// Package netutil contains small networking helpers.
package netutil

import "net"

// GetOutboundIP returns the preferred outbound IP of this host
// by opening a UDP "connection" to a well-known address. It does
// not actually send any packets.
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
