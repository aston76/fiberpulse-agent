package app

import (
	"testing"

	"fiberpulse.dev/agent/internal/health"
)

func TestLifecycleTransitionTable(t *testing.T) {
	machine := LifecycleMachine{State: LifecycleStarting}
	for _, state := range []LifecycleState{LifecycleConsentRequired, LifecyclePaused, LifecycleMonitoring, LifecycleDegraded, LifecycleMonitoring, LifecycleStopping, LifecycleStopped} {
		if err := machine.Transition(state); err != nil {
			t.Fatalf("transition to %s: %v", state, err)
		}
	}
	if err := machine.Transition(LifecycleMonitoring); err == nil {
		t.Fatal("stopped lifecycle restarted without a new application instance")
	}
}

func TestLifecycleRejectsSkippingStopping(t *testing.T) {
	machine := LifecycleMachine{State: LifecycleMonitoring}
	if err := machine.Transition(LifecycleStopped); err == nil {
		t.Fatal("monitoring transitioned directly to stopped")
	}
}

func TestHealthTransitionsMonitoringAndDegraded(t *testing.T) {
	a := newTestApp(t)
	a.lifecycle.State = LifecycleMonitoring
	if err := a.processHealthSample(t.Context(), healthSample("internet_degraded")); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecycleDegraded {
		t.Fatalf("state=%s", a.lifecycle.State)
	}
	if err := a.processHealthSample(t.Context(), healthSample("internet_usable")); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecycleMonitoring {
		t.Fatalf("state=%s", a.lifecycle.State)
	}
}

func healthSample(state string) health.Sample {
	return health.Sample{State: state, Category: "unknown"}
}
