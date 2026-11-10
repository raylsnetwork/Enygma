package core

// On-chain trade agreement (Idea 1): Alice publishes an encrypted offer as a
// channel setup addressed to Bob, Bob answers with a private tag, and both
// derive the swap salts from the channel's shared secret. This replaces the
// off-chain step in which the parties agreed on the terms and Alice handed Bob
// the ML-KEM capsule and salts.
//
//	Alice: (ss, c1) = ML-KEM.Encaps(pkView_B)
//	       c2       = AES-GCM(k, Offer, aad = "dvp-offer-v1" ‖ c1)
//	       openOffer(c1, c2, bitmap)
//	Bob:   ss = ML-KEM.Decaps(skView_B, c1); decrypt c2 → Offer
//	       t    = Poseidon(block, pkSpend_A, ss mod p)
//	       ctxt = AES-GCM(k, Accept, aad = "dvp-reply-v1" ‖ pkSpend_A ‖ c1)
//	       publishReply(window of t, ctxt)
//	Both:  salt_B, salt_A = DeriveAgreementSwapSalts(ss, offerId)
//
// with k = HKDF(ss, "enygma-dvp-channel-key-v1").
//
// The circuits do not constrain how salts are chosen, so the existing
// DvPInitiatorProofFromSalts / DvPDestinationProof work unchanged with the
// salts derived here.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand/v2"

	"github.com/iden3/go-iden3-crypto/poseidon"
)

const (
	agreementChannelKeyInfo = "enygma-dvp-channel-key-v1"
	agreementOfferAAD       = "dvp-offer-v1"
	agreementReplyAAD       = "dvp-reply-v1"
	agreementSwapInfo       = "dvp-swap"

	// AgreementC1Size is the ML-KEM-768 ciphertext size the registry enforces.
	AgreementC1Size = 1088

	agreementVersion = 1
	offerEncodedSize = 1 + 32*9 // version + 9 fixed 32-byte fields
)

// ── Offer ─────────────────────────────────────────────────────────────────────

// AgreementOffer is the trade the initiator (Alice) proposes. "Give" is what
// she delivers to the counterparty, "Take" is what she receives in return.
type AgreementOffer struct {
	OfferId [32]byte // random; binds replies and swap salts to this offer

	GiveVaultId *big.Int
	GiveTokenId *big.Int
	GiveAmount  *big.Int

	TakeVaultId *big.Int
	TakeTokenId *big.Int
	TakeAmount  *big.Int

	// Deadline is the exact swap deadline (unix seconds) the initiator will
	// pass to submitPartialSettlement.
	Deadline *big.Int

	// InitiatorPkSpend lets the counterparty recompute COMMIT_A and address
	// its reply tag to the initiator.
	InitiatorPkSpend *big.Int
}

// NewOfferId returns a fresh random offer identifier.
func NewOfferId() ([32]byte, error) {
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return id, fmt.Errorf("offer id: %w", err)
	}
	return id, nil
}

func (o *AgreementOffer) fields() []*big.Int {
	return []*big.Int{
		o.GiveVaultId, o.GiveTokenId, o.GiveAmount,
		o.TakeVaultId, o.TakeTokenId, o.TakeAmount,
		o.Deadline, o.InitiatorPkSpend,
	}
}

// Encode serialises the offer as version ‖ offerId ‖ 8 × 32-byte big-endian fields.
func (o *AgreementOffer) Encode() ([]byte, error) {
	out := make([]byte, 0, offerEncodedSize)
	out = append(out, agreementVersion)
	out = append(out, o.OfferId[:]...)
	for i, f := range o.fields() {
		if f == nil || f.Sign() < 0 || f.BitLen() > 256 {
			return nil, fmt.Errorf("offer field %d is not a uint256", i)
		}
		out = append(out, f.FillBytes(make([]byte, 32))...)
	}
	return out, nil
}

