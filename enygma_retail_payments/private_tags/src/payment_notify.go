package tags

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

// PaymentNote holds the note details for an incoming payment delivered via a
// private tag notification.
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

// PreparePaymentTag creates a tag window and encrypted note payload ready for
// submission to POST /relay/tag (window mode).
//
// This function bridges the ZK payment layer and the private tag notification
// layer. Alice calls it after generating a ZK payment proof to notify Bob that
// a payment has been sent to him.
//
// The note payload is encoded as amount(32 BE) || tokenId(32 BE) || saltB(32 BE)
// and encrypted with the channel key derived from the pre-established channel
// shared secret (from Phase 2 channel setup — separate from the per-payment
// ML-KEM shared secret).
//
// Parameters:
//   - channelSS: shared secret from the Phase 2 channel Alice established with Bob
//   - saltB: the saltB field element from PaymentResult.SaltB — the salt used in
//     Bob's output commitment, derived by Alice during ML-KEM encapsulation
//
// Returns (startBlock, tags, ctxt) ready for the relayer window-mode request.
func PreparePaymentTag(
	client *ethclient.Client,
	windowSize int,
	recipientPkSpend *big.Int,
	channelSS []byte,
	amount, tokenId, saltB *big.Int,
) (startBlock uint64, tagWindow [][32]byte, ctxt []byte, err error) {
	channelKey := DeriveChannelKey(channelSS)

	payload := make([]byte, 96)
	amount.FillBytes(payload[0:32])
	tokenId.FillBytes(payload[32:64])
	saltB.FillBytes(payload[64:96])

	return PrepareTagWithWindow(client, windowSize, recipientPkSpend, channelSS, channelKey, payload)
}

// DecryptPaymentNote decrypts a tag ctxt produced by PreparePaymentTag.
// Returns the PaymentNote containing amount, tokenId, and saltB.
//
// Bob verifies his commitment with:
//
//	Poseidon4(bobPkSpend, note.Salt, note.Amount, note.TokenId) == destinationCommitment
func DecryptPaymentNote(channelSS []byte, ctxt []byte) (*PaymentNote, error) {
	channelKey := DeriveChannelKey(channelSS)
	plaintext, err := DecryptPayload(channelKey, ctxt)
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
