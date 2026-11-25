package tags

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/iden3/go-iden3-crypto/poseidon"
)

// The channel payment tag is H("tag", ss, block_n, pk_recipient) (#451):
// Poseidon over the domain constant (the ASCII bytes of "enygma-tag-v1" as a
// big-endian field element), the channel secret reduced mod p, the block and
// the recipient's spend key, in that order.
func TestChannelTagIsTheDomainSeparatedHash(t *testing.T) {
	ss := bytes.Repeat([]byte{0xFF}, 32) // above p, so the reduction matters
	pk, block := big.NewInt(0xB0B), uint64(42)

	ssField := new(big.Int).Mod(new(big.Int).SetBytes(ss), bn254ScalarField)
	want, err := poseidon.Hash([]*big.Int{new(big.Int).SetBytes([]byte("enygma-tag-v1")), ssField, big.NewInt(42), big.NewInt(0xB0B)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ChannelTag(ss, block, pk)
	if err != nil {
		t.Fatalf("ChannelTag: %v", err)
	}
	if new(big.Int).SetBytes(got[:]).Cmp(want) != 0 {
		t.Fatalf("ChannelTag = %x, want Poseidon(\"enygma-tag-v1\", ss mod p, 42, pk) = %x", got, want)
	}
}

// A→B and B→A on one channel in one block get different tags, and no input
// can be dropped: each one changes the tag. The domain constant keeps it
// apart from the messaging tag over the same inputs.
func TestChannelTagSeparatesDirectionsAndDomains(t *testing.T) {
	ss := bytes.Repeat([]byte{0xAB}, 32)
	alice, bob, block := big.NewInt(0xA11CE), big.NewInt(0xB0B), uint64(7)

	toBob, _ := ChannelTag(ss, block, bob)
	toAlice, _ := ChannelTag(ss, block, alice)
	if toBob == toAlice {
		t.Error("A→B and B→A in one block share a tag")
	}
	if next, _ := ChannelTag(ss, block+1, bob); next == toBob {
		t.Error("the next block gives the same tag")
	}
	if other, _ := ChannelTag(bytes.Repeat([]byte{0xCD}, 32), block, bob); other == toBob {
		t.Error("another channel secret gives the same tag")
	}
	if msg, _ := DeriveTag(block, bob, ss); msg == toBob {
		t.Error("the channel payment tag equals the messaging tag over the same inputs")
	}
}

// PrepareChannelPayment draws a fresh random salt in the field, tags the
// payment for its block and seals amount ‖ tokenId ‖ salt under that block's
// key in the payee's direction: the payee opens it with DecryptPaymentNote,
// the payer's own direction does not, and no two payments share a salt or a
// nonce, even for one payee in one block.
func TestPrepareChannelPayment(t *testing.T) {
	ss := bytes.Repeat([]byte{0x5A}, 32)
	alice, bob, block := big.NewInt(0xA11CE), big.NewInt(0xB0B), uint64(100)
	amount, token := big.NewInt(60), big.NewInt(0)

	p1, err := PrepareChannelPayment(ss, block, bob, amount, token)
	if err != nil {
		t.Fatalf("PrepareChannelPayment: %v", err)
	}
	p2, err := PrepareChannelPayment(ss, block, bob, amount, token)
	if err != nil {
		t.Fatalf("PrepareChannelPayment: %v", err)
	}

	if p1.BlockNumber != block {
		t.Errorf("BlockNumber = %d, want %d", p1.BlockNumber, block)
	}
	if want, _ := ChannelTag(ss, block, bob); p1.Tag != want || p2.Tag != want {
		t.Error("the payment's tag is not ChannelTag(ss, block, payee)")
	}
	if p1.Salt.Sign() <= 0 || p1.Salt.Cmp(bn254ScalarField) >= 0 {
		t.Errorf("salt %s is not a nonzero field element", p1.Salt)
	}
	if p1.Salt.Cmp(p2.Salt) == 0 {
		t.Error("two payments drew the same salt")
	}
	if bytes.Equal(p1.Ctxt[:12], p2.Ctxt[:12]) {
		t.Error("two payments under one block key reused a nonce")
	}

	note, err := DecryptPaymentNote(ss, bob, block, p1.Ctxt)
	if err != nil {
		t.Fatalf("the payee cannot open ctxt: %v", err)
	}
	if note.Amount.Cmp(amount) != 0 || note.TokenId.Cmp(token) != 0 || note.Salt.Cmp(p1.Salt) != 0 {
		t.Errorf("ctxt opens to (%s, %s, %s), want (%s, %s, %s)", note.Amount, note.TokenId, note.Salt, amount, token, p1.Salt)
	}
	if _, err := DecryptPaymentNote(ss, alice, block, p1.Ctxt); err == nil {
		t.Error("the reverse direction's block key opens ctxt")
	}
	if _, err := DecryptPaymentNote(ss, bob, block+1, p1.Ctxt); err == nil {
		t.Error("the next block's key opens ctxt")
	}
}

// Open trusts a note only when its opening hashes to the commitment the
// payment published, Poseidon(pk_spend, salt, amount, tokenId): a ctxt that
// decrypts but names another note is refused.
func TestFoundChannelPaymentOpenChecksTheCommitment(t *testing.T) {
	ss, bob, block := bytes.Repeat([]byte{0x11}, 32), big.NewInt(0xB0B), uint64(9)
	p, err := PrepareChannelPayment(ss, block, bob, big.NewInt(25), big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	c, err := poseidon.Hash([]*big.Int{bob, p.Salt, big.NewInt(25), big.NewInt(3)})
	if err != nil {
		t.Fatal(err)
	}
	f := FoundChannelPayment{BlockNumber: block, Commitment: c, Tag: p.Tag, Ctxt: p.Ctxt, SharedSecret: ss, PkSpend: bob}
	if note, err := f.Open(); err != nil || note.Amount.Cmp(big.NewInt(25)) != 0 {
		t.Fatalf("Open = %v, %v; want the note of 25", note, err)
	}
	f.Commitment = new(big.Int).Add(c, big.NewInt(1))
	if _, err := f.Open(); err == nil {
		t.Fatal("Open accepted a note whose opening does not hash to the published commitment")
	}
}
