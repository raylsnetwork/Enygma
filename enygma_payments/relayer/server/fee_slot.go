package server

import (
	"context"
	"fmt"
	"math/big"

	enygma "enygma_payments/relayer/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

// USDr public-signal offsets this check reads (Enygma.sol: FP_TX_COMMIT_OFFSET
// and USDR_FEE_AMOUNT_OFFSET).
const (
	usdrTxCommitOffset  = 54
	usdrFeeAmountOffset = 80
)

// Pedersen generators — identical to Enygma.sol's GX/GY/HX/HY and
// gnark-server/utils (Fix H-11's NUMS derivation).
var (
	feeG = &babyjub.Point{
		X: mustDecimal("12337812418750581066638756637363471856433191340622504180842886595232027947307"),
		Y: mustDecimal("15225366398330386329633463986700597127113326976080712967801565482915963669722"),
	}
	feeH = &babyjub.Point{
		X: mustDecimal("10100005861917718053548237064487763771145251762383025193119768015180892676690"),
		Y: mustDecimal("7512830269827713629724023825249861327768672768516116945507944076335453576011"),
	}
)

func mustDecimal(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad decimal constant " + s)
	}
	return n
}

// pedersen returns v*G + r*H.
func pedersen(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, feeG)
	rH := babyjub.NewPoint().Mul(r, feeH)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

// feeSlotLookupError marks a failure to read chain state (as opposed to the
// client sending a transfer that does not pay the relayer), so the handler can
// answer 502 instead of 402.
type feeSlotLookupError struct{ err error }

func (e *feeSlotLookupError) Error() string { return e.err.Error() }

// verifyFeeSlot rejects a transfer whose USDr proof does not pay THIS relayer
// the full fixed fee.
//
// The USDr circuit constrains the non-sender credits to sum to FeeAmount but
// leaves the recipient and the split to the prover, and the contract does not
// check who is paid. So the relayer has to: it finds its own slot (its
// address's account id in participantIds), takes that slot's delta commitment
// from the proof's public signals, and requires it to equal
// Com(usdrFixedFeeAmount, usdrFeeRandomness). A commitment opens uniquely, so
// a client that pays the relayer less, or pays another slot, cannot supply a
// randomness that makes this hold.
func (h *Handler) verifyFeeSlot(
	ctx context.Context,
	usdrSignal [UsdrFeePublicSignalLen]*big.Int,
	usdrCommitments []enygma.IEnygmaPoint,
	participantIds []*big.Int,
	usdrFeeRandomness string,
) error {
	if !h.cfg.VerifyFeeSlot {
		return nil
	}
	if usdrFeeRandomness == "" {
		return fmt.Errorf("usdrFeeRandomness is required: the relayer verifies it is paid before relaying")
	}
	r, err := checkFieldElement("usdrFeeRandomness", usdrFeeRandomness, bn254Fr)
	if err != nil {
		return err
	}

	callOpts := &bind.CallOpts{Context: ctx}
	fee, err := h.instance.UsdrFixedFeeAmount(callOpts)
	if err != nil {
		return &feeSlotLookupError{fmt.Errorf("read usdrFixedFeeAmount: %w", err)}
	}
	if usdrSignal[usdrFeeAmountOffset].Cmp(fee) != 0 {
		return fmt.Errorf("USDr proof's FeeAmount %s does not equal the contract's usdrFixedFeeAmount %s",
			usdrSignal[usdrFeeAmountOffset], fee)
	}

	relayerID, err := h.instance.AddressToAccountId(callOpts, h.auth.From)
	if err != nil {
		return &feeSlotLookupError{fmt.Errorf("read relayer account id: %w", err)}
	}
	if relayerID.Sign() == 0 {
		return fmt.Errorf("relayer address %s is not a registered account, so it cannot be paid a USDr fee", h.auth.From.Hex())
	}

	slot := -1
	for i, id := range participantIds {
		if id.Cmp(relayerID) == 0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("the relayer's account (%s) is not one of the transfer's participants, so the fee cannot be paid to it", relayerID)
	}

	dx, dy := usdrSignal[usdrTxCommitOffset+2*slot], usdrSignal[usdrTxCommitOffset+2*slot+1]
	if usdrCommitments[slot].C1.Cmp(dx) != 0 || usdrCommitments[slot].C2.Cmp(dy) != 0 {
		return fmt.Errorf("usdrCommitments[%d] does not match the USDr proof's TxCommit for the relayer's slot", slot)
	}

	want := pedersen(fee, r)
	if want.X.Cmp(dx) != 0 || want.Y.Cmp(dy) != 0 {
		return fmt.Errorf("the USDr note in the relayer's slot (%d) does not open to the fixed fee %s with the supplied randomness: the relayer is not being paid", slot, fee)
	}
	return nil
}
