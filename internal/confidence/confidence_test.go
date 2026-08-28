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
