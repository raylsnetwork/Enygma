package paymentFee

import (
	"math/big"
	"testing"

	ht "enygma_retail_payments/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_retail_payments/gnark_circuits/templates"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the paymentFee prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, and the proof must verify against them.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	cfg := templates.PaymentFeeConfig
	empty := templates.NewPaymentFeeCircuit(cfg)
	pkPath, vkPath, vk := ht.Keys(t, &empty)

	in := ht.NewNote(t, 12345, 100, 7, cfg.TmMerkleTreeDepth)
	fee := big.NewInt(10)
	values := []*big.Int{big.NewInt(50), big.NewInt(40)} // + fee == 100
	pks := []*big.Int{ht.Hash(t, big.NewInt(999)), in.Pk}
	salts := []*big.Int{big.NewInt(444), big.NewInt(555)}
	cmts := []*big.Int{
		ht.Commitment(t, pks[0], salts[0], values[0], in.TokenId),
		ht.Commitment(t, pks[1], salts[1], values[1], in.TokenId),
	}

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	request := PaymentFeeRequest{
		StMessage:            "0",
		StTreeNumbers:        [1]string{"0"},
		StMerkleRoots:        [1]string{in.Root.String()},
		StNullifiers:         [1]string{in.Nullifier.String()},
		StCommitmentsOut:     [2]string(ht.Strings(cmts...)),
		StContractAddress:    ht.VaultHex,
		StFee:                fee.String(),
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
	proof, signals := ht.Post(t, NewHandler(pkPath, vkPath), request)

	w := templates.NewPaymentFeeCircuit(cfg)
	w.StMessage = 0
	w.StTreeNumbers[0], w.StMerkleRoots[0], w.StNullifiers[0] = 0, in.Root, in.Nullifier
	for i := range cmts {
		w.StCommitmentsOut[i] = cmts[i]
	}
	w.StContractAddress = ht.Vault
	w.StFee = fee
	ht.AssertProvedWitness(t, &w, vk, proof, signals)
}
