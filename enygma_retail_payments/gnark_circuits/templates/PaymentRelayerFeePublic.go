package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// PaymentRelayerFeePublicCircuit combines PaymentRelayerCircuit and PaymentFeeCircuit:
//   - 3 output commitments: Bob's payment, Alice's change, Relayer's spendable fee note
//   - StFee is a public signal — the chain can verify the exact fee being paid
//   - WtValuesOut[2] (relayer's amount) is constrained to equal StFee
//
// Conservation law (nothing burned — all value accounted for in outputs):
//
//	Σ(valuesIn) == valOut[0] + valOut[1] + valOut[2]
//
// Fee binding (links public StFee to the relayer's private note amount):
//
//	WtValuesOut[2] == StFee
//
// Combined this means: valueIn = payAmt + changeAmt + StFee
//
// Output layout:
//
//	[0] Bob's payment  — no ownership constraint (any pk_spend)
//	[1] Alice's change — constrained to senderPk
//	[2] Relayer fee    — any pk_spend, but amount == StFee (publicly verifiable)
//
// Public signal layout (9 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StCommitmentsOut[2],
//	 StContractAddress, StFee]
type PaymentRelayerFeePublicCircuit struct {
	Config PaymentCircuitConfig

	// --- public inputs ---
	StMessage         frontend.Variable   `gnark:",public"`
	StTreeNumbers     []frontend.Variable `gnark:",public"`
	StMerkleRoots     []frontend.Variable `gnark:",public"`
	StNullifiers      []frontend.Variable `gnark:",public"`
	StCommitmentsOut  []frontend.Variable `gnark:",public"` // length 3: [bob, change, relayer]
	StContractAddress frontend.Variable   `gnark:",public"`
	StFee             frontend.Variable   `gnark:",public"` // relayer fee amount, publicly verifiable

	// --- private witnesses: inputs ---
	WtPrivateKeysIn []frontend.Variable
	WtValuesIn      []frontend.Variable
	WtSaltsIn       []frontend.Variable
	WtPathElements  [][]frontend.Variable
	WtPathIndices   []frontend.Variable

	WtTokenId frontend.Variable

	// --- private witnesses: outputs (3 total) ---
	WtSpendPublicKeysOut []frontend.Variable // [bobPk, alicePk, relayerPk]
	WtValuesOut          []frontend.Variable // [payAmt, changeAmt, feeAmt]
	WtSaltsOut           []frontend.Variable // [saltBob, saltChange, saltRelayer]
}

func (circuit *PaymentRelayerFeePublicCircuit) Define(api frontend.API) error {

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
		// output[0] = Bob's payment (any key), output[2] = relayer fee (any key).
		if j == 1 {
			api.AssertIsEqual(circuit.WtSpendPublicKeysOut[j], senderPk)
		}

		outputsTotal = api.Add(outputsTotal, circuit.WtValuesOut[j])
	}

	// Conservation: all 3 outputs sum to all inputs — nothing burned.
	api.AssertIsEqual(outputsTotal, inputsTotal)

	// Fee binding: relayer's note amount must equal the public StFee signal.
	// This prevents Alice from under-paying the relayer while claiming a higher fee on-chain.
	api.AssertIsEqual(circuit.WtValuesOut[2], circuit.StFee)

	return nil
}
