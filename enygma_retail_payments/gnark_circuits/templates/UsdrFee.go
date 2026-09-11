package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// UsdrFeeCircuit is a second, independent relayer-fee asset: a 1-in/2-out
// circuit structurally identical to PaymentFeeCircuit's input/output note
// verification, but with two differences that give it its own identity:
//
//   - StTokenId is a PUBLIC signal here (private WtTokenId everywhere else
//     in this codebase). USDr is a single, fixed, known token — exactly like
//     making FeeAmount public was the core design decision for enygma_payments'
//     USDrCircuit, there is nothing to hide about USDr's token identity, and
//     making it public is what gives this circuit's statement a distinct
//     9-element length (7/8/9/10 are otherwise all already claimed by other
//     circuits' statement lengths — see Erc20CoinVault.checkReceiptConditions).
//   - The fee is PAID, not burned: output[0] is a real spendable note for the
//     relayer, constrained to StFee, unlike PaymentFeeCircuit's StFee (which
//     is simply subtracted from the conservation check with no matching
//     output commitment).
//
// Conservation law (nothing burned — all value accounted for in outputs):
//
//	Σ(valuesIn) == valOut[0] + valOut[1]
//
// Fee binding (links the public StFee to the relayer's private note amount):
//
//	WtValuesOut[0] == StFee
//
// Output layout:
//
//	[0] Relayer fee note — any pk_spend, amount == StFee (publicly verifiable)
//	[1] Sender's change  — constrained to senderPk
//
// Public signal layout (9 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StContractAddress, StFee, StTokenId]
type UsdrFeeCircuit struct {
	Config PaymentCircuitConfig

	// --- public inputs ---
	StMessage         frontend.Variable   `gnark:",public"`
	StTreeNumbers     []frontend.Variable `gnark:",public"`
	StMerkleRoots     []frontend.Variable `gnark:",public"`
	StNullifiers      []frontend.Variable `gnark:",public"`
	StCommitmentsOut  []frontend.Variable `gnark:",public"` // length 2: [relayerFee, change]
	StContractAddress frontend.Variable   `gnark:",public"`
	StFee             frontend.Variable   `gnark:",public"` // relayer fee amount, publicly verifiable
	StTokenId         frontend.Variable   `gnark:",public"` // USDr's fixed token id — public, unlike WtTokenId elsewhere

	// --- private witnesses: inputs ---
	WtPrivateKeysIn []frontend.Variable
	WtValuesIn      []frontend.Variable
	WtSaltsIn       []frontend.Variable
	WtPathElements  [][]frontend.Variable
	WtPathIndices   []frontend.Variable

	// --- private witnesses: outputs (2 total) ---
	WtSpendPublicKeysOut []frontend.Variable // [relayerPk, senderPk]
	WtValuesOut          []frontend.Variable // [feeAmt, changeAmt]
	WtSaltsOut           []frontend.Variable // [saltFee, saltChange]
}

func (circuit *UsdrFeeCircuit) Define(api frontend.API) error {

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

		nullifier := primitives.Nullifier(api, circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i])
		nullifierDiff := api.Sub(nullifier, circuit.StNullifiers[i])
		api.AssertIsEqual(api.Mul(nullifierDiff, enable), 0)
		api.AssertIsEqual(api.Mul(circuit.StNullifiers[i], isZero), 0)

		commitment := primitives.Erc20CommitmentV2(api,
			pkIn,
			circuit.WtSaltsIn[i],
			circuit.WtValuesIn[i],
			circuit.StTokenId,
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
			circuit.StTokenId,
		)
		api.AssertIsEqual(commitment, circuit.StCommitmentsOut[j])

		// output[1] (sender's change) must be owned by sender.
		// output[0] = relayer fee note (any key, but amount == StFee below).
		if j == 1 {
			api.AssertIsEqual(circuit.WtSpendPublicKeysOut[j], senderPk)
		}

		outputsTotal = api.Add(outputsTotal, circuit.WtValuesOut[j])
	}

	// Conservation: both outputs sum to all inputs — nothing burned, unlike
	// PaymentFeeCircuit where StFee is subtracted with no matching output.
	api.AssertIsEqual(outputsTotal, inputsTotal)

	// Fee binding: relayer's note amount must equal the public StFee signal.
	api.AssertIsEqual(circuit.WtValuesOut[0], circuit.StFee)

	return nil
}
