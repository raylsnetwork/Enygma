package paymentRelayerFeePublic

import (
	"math/big"
	"testing"

	ht "enygma_dvp/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_dvp/gnark_circuits/templates"
	"enygma_dvp/layout"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the paymentRelayerFeePublic
// prove handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, as many as its statement length, and the proof
// must verify against them. A request whose public values the witness does not
// satisfy gets an error.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	cfg := templates.PaymentRelayerFeePublicConfig
	empty := templates.NewPaymentRelayerFeePublicCircuit(cfg)
	pkPath, vkPath, vk := ht.Keys(t, &empty)
	handler := NewHandler(pkPath, vkPath)

	in := ht.NewNote(t, 12345, 100, 7, cfg.TmMerkleTreeDepth)
	fee := big.NewInt(20)
	values := []*big.Int{big.NewInt(50), big.NewInt(30), fee} // == 100
	pks := []*big.Int{ht.Hash(t, big.NewInt(999)), in.Pk, ht.Hash(t, big.NewInt(31337))}
	salts := []*big.Int{big.NewInt(444), big.NewInt(555), big.NewInt(666)}
	cmts := make([]*big.Int, 3)
	for i := range cmts {
		cmts[i] = ht.Commitment(t, pks[i], salts[i], values[i], in.TokenId)
	}

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	request := PaymentRelayerFeePublicRequest{
		StMessage:            "0",
		StTreeNumbers:        [1]string{"0"},
		StMerkleRoots:        [1]string{in.Root.String()},
		StNullifiers:         [1]string{in.Nullifier.String()},
		StCommitmentsOut:     [3]string(ht.Strings(cmts...)),
		StContractAddress:    ht.VaultHex,
		StFee:                fee.String(),
		WtPrivateKeysIn:      [1]string{in.Sk.String()},
		WtValuesIn:           [1]string{in.Value.String()},
		WtSaltsIn:            [1]string{in.Salt.String()},
		WtPathElements:       [1][8]string{path},
		WtPathIndices:        [1]string{"0"},
		WtTokenId:            in.TokenId.String(),
		WtSpendPublicKeysOut: [3]string(ht.Strings(pks...)),
		WtValuesOut:          [3]string(ht.Strings(values...)),
		WtSaltsOut:           [3]string(ht.Strings(salts...)),
	}
	proof, signals := ht.Post(t, handler, request)

	w := templates.NewPaymentRelayerFeePublicCircuit(cfg)
	w.StMessage = 0
	w.StTreeNumbers[0], w.StMerkleRoots[0], w.StNullifiers[0] = 0, in.Root, in.Nullifier
	for i := range cmts {
		w.StCommitmentsOut[i] = cmts[i]
	}
	w.StContractAddress = ht.Vault
	w.StFee = fee
	ht.AssertStatementLen(t, signals, layout.PaymentRelayerFeeLen)
	ht.AssertProvedWitness(t, &w, vk, proof, signals)

	request.StNullifiers[0] = "1"
	ht.PostRejected(t, handler, request)
}
