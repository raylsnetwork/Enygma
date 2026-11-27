package payment2in

import (
	"math/big"
	"testing"

	ht "enygma_dvp/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_dvp/layout"
	"enygma_retail_payments/gnark_circuits/templates"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the payment2in prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, as many as its statement length, and the proof
// must verify against them. A request whose public values the witness does not
// satisfy gets an error. Input 1 is zero-value padding (root and nullifier 0).
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	cfg := templates.Payment2inConfig
	empty := templates.NewPaymentCircuit(cfg)
	pkPath, vkPath, vk := ht.Keys(t, &empty)
	handler := NewHandler(pkPath, vkPath)

	in := ht.NewNote(t, 12345, 100, 7, cfg.TmMerkleTreeDepth)
	values := []*big.Int{big.NewInt(60), big.NewInt(40)}
	pks := []*big.Int{ht.Hash(t, big.NewInt(999)), in.Pk}
	salts := []*big.Int{big.NewInt(444), in.ChangeSalt}
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
		StCiphertextHash:     ht.CiphertextHashHex,
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
	proof, signals := ht.Post(t, handler, request)

	w := templates.NewPaymentCircuit(cfg)
	w.StMessage = 0
	w.StTreeNumbers[0], w.StMerkleRoots[0], w.StNullifiers[0] = 0, in.Root, in.Nullifier
	w.StTreeNumbers[1], w.StMerkleRoots[1], w.StNullifiers[1] = 0, 0, 0
	for i := range cmts {
		w.StCommitmentsOut[i] = cmts[i]
	}
	w.StContractAddress = ht.Vault
	w.StCiphertextHash = ht.CiphertextHash
	ht.AssertStatementLen(t, signals, layout.Payment2inLen)
	ht.AssertProvedWitness(t, &w, vk, proof, signals)

	valid := request
	request.StNullifiers[0] = "1"
	ht.PostRejected(t, handler, request)

	// The circuit is enygma_retail_payments' Payment. Its change note
	// (output 1) must be salted Poseidon(sk, nullifier0), and its input 0
	// must be a real note.
	t.Run("free change salt", func(t *testing.T) {
		free := valid
		free.WtSaltsOut[1] = "555"
		free.StCommitmentsOut[1] = ht.Commitment(t, pks[1], big.NewInt(555), values[1], in.TokenId).String()
		ht.PostRejected(t, handler, free)
	})
	t.Run("no input note", func(t *testing.T) {
		none := valid
		none.StMerkleRoots[0], none.StNullifiers[0] = "0", "0"
		none.WtValuesIn[0], none.WtSaltsIn[0] = "0", "0"
		for j := range none.WtPathElements[0] {
			none.WtPathElements[0][j] = "0"
		}
		outValues, outSalts, outCommitments := ht.NoNoteOutputs(t, in, pks, salts)
		copy(none.WtValuesOut[:], outValues)
		copy(none.WtSaltsOut[:], outSalts)
		copy(none.StCommitmentsOut[:], outCommitments)
		ht.PostRejected(t, handler, none)
	})
}
