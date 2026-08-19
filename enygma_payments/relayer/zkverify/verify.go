// Package zkverify performs native Groth16 (BN254) proof verification
// in-process, using gnark directly — independent of the chain.
//
// This is deliberately narrow in scope: it answers exactly one question —
// "is this a valid proof of the circuit's constraints against these public
// signals?" — nothing more. It has no notion of current on-chain state, so
// it cannot (and does not try to) catch a well-formed proof that's stale
// against the chain (an already-spent nullifier, balances that moved since
// the proof was built, an unconfigured treasury, etc). That's what the
// relayer's eth_call dry-run (server.EnygmaContract.Simulate*) is for, and
// the contract's own on-chain verifier remains the sole source of truth —
// this package exists purely to reject garbage before it ever reaches the
// network, at effectively zero cost (no RPC round-trip, milliseconds of
// local pairing arithmetic).
package zkverify

import (
	"fmt"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/witness"
)

// Verifier holds one circuit's loaded Groth16 verifying key.
type Verifier struct {
	vk       groth16.VerifyingKey
	nbPublic int
}

// Load reads a Groth16 verifying key (BN254) from disk — the same .key file
// the gnark server loads to prove against. Verifying keys are public; this
// is not a secret. nbPublic is the exact public-signal count the circuit
// declares (e.g. 54 for enygma_fee, 80 for the FingerPrint transfer
// circuit); Verify fails closed if the supplied signal length doesn't match.
//
// This deliberately duplicates the open-file/ReadFrom sequence the gnark
// server's own loader uses, rather than importing it: relayer and
// gnark-server are separate Go modules deployed as separate services, and
// pulling gnark-server in as a relayer dependency would couple the two
// services' build/deploy lifecycles for the sake of ~10 lines of stable,
// unlikely-to-change gnark boilerplate.
func Load(vkPath string, nbPublic int) (*Verifier, error) {
	f, err := os.Open(vkPath)
	if err != nil {
		return nil, fmt.Errorf("open verifying key %s: %w", vkPath, err)
	}
	defer f.Close()

	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(f); err != nil {
		return nil, fmt.Errorf("read verifying key %s: %w", vkPath, err)
	}
	return &Verifier{vk: vk, nbPublic: nbPublic}, nil
}

// Verify checks that proof is a valid Groth16 proof against publicSignal,
// under this Verifier's verifying key. Returns nil iff valid.
//
// proof's 8-element layout matches the contract-facing encoding the gnark
// server produces: [Ar.X, Ar.Y, Bs.X.A1, Bs.X.A0, Bs.Y.A1, Bs.Y.A0, Krs.X,
// Krs.Y]. The B-coordinate A0/A1 swap (note: index 2 is A1, not A0) matches
// the convention shared with the Solidity Groth16 verifier's pairing
// precompile call — get this backwards and every proof fails to verify.
func (v *Verifier) Verify(proof [8]*big.Int, publicSignal []*big.Int) error {
	if len(publicSignal) != v.nbPublic {
		return fmt.Errorf("publicSignal: expected %d elements, got %d", v.nbPublic, len(publicSignal))
	}
	for i, p := range proof {
		if p == nil {
			return fmt.Errorf("proof[%d]: nil", i)
		}
	}

	var pf groth16_bn254.Proof
	pf.Ar.X.SetBigInt(proof[0])
	pf.Ar.Y.SetBigInt(proof[1])
	pf.Bs.X.A1.SetBigInt(proof[2])
	pf.Bs.X.A0.SetBigInt(proof[3])
	pf.Bs.Y.A1.SetBigInt(proof[4])
	pf.Bs.Y.A0.SetBigInt(proof[5])
	pf.Krs.X.SetBigInt(proof[6])
	pf.Krs.Y.SetBigInt(proof[7])

	w, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		return fmt.Errorf("build witness: %w", err)
	}
	ch := make(chan any, len(publicSignal))
	for _, s := range publicSignal {
		ch <- s
	}
	close(ch)
	if err := w.Fill(v.nbPublic, 0, ch); err != nil {
		return fmt.Errorf("fill witness: %w", err)
	}

	return groth16.Verify(&pf, v.vk, w)
}