// DecodeAgreementOffer parses the output of AgreementOffer.Encode.
func DecodeAgreementOffer(b []byte) (*AgreementOffer, error) {
	if len(b) != offerEncodedSize {
		return nil, fmt.Errorf("offer: want %d bytes, got %d", offerEncodedSize, len(b))
	}
	if b[0] != agreementVersion {
		return nil, fmt.Errorf("offer: unsupported version %d", b[0])
	}
	o := &AgreementOffer{}
	copy(o.OfferId[:], b[1:33])
	vals := make([]*big.Int, 8)
	for i := range vals {
		off := 33 + 32*i
		vals[i] = new(big.Int).SetBytes(b[off : off+32])
	}
	o.GiveVaultId, o.GiveTokenId, o.GiveAmount = vals[0], vals[1], vals[2]
	o.TakeVaultId, o.TakeTokenId, o.TakeAmount = vals[3], vals[4], vals[5]
	o.Deadline, o.InitiatorPkSpend = vals[6], vals[7]
	return o, nil
}

// ── Reply ─────────────────────────────────────────────────────────────────────

// ReplyKind is the counterparty's answer to an offer.
type ReplyKind uint8

const (
	ReplyAccept  ReplyKind = 1
	ReplyReject  ReplyKind = 2
	ReplyCounter ReplyKind = 3 // Counter carries new terms under a new OfferId
)

// AgreementReply answers the offer identified by OfferId. Counter is set only
// for ReplyCounter and is written from the initiator's point of view (Give is
// still what the initiator delivers), so the initiator can accept it as is.
type AgreementReply struct {
	Kind    ReplyKind
	OfferId [32]byte
	Counter *AgreementOffer
}

// Encode serialises the reply as version ‖ kind ‖ offerId [‖ counter offer].
func (r *AgreementReply) Encode() ([]byte, error) {
	out := []byte{agreementVersion, byte(r.Kind)}
	out = append(out, r.OfferId[:]...)
	switch r.Kind {
	case ReplyAccept, ReplyReject:
		if r.Counter != nil {
			return nil, errors.New("reply: only a counter-offer carries terms")
		}
	case ReplyCounter:
		if r.Counter == nil {
			return nil, errors.New("reply: counter-offer without terms")
		}
		enc, err := r.Counter.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, enc...)
	default:
		return nil, fmt.Errorf("reply: unknown kind %d", r.Kind)
	}
	return out, nil
}

// DecodeAgreementReply parses the output of AgreementReply.Encode.
func DecodeAgreementReply(b []byte) (*AgreementReply, error) {
	if len(b) < 34 {
		return nil, fmt.Errorf("reply: too short (%d bytes)", len(b))
	}
	if b[0] != agreementVersion {
		return nil, fmt.Errorf("reply: unsupported version %d", b[0])
	}
	r := &AgreementReply{Kind: ReplyKind(b[1])}
	copy(r.OfferId[:], b[2:34])
	rest := b[34:]
	switch r.Kind {
	case ReplyAccept, ReplyReject:
		if len(rest) != 0 {
			return nil, errors.New("reply: unexpected trailing bytes")
		}
	case ReplyCounter:
		o, err := DecodeAgreementOffer(rest)
		if err != nil {
			return nil, fmt.Errorf("reply: counter-offer: %w", err)
		}
		r.Counter = o
	default:
		return nil, fmt.Errorf("reply: unknown kind %d", r.Kind)
	}
	return r, nil
}

// ── Key derivation ────────────────────────────────────────────────────────────

// DeriveAgreementKey derives the channel's AES-256-GCM key from the ML-KEM
// shared secret: k = HKDF-SHA256(ss, "enygma-dvp-channel-key-v1").
func DeriveAgreementKey(ss []byte) ([]byte, error) {
	return hkdfDerive(ss, agreementChannelKeyInfo, 32)
}

// DeriveAgreementTag derives the private tag addressed to recipientPkSpend for
// the given block: t = Poseidon(blockNumber, recipientPkSpend, ss mod p).
// It matches the retail private-tag construction.
func DeriveAgreementTag(blockNumber uint64, recipientPkSpend *big.Int, ss []byte) ([32]byte, error) {
	var tag [32]byte
	ssField := SaltBToField(ss)
	h, err := poseidon.Hash([]*big.Int{new(big.Int).SetUint64(blockNumber), recipientPkSpend, ssField})
	if err != nil {
		return tag, fmt.Errorf("agreement tag: %w", err)
	}
	h.FillBytes(tag[:])
	return tag, nil
}

