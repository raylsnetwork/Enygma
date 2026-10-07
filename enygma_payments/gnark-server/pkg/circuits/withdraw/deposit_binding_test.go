package withdraw

// The DvP deposit commitments (Hashes) are public signals 52-61, so
// Enygma.withdraw() can require the notes it creates to be exactly these.
// While they were private, only TotalDepositValue was bound: whoever submitted
// the proof chose the recipients' keys, so another bank could replay a pending
// withdrawal with its own keys and the same total, and receive the funds.

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"

	utils "enygma_payments/gnark-server/utils"
)

func honestWithdrawValues(t *testing.T) m03Values {
	return m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
}

// An unused deposit slot (VPerDeposit 0) must publish Hashes 0, so the
// contract can tell exactly which slots carry a deposit.
func TestWithdraw_UnusedSlotMustPublishZeroHash(t *testing.T) {
	v := honestWithdrawValues(t)
	v.hashes[3] = big.NewInt(12345) // slot 3 carries no value
	if err := m03Solve(t, v.circuit()); err == nil {
		t.Fatal("a non-zero commitment for an unused deposit slot was accepted")
	}
}

// A proof made for one set of deposit commitments must not verify for
// another: an attacker replaying the proof with a commitment to its own key
// (same amount) is rejected by the verifier itself.
func TestWithdraw_DepositCommitmentsAreBoundByTheProof(t *testing.T) {
	v := honestWithdrawValues(t)

	solver.RegisterHint(utils.ModHint)
	template := createWithdrawCircuitTemplate(WithdrawEnygmaCircuitConfig{NCommitment: 6})
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &template)
	if err != nil {
		t.Fatal(err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	full, err := frontend.NewWitness(v.circuit(), ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	proof, err := groth16.Prove(ccs, pk, full)
	if err != nil {
		t.Fatal(err)
	}

	verify := func(w m03Values) error {
		pub, err := frontend.NewWitness(w.circuit(), ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			t.Fatal(err)
		}
		return groth16.Verify(proof, vk, pub)
	}
	if err := verify(v); err != nil {
		t.Fatalf("proof does not verify with its own deposit commitments: %v", err)
	}

	// The attacker's commitment: same amount, its own key.
	attacker := v
	first, err := iden3poseidon.Hash([]*big.Int{v.depositAddr, v.vPerDeposit[0]})
	if err != nil {
		t.Fatal(err)
	}
	attackerPk, err := iden3poseidon.Hash([]*big.Int{big.NewInt(999999)})
	if err != nil {
		t.Fatal(err)
	}
	attacker.hashes[0], err = iden3poseidon.Hash([]*big.Int{first, attackerPk})
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(attacker); err == nil {
		t.Fatal("VULNERABLE: the withdraw proof verified with a different deposit recipient")
	}
}
