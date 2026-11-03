package core

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// agreementRegistryABI is the subset of DvpAgreementRegistry used here.
const agreementRegistryABI = `[
 {"type":"function","name":"openOffer","stateMutability":"nonpayable",
  "inputs":[{"name":"c1","type":"bytes"},{"name":"c2","type":"bytes"},{"name":"bitmap","type":"bytes"}],
  "outputs":[{"name":"offerIdx","type":"uint256"}]},
 {"type":"function","name":"publishReply","stateMutability":"nonpayable",
  "inputs":[{"name":"startBlock","type":"uint256"},{"name":"tags","type":"bytes32[]"},{"name":"ctxt","type":"bytes"}],
  "outputs":[{"name":"tag","type":"bytes32"}]},
 {"type":"event","name":"OfferOpened","anonymous":false,
  "inputs":[{"name":"offerIdx","type":"uint256","indexed":true},{"name":"c1","type":"bytes","indexed":false},
            {"name":"c2","type":"bytes","indexed":false},{"name":"bitmap","type":"bytes","indexed":false}]},
 {"type":"event","name":"ReplyPublished","anonymous":false,
  "inputs":[{"name":"blockNumber","type":"uint256","indexed":true},{"name":"tag","type":"bytes32","indexed":true},
            {"name":"ctxt","type":"bytes","indexed":false}]}
]`

// AgreementRegistryABI returns the parsed DvpAgreementRegistry ABI subset.
func AgreementRegistryABI() abi.ABI {
	a, err := abi.JSON(strings.NewReader(agreementRegistryABI))
	if err != nil {
		panic(err) // constant input
	}
	return a
}

var (
	offerOpenedSig    = crypto.Keccak256Hash([]byte("OfferOpened(uint256,bytes,bytes,bytes)"))
	replyPublishedSig = crypto.Keccak256Hash([]byte("ReplyPublished(uint256,bytes32,bytes)"))
	swapInitiatedSig  = crypto.Keccak256Hash([]byte("SwapInitiated(uint256,uint256,uint256,uint256)"))
)

// OnChainOffer is one OfferOpened event.
type OnChainOffer struct {
	OfferIdx    uint64
	BlockNumber uint64
	C1, C2      []byte
	Bitmap      []byte
}

// OnChainReply is one ReplyPublished event.
type OnChainReply struct {
	BlockNumber uint64
	Tag         [32]byte
	Ctxt        []byte
}

// PublishAgreementOffer calls openOffer directly from auth. Prefer the
// relayer (POST /relay/offer) in production so msg.sender is not the trader.
func PublishAgreementOffer(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts,
	registry common.Address, c1, c2, bitmap []byte) (*types.Receipt, error) {
	c := bind.NewBoundContract(registry, AgreementRegistryABI(), client, client, client)
	tx, err := c.Transact(auth, "openOffer", c1, c2, bitmap)
	if err != nil {
		return nil, fmt.Errorf("openOffer: %w", err)
	}
	return waitSuccess(ctx, client, tx, "openOffer")
}

// PublishAgreementReply calls publishReply directly from auth with a tag
// window starting at startBlock. Prefer the relayer (POST /relay/reply).
func PublishAgreementReply(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts,
	registry common.Address, startBlock uint64, tags [][32]byte, ctxt []byte) (*types.Receipt, error) {
	c := bind.NewBoundContract(registry, AgreementRegistryABI(), client, client, client)
	tx, err := c.Transact(auth, "publishReply", new(big.Int).SetUint64(startBlock), tags, ctxt)
	if err != nil {
		return nil, fmt.Errorf("publishReply: %w", err)
	}
	return waitSuccess(ctx, client, tx, "publishReply")
}

func waitSuccess(ctx context.Context, client *ethclient.Client, tx *types.Transaction, what string) (*types.Receipt, error) {
	r, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, fmt.Errorf("%s: wait mined: %w", what, err)
	}
	if r.Status != types.ReceiptStatusSuccessful {
		return r, fmt.Errorf("%s: transaction reverted", what)
	}
	return r, nil
}

// FetchAgreementOffers returns the OfferOpened events in [fromBlock, toBlock]
// (toBlock nil = latest).
func FetchAgreementOffers(ctx context.Context, client *ethclient.Client, registry common.Address,
	fromBlock uint64, toBlock *big.Int) ([]OnChainOffer, error) {
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock), ToBlock: toBlock,
		Addresses: []common.Address{registry},
		Topics:    [][]common.Hash{{offerOpenedSig}},
	})
	if err != nil {
		return nil, fmt.Errorf("filter OfferOpened: %w", err)
	}
	a := AgreementRegistryABI()
	out := make([]OnChainOffer, 0, len(logs))
	for _, l := range logs {
		vals, err := a.Unpack("OfferOpened", l.Data)
		if err != nil || len(vals) != 3 || len(l.Topics) < 2 {
			continue // malformed log: skip rather than abort the scan
		}
		out = append(out, OnChainOffer{
			OfferIdx:    l.Topics[1].Big().Uint64(),
			BlockNumber: l.BlockNumber,
			C1:          vals[0].([]byte),
			C2:          vals[1].([]byte),
			Bitmap:      vals[2].([]byte),
		})
	}
	return out, nil
}

// FetchAgreementReplies returns the ReplyPublished events in [fromBlock, toBlock].
func FetchAgreementReplies(ctx context.Context, client *ethclient.Client, registry common.Address,
	fromBlock uint64, toBlock *big.Int) ([]OnChainReply, error) {
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock), ToBlock: toBlock,
		Addresses: []common.Address{registry},
		Topics:    [][]common.Hash{{replyPublishedSig}},
	})
	if err != nil {
		return nil, fmt.Errorf("filter ReplyPublished: %w", err)
	}
	a := AgreementRegistryABI()
	out := make([]OnChainReply, 0, len(logs))
	for _, l := range logs {
		vals, err := a.Unpack("ReplyPublished", l.Data)
		if err != nil || len(vals) != 1 || len(l.Topics) < 3 {
			continue
		}
		out = append(out, OnChainReply{
			BlockNumber: l.Topics[1].Big().Uint64(),
			Tag:         l.Topics[2],
			Ctxt:        vals[0].([]byte),
		})
	}
	return out, nil
}

