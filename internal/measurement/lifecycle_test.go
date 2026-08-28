package measurement

import "testing"

func TestMeasurementLifecycleCompleteAndReset(t *testing.T) {
	machine := TestMachine{State: TestIdle}
	for _, next := range []TestState{TestPreflight, TestQuotaReserved, TestLocate, TestDownload, TestUpload, TestValidate, TestPersist, TestShareQueued, TestComplete, TestIdle} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
}

func TestMeasurementLifecyclePersistsFailureBeforeReset(t *testing.T) {
	machine := TestMachine{State: TestIdle}
	for _, next := range []TestState{TestPreflight, TestQuotaReserved, TestLocate, TestValidate, TestPersist, TestFailed, TestIdle} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if err := machine.Transition(TestUpload); err == nil {
		t.Fatal("idle measurement skipped preflight")
	}
}
