package tags

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// A channel payment (#451) is one authorised payload
// p = (t, C_dest, C_chg, ctxt, nf, π), settled by EnygmaDvp.channelPayment:
// the payment proof binds the tag t, the note ciphertext ctxt and the block
// block_n the tag is for, so nobody can settle the payment without them.
// It uses the tag channel's secret ss for everything a per-payment ML-KEM
// capsule did before: the payee finds the payment by its tag and opens the
// note from ctxt, under the channel's block key.

// ChannelPaymentWindow is how many blocks after block_n a channel payment may
// still land: EnygmaDvp.channelPayment settles it only in blocks
// block_n … block_n + ChannelPaymentWindow (its CHANNEL_PAYMENT_WINDOW).
const ChannelPaymentWindow = 30

// channelTagDomain is the domain constant of the channel payment tag: the
// ASCII bytes of "enygma-tag-v1" as a big-endian field element. It keeps the
// tag apart from every other Poseidon hash over the channel secret.
var channelTagDomain = new(big.Int).SetBytes([]byte("enygma-tag-v1"))

// ChannelTag is the tag of a channel payment to the payee whose spend key is
// recipientPkSpend, for block blockNumber:
//
//	t = Poseidon("enygma-tag-v1", ss mod p, blockNumber, recipientPkSpend)
//
// The recipient's key makes it differ by direction, so A→B and B→A on one
// channel in one block get different tags.
func ChannelTag(sharedSecret []byte, blockNumber uint64, recipientPkSpend *big.Int) ([32]byte, error) {
	ssField := new(big.Int).Mod(new(big.Int).SetBytes(sharedSecret), bn254ScalarField)
	h, err := poseidon.Hash([]*big.Int{channelTagDomain, ssField, new(big.Int).SetUint64(blockNumber), recipientPkSpend})
	if err != nil {
		return [32]byte{}, fmt.Errorf("poseidon channel tag: %w", err)
	}
	var t [32]byte
	h.FillBytes(t[:])
	return t, nil
}

// ChannelPayment is what the payer publishes for a channel payment besides
// its proof: the tag, the block it is for and the sealed note, plus the salt
// the payer puts in the payee's commitment.
type ChannelPayment struct {
	BlockNumber uint64   // block_n, the block the tag and the note's key are for
	Tag         [32]byte // ChannelTag(ss, block_n, payee)
	Ctxt        []byte   // AES-256-GCM(k_b, amount ‖ tokenId ‖ salt), nonce first
	Salt        *big.Int // the payee's note salt, random in the field; private
}

// PrepareChannelPayment draws the payee's salt at random in the field and
// seals the note's opening, amount ‖ tokenId ‖ salt as three 32-byte words,
// under the block key DeriveBlockKey(ss, blockNumber, payee) with a fresh
// random nonce. The payer then proves the payment with salt as the payee's
// note salt (BoundChannelPaymentProof), which binds BlockNumber, Tag and Ctxt.
// The payee opens Ctxt with DecryptPaymentNote.
func PrepareChannelPayment(sharedSecret []byte, blockNumber uint64, recipientPkSpend, amount, tokenId *big.Int) (*ChannelPayment, error) {
	tag, err := ChannelTag(sharedSecret, blockNumber, recipientPkSpend)
	if err != nil {
		return nil, err
	}
	salt, err := randomNonzeroField()
	if err != nil {
		return nil, fmt.Errorf("channel payment salt: %w", err)
	}
	payload := make([]byte, 96)
	amount.FillBytes(payload[0:32])
	tokenId.FillBytes(payload[32:64])
	salt.FillBytes(payload[64:96])
	ctxt, err := EncryptPayload(DeriveBlockKey(sharedSecret, blockNumber, recipientPkSpend), payload)
	if err != nil {
		return nil, fmt.Errorf("seal channel payment note: %w", err)
	}
	return &ChannelPayment{BlockNumber: blockNumber, Tag: tag, Ctxt: ctxt, Salt: salt}, nil
}

// randomNonzeroField draws a uniform element of the BN254 scalar field other
// than 0.
func randomNonzeroField() (*big.Int, error) {
	for {
		n, err := rand.Int(rand.Reader, bn254ScalarField)
		if err != nil {
			return nil, err
		}
		if n.Sign() != 0 {
			return n, nil
		}
	}
}

