package network

import (
	"testing"

	"fiberpulse.dev/agent/internal/measurement"
)

func TestDarwinRouteAndHardwarePortParsing(t *testing.T) {
	device, fingerprint := parseDarwinDefaultRoute("gateway: 10.0.0.1\n interface: en1\n")
	if device != "en1" || fingerprint != "en1\x0010.0.0.1" {
		t.Fatalf("route device=%q fingerprint=%q", device, fingerprint)
	}
	ports := parseDarwinHardwarePorts("Hardware Port: Ethernet\nDevice: en0\n\nHardware Port: Wi-Fi\nDevice: en1\n")
	if got := classifyInterface("en1", ports["en1"]); got != measurement.ConnectionWiFi {
		t.Fatalf("en1 classified as %q", got)
	}
	if got := classifyInterface("en0", ports["en0"]); got != measurement.ConnectionEthernet {
		t.Fatalf("en0 classified as %q", got)
	}
}

func TestTunnelAndSystemProxyDetection(t *testing.T) {
	if !isTunnelInterface("utun4") || !isTunnelInterface("NordLynx") || isTunnelInterface("en1") {
		t.Fatal("tunnel interface classification is incorrect")
	}
	if !darwinProxyEnabled("HTTPEnable : 0\nProxyAutoConfigEnable : 1\n") {
		t.Fatal("enabled PAC proxy was not detected")
	}
	if darwinProxyEnabled("HTTPEnable : 0\nHTTPSEnable : 0\n") {
		t.Fatal("disabled proxy was detected")
	}
}
