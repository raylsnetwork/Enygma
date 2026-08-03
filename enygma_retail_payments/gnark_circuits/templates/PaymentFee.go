package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// PaymentFeeCircuit is the fee-absorbed variant of the plain 1-in/2-out payment
// circuit: the protocol fee is deducted from the sender's input rather than
// paid out as a separate spendable note.
//
//   - StFee is a public signal — the chain can verify the exact fee Alice paid.
//   - Fee is NOT an output note; it is simply unaccounted-for value, absorbed
//     into the conservation check below (mirrors EnygmaDvp.paymentWithFee()).
//
// Conservation law (fee absorbed, not an output):
//
//	Σ(valuesIn) == valOut[0] + valOut[1] + StFee
//
// Output layout:
//
//	[0] Bob's payment  — no ownership constraint (any pk_spend)
//	[1] Alice's change — constrained to senderPk
//
// Public signal layout (8 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StContractAddress, StFee]
type PaymentFeeCircuit struct {
	Config PaymentCircuitConfig

	// --- public inputs ---
	StMessage         frontend.Variable   `gnark:",public"`
	StTreeNumbers     []frontend.Variable `gnark:",public"`
	StMerkleRoots     []frontend.Variable `gnark:",public"`
	StNullifiers      []frontend.Variable `gnark:",public"`
	StCommitmentsOut  []frontend.Variable `gnark:",public"` // length 2: [bob, change]
	StContractAddress frontend.Variable   `gnark:",public"`
	StFee             frontend.Variable   `gnark:",public"` // protocol fee, publicly verifiable

	// --- private witnesses: inputs ---
	WtPrivateKeysIn []frontend.Variable
	WtValuesIn      []frontend.Variable
	WtSaltsIn       []frontend.Variable
	WtPathElements  [][]frontend.Variable
	WtPathIndices   []frontend.Variable

	WtTokenId frontend.Variable

	// --- private witnesses: outputs (2 total) ---
	WtSpendPublicKeysOut []frontend.Variable // [bobPk, alicePk]
	WtValuesOut          []frontend.Variable // [payAmt, changeAmt]
	WtSaltsOut           []frontend.Variable // [saltBob, saltChange]
}

func (circuit *PaymentFeeCircuit) Define(api frontend.API) error {

	api.AssertIsEqual(circuit.StMessage, 0)

	senderPk := primitives.PublicKey(api, circuit.WtPrivateKeysIn[0])

	inputsTotal := frontend.Variable(0)
	outputsTotal := frontend.Variable(0)

	// --- verify input notes ---
	for i := 0; i < circuit.Config.TmNInputs; i++ {
		isValid0 := cmp.IsLess(api, circuit.WtValuesIn[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)
		isValid1 := cmp.IsLessOrEqual(api, 0, circuit.WtValuesIn[i])
		api.AssertIsEqual(isValid1, 1)

		isZero := api.IsZero(circuit.WtValuesIn[i])
		enable := api.Sub(1, isZero)

		pkIn := primitives.PublicKey(api, circuit.WtPrivateKeysIn[i])

		if i > 0 {
			pkDiff := api.Sub(pkIn, senderPk)
			api.AssertIsEqual(api.Mul(pkDiff, enable), 0)
		}

		nullifier := primitives.NullifierBound(api, circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i], circuit.StContractAddress)
		nullifierDiff := api.Sub(nullifier, circuit.StNullifiers[i])
		api.AssertIsEqual(api.Mul(nullifierDiff, enable), 0)
		api.AssertIsEqual(api.Mul(circuit.StNullifiers[i], isZero), 0)

		commitment := primitives.Erc20CommitmentV2(api,
			pkIn,
			circuit.WtSaltsIn[i],
			circuit.WtValuesIn[i],
			circuit.WtTokenId,
		)

		pathElements := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
		for j := 0; j < circuit.Config.TmMerkleTreeDepth; j++ {
			pathElements[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment, circuit.WtPathIndices[i], pathElements)
		diff := api.Sub(circuit.StMerkleRoots[i], root)
		api.AssertIsEqual(api.Mul(diff, enable), 0)
		api.AssertIsEqual(api.Mul(circuit.StMerkleRoots[i], isZero), 0)

		inputsTotal = api.Add(inputsTotal, circuit.WtValuesIn[i])
	}

	// --- verify output notes ---
	for j := 0; j < circuit.Config.TmMOutputs; j++ {
		isValid0 := cmp.IsLess(api, circuit.WtValuesOut[j], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)
		isValid1 := cmp.IsLessOrEqual(api, 0, circuit.WtValuesOut[j])
		api.AssertIsEqual(isValid1, 1)

		commitment := primitives.Erc20CommitmentV2(api,
			circuit.WtSpendPublicKeysOut[j],
			circuit.WtSaltsOut[j],
			circuit.WtValuesOut[j],
			circuit.WtTokenId,
		)
		api.AssertIsEqual(commitment, circuit.StCommitmentsOut[j])

		// output[1] (Alice's change) must be owned by sender.
		// output[0] = Bob's payment (any key).
		if j == 1 {
			api.AssertIsEqual(circuit.WtSpendPublicKeysOut[j], senderPk)
		}

		outputsTotal = api.Add(outputsTotal, circuit.WtValuesOut[j])
	}

	// Conservation: outputs + fee must equal inputs — fee is absorbed, not an output.
	api.AssertIsEqual(api.Add(outputsTotal, circuit.StFee), inputsTotal)

	return nil
}