// channelPaymentTopic and channelPaymentData decode EnygmaDvp's
// ChannelPayment(uint256 indexed vaultId, uint256 indexed blockNumber,
// bytes32 indexed tag, uint256 commitment, bytes ctxt).
var (
	channelPaymentTopic = crypto.Keccak256Hash([]byte("ChannelPayment(uint256,uint256,bytes32,uint256,bytes)"))
	channelPaymentData  = func() abi.Arguments {
		u256, _ := abi.NewType("uint256", "", nil)
		bs, _ := abi.NewType("bytes", "", nil)
		return abi.Arguments{{Type: u256}, {Type: bs}}
	}()
)

// FoundChannelPayment is a channel payment a scan matched to one of the
// payee's channels.
type FoundChannelPayment struct {
	BlockNumber  uint64      // block_n, the block its tag is for
	MinedIn      uint64      // the block it settled in, block_n … block_n + ChannelPaymentWindow
	TxHash       common.Hash //
	VaultId      *big.Int    //
	Commitment   *big.Int    // C_dest, the payee's note commitment
	Tag          [32]byte    //
	Ctxt         []byte      // the sealed note
	SharedSecret []byte      // the channel secret whose tag matched
	PkSpend      *big.Int    // the payee's spend key
}

// Open decrypts the payment's note under its block key in the payee's
// direction and checks that its opening hashes to the published commitment:
//
//	Poseidon(pkSpend, salt, amount, tokenId) == C_dest
func (f FoundChannelPayment) Open() (*PaymentNote, error) {
	note, err := DecryptPaymentNote(f.SharedSecret, f.PkSpend, f.BlockNumber, f.Ctxt)
	if err != nil {
		return nil, err
	}
	c, err := poseidon.Hash([]*big.Int{f.PkSpend, note.Salt, note.Amount, note.TokenId})
	if err != nil {
		return nil, fmt.Errorf("poseidon commitment: %w", err)
	}
	if c.Cmp(f.Commitment) != 0 {
		return nil, fmt.Errorf("the note opens to commitment %s, the payment published %s", c, f.Commitment)
	}
	return note, nil
}

// ScanChannelPayments finds the channel payments tagged for blockNumber to the
// payee of any of channels: it computes each channel's tag for that block and
// looks those tags up among the ChannelPayment events EnygmaDvp at dvpAddr
// emitted for it, in blocks blockNumber … blockNumber + ChannelPaymentWindow
// (up to the chain's head). No ciphertext is tried: only a matching tag is a
// payment to the payee. Payments come back in the order they settled; two
// payments one payer sends one payee for the same block share a tag, and both
// are returned. It sees only the blocks mined so far: a scan before
// blockNumber + ChannelPaymentWindow misses a payment that lands later, so
// scan each block_n again once its window has passed.
func ScanChannelPayments(ctx context.Context, client *ethclient.Client, dvpAddr common.Address, blockNumber uint64, channels []Channel) ([]FoundChannelPayment, error) {
	if len(channels) == 0 {
		return nil, nil
	}
	head, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("block number: %w", err)
	}
	if head < blockNumber {
		return nil, nil
	}
	to := min(head, blockNumber+ChannelPaymentWindow)

	byTag := make(map[common.Hash]Channel, len(channels))
	topics := make([]common.Hash, 0, len(channels))
	for _, ch := range channels {
		t, err := ChannelTag(ch.SharedSecret, blockNumber, ch.PkSpend)
		if err != nil {
			return nil, err
		}
		byTag[common.Hash(t)] = ch
		topics = append(topics, common.Hash(t))
	}

	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(blockNumber),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{dvpAddr},
		Topics:    [][]common.Hash{{channelPaymentTopic}, nil, {common.BigToHash(new(big.Int).SetUint64(blockNumber))}, topics},
	})
	if err != nil {
		return nil, fmt.Errorf("ChannelPayment logs for block %d: %w", blockNumber, err)
	}

	var found []FoundChannelPayment
	// The filter matches only ChannelPayment logs, whose four topics end
	// with one of these tags.
	for _, l := range logs {
		ch := byTag[l.Topics[3]]
		vals, err := channelPaymentData.Unpack(l.Data)
		if err != nil {
			return nil, fmt.Errorf("decode ChannelPayment in tx %s: %w", l.TxHash.Hex(), err)
		}
		found = append(found, FoundChannelPayment{
			BlockNumber:  blockNumber,
			MinedIn:      l.BlockNumber,
			TxHash:       l.TxHash,
			VaultId:      l.Topics[1].Big(),
			Commitment:   vals[0].(*big.Int),
			Tag:          l.Topics[3],
			Ctxt:         vals[1].([]byte),
			SharedSecret: ch.SharedSecret,
			PkSpend:      ch.PkSpend,
		})
	}
	return found, nil
}
