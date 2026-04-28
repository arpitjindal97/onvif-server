package netutil

import (
	"net"
	"testing"
)

func TestGetOutboundIP_ReturnsParsableAddress(t *testing.T) {
	ip := GetOutboundIP()
	if ip == "" {
		t.Fatal("GetOutboundIP returned empty string")
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		t.Errorf("GetOutboundIP returned %q, which is not a valid IP", ip)
	}
}
