// Package gadgets uses, on each line marked "want <name>", a gnark API that
// circuittest's scan flags, and on no other line. The go tool skips
// testdata, so this file is only parsed, never built.
package gadgets

import (
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/bits"
	"github.com/consensys/gnark/std/math/bitslice"
)

// byID is how a gadget would reach the R1CS builder's NewHintForId.
type byID interface {
	NewHintForId(id solver.HintID, n int, in ...frontend.Variable) ([]frontend.Variable, error) // want NewHintForId
}

func uses(api frontend.API, v frontend.Variable, f solver.Hint, b constraint.Blueprint) {
	api.Compiler().NewHint(f, 1, v) // want NewHint
	api.NewHint(f, 1, v)            // want NewHint
	hint := api.Compiler().NewHint  // want NewHint
	_, _ = hint(f, 1, v)
	_, _ = api.Compiler().(byID)
	api.Compiler().MarkBoolean(v)                                      // want MarkBoolean
	id := api.Compiler().AddBlueprint(b)                               // want AddBlueprint
	api.Compiler().AddInstruction(id, nil)                             // want AddInstruction
	bits.ToBinary(api, v, bits.WithUnconstrainedOutputs())             // want WithUnconstrainedOutputs
	bits.FromBinary(api, nil, bits.WithUnconstrainedInputs())          // want WithUnconstrainedInputs
	bits.ToBinary(api, v, bits.OmitModulusCheck())                     // want OmitModulusCheck
	bitslice.Partition(api, v, 8, bitslice.WithUnconstrainedOutputs()) // want WithUnconstrainedOutputs

	// The constrained forms are not flagged.
	bits.ToBinary(api, v, bits.WithNbDigits(8))
	api.AssertIsBoolean(v)
	api.AssertIsEqual(api.Mul(v, v), v)
}
