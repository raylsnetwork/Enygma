package payment

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/backend/groth16"

	ht "enygma_retail_payments/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_retail_payments/gnark_circuits/templates"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the payment prove handler
// with throwaway keys: the public signals it returns must be the proof's own
// public witness, as many as its statement length, and the proof must verify
// against them.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	c := paymentCase(t, 555)
	proof, signals := ht.Post(t, NewHandler(pkPath, vkPath), c.Request)
	ht.AssertStatementLen(t, signals, templates.PaymentStatementLayout())
	ht.AssertProvedWitness(t, c.Assignment, vk, proof, signals)
}

// TestHandler_ProvesConcurrentRequests sends distinct payments to one
// handler at once: each must come back proving its own payment.
func TestHandler_ProvesConcurrentRequests(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	var cases []ht.Case
	for salt := int64(1); salt <= 6; salt++ {
		cases = append(cases, paymentCase(t, salt))
	}
	ht.AssertConcurrentProofs(t, NewHandler(pkPath, vkPath), vk, cases)
}

// TestHandler_ReportsItsStats checks the response's stats: the Payment
// circuit, its constraint count, and its compile, prove and verify times.
func TestHandler_ReportsItsStats(t *testing.T) {
	pkPath, vkPath, _ := keys(t)
	empty := templates.NewPaymentCircuit(templates.PaymentConfig)
	ht.AssertStats(t, NewHandler(pkPath, vkPath), paymentCase(t, 555).Request, "Payment", &empty)
}

func keys(t *testing.T) (pkPath, vkPath string, vk groth16.VerifyingKey) {
	t.Helper()
	empty := templates.NewPaymentCircuit(templates.PaymentConfig)
	return ht.Keys(t, &empty)
}

// paymentCase pays 60 of a 100 note to Bob and 40 back as change, the change
// note salted with changeSalt.
func paymentCase(t *testing.T, changeSalt int64) ht.Case {
	t.Helper()
	cfg := templates.PaymentConfig
	in := ht.NewNote(t, 12345, 100, 7, cfg.TmMerkleTreeDepth)
	bobPk := ht.Hash(t, big.NewInt(999))
	values := []*big.Int{big.NewInt(60), big.NewInt(40)}
	pks := []*big.Int{bobPk, in.Pk}
	salts := []*big.Int{big.NewInt(444), big.NewInt(changeSalt)}
	cmts := []*big.Int{
		ht.Commitment(t, pks[0], salts[0], values[0], in.TokenId),
		ht.Commitment(t, pks[1], salts[1], values[1], in.TokenId),
	}

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	request := PaymentRequest{
		StMessage:            "0",
		StTreeNumbers:        [1]string{"0"},
		StMerkleRoots:        [1]string{in.Root.String()},
		StNullifiers:         [1]string{in.Nullifier.String()},
		StCommitmentsOut:     [2]string(ht.Strings(cmts...)),
		StContractAddress:    ht.VaultHex,
		WtPrivateKeysIn:      [1]string{in.Sk.String()},
		WtValuesIn:           [1]string{in.Value.String()},
		WtSaltsIn:            [1]string{in.Salt.String()},
		WtPathElements:       [1][8]string{path},
		WtPathIndices:        [1]string{"0"},
		WtTokenId:            in.TokenId.String(),
		WtSpendPublicKeysOut: [2]string(ht.Strings(pks...)),
		WtValuesOut:          [2]string(ht.Strings(values...)),
		WtSaltsOut:           [2]string(ht.Strings(salts...)),
	}

	w := templates.NewPaymentCircuit(cfg)
	w.StMessage = 0
	w.StTreeNumbers[0], w.StMerkleRoots[0], w.StNullifiers[0] = 0, in.Root, in.Nullifier
	for i := range cmts {
		w.StCommitmentsOut[i] = cmts[i]
	}
	w.StContractAddress = ht.Vault
	return ht.Case{Request: request, Assignment: &w}
}
