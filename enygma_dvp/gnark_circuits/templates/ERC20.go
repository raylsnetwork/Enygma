package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

type Erc20CircuitConfig struct {
	TmNInputs int
	TmMOutputs  int
	TmMerkleTreeDepth int
	TmRange frontend.Variable 	
}

type Erc20Circuit struct {

	Config Erc20CircuitConfig

	// --- public inputs (statement) ---
	StMessage        frontend.Variable   `gnark:",public"`
	StTreeNumber     []frontend.Variable `gnark:",public"` // nInputsERC20
	StMerkleRoots    []frontend.Variable `gnark:",public"` // nInputsERC20
	StNullifiers     []frontend.Variable `gnark:",public"` // nInputsERC20
	StCommitmentOut  []frontend.Variable `gnark:",public"` // MOutputs

	// --- private witnesses: inputs (coins being spent) ---
	WtPrivateKeysIn []frontend.Variable   // nInputsERC20 — sk_spend, proves ownership
	WtValuesIn      []frontend.Variable   // nInputsERC20
	WtSaltsIn       []frontend.Variable   // nInputsERC20 — saltB from when this note was received
	WtPathElements  [][]frontend.Variable // nInputsERC20 x MerkleTreeDepth
	WtPathIndices   []frontend.Variable   // nInputsERC20

	// Shared across all inputs and outputs (single token per proof)
	WtTokenId frontend.Variable

	// --- private witnesses: outputs (new notes being created) ---
	WtSpendPublicKeysOut []frontend.Variable // MOutputs — pk_spend of each recipient
	WtValuesOut          []frontend.Variable // MOutputs
	WtSaltsOut           []frontend.Variable // MOutputs — saltB from KEM for each output
}



func (circuit *Erc20Circuit) Define(api frontend.API) error {

	inputsTotals := frontend.Variable(0)
	outputsTotals := frontend.Variable(0)

	// --- verify input notes ---
	for i := 0; i < circuit.Config.TmNInputs; i++ {
		isValid0 := cmp.IsLess(api, circuit.WtValuesIn[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0, circuit.WtValuesIn[i])
		api.AssertIsEqual(isValid1, 1)

		// Derive pk_spend from sk_spend inside the circuit
		pkSpendIn := primitives.PublicKey(api, circuit.WtPrivateKeysIn[i])

		nullifier := primitives.Nullifier(api, circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i])
		api.AssertIsEqual(nullifier, circuit.StNullifiers[i])

		// V2 commitment: Poseidon(pk_spend, saltB, amount, tokenId)
		commitment := primitives.Erc20CommitmentV2(api, pkSpendIn, circuit.WtSaltsIn[i], circuit.WtValuesIn[i], circuit.WtTokenId)

		pathElement := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
		for j := 0; j < circuit.Config.TmMerkleTreeDepth; j++ {
			pathElement[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment, circuit.WtPathIndices[i], pathElement)

		// Skip Merkle check for dummy (zero-value) inputs
		isZero := api.IsZero(circuit.WtValuesIn[i])
		enable := api.Sub(1, isZero)
		// H-1 fix: force StNullifiers[i] = 0 for zero-value padding inputs.
		// Without this, the prover must supply a valid nullifier for dummy slots —
		// any (sk, treeNum, pathIndex) triple satisfies the unconditional AssertIsEqual
		// above, so a malicious prover can plant a victim's nullifier in a dummy slot,
		// permanently leaking the nullifier-to-note link (privacy violation).
		api.AssertIsEqual(api.Mul(circuit.StNullifiers[i], isZero), 0)
		diff := api.Sub(circuit.StMerkleRoots[i], root)
		api.AssertIsEqual(api.Mul(diff, enable), 0)

		inputsTotals = api.Add(inputsTotals, circuit.WtValuesIn[i])
	}

	// --- verify output notes ---
	for i := 0; i < circuit.Config.TmMOutputs; i++ {
		isValid0 := cmp.IsLess(api, circuit.WtValuesOut[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0, circuit.WtValuesOut[i])
		api.AssertIsEqual(isValid1, 1)

		// V2 commitment: Poseidon(pk_spendRecipient, saltB, amount, tokenId)
		// pk_spendRecipient is provided by the sender (Bob's public spend key)
		// saltB is the KEM-derived salt — matches what is inside ciphertextII on-chain
		commitment := primitives.Erc20CommitmentV2(api, circuit.WtSpendPublicKeysOut[i], circuit.WtSaltsOut[i], circuit.WtValuesOut[i], circuit.WtTokenId)
		api.AssertIsEqual(commitment, circuit.StCommitmentOut[i])

		outputsTotals = api.Add(outputsTotals, circuit.WtValuesOut[i])
	}

	api.AssertIsEqual(outputsTotals, inputsTotals)

	return nil
}



