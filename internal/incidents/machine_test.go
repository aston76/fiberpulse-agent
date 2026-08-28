package incidents

import (
	"testing"
	"time"
)

func TestIncidentHysteresis(t *testing.T) {
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	m := &Machine{}
	if got := m.Observe(Observation{At: base, Degraded: true, Category: "dns"}); got != None {
		t.Fatalf("first bad: %s", got)
	}
	if got := m.Observe(Observation{At: base.Add(time.Minute), Degraded: true, Category: "dns"}); got != Suspected {
		t.Fatalf("second bad: %s", got)
	}
	if got := m.Observe(Observation{At: base.Add(2 * time.Minute), Degraded: true, Category: "dns"}); got != Active {
		t.Fatalf("third bad: %s", got)
	}
	for i := 3; i < 6; i++ {
		m.Observe(Observation{At: base.Add(time.Duration(i) * time.Minute), Degraded: false})
	}
	if m.State != Recovering {
		t.Fatalf("expected recovering, got %s", m.State)
	}
	if got := m.Observe(Observation{At: base.Add(21 * time.Minute), Degraded: false}); got != Resolved {
		t.Fatalf("expected resolved, got %s", got)
	}
}