// DeriveAgreementTagWindow derives tags for blocks startBlock .. startBlock+size-1,
// the window DvpAgreementRegistry.publishReply picks from.
func DeriveAgreementTagWindow(startBlock uint64, size int, recipientPkSpend *big.Int, ss []byte) ([][32]byte, error) {
	tags := make([][32]byte, size)
	for i := range tags {
		t, err := DeriveAgreementTag(startBlock+uint64(i), recipientPkSpend, ss)
		if err != nil {
			return nil, err
		}
		tags[i] = t
	}
	return tags, nil
}

// DeriveAgreementSwapSalts derives the two output-commitment salts of the swap
// agreed under offerId:
//
//	swapSecret = HKDF(ss, "dvp-swap" ‖ offerId)
//	salt_B     = HKDF(swapSecret, "Bob salt")    → COMMIT_B (counterparty receives Give)
//	salt_A     = HKDF(swapSecret, "Alice salt")  → COMMIT_A (initiator receives Take)
//
// Both are reduced to field elements. Including offerId gives every offer its
// own salts, so a channel can later be reused for several swaps.
func DeriveAgreementSwapSalts(ss []byte, offerId [32]byte) (saltB, saltA *big.Int, err error) {
	swapSecret, err := hkdfDerive(ss, agreementSwapInfo+string(offerId[:]), 32)
	if err != nil {
		return nil, nil, err
	}
	b, err := hkdfDerive(swapSecret, "Bob salt", 32)
	if err != nil {
		return nil, nil, err
	}
	a, err := hkdfDerive(swapSecret, "Alice salt", 32)
	if err != nil {
		return nil, nil, err
	}
	return SaltBToField(b), SaltBToField(a), nil
}

// AgreementCommitments returns (COMMIT_B, COMMIT_A) for an agreed offer:
// COMMIT_B gives the counterparty the initiator's asset and COMMIT_A gives
// the initiator the counterparty's asset.
func AgreementCommitments(ss []byte, offer *AgreementOffer, counterpartyPkSpend *big.Int) (commitB, commitA *big.Int, err error) {
	saltB, saltA, err := DeriveAgreementSwapSalts(ss, offer.OfferId)
	if err != nil {
		return nil, nil, err
	}
	commitB, err = Erc20CommitmentV2(counterpartyPkSpend, saltB, offer.GiveAmount, offer.GiveTokenId)
	if err != nil {
		return nil, nil, err
	}
	commitA, err = Erc20CommitmentV2(offer.InitiatorPkSpend, saltA, offer.TakeAmount, offer.TakeTokenId)
	if err != nil {
		return nil, nil, err
	}
	return commitB, commitA, nil
}

// ── Offer / reply encryption ──────────────────────────────────────────────────

// PrepareAgreementOffer encapsulates to the counterparty's view key and
// encrypts the offer. It returns the shared secret (keep it: it is the
// channel) and the c1, c2 blobs for DvpAgreementRegistry.openOffer.
func PrepareAgreementOffer(counterpartyPkView []byte, offer *AgreementOffer) (ss, c1, c2 []byte, err error) {
	ss, c1, err = Encapsulate(counterpartyPkView)
	if err != nil {
		return nil, nil, nil, err
	}
	pt, err := offer.Encode()
	if err != nil {
		return nil, nil, nil, err
	}
	k, err := DeriveAgreementKey(ss)
	if err != nil {
		return nil, nil, nil, err
	}
	c2, err = sealGCM(k, pt, offerAAD(c1))
	if err != nil {
		return nil, nil, nil, err
	}
	return ss, c1, c2, nil
}

