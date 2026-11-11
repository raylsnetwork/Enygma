package templates

import (
	"testing"

	"enygma_gnark_shared/circuitconfig"
)

// TestCircuitConfigs_HoldNoVariables fails if any auction circuit keeps a
// frontend.Variable (or any other non-constant) in its config, which gnark
// would compile as a prover-chosen witness. AuctionBid's bid bound used to be
// one (Config.TmRange); it is now the fixed valuebound constant.
func TestCircuitConfigs_HoldNoVariables(t *testing.T) {
	for name, c := range map[string]any{
		"AuctionLock":     AuctionLockCircuit{},
		"AuctionBid":      AuctionBidCircuit{},
		"AuctionBatch":    AuctionBatchCircuit{},
		"AuctionFinal":    AuctionFinalCircuit{},
		"AuctionRevert":   AuctionRevertCircuit{},
		"AuctionWithdraw": AuctionWithdrawCircuit{},
	} {
		if err := circuitconfig.Check(c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
