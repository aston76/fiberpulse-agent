package health

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/network"
)

type Sample struct {
	At         time.Time                  `json:"at"`
	Network    measurement.NetworkContext `json:"network"`
	DNSOK      bool                       `json:"dns_ok"`
	ProbeOK    bool                       `json:"probe_ok"`
	ProbeRTTUS int64                      `json:"probe_rtt_us"`
	State      string                     `json:"state"`
	Category   string                     `json:"category"`
	DetailCode string                     `json:"detail_code,omitempty"`
}

type Checker struct {
	Inspector network.Inspector
	DNSName   string
	ProbeURL  string
	Client    *http.Client
}

func (c Checker) Check(ctx context.Context) Sample {
	s := Sample{At: time.Now().UTC(), State: "unknown", Category: "unknown"}
	s.Network, _ = c.Inspector.Snapshot()
	if !s.Network.Online {
		s.State = "offline"
		s.Category = "local_interface"
		return s
	}
	s.State = "internet_usable"
	if c.DNSName != "" {
		dnsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := net.DefaultResolver.LookupHost(dnsCtx, c.DNSName)
		s.DNSOK = err == nil
		if err != nil {
			s.State = "internet_degraded"
			s.Category = "dns"
			s.DetailCode = "dns.lookup_failed"
		}
	} else {
		s.DNSOK = true
	}
	if c.ProbeURL != "" {
		if parsed, err := url.Parse(c.ProbeURL); err != nil || parsed.Scheme != "https" {
			s.State = "internet_degraded"
			s.Category = "internet_reachability"
			s.DetailCode = "probe.invalid_url"
			return s
		}
		client := c.Client
		if client == nil {
			client = &http.Client{Timeout: 5 * time.Second}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.ProbeURL, nil)
		start := time.Now()
		if err == nil {
			resp, doErr := client.Do(req)
			err = doErr
			if resp != nil {
				resp.Body.Close()
				s.ProbeOK = resp.StatusCode >= 200 && resp.StatusCode < 400
			}
		}
		s.ProbeRTTUS = time.Since(start).Microseconds()
		if err != nil || !s.ProbeOK {
			s.State = "internet_degraded"
			s.Category = "internet_reachability"
			s.DetailCode = "probe.failed"
		}
	} else {
		s.ProbeOK = true
	}
	return s
}
