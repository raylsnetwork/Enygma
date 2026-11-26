package tags

import (
	"fmt"
	"math/big"
)

// PaymentNote holds the note details of an incoming payment, sealed in the
// ctxt of a channel payment (PrepareChannelPayment).
//
// Bob recomputes his commitment with:
//
//	commitment = Poseidon4(pkSpend, Salt, Amount, TokenId)
//
// and verifies it equals the destination commitment in the payment transaction.
type PaymentNote struct {
	Amount  *big.Int
	TokenId *big.Int
	Salt    *big.Int // saltB field element used in Bob's output commitment
}

// DecryptPaymentNote decrypts a payment's sealed note (PrepareChannelPayment's
// ctxt) for blockNumber, the block its tag is for, to the recipient whose
// spend key is recipientPkSpend.
// Returns the PaymentNote containing amount, tokenId, and saltB.
//
// Bob verifies his commitment with:
//
//	Poseidon4(bobPkSpend, note.Salt, note.Amount, note.TokenId) == destinationCommitment
func DecryptPaymentNote(channelSS []byte, recipientPkSpend *big.Int, blockNumber uint64, ctxt []byte) (*PaymentNote, error) {
	plaintext, err := DecryptPayload(DeriveBlockKey(channelSS, blockNumber, recipientPkSpend), ctxt)
	if err != nil {
		return nil, fmt.Errorf("decrypt payment note: %w", err)
	}
	if len(plaintext) != 96 {
		return nil, fmt.Errorf("unexpected plaintext length: got %d, want 96", len(plaintext))
	}
	return &PaymentNote{
		Amount:  new(big.Int).SetBytes(plaintext[0:32]),
		TokenId: new(big.Int).SetBytes(plaintext[32:64]),
		Salt:    new(big.Int).SetBytes(plaintext[64:96]),
	}, nil
}
