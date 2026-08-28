package health

import (
	"context"
	"errors"
	"testing"

	"fiberpulse.dev/agent/internal/measurement"
)

type fixedInspector struct{ context measurement.NetworkContext }

func (f fixedInspector) Snapshot() (measurement.NetworkContext, error) { return f.context, nil }

type failingInspector struct{}

func (failingInspector) Snapshot() (measurement.NetworkContext, error) {
	return measurement.NetworkContext{}, errors.New("system API unavailable")
}

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

func TestInspectorFailureStaysUnknown(t *testing.T) {
	checker := Checker{Inspector: failingInspector{}, DNSName: "example.com", ProbeURL: "https://example.com"}
	sample := checker.Check(context.Background())
	if sample.State != "unknown" || sample.Category != "unknown" || sample.DetailCode != "network.snapshot_failed" {
		t.Fatalf("inspector failure became invented connectivity evidence: %+v", sample)
	}
	if sample.DNSConfigured || sample.ProbeConfigured {
		t.Fatalf("external checks ran without trustworthy network context: %+v", sample)
	}
}

func TestMissingInspectorStaysUnknown(t *testing.T) {
	sample := (Checker{}).Check(context.Background())
	if sample.State != "unknown" || sample.DetailCode != "network.inspector_unavailable" {
		t.Fatalf("missing inspector result: %+v", sample)
	}
}
