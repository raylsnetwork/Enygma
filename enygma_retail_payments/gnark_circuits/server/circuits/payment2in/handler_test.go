package payment2in

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/backend/groth16"

	ht "enygma_retail_payments/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_retail_payments/gnark_circuits/templates"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the payment2in prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, as many as its statement length, and the proof
// must verify against them. Input 1 is zero-value padding (root and
// nullifier 0).
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	c := payment2inCase(t, 555)
	proof, signals := ht.Post(t, NewHandler(pkPath, vkPath), c.Request)
	ht.AssertStatementLen(t, signals, templates.Payment2inStatementLayout())
	ht.AssertProvedWitness(t, c.Assignment, vk, proof, signals)
}

// TestHandler_ProvesConcurrentRequests sends distinct payments to one handler
// at once: each must come back proving its own.
func TestHandler_ProvesConcurrentRequests(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	var cases []ht.Case
	for salt := int64(1); salt <= 6; salt++ {
		cases = append(cases, payment2inCase(t, salt))
	}
	ht.AssertConcurrentProofs(t, NewHandler(pkPath, vkPath), vk, cases)
}

func keys(t *testing.T) (pkPath, vkPath string, vk groth16.VerifyingKey) {
	t.Helper()
	empty := templates.NewPaymentCircuit(templates.Payment2inConfig)
	return ht.Keys(t, &empty)
}

// payment2inCase pays 60 of a 100 note to Bob and 40 back as change, the
// change note salted with changeSalt.
func payment2inCase(t *testing.T, changeSalt int64) ht.Case {
	t.Helper()
	cfg := templates.Payment2inConfig
	in := ht.NewNote(t, 12345, 100, 7, cfg.TmMerkleTreeDepth)
	values := []*big.Int{big.NewInt(60), big.NewInt(40)}
	pks := []*big.Int{ht.Hash(t, big.NewInt(999)), in.Pk}
	salts := []*big.Int{big.NewInt(444), big.NewInt(changeSalt)}
	cmts := []*big.Int{
		ht.Commitment(t, pks[0], salts[0], values[0], in.TokenId),
		ht.Commitment(t, pks[1], salts[1], values[1], in.TokenId),
	}

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	zeroPath := [8]string{"0", "0", "0", "0", "0", "0", "0", "0"}
	request := PaymentRequest2in{
		StMessage:            "0",
		StTreeNumbers:        [2]string{"0", "0"},
		StMerkleRoots:        [2]string{in.Root.String(), "0"},
		StNullifiers:         [2]string{in.Nullifier.String(), "0"},
		StCommitmentsOut:     [2]string(ht.Strings(cmts...)),
		StContractAddress:    ht.VaultHex,
		WtPrivateKeysIn:      [2]string{in.Sk.String(), "1"},
		WtValuesIn:           [2]string{in.Value.String(), "0"},
		WtSaltsIn:            [2]string{in.Salt.String(), "0"},
		WtPathElements:       [2][8]string{path, zeroPath},
		WtPathIndices:        [2]string{"0", "0"},
		WtTokenId:            in.TokenId.String(),
		WtSpendPublicKeysOut: [2]string(ht.Strings(pks...)),
		WtValuesOut:          [2]string(ht.Strings(values...)),
		WtSaltsOut:           [2]string(ht.Strings(salts...)),
	}

	w := templates.NewPaymentCircuit(cfg)
	w.StMessage = 0
	w.StTreeNumbers[0], w.StMerkleRoots[0], w.StNullifiers[0] = 0, in.Root, in.Nullifier
	w.StTreeNumbers[1], w.StMerkleRoots[1], w.StNullifiers[1] = 0, 0, 0
	for i := range cmts {
		w.StCommitmentsOut[i] = cmts[i]
	}
	w.StContractAddress = ht.Vault
	return ht.Case{Request: request, Assignment: &w}
}
