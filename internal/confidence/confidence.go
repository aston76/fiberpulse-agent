package confidence

import "fiberpulse.dev/agent/internal/measurement"

type Input struct {
	Complete         bool
	Cancelled        bool
	ImpossibleValue  bool
	InterfaceChanged bool
	RouteChanged     bool
	Metered          bool
	ConnectionType   measurement.ConnectionType
	WiFiQuality      int
	PHYBelowPlan     bool
	CompetingTraffic bool
	ResourcePressure bool
	VPNSuspected     bool
	ProxySuspected   bool
	ASNMismatch      bool
}

type Result struct {
	Score          int      `json:"score"`
	Level          string   `json:"level"`
	Reasons        []string `json:"reasons"`
	PublicEligible bool     `json:"public_eligible"`
}

func Calculate(in Input) Result {
	score := 100
	blocking := false
	var reasons []string
	deduct := func(code string, points int, block bool) {
		score -= points
		reasons = append(reasons, code)
		blocking = blocking || block
	}
	if !in.Complete {
		deduct("result.incomplete", 30, true)
	}
	if in.Cancelled {
		deduct("result.cancelled", 30, true)
	}
	if in.ImpossibleValue {
		deduct("result.impossible_value", 30, true)
	}
	if in.InterfaceChanged {
		deduct("network.interface_changed", 20, true)
	}
	if in.RouteChanged {
		deduct("network.route_changed", 15, true)
	}
	if in.Metered {
		deduct("network.metered", 10, true)
	}
	if in.ConnectionType == measurement.ConnectionWiFi {
		switch {
		case in.WiFiQuality > 0 && in.WiFiQuality < 50:
			deduct("wifi.signal_low", 15, false)
		case in.WiFiQuality == 0:
			deduct("local_link.unknown", 10, false)
		}
		if in.PHYBelowPlan {
			deduct("wifi.phy_below_plan", 15, false)
		}
	} else if in.ConnectionType == measurement.ConnectionUnknown {
		deduct("local_link.unknown", 10, false)
	}
	if in.CompetingTraffic {
		deduct("host.competing_traffic", 10, false)
	}
	if in.ResourcePressure {
		deduct("host.resource_pressure", 5, false)
	}
	if in.VPNSuspected {
		deduct("routing.vpn_suspected", 10, true)
	}
	if in.ProxySuspected {
		deduct("routing.proxy_suspected", 7, true)
	}
	if in.ASNMismatch {
		deduct("routing.asn_mismatch", 10, true)
	}
	if score < 0 {
		score = 0
	}
	level := "low"
	if score >= 80 {
		level = "high"
	} else if score >= 60 {
		level = "medium"
	}
	return Result{Score: score, Level: level, Reasons: reasons, PublicEligible: level == "high" && !blocking}
}
