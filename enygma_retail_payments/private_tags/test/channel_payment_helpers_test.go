package tags_test

import (
	"context"
	"math/big"
	"testing"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// The events a payment's receipt is checked for: a channel payment (#451)
// emits ChannelPayment, never the capsule-carrying Payment.
var (
	channelPaymentSig = crypto.Keccak256Hash([]byte("ChannelPayment(uint256,uint256,bytes32,uint256,bytes)"))
	paymentEventSig   = crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
)

// expectOneChannelPayment wants the receipt to hold exactly one ChannelPayment
// event, for block b and tag, and wantPayments Payment events: none, unless
// another leg settled in the same transaction (paymentWithUsdrFee's USDr leg).
func expectOneChannelPayment(t *testing.T, receipt *types.Receipt, b uint64, tag [32]byte, wantPayments int) {
	t.Helper()
	var channel, payment int
	for _, l := range receipt.Logs {
		switch l.Topics[0] {
		case channelPaymentSig:
			channel++
			if got := l.Topics[2].Big().Uint64(); got != b {
				t.Errorf("ChannelPayment event names block %d, want %d", got, b)
			}
			if l.Topics[3] != common.Hash(tag) {
				t.Errorf("ChannelPayment event names tag %x, want %x", l.Topics[3], tag)
			}
		case paymentEventSig:
			payment++
		}
	}
	if channel != 1 || payment != wantPayments {
		t.Errorf("got %d ChannelPayment and %d Payment events, want 1 and %d", channel, payment, wantPayments)
	}
}

// bobOpensChannelPayment is the payee's side of a channel payment: he scans
// EnygmaDvp at dvpAddr for his tag for block b over the channel secret ss,
// wants exactly one hit, opens its sealed note, and checks the opening
// against the commitment the event names and wantCm (the proof's C_dest).
func bobOpensChannelPayment(t *testing.T, client *ethclient.Client, dvpAddr common.Address, b uint64,
	ss []byte, pkSpend, wantAmount, wantToken, wantCm *big.Int) *tags.PaymentNote {
	t.Helper()
	found, err := tags.ScanChannelPayments(context.Background(), client, dvpAddr, b,
		[]tags.Channel{{SharedSecret: ss, PkSpend: pkSpend}})
	if err != nil {
		t.Fatalf("ScanChannelPayments (Bob): %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Bob expected 1 channel payment for block %d, found %d", b, len(found))
	}
	f := found[0]
	t.Logf("  Bob found his tag for block %d among the ChannelPayment events (settled in %d) ✓", f.BlockNumber, f.MinedIn)
	note, err := f.Open()
	if err != nil {
		t.Fatalf("open the sealed note: %v", err)
	}
	if note.Amount.Cmp(wantAmount) != 0 || note.TokenId.Cmp(wantToken) != 0 {
		t.Errorf("note opens to %s of token %s, want %s of %s", note.Amount, note.TokenId, wantAmount, wantToken)
	}
	if f.Commitment.Cmp(wantCm) != 0 {
		t.Errorf("event names commitment %s, want the proof's C_dest %s", f.Commitment, wantCm)
	}
	recomputed, err := rpcore.Erc20CommitmentV2(pkSpend, note.Salt, note.Amount, note.TokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err)
	}
	if recomputed.Cmp(wantCm) != 0 {
		t.Errorf("Bob's recomputed commitment %s, want %s", recomputed, wantCm)
	}
	t.Logf("  amount ✓  tokenId ✓  commitment %s ✓", recomputed)
	return note
}

// nextBlock is the block a transaction sent now is expected to land in, the
// block a channel payment's tag is for; it settles in that block or within
// tags.ChannelPaymentWindow after it.
func nextBlock(t *testing.T, client *ethclient.Client) uint64 {
	t.Helper()
	head, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	return head + 1
}
