package network

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
)

type Inspector interface {
	Snapshot() (measurement.NetworkContext, error)
}

type SystemInspector struct{}

func (SystemInspector) Snapshot() (measurement.NetworkContext, error) {
	result := measurement.NetworkContext{ConnectionType: measurement.ConnectionUnknown, CapturedAt: time.Now().UTC()}
	interfaces, err := net.Interfaces()
	if err != nil {
		return result, err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, addrErr := iface.Addrs()
		if addrErr != nil || len(addresses) == 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		result.Online = true
		result.InterfaceID = opaqueID(iface.Name)
		switch {
		case containsAny(name, "wi-fi", "wifi", "wlan"):
			result.ConnectionType = measurement.ConnectionWiFi
		case containsAny(name, "ethernet", "eth", "en"):
			result.ConnectionType = measurement.ConnectionEthernet
		case containsAny(name, "cellular", "wwan"):
			result.ConnectionType = measurement.ConnectionCellular
		default:
			result.ConnectionType = measurement.ConnectionOther
		}
		if containsAny(name, "vpn", "tun", "tap", "wireguard", "tailscale", "nordlynx") {
			result.VPNDetected = true
		}
		break
	}
	for _, value := range []string{os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"), os.Getenv("ALL_PROXY")} {
		if strings.TrimSpace(value) != "" {
			result.ProxyDetected = true
			break
		}
	}
	return result, nil
}

func opaqueID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
func containsAny(value string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(value, n) {
			return true
		}
	}
	return false
}
