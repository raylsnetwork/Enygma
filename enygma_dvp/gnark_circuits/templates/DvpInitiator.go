package templates

import (
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

type DvPInitiatorCircuitConfig struct {
	TmMerkleTreeDepth int
}

// DvPInitiatorCircuit proves Alice's side of a DvP (Delivery vs Payment) swap.
//
// Alice spends her current note (asset 1) and proves three new commitments:
//
//   - COMMIT_B        = Poseidon4(spendPkBob,   saltB,       valueIn, tokenIdIn)
//     Bob receives Alice's asset at the same amount and token.
//
//   - COMMIT_A        = Poseidon4(spendPkAlice,  saltA,       valueBob, tokenIdBob)
//     Alice receives Bob's asset.
//
//   - REVERT_COMMIT_A = Poseidon4(spendPkAlice,  revertSalt, valueIn, tokenIdIn)
//     If the swap times out on-chain, Alice's asset reverts to this commitment.
//
// Off-chain Alice runs ML-KEM.Encapsulate(bob_view_pk) → (ss_B, CTXT) then:
//
//	saltB       = HKDF(ss_B, "note salt")       → WtSaltB
//	saltA       = HKDF(ss_B, "Init Salt")      → WtSaltA
//	encKey      = HKDF(ss_B, "encryption key")
//	ENC_TX_DATA = AES-GCM-ENC(encKey, tokenIdIn || valueIn)
//
// Bob can decapsulate CTXT, re-derive saltB/saltA/encKey and independently
// verify both COMMIT_B and COMMIT_A before creating his response proof.
//
// Public statement (7 elements):
//
//	[msg, treeNum, root, nf_A, commitB, commitA, revertCommitA]
type DvPInitiatorCircuit struct {
	Config DvPInitiatorCircuitConfig

	// --- public inputs ---
	StMessage       frontend.Variable `gnark:",public"` // domain separator / swap link (0 = standalone)
	StTreeNumber    frontend.Variable `gnark:",public"` // sub-tree index for Alice's input note
	StMerkleRoot    frontend.Variable `gnark:",public"` // Merkle root
	StNullifier     frontend.Variable `gnark:",public"` // nf_A = Poseidon(sk_A, leafIndex)
	StCommitB       frontend.Variable `gnark:",public"` // Bob receives Alice's asset
	StCommitA       frontend.Variable `gnark:",public"` // Alice receives Bob's asset
	StRevertCommitA frontend.Variable `gnark:",public"` // Alice's fallback if swap times out

	// --- private witnesses: Alice's input note ---
	WtSpendKeyIn   frontend.Variable   // Alice's spend secret key
	WtValueIn      frontend.Variable   // amount_1 (Alice's asset)
	WtSaltIn       frontend.Variable   // saltBField of Alice's current note
	WtTokenIdIn    frontend.Variable   // token_id_1
	WtPathElements []frontend.Variable // Merkle path (TmMerkleTreeDepth elements)
	WtPathIndex    frontend.Variable   // leaf index in the tree

	// --- private witnesses: outputs ---
	WtSpendPkBob frontend.Variable // Bob's spend public key
	WtSaltB      frontend.Variable // HKDF(ss_B, "note salt")  → for COMMIT_B
	WtValueBob   frontend.Variable // amount_2 (Bob's asset Alice expects to receive)
	WtTokenIdBob frontend.Variable // token_id_2 (Bob's asset)
	WtSaltA      frontend.Variable // HKDF(ss_B, "Init Salt") → for COMMIT_A
	WtRevertSalt frontend.Variable // fresh random salt         → for REVERT_COMMIT_A
}

func (circuit *DvPInitiatorCircuit) Define(api frontend.API) error {
	// HIGH-5 fix: StMessage must equal StCommitA (Alice's receiving commitment).
	// On-chain _settleOnGroupPair verifies: receipt1.statement[0] == receipt2.statement[commitA_idx]
	// Both sides must reference the same commitA — constraining StMessage here prevents
	// a prover from setting it to an arbitrary value to forge the cross-swap linkage.
	api.AssertIsEqual(circuit.StMessage, circuit.StCommitA)

	// 1. Derive Alice's spend public key from her secret key.
	pkAlice := primitives.PublicKey(api, circuit.WtSpendKeyIn)

	// 2. Nullifier: nf_A = Poseidon(sk_A, leafIndex).
	nullifier := primitives.Nullifier(api, circuit.WtSpendKeyIn, circuit.WtPathIndex)
	api.AssertIsEqual(nullifier, circuit.StNullifier)

	// 3. Input commitment: Poseidon4(pk_A, saltIn, valueIn, tokenIdIn).
	commitIn := primitives.Erc20CommitmentV2(api,
		pkAlice,
		circuit.WtSaltIn,
		circuit.WtValueIn,
		circuit.WtTokenIdIn,
	)

	// 4. Merkle membership: commitIn is in the tree at WtPathIndex.
	pathElems := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	for j := 0; j < circuit.Config.TmMerkleTreeDepth; j++ {
		pathElems[j] = circuit.WtPathElements[j]
	}
	root := primitives.MerkleProof(api, commitIn, circuit.WtPathIndex, pathElems)
	api.AssertIsEqual(root, circuit.StMerkleRoot)

	// 5. COMMIT_B: Bob receives Alice's asset (same value and tokenId as Alice's input).
	commitB := primitives.Erc20CommitmentV2(api,
		circuit.WtSpendPkBob,
		circuit.WtSaltB,
		circuit.WtValueIn,
		circuit.WtTokenIdIn,
	)
	api.AssertIsEqual(commitB, circuit.StCommitB)

	// 6. COMMIT_A: Alice receives Bob's asset (valueBob, tokenIdBob).
	commitA := primitives.Erc20CommitmentV2(api,
		pkAlice,
		circuit.WtSaltA,
		circuit.WtValueBob,
		circuit.WtTokenIdBob,
	)
	api.AssertIsEqual(commitA, circuit.StCommitA)

	// 7. REVERT_COMMIT_A: Alice's fallback (same asset as her input, fresh salt).
	revertCommitA := primitives.Erc20CommitmentV2(api,
		pkAlice,
		circuit.WtRevertSalt,
		circuit.WtValueIn,
		circuit.WtTokenIdIn,
	)
	api.AssertIsEqual(revertCommitA, circuit.StRevertCommitA)

	return nil
}
