package relayer

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/emulated"
	stdgroth16 "github.com/consensys/gnark/std/recursion/groth16"
)

// NPublicSignals is the size of EnygmaCircuit's public signal for
// NCommitment=6 (see enygma.EnygmaCircuitConfig / enygma/circuit.go's field
// comments): FingerPrint (6x6=36) + PublicKey (6) + PreviousCommit (6x2=12) +
// TxCommit (6x2=12) + BlockNumber (1) + AnonymitySet (6) + MessageTags (6) +
// Nullifier (1) = 80.
const NPublicSignals = 80

// nativeFieldBits is the bit length of the BN254 scalar field (the native
// field both the inner EnygmaCircuit proof and this outer recursive circuit
// are defined over). Any valid native public-signal value fits in this many
// bits; used to size the decompose-and-compare binding check below.
const nativeFieldBits = 254

// RelayerCircuit independently re-verifies a sender's EnygmaCircuit
// (transfer) proof via in-circuit (recursive) Groth16 verification, so the
// relayer's own proof attests "I checked the sender's proof against the
// known transfer-circuit VK for these exact public inputs" without the
// relayer ever needing the sender's private witness (secret key, blinding
// randomness, amounts never leave the sender's machine).
//
// Two things happen here, not one:
//  1. The recursive pairing check (AssertProof) — this alone only proves the
//     *emulated* InnerWitness values pair correctly with Proof under vk.
//  2. The binding check — constrains that the plain *native* PublicSignal
//     values (the ones this circuit's own proof exposes on-chain, in the
//     same order/shape as EnygmaCircuit's existing ABI) are the exact same
//     values used in (1). Without this, a malicious relayer could satisfy
//     the pairing check with one witness while publishing a different
//     PublicSignal on-chain — the two representations must be proven equal.
type RelayerCircuit struct {
	// Public: identical shape to the sender's existing on-chain
	// `public_signal` array (see enygma/handler.go's publicSignal build
	// order) — kept native so the on-chain ABI/verifier stays a plain
	// uint256[80], not a 4x-larger emulated-limb encoding.
	PublicSignal [NPublicSignals]frontend.Variable `gnark:",public"`

	// Private witness: the sender's transfer proof, and the same 80 public
	// values re-expressed as the emulated field elements the recursive
	// verifier needs for the pairing check. No sender secret data (secret
	// key, randomness, amounts) appears anywhere here — only the proof and
	// public signal the relayer already legitimately receives today.
	Proof        stdgroth16.Proof[sw_bn254.G1Affine, sw_bn254.G2Affine]
	InnerWitness stdgroth16.Witness[sw_bn254.ScalarField]

	// vk is the transfer circuit's verifying key, embedded as a compile-time
	// constant (via stdgroth16.ValueOfVerifyingKeyFixed when this circuit is
	// constructed — see keygen/generate_keys.go's generateKeysRelayer and
	// handler.go's NewHandler) rather than a witness value: it never changes
	// across proofs, so fixing it adds zero per-proof witness cost.
	vk stdgroth16.VerifyingKey[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl] `gnark:"-"`
}

// NewCircuit returns a RelayerCircuit with its (compile-time-fixed)
// verifying key set. vk is intentionally unexported on RelayerCircuit — this
// exported constructor is the only way to set it from outside this package —
// so it can never accidentally be supplied as per-request witness data. All
// other fields (Proof, InnerWitness, PublicSignal) are ordinary exported
// fields; set them directly on the returned circuit.
func NewCircuit(vk stdgroth16.VerifyingKey[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl]) *RelayerCircuit {
	return &RelayerCircuit{vk: vk}
}

func (c *RelayerCircuit) Define(api frontend.API) error {
	if len(c.InnerWitness.Public) != NPublicSignals {
		return fmt.Errorf("relayer circuit: InnerWitness.Public has %d entries, want %d", len(c.InnerWitness.Public), NPublicSignals)
	}

	field, err := emulated.NewField[sw_bn254.ScalarField](api)
	if err != nil {
		return fmt.Errorf("relayer circuit: new emulated field: %w", err)
	}

	// Binding check: native PublicSignal[i] == InnerWitness.Public[i],
	// via bit-decompose-and-recompose (there is no ready-made gnark helper
	// for native<->emulated equality at v0.14.0).
	for i := 0; i < NPublicSignals; i++ {
		bits := api.ToBinary(c.PublicSignal[i], nativeFieldBits)
		recomposed := field.FromBits(bits...)
		field.AssertIsEqual(recomposed, &c.InnerWitness.Public[i])
	}

	// Recursive re-verification of the sender's transfer proof.
	verifier, err := stdgroth16.NewVerifier[sw_bn254.ScalarField, sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](api)
	if err != nil {
		return fmt.Errorf("relayer circuit: new verifier: %w", err)
	}
	// WithCompleteArithmetic: EnygmaCircuit's public signal legitimately
	// contains many exact-zero entries (e.g. FingerPrint diagonal slots —
	// see enygma/circuit.go). AssertProof's default MultiScalarMul path
	// (jointScalarMulGLVUnsafe) divides by zero when it internally collides
	// two identical points while combining zero/near-zero scalars — this
	// forces the safe/complete addition formulas instead, which is slightly
	// more expensive but has no such edge case.
	if err := verifier.AssertProof(c.vk, c.Proof, c.InnerWitness, stdgroth16.WithCompleteArithmetic()); err != nil {
		return fmt.Errorf("relayer circuit: assert proof: %w", err)
	}
	return nil
}
