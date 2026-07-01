package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

type DvPDestinationCircuitConfig struct {
	TmMerkleTreeDepth int
	// H-2 fix: exclusive upper bound for amount witnesses — same constant used by
	// JoinSplit ERC20. Keys must be regenerated if this value changes.
	TmRange frontend.Variable
}

// DvPDestinationCircuit proves Bob's side of a DvP (Delivery vs Payment) swap.
//
// Bob spends his current note (asset 2) and proves:
//   1. COMMIT_A — Alice's receiving commitment — correctly encodes Bob's asset.
//   2. COMMIT_B — Alice's delivery commitment to Bob — is computed from the same
//      parameters Alice used, binding StMessage to this specific swap.
//
// This is the key linking constraint of the protocol:
//
//	COMMIT_A = Poseidon4(spendPkAlice, saltA, valueIn, tokenIdIn)
//	COMMIT_B = Poseidon4(spendPkBob,   saltB,  valueAlice, tokenIdAlice)
//
// HIGH-3 fix: StMessage is constrained to equal COMMIT_B. Without this constraint,
// StMessage is an under-constrained public input — the prover can set it to any
// value, breaking the binding between this proof and the specific swap context.
// Constraining StMessage = COMMIT_B ensures each DvP-Destination proof is uniquely
// tied to one swap (the one where Alice delivers exactly the note encoded in COMMIT_B).
//
// Off-chain Bob decapsulates CTXT to recover ss_B, then derives:
//
//	saltA  = HKDF(ss_B, "Init Salt")  → WtSaltA
//	saltB  = HKDF(ss_B, "note salt")  → WtSaltB
//	encKey = HKDF(ss_B, "encryption key")
//
// Bob decrypts ENC_TX_DATA to get (tokenIdAlice, valueAlice) and verifies
// both COMMIT_B and COMMIT_A before submitting his proof.
//
// Public statement (5 elements):
//
//	[commitB, treeNum, root, nf_B, commitA]
type DvPDestinationCircuit struct {
	Config DvPDestinationCircuitConfig

	// --- public inputs ---
	StMessage    frontend.Variable `gnark:",public"` // COMMIT_B — Alice's delivery commitment to Bob (HIGH-3: must equal computed commitB)
	StTreeNumber frontend.Variable `gnark:",public"` // sub-tree index for Bob's input note
	StMerkleRoot frontend.Variable `gnark:",public"` // Merkle root
	StNullifier  frontend.Variable `gnark:",public"` // nf_B = NullifierWithTree(sk_B, treeNum, leafIndex)
	StCommitA    frontend.Variable `gnark:",public"` // COMMIT_A from Alice's initiator proof

	// --- private witnesses: Bob's input note ---
	WtSpendKeyIn   frontend.Variable   // Bob's spend secret key
	WtValueIn      frontend.Variable   // amount_2 (Bob's asset)
	WtSaltIn       frontend.Variable   // saltBField of Bob's current note
	WtTokenIdIn    frontend.Variable   // token_id_2
	WtPathElements []frontend.Variable // Merkle path (TmMerkleTreeDepth elements)
	WtPathIndex    frontend.Variable   // leaf index in the tree

	// --- private witnesses: cross-commitment inputs ---
	WtSpendPkAlice    frontend.Variable // Alice's spend public key
	WtSaltA           frontend.Variable // HKDF(ss_B, "Init Salt") — Bob derived this by decapsulating
	WtSaltB           frontend.Variable // HKDF(ss_B, "note salt") — used by Alice to build COMMIT_B
	WtValueAlice      frontend.Variable // amount_1 (Alice's asset — from ENC_TX_DATA decryption)
	WtTokenIdAlice    frontend.Variable // token_id_1 (Alice's asset — from ENC_TX_DATA decryption)
}

func (circuit *DvPDestinationCircuit) Define(api frontend.API) error {
	// H-2 fix: amount range checks on all value witnesses.
	// Without these, a prover can set WtValueIn or WtValueAlice to a large BN254 field
	// element (≈2^254) that wraps during on-chain uint256 arithmetic, enabling vault
	// drain or permanently bricking the DvP flow for a token pair.
	api.AssertIsEqual(cmp.IsLess(api, circuit.WtValueIn, circuit.Config.TmRange), 1)
	api.AssertIsEqual(cmp.IsLessOrEqual(api, 0, circuit.WtValueIn), 1)
	api.AssertIsEqual(cmp.IsLess(api, circuit.WtValueAlice, circuit.Config.TmRange), 1)
	api.AssertIsEqual(cmp.IsLessOrEqual(api, 0, circuit.WtValueAlice), 1)

	// 1. Derive Bob's spend public key from his secret key.
	pkBob := primitives.PublicKey(api, circuit.WtSpendKeyIn)

	// 2. Nullifier: nf_B = Poseidon(sk_B, leafIndex).
	nullifier := primitives.Nullifier(api, circuit.WtSpendKeyIn, circuit.WtPathIndex)
	api.AssertIsEqual(nullifier, circuit.StNullifier)

	// 3. Input commitment: Poseidon4(pk_B, saltIn, valueIn, tokenIdIn).
	commitIn := primitives.Erc20CommitmentV2(api,
		pkBob,
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

	// 5. COMMIT_A: must encode the same amount and token that Bob is delivering.
	commitA := primitives.Erc20CommitmentV2(api,
		circuit.WtSpendPkAlice,
		circuit.WtSaltA,
		circuit.WtValueIn,
		circuit.WtTokenIdIn,
	)
	api.AssertIsEqual(commitA, circuit.StCommitA)

	// 6. COMMIT_B: Alice's delivery commitment to Bob — computed from Alice's asset values
	//    that Bob recovered from ENC_TX_DATA decryption.
	// HIGH-3 fix: StMessage must equal COMMIT_B to bind this proof to the specific swap.
	commitB := primitives.Erc20CommitmentV2(api,
		pkBob,
		circuit.WtSaltB,
		circuit.WtValueAlice,
		circuit.WtTokenIdAlice,
	)
	api.AssertIsEqual(circuit.StMessage, commitB)

	return nil
}
