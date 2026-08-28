package network

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
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
	active := make(map[string]net.Interface)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, addrErr := iface.Addrs()
		if addrErr != nil || len(addresses) == 0 {
			continue
		}
		active[iface.Name] = iface
		if isTunnelInterface(iface.Name) {
			result.VPNDetected = true
		}
	}
	if len(active) == 0 {
		return result, nil
	}

	selected := ""
	routeFingerprint := ""
	hardwarePorts := map[string]string{}
	if runtime.GOOS == "darwin" {
		if output, commandErr := exec.Command("/sbin/route", "-n", "get", "default").Output(); commandErr == nil {
			selected, routeFingerprint = parseDarwinDefaultRoute(string(output))
		}
		if output, commandErr := exec.Command("/usr/sbin/networksetup", "-listallhardwareports").Output(); commandErr == nil {
			hardwarePorts = parseDarwinHardwarePorts(string(output))
		}
		if output, commandErr := exec.Command("/usr/sbin/scutil", "--proxy").Output(); commandErr == nil && darwinProxyEnabled(string(output)) {
			result.ProxyDetected = true
		}
	}
	if _, ok := active[selected]; !ok {
		selected = preferredInterface(active)
		routeFingerprint = ""
	}
	result.Online = selected != ""
	if selected != "" {
		result.InterfaceID = opaqueID(selected)
		if routeFingerprint != "" {
			result.RouteID = opaqueID(routeFingerprint)
		}
		result.ConnectionType = classifyInterface(selected, hardwarePorts[selected])
		result.VPNDetected = result.VPNDetected || isTunnelInterface(selected)
	}
	for _, value := range []string{os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"), os.Getenv("ALL_PROXY")} {
		if strings.TrimSpace(value) != "" {
			result.ProxyDetected = true
			break
		}
	}
	return result, nil
}

func preferredInterface(active map[string]net.Interface) string {
	names := make([]string, 0, len(active))
	for name := range active {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !isTunnelInterface(name) && classifyInterface(name, "") != measurement.ConnectionOther {
			return name
		}
	}
	for _, name := range names {
		if !isTunnelInterface(name) {
			return name
		}
	}
	return names[0]
}

func classifyInterface(device, hardwarePort string) measurement.ConnectionType {
	name := strings.ToLower(strings.TrimSpace(device))
	port := strings.ToLower(strings.TrimSpace(hardwarePort))
	switch {
	case containsAny(port, "wi-fi", "wifi", "airport") || containsAny(name, "wi-fi", "wifi", "wlan"):
		return measurement.ConnectionWiFi
	case containsAny(port, "ethernet") || strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "en"):
		return measurement.ConnectionEthernet
	case containsAny(port, "cellular") || containsAny(name, "cellular", "wwan"):
		return measurement.ConnectionCellular
	case name == "":
		return measurement.ConnectionUnknown
	default:
		return measurement.ConnectionOther
	}
}

func isTunnelInterface(name string) bool {
	name = strings.ToLower(name)
	return containsAny(name, "vpn", "tun", "tap", "wireguard", "tailscale", "nordlynx") || strings.HasPrefix(name, "wg")
}

func parseDarwinDefaultRoute(output string) (device, fingerprint string) {
	gateway := ""
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "interface":
			device = strings.TrimSpace(value)
		case "gateway":
			gateway = strings.TrimSpace(value)
		}
	}
	if device != "" {
		fingerprint = device + "\x00" + gateway
	}
	return device, fingerprint
}

func parseDarwinHardwarePorts(output string) map[string]string {
	ports := make(map[string]string)
	hardwarePort := ""
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Hardware Port":
			hardwarePort = strings.TrimSpace(value)
		case "Device":
			device := strings.TrimSpace(value)
			if device != "" && hardwarePort != "" {
				ports[device] = hardwarePort
			}
		}
	}
	return ports
}

func darwinProxyEnabled(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || strings.TrimSpace(value) != "1" {
			continue
		}
		switch strings.TrimSpace(key) {
		case "HTTPEnable", "HTTPSEnable", "SOCKSEnable", "ProxyAutoConfigEnable", "ProxyAutoDiscoveryEnable":
			return true
		}
	}
	return false
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
