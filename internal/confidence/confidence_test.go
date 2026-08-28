package confidence

import (
	"fiberpulse.dev/agent/internal/measurement"
	"testing"
)

func TestHighConfidenceEthernet(t *testing.T) {
	r := Calculate(Input{Complete: true, ConnectionType: measurement.ConnectionEthernet})
	if r.Score != 100 || !r.PublicEligible {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestVPNBlocksPublicWithoutClaimingProbability(t *testing.T) {
	r := Calculate(Input{Complete: true, ConnectionType: measurement.ConnectionEthernet, VPNSuspected: true})
	if r.Score != 90 || r.PublicEligible {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestIncompleteAlwaysBlocks(t *testing.T) {
	r := Calculate(Input{Complete: false, ConnectionType: measurement.ConnectionEthernet})
	if r.PublicEligible {
		t.Fatal("incomplete result became public eligible")
	}
}

func TestDevelopmentProviderNeverBecomesPublicEligible(t *testing.T) {
	r := Calculate(Input{Complete: true, ConnectionType: measurement.ConnectionEthernet, NonPublicProvider: true})
	if r.Score != 100 || r.PublicEligible {
		t.Fatalf("unexpected result: %+v", r)
	}
	if len(r.Reasons) != 1 || r.Reasons[0] != "provider.not_public" {
		t.Fatalf("missing provider gate reason: %+v", r.Reasons)
	}
}