// OpenAgreementOffer tries to open an offer with the recipient's view key.
// ok is false when the offer is addressed to someone else (ML-KEM implicit
// rejection makes the AES-GCM check fail).
func OpenAgreementOffer(dk interface {
	Decapsulate([]byte) ([]byte, error)
}, c1, c2 []byte) (offer *AgreementOffer, ss []byte, ok bool) {
	if len(c1) != AgreementC1Size {
		return nil, nil, false
	}
	ss, err := dk.Decapsulate(c1)
	if err != nil {
		return nil, nil, false
	}
	k, err := DeriveAgreementKey(ss)
	if err != nil {
		return nil, nil, false
	}
	pt, err := openGCM(k, c2, offerAAD(c1))
	if err != nil {
		return nil, nil, false
	}
	offer, err = DecodeAgreementOffer(pt)
	if err != nil {
		return nil, nil, false
	}
	return offer, ss, true
}

// EncryptAgreementReply encrypts a reply for recipientPkSpend on the channel
// opened by c1. The recipient's key and c1 are bound as associated data, so a
// reply cannot be reflected to the other party or moved to another channel.
func EncryptAgreementReply(ss, c1 []byte, recipientPkSpend *big.Int, reply *AgreementReply) ([]byte, error) {
	pt, err := reply.Encode()
	if err != nil {
		return nil, err
	}
	k, err := DeriveAgreementKey(ss)
	if err != nil {
		return nil, err
	}
	return sealGCM(k, pt, replyAAD(recipientPkSpend, c1))
}

// DecryptAgreementReply decrypts a reply addressed to recipientPkSpend.
func DecryptAgreementReply(ss, c1 []byte, recipientPkSpend *big.Int, ctxt []byte) (*AgreementReply, error) {
	k, err := DeriveAgreementKey(ss)
	if err != nil {
		return nil, err
	}
	pt, err := openGCM(k, ctxt, replyAAD(recipientPkSpend, c1))
	if err != nil {
		return nil, err
	}
	return DecodeAgreementReply(pt)
}

func offerAAD(c1 []byte) []byte {
	return append([]byte(agreementOfferAAD), c1...)
}

func replyAAD(recipientPkSpend *big.Int, c1 []byte) []byte {
	aad := append([]byte(agreementReplyAAD), recipientPkSpend.FillBytes(make([]byte, 32))...)
	return append(aad, c1...)
}

// sealGCM encrypts with AES-256-GCM; output is nonce ‖ ciphertext ‖ tag.
func sealGCM(key, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

func openGCM(key, data, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], aad)
}

// ── Anonymity set ─────────────────────────────────────────────────────────────

// BuildAnonymityBitmap returns a bitmap over UserRegistry indices with the
// recipient's bit set plus setSize-1 random decoys. setSize <= 1 discloses the
// recipient; setSize >= totalUsers sets every bit (full privacy).
// Bit i is byte i/8, bit i%8 (least significant first).
func BuildAnonymityBitmap(totalUsers, recipientIdx, setSize int) ([]byte, error) {
	if totalUsers <= 0 || recipientIdx < 0 || recipientIdx >= totalUsers {
		return nil, fmt.Errorf("recipient index %d outside 0..%d", recipientIdx, totalUsers-1)
	}
	bm := make([]byte, (totalUsers+7)/8)
	set := func(i int) { bm[i/8] |= 1 << (i % 8) }
	if setSize >= totalUsers {
		for i := 0; i < totalUsers; i++ {
			set(i)
		}
		return bm, nil
	}
	set(recipientIdx)
	// Decoys are public information, so a non-cryptographic shuffle is fine.
	others := make([]int, 0, totalUsers-1)
	for i := 0; i < totalUsers; i++ {
		if i != recipientIdx {
			others = append(others, i)
		}
	}
	mrand.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	for i := 0; i < setSize-1 && i < len(others); i++ {
		set(others[i])
	}
	return bm, nil
}

// BitmapHasIndex reports whether user index idx is in the anonymity set.
func BitmapHasIndex(bitmap []byte, idx int) bool {
	if idx < 0 || idx/8 >= len(bitmap) {
		return false
	}
	return bitmap[idx/8]&(1<<(idx%8)) != 0
}
