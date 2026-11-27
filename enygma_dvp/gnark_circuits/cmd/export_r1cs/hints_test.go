package main

import (
	"testing"

	"enygma_gnark_shared/circuittest"
)

// deployed returns the circuits export writes, the ones the server proves
// with keys of its own (TestExportsEveryServedCircuit), for circuittest's
// checks. fromRetail's keys are made from retail's circuits, which retail's
// module checks the same way; this module's own source for them is in
// templates, which TestCircuitSourcesLeaveNothingUnconstrained reads.
func deployed() []circuittest.Named {
	named := make([]circuittest.Named, 0, len(circuits))
	for _, c := range circuits {
		named = append(named, circuittest.Named{Name: c.name, Circuit: c.circuit()})
	}
	return named
}

// TestCircuitsUseOnlyGnarkHints: no circuit the server proves with keys of
// its own depends on a hint that gnark does not define.
func TestCircuitsUseOnlyGnarkHints(t *testing.T) {
	circuittest.CheckHints(t, deployed())
}

// TestCircuitSourcesLeaveNothingUnconstrained: no package of this repo that
// export builds, among them each circuit's own and the Poseidon gadget's,
// uses a gnark API that can leave a value unconstrained.
func TestCircuitSourcesLeaveNothingUnconstrained(t *testing.T) {
	circuittest.CheckSources(t, deployed(), "enygma_gnark_shared/poseidon")
}
