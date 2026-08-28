package health

import (
	"testing"
	"time"
)

func TestConnectivityRequiresRepeatedConflictingEvidence(t *testing.T) {
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	var machine ConnectivityMachine
	state, err := machine.Observe(ConnectivityInternetUsable, base)
	if err != nil || state != ConnectivityInternetUsable {
		t.Fatalf("initial state=%s err=%v", state, err)
	}
	state, err = machine.Observe(ConnectivityOffline, base.Add(time.Minute))
	if err != nil || state != ConnectivityUnstable {
		t.Fatalf("first conflict state=%s err=%v", state, err)
	}
	state, err = machine.Observe(ConnectivityOffline, base.Add(2*time.Minute))
	if err != nil || state != ConnectivityOffline {
		t.Fatalf("confirmed state=%s err=%v", state, err)
	}
}

func TestConnectivityRecoversFromSingleConflict(t *testing.T) {
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	var machine ConnectivityMachine
	_, _ = machine.Observe(ConnectivityInternetUsable, base)
	_, _ = machine.Observe(ConnectivityOffline, base.Add(time.Minute))
	state, err := machine.Observe(ConnectivityInternetUsable, base.Add(2*time.Minute))
	if err != nil || state != ConnectivityInternetUsable {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestConnectivitySnapshotResumesPendingHysteresis(t *testing.T) {
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	var first ConnectivityMachine
	_, _ = first.Observe(ConnectivityInternetUsable, base)
	_, _ = first.Observe(ConnectivityInternetDegraded, base.Add(time.Minute))
	var second ConnectivityMachine
	if err := second.Restore(first.Snapshot()); err != nil {
		t.Fatal(err)
	}
	state, err := second.Observe(ConnectivityInternetDegraded, base.Add(2*time.Minute))
	if err != nil || state != ConnectivityInternetDegraded {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestConnectivityContinuesAcrossBackwardClockChange(t *testing.T) {
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	var machine ConnectivityMachine
	_, _ = machine.Observe(ConnectivityInternetUsable, base)
	state, err := machine.Observe(ConnectivityOffline, base.Add(-time.Minute))
	if err != nil || state != ConnectivityUnstable {
		t.Fatalf("first observation after clock rollback state=%s err=%v", state, err)
	}
	state, err = machine.Observe(ConnectivityOffline, base.Add(-2*time.Minute))
	if err != nil || state != ConnectivityOffline {
		t.Fatalf("confirmed observation after clock rollback state=%s err=%v", state, err)
	}
	if observedAt := machine.Snapshot().LastObservedAt; observedAt != base {
		t.Fatalf("last observation time moved backward: %s", observedAt)
	}
}
