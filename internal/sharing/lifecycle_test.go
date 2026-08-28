package sharing

import "testing"

func TestSharingEnableAndRevokeLifecycle(t *testing.T) {
	machine := Machine{State: NotAsked}
	for _, next := range []State{Enabling, Enabled, Suspended, Enabled, Revoking, Revoked, Enabling, Enabled} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
}

func TestSharingCannotSkipExplicitEnableOrRevoke(t *testing.T) {
	if err := (&Machine{State: NotAsked}).Transition(Enabled); err == nil {
		t.Fatal("sharing enabled without enabling state")
	}
	if err := (&Machine{State: Enabled}).Transition(Revoked); err == nil {
		t.Fatal("sharing revoked without revoking state")
	}
}