// FoundAgreementOffer is an offer addressed to the scanning user.
type FoundAgreementOffer struct {
	OfferIdx     uint64
	BlockNumber  uint64
	Offer        *AgreementOffer
	SharedSecret []byte // the channel secret; keep it for replies and salts
	C1           []byte // the channel id for reply encryption
}

// ScanAgreementOffers returns the offers addressed to the holder of dk.
// myUserIdx is the scanner's UserRegistry index; offers whose anonymity set
// excludes it are skipped without decapsulating (pass -1 to try all).
func ScanAgreementOffers(dk interface {
	Decapsulate([]byte) ([]byte, error)
}, myUserIdx int, offers []OnChainOffer) []FoundAgreementOffer {
	var found []FoundAgreementOffer
	for _, o := range offers {
		if myUserIdx >= 0 && !BitmapHasIndex(o.Bitmap, myUserIdx) {
			continue
		}
		offer, ss, ok := OpenAgreementOffer(dk, o.C1, o.C2)
		if !ok {
			continue
		}
		found = append(found, FoundAgreementOffer{
			OfferIdx: o.OfferIdx, BlockNumber: o.BlockNumber,
			Offer: offer, SharedSecret: ss, C1: o.C1,
		})
	}
	return found
}

// AgreementChannel is an open channel the scanner is waiting on replies for.
type AgreementChannel struct {
	SharedSecret []byte
	C1           []byte
}

// FoundAgreementReply is a reply addressed to the scanning user.
type FoundAgreementReply struct {
	BlockNumber uint64
	Channel     int // index into the channels slice passed to ScanAgreementReplies
	Reply       *AgreementReply
}

// ScanAgreementReplies finds replies addressed to myPkSpend on any of the
// given channels: for each event it recomputes Poseidon(block, myPkSpend, ss)
// per channel and decrypts only on a tag match.
func ScanAgreementReplies(myPkSpend *big.Int, channels []AgreementChannel, replies []OnChainReply) ([]FoundAgreementReply, error) {
	type key struct {
		block uint64
		ch    int
	}
	expected := map[key][32]byte{}
	var found []FoundAgreementReply
	for _, r := range replies {
		for ci, ch := range channels {
			k := key{r.BlockNumber, ci}
			t, ok := expected[k]
			if !ok {
				var err error
				t, err = DeriveAgreementTag(r.BlockNumber, myPkSpend, ch.SharedSecret)
				if err != nil {
					return nil, err
				}
				expected[k] = t
			}
			if t != r.Tag {
				continue
			}
			reply, err := DecryptAgreementReply(ch.SharedSecret, ch.C1, myPkSpend, r.Ctxt)
			if err != nil {
				continue // tag collision or tampered ciphertext
			}
			found = append(found, FoundAgreementReply{BlockNumber: r.BlockNumber, Channel: ci, Reply: reply})
		}
	}
	return found, nil
}

// PendingSwap is the first leg of a swap as recorded by EnygmaDvp.
type PendingSwap struct {
	CommitB     *big.Int
	CommitA     *big.Int
	Deadline    *big.Int
	BlockNumber uint64
}

// FindPendingSwap looks up the SwapInitiated event for commitB (the contract
// keys pending swaps by COMMIT_B). It returns nil when Alice has not started
// the swap yet.
func FindPendingSwap(ctx context.Context, client *ethclient.Client, dvp common.Address,
	commitB *big.Int, fromBlock uint64) (*PendingSwap, error) {
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		Addresses: []common.Address{dvp},
		Topics:    [][]common.Hash{{swapInitiatedSig}, {common.BigToHash(commitB)}},
	})
	if err != nil {
		return nil, fmt.Errorf("filter SwapInitiated: %w", err)
	}
	if len(logs) == 0 {
		return nil, nil
	}
	l := logs[len(logs)-1]
	if len(l.Topics) < 4 || len(l.Data) != 32 {
		return nil, fmt.Errorf("malformed SwapInitiated log")
	}
	return &PendingSwap{
		CommitB:     l.Topics[3].Big(),
		CommitA:     l.Topics[2].Big(),
		Deadline:    new(big.Int).SetBytes(l.Data),
		BlockNumber: l.BlockNumber,
	}, nil
}

// CheckPendingSwapMatchesOffer verifies that the swap Alice started is the one
// Bob accepted: the commitments must be the ones derived from the channel
// secret and the offer terms, and the deadline must be the offered one.
func CheckPendingSwapMatchesOffer(p *PendingSwap, ss []byte, offer *AgreementOffer, myPkSpend *big.Int) error {
	commitB, commitA, err := AgreementCommitments(ss, offer, myPkSpend)
	if err != nil {
		return err
	}
	if p.CommitB.Cmp(commitB) != 0 {
		return fmt.Errorf("pending swap COMMIT_B %s does not match the offer (%s)", p.CommitB, commitB)
	}
	if p.CommitA.Cmp(commitA) != 0 {
		return fmt.Errorf("pending swap COMMIT_A %s does not match the offer (%s)", p.CommitA, commitA)
	}
	if p.Deadline.Cmp(offer.Deadline) != 0 {
		return fmt.Errorf("pending swap deadline %s differs from the offered %s", p.Deadline, offer.Deadline)
	}
	return nil
}
