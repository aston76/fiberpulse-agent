package scheduler

import "testing"

func TestSchedulerTransitionTable(t *testing.T) {
	machine := Machine{}
	for _, next := range []State{Recovered, Waiting, Due, Running, Cooldown, Waiting, Stopped, Recovered, Disabled, Waiting} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
}

func TestSchedulerBlockedStatesCanRetryOnlyThroughDue(t *testing.T) {
	for _, blocked := range []State{BlockedMetered, BlockedQuota, DeferredBusy} {
		machine := Machine{State: Due}
		if err := machine.Transition(blocked); err != nil {
			t.Fatal(err)
		}
		if !machine.CanBecomeDue() {
			t.Fatalf("%s cannot retry", blocked)
		}
		if err := machine.Transition(Running); err == nil {
			t.Fatalf("%s skipped due", blocked)
		}
	}
}

func TestDisabledSchedulerCannotBecomeDue(t *testing.T) {
	machine := Machine{State: Disabled}
	if machine.CanBecomeDue() {
		t.Fatal("disabled scheduler became due")
	}
	if err := machine.Transition(Due); err == nil {
		t.Fatal("disabled scheduler transitioned directly to due")
	}
}
