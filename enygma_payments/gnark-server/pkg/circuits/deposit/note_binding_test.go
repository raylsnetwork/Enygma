package deposit

// A deposit credits SenderTxValue and has the DvP vault spend the note Hash
// names (Enygma.sol's deposit()). These tests pin that Hash is a note the
// prover owns and whose amount is the credit: the vault's note for a
// withdrawal is Poseidon(Poseidon(asset, amount), Poseidon(sk)), and its
// amount and public key are public (withdraw()'s depositParams).
//
//	CC=/usr/bin/clang go test ./pkg/circuits/deposit -run TestNote -v

import (
	"math/big"
	"testing"
)

// noteWitness is depositWitness's honest deposit of amount, redeeming note.
func noteWitness(t *testing.T, amount int64, note *big.Int) *DepositEnygmaCircuit {
	t.Helper()
	s := realSecrets()
	w := depositWitness(t, amount, s, fingerprintsOf(t, s))
	w.Hash = note
	return w
}

// vaultNote is the note the vault inserts for a withdrawal of amount to the
// owner of sk, at depositWitness's asset address.
func vaultNote(t *testing.T, amount int64, sk *big.Int) *big.Int {
	t.Helper()
	return hash(t, hash(t, big.NewInt(777), big.NewInt(amount)), hash(t, sk))
}

func TestNote_DepositRedeemsItsOwnNoteForItsAmount(t *testing.T) {
	if err := solve(t, noteWitness(t, 100, vaultNote(t, 100, big.NewInt(555)))); err != nil {
		t.Fatalf("a deposit of a note's amount, by its owner, rejected: %v", err)
	}
}

func TestNote_DepositCannotCreditMoreThanTheNote(t *testing.T) {
	if err := solve(t, noteWitness(t, 100, vaultNote(t, 99, big.NewInt(555)))); err == nil {
		t.Fatal("a deposit crediting 100 solved while redeeming a note of 99")
	}
}

func TestNote_DepositNeedsTheNoteSecretKeyNotItsPublicKey(t *testing.T) {
	// Anyone can read a note's public key from the withdrawal that made it.
	pk := hash(t, big.NewInt(555))
	w := noteWitness(t, 100, vaultNote(t, 100, big.NewInt(555)))
	w.SkDeposit = pk
	if err := solve(t, w); err == nil {
		t.Fatal("a deposit solved with the note's public key in place of its secret key")
	}
}
