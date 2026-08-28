package health

import (
	"context"
	"testing"

	"fiberpulse.dev/agent/internal/measurement"
)

type fixedInspector struct{ context measurement.NetworkContext }

func (f fixedInspector) Snapshot() (measurement.NetworkContext, error) { return f.context, nil }

func TestUnconfiguredExternalChecksAreNotReportedHealthy(t *testing.T) {
	checker := Checker{Inspector: fixedInspector{context: measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}}}
	sample := checker.Check(context.Background())
	if sample.State != "local_only" || sample.DNSConfigured || sample.DNSOK || sample.ProbeConfigured || sample.ProbeOK {
		t.Fatalf("unexpected sample: %+v", sample)
	}
}

func TestOfflineStopsBeforeExternalChecks(t *testing.T) {
	checker := Checker{Inspector: fixedInspector{context: measurement.NetworkContext{Online: false}}, DNSName: "invalid.example", ProbeURL: "https://invalid.example"}
	sample := checker.Check(context.Background())
	if sample.State != "offline" || sample.Category != "local_interface" {
		t.Fatalf("unexpected sample: %+v", sample)
	}
}
