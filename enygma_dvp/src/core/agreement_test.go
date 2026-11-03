package core

import (
	"math/big"
	"testing"
)

func testOffer(t *testing.T, pkA *big.Int) *AgreementOffer {
	t.Helper()
	id, err := NewOfferId()
	if err != nil {
		t.Fatal(err)
	}
	return &AgreementOffer{
		OfferId:     id,
		GiveVaultId: big.NewInt(0), GiveTokenId: big.NewInt(0), GiveAmount: big.NewInt(50),
		TakeVaultId: big.NewInt(1), TakeTokenId: big.NewInt(25), TakeAmount: big.NewInt(1),
		Deadline:         big.NewInt(1_900_000_000),
		InitiatorPkSpend: pkA,
	}
}

func mustKeys(t *testing.T) (*SpendKeyPair, *ViewKeyPair) {
	t.Helper()
	s, err := NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return s, v
}

// The full Idea 1 handshake, off-chain: Bob opens Alice's offer, replies,
// Alice reads the reply, and both derive the same commitments.
func TestAgreementHandshake(t *testing.T) {
	aliceSpend, _ := mustKeys(t)
	bobSpend, bobView := mustKeys(t)
	_, eveView := mustKeys(t)

	offer := testOffer(t, aliceSpend.PublicKey)
	ssA, c1, c2, err := PrepareAgreementOffer(bobView.EncapsKey, offer)
	if err != nil {
		t.Fatal(err)
	}
	if len(c1) != AgreementC1Size {
		t.Fatalf("c1 is %d bytes", len(c1))
	}

	if _, _, ok := OpenAgreementOffer(eveView.DecapsKey, c1, c2); ok {
		t.Fatal("an unrelated view key opened the offer")
	}
	got, ssB, ok := OpenAgreementOffer(bobView.DecapsKey, c1, c2)
	if !ok {
		t.Fatal("Bob could not open his offer")
	}
	if string(ssA) != string(ssB) {
		t.Fatal("shared secrets differ")
	}
	if got.OfferId != offer.OfferId || got.GiveAmount.Cmp(offer.GiveAmount) != 0 ||
		got.TakeTokenId.Cmp(offer.TakeTokenId) != 0 || got.Deadline.Cmp(offer.Deadline) != 0 ||
		got.InitiatorPkSpend.Cmp(aliceSpend.PublicKey) != 0 {
		t.Fatalf("decoded offer differs: %+v", got)
	}

	// Bob accepts, tagging the reply to Alice.
	tags, err := DeriveAgreementTagWindow(100, 3, got.InitiatorPkSpend, ssB)
	if err != nil {
		t.Fatal(err)
	}
	ctxt, err := EncryptAgreementReply(ssB, c1, got.InitiatorPkSpend, &AgreementReply{Kind: ReplyAccept, OfferId: got.OfferId})
	if err != nil {
		t.Fatal(err)
	}
	onchain := []OnChainReply{
		{BlockNumber: 101, Tag: [32]byte{1}, Ctxt: []byte("noise")},
		{BlockNumber: 101, Tag: tags[1], Ctxt: ctxt},
	}
	found, err := ScanAgreementReplies(aliceSpend.PublicKey, []AgreementChannel{{SharedSecret: ssA, C1: c1}}, onchain)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Reply.Kind != ReplyAccept || found[0].Reply.OfferId != offer.OfferId {
		t.Fatalf("Alice found %+v", found)
	}

	// A reply addressed to Alice is not found by Bob scanning for himself.
	found, _ = ScanAgreementReplies(bobSpend.PublicKey, []AgreementChannel{{SharedSecret: ssB, C1: c1}}, onchain)
	if len(found) != 0 {
		t.Fatal("Bob matched a reply addressed to Alice")
	}

	// Both sides derive the same swap commitments.
	cbA, caA, err := AgreementCommitments(ssA, offer, bobSpend.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	cbB, caB, err := AgreementCommitments(ssB, got, bobSpend.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if cbA.Cmp(cbB) != 0 || caA.Cmp(caB) != 0 {
		t.Fatal("parties derived different commitments")
	}

	// A swap started with the agreed values passes Bob's check; one with a
	// changed deadline or a different amount does not.
	p := &PendingSwap{CommitB: cbA, CommitA: caA, Deadline: offer.Deadline}
	if err := CheckPendingSwapMatchesOffer(p, ssB, got, bobSpend.PublicKey); err != nil {
		t.Fatal(err)
	}
	p.Deadline = big.NewInt(1)
	if CheckPendingSwapMatchesOffer(p, ssB, got, bobSpend.PublicKey) == nil {
		t.Fatal("accepted a different deadline")
	}
	cheaper := *offer
	cheaper.GiveAmount = big.NewInt(49)
	cb49, _, _ := AgreementCommitments(ssA, &cheaper, bobSpend.PublicKey)
	p = &PendingSwap{CommitB: cb49, CommitA: caA, Deadline: offer.Deadline}
	if CheckPendingSwapMatchesOffer(p, ssB, got, bobSpend.PublicKey) == nil {
		t.Fatal("accepted a swap that delivers less than offered")
	}
}

// A reply is bound to its recipient and channel.
func TestAgreementReplyBinding(t *testing.T) {
	aliceSpend, _ := mustKeys(t)
	bobSpend, bobView := mustKeys(t)
	offer := testOffer(t, aliceSpend.PublicKey)
	ss, c1, _, err := PrepareAgreementOffer(bobView.EncapsKey, offer)
	if err != nil {
		t.Fatal(err)
	}
	ctxt, err := EncryptAgreementReply(ss, c1, aliceSpend.PublicKey, &AgreementReply{Kind: ReplyReject, OfferId: offer.OfferId})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptAgreementReply(ss, c1, bobSpend.PublicKey, ctxt); err == nil {
		t.Fatal("reply decrypted for the wrong recipient")
	}
	other := append([]byte{}, c1...)
	other[0] ^= 1
	if _, err := DecryptAgreementReply(ss, other, aliceSpend.PublicKey, ctxt); err == nil {
		t.Fatal("reply decrypted on the wrong channel")
	}
	r, err := DecryptAgreementReply(ss, c1, aliceSpend.PublicKey, ctxt)
	if err != nil || r.Kind != ReplyReject {
		t.Fatalf("reply: %+v, %v", r, err)
	}
}

func TestAgreementCounterOfferRoundTrip(t *testing.T) {
	aliceSpend, _ := mustKeys(t)
	orig := testOffer(t, aliceSpend.PublicKey)
	counter := testOffer(t, aliceSpend.PublicKey)
	counter.GiveAmount = big.NewInt(60)
	enc, err := (&AgreementReply{Kind: ReplyCounter, OfferId: orig.OfferId, Counter: counter}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeAgreementReply(enc)
	if err != nil {
		t.Fatal(err)
	}
	if r.OfferId != orig.OfferId || r.Counter.OfferId != counter.OfferId || r.Counter.GiveAmount.Int64() != 60 {
		t.Fatalf("counter-offer round trip: %+v", r)
	}
	if _, err := (&AgreementReply{Kind: ReplyCounter, OfferId: orig.OfferId}).Encode(); err == nil {
		t.Fatal("encoded a counter-offer without terms")
	}
	if _, err := DecodeAgreementReply(append(enc[:34:34], 0)); err == nil {
		t.Fatal("decoded a truncated counter-offer")
	}
}

// Different offers on the same channel get different salts.
func TestAgreementSaltsPerOffer(t *testing.T) {
	ss := make([]byte, 32)
	ss[0] = 7
	b1, a1, _ := DeriveAgreementSwapSalts(ss, [32]byte{1})
	b2, a2, _ := DeriveAgreementSwapSalts(ss, [32]byte{2})
	if b1.Cmp(b2) == 0 || a1.Cmp(a2) == 0 || b1.Cmp(a1) == 0 {
		t.Fatal("salts are not independent")
	}
	if b1.Cmp(SNARK_SCALAR_FIELD) >= 0 || a1.Cmp(SNARK_SCALAR_FIELD) >= 0 {
		t.Fatal("salt not reduced to the field")
	}
}

func TestBuildAnonymityBitmap(t *testing.T) {
	bm, err := BuildAnonymityBitmap(20, 13, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !BitmapHasIndex(bm, 13) || popcount(bm) != 1 {
		t.Fatal("disclosed mode must set only the recipient")
	}
	bm, _ = BuildAnonymityBitmap(20, 13, 5)
	if !BitmapHasIndex(bm, 13) || popcount(bm) != 5 {
		t.Fatalf("subset mode: %d bits", popcount(bm))
	}
	bm, _ = BuildAnonymityBitmap(20, 13, 20)
	if popcount(bm) != 20 {
		t.Fatal("full mode must set every user")
	}
	if _, err := BuildAnonymityBitmap(20, 20, 1); err == nil {
		t.Fatal("accepted an out-of-range recipient")
	}
}

func popcount(b []byte) int {
	n := 0
	for _, x := range b {
		for ; x != 0; x &= x - 1 {
			n++
		}
	}
	return n
}
