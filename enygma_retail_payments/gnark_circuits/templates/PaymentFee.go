package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// PaymentFeeCircuit extends PaymentCircuit with a fixed protocol fee.
//
// Conservation law (with fee):
//
//	Σ(valuesIn) == Σ(valuesOut) + StFee
//
// StFee is a public signal. The relayer verifies it equals PROTOCOL_FEE before
// submitting the transaction on-chain.
//
// Public signal layout (1-in / 2-out):
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
	StCommitmentsOut  []frontend.Variable `gnark:",public"`
	StContractAddress frontend.Variable   `gnark:",public"`
	StFee             frontend.Variable   `gnark:",public"` // relayer checks == PROTOCOL_FEE

	// --- private witnesses: inputs ---
	WtPrivateKeysIn []frontend.Variable
	WtValuesIn      []frontend.Variable
	WtSaltsIn       []frontend.Variable
	WtPathElements  [][]frontend.Variable
	WtPathIndices   []frontend.Variable

	WtTokenId frontend.Variable

	// --- private witnesses: outputs ---
	WtSpendPublicKeysOut []frontend.Variable
	WtValuesOut          []frontend.Variable
	WtSaltsOut           []frontend.Variable
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

		if j >= 1 {
			api.AssertIsEqual(circuit.WtSpendPublicKeysOut[j], senderPk)
		}

		outputsTotal = api.Add(outputsTotal, circuit.WtValuesOut[j])
	}

	// Conservation with fee: inputsTotal == outputsTotal + fee
	api.AssertIsEqual(inputsTotal, api.Add(outputsTotal, circuit.StFee))

	return nil
}
