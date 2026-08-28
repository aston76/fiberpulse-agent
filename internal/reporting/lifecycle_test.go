package reporting

import "testing"

func TestReportLifecycle(t *testing.T) {
	machine := Machine{State: Drafting}
	for _, next := range []State{Ready, Exporting, Exported, Deleted} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if err := machine.Transition(Ready); err == nil {
		t.Fatal("deleted report returned to ready")
	}
}

func TestReportLifecycleAllowsFailuresOnlyFromActiveWork(t *testing.T) {
	machine := Machine{State: Drafting}
	if err := machine.Transition(Failed); err != nil {
		t.Fatal(err)
	}
	if err := machine.Transition(Exported); err == nil {
		t.Fatal("failed report became exported")
	}
}
