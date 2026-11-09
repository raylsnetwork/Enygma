package tests

import (
	"math/big"
	"slices"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
)

// wallet holds the view and spend keys a scan takes.
type wallet struct {
	view  *core.ViewKeyPair
	spend *core.SpendKeyPair
}

func newWallet(t *testing.T) wallet {
	t.Helper()
	view, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}
	spend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	return wallet{view: view, spend: spend}
}

// seal is a note payload sealed to a wallet's view key: the capsule, the
// sealed payload, and the note's salt as a field element.
func seal(t *testing.T, w wallet, first, second *big.Int) (capsule, payload []byte, salt *big.Int) {
	t.Helper()
	ss, capsule, err := core.Encapsulate(w.view.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	saltB, err := core.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("DerivePaymentSalt: %v", err)
	}
	encKey, err := core.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	payload, err = core.EncryptPayload(encKey, first, second)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}
	return capsule, payload, core.SaltBToField(saltB)
}

func goodErc20Event(t *testing.T, w wallet, token, amount *big.Int) core.OnChainErc20Event {
	t.Helper()
	capsule, payload, salt := seal(t, w, token, amount)
	commitment, err := core.Erc20CommitmentV2(w.spend.PublicKey, salt, amount, token)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}
	return core.OnChainErc20Event{Commitment: commitment, CipherText: capsule, EncTxData: payload}
}

// badErc20Batch is one good note of 5 at index 1 among three bad events: a
// short capsule, a payload that opens but doesn't match its commitment, and
// a sealed amount equal to the field modulus.
func badErc20Batch(t *testing.T, w wallet) []core.OnChainErc20Event {
	t.Helper()
	token := big.NewInt(10)

	good := goodErc20Event(t, w, token, big.NewInt(5))

	short := goodErc20Event(t, w, token, big.NewInt(6))
	short.CipherText = short.CipherText[:len(short.CipherText)-1]

	mismatched := goodErc20Event(t, w, token, big.NewInt(7))
	mismatched.Commitment = big.NewInt(1)

	capsule, payload, _ := seal(t, w, token, core.SNARK_SCALAR_FIELD)
	outOfField := core.OnChainErc20Event{Commitment: big.NewInt(1), CipherText: capsule, EncTxData: payload}

	return []core.OnChainErc20Event{short, good, mismatched, outOfField}
}

func TestScanForErc20Notes_ReturnsTheGoodNoteAmongBadEvents(t *testing.T) {
	w := newWallet(t)

	owned, err := core.ScanForErc20Notes(w.view.DecapsKey, w.spend.PublicKey, badErc20Batch(t, w))
	if err != nil {
		t.Fatalf("a bad event stopped the scan: %v", err)
	}
	if len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("owned = %+v, want exactly the note of 5", owned)
	}
}

func TestScanForErc20NotesWithRejected_NamesEachBadEventAndWhy(t *testing.T) {
	w := newWallet(t)

	owned, rejected, err := core.ScanForErc20NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, badErc20Batch(t, w))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("owned = %+v, want exactly the note of 5", owned)
	}
	want := []core.RejectedEvent{
		{Index: 0, Reason: core.RejectBadCapsule},
		{Index: 2, Reason: core.RejectCommitmentMismatch},
		{Index: 3, Reason: core.RejectCommitmentNotComputable},
	}
	if !slices.Equal(rejected, want) {
		t.Fatalf("rejected = %+v, want %+v", rejected, want)
	}
}

func TestScanForErc20NotesWithRejected_DoesNotReportSomeoneElsesNote(t *testing.T) {
	w, other := newWallet(t), newWallet(t)
	events := []core.OnChainErc20Event{goodErc20Event(t, other, big.NewInt(10), big.NewInt(5))}

	owned, rejected, err := core.ScanForErc20NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, events)
	if err != nil || len(owned) != 0 || len(rejected) != 0 {
		t.Fatalf("owned = %+v, rejected = %+v, err = %v; want nothing", owned, rejected, err)
	}
}

func goodErc721Event(t *testing.T, w wallet, tokenId *big.Int) core.OnChainErc721Event {
	t.Helper()
	capsule, payload, salt := seal(t, w, big.NewInt(99), tokenId)
	commitment, err := core.Erc721Commitment(tokenId, w.spend.PublicKey, salt)
	if err != nil {
		t.Fatalf("Erc721Commitment: %v", err)
	}
	return core.OnChainErc721Event{Commitment: commitment, CipherText: capsule, EncTxData: payload}
}

func TestScanForErc721Notes_ReturnsTheGoodNoteAmongBadEvents(t *testing.T) {
	w := newWallet(t)

	short := goodErc721Event(t, w, big.NewInt(6))
	short.CipherText = short.CipherText[:len(short.CipherText)-1]
	mismatched := goodErc721Event(t, w, big.NewInt(7))
	mismatched.Commitment = big.NewInt(1)
	capsule, payload, _ := seal(t, w, big.NewInt(99), core.SNARK_SCALAR_FIELD)
	outOfField := core.OnChainErc721Event{Commitment: big.NewInt(1), CipherText: capsule, EncTxData: payload}
	good := goodErc721Event(t, w, big.NewInt(5))

	owned, err := core.ScanForErc721Notes(w.view.DecapsKey, w.spend.PublicKey,
		[]core.OnChainErc721Event{short, mismatched, outOfField, good})
	if err != nil {
		t.Fatalf("a bad event stopped the scan: %v", err)
	}
	if len(owned) != 1 || owned[0].TokenId.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("owned = %+v, want exactly token 5", owned)
	}
}

func goodErc1155Event(t *testing.T, w wallet, amount *big.Int) core.OnChainErc1155Event {
	t.Helper()
	token := big.NewInt(10)
	capsule, payload, salt := seal(t, w, token, amount)
	commitment, err := core.Erc1155Commitment(token, amount, w.spend.PublicKey, salt)
	if err != nil {
		t.Fatalf("Erc1155Commitment: %v", err)
	}
	return core.OnChainErc1155Event{Commitment: commitment, ContractAddress: big.NewInt(99), CipherText: capsule, EncTxData: payload}
}

func TestScanForErc1155Notes_ReturnsTheGoodNoteAmongBadEvents(t *testing.T) {
	w := newWallet(t)

	short := goodErc1155Event(t, w, big.NewInt(6))
	short.CipherText = short.CipherText[:len(short.CipherText)-1]
	mismatched := goodErc1155Event(t, w, big.NewInt(7))
	mismatched.Commitment = big.NewInt(1)
	capsule, payload, _ := seal(t, w, big.NewInt(10), core.SNARK_SCALAR_FIELD)
	outOfField := core.OnChainErc1155Event{Commitment: big.NewInt(1), ContractAddress: big.NewInt(99), CipherText: capsule, EncTxData: payload}
	good := goodErc1155Event(t, w, big.NewInt(5))

	owned, err := core.ScanForErc1155Notes(w.view.DecapsKey, w.spend.PublicKey,
		[]core.OnChainErc1155Event{short, mismatched, outOfField, good})
	if err != nil {
		t.Fatalf("a bad event stopped the scan: %v", err)
	}
	if len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("owned = %+v, want exactly the note of 5", owned)
	}
}

// swapAmountIn and swapTokenIn are what Bob contributes to a swap: the values
// of CommitmentB.
var swapAmountIn, swapTokenIn = big.NewInt(3), big.NewInt(20)

func goodSwapEvent(t *testing.T, alice, bob wallet, amountOut *big.Int) core.OnChainZkDvpEvent {
	t.Helper()
	tokenOut := big.NewInt(10)
	saltStar := []byte("salt star of the initiator 32 by")
	ss, capsule, err := core.Encapsulate(bob.view.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	payload, err := core.EncryptSwapPayload(ss, tokenOut, amountOut, saltStar)
	if err != nil {
		t.Fatalf("EncryptSwapPayload: %v", err)
	}
	commitA, err := core.Erc20CommitmentV2(alice.spend.PublicKey, core.SaltBToField(saltStar), amountOut, tokenOut)
	if err != nil {
		// An out-of-field amountOut has no CommitmentA; the event publishes a stand-in.
		commitA = big.NewInt(1)
	}
	commitB, err := core.Erc20CommitmentV2(bob.spend.PublicKey, core.SaltBToField(ss), swapAmountIn, swapTokenIn)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 B: %v", err)
	}
	return core.OnChainZkDvpEvent{CommitmentA: commitA, CommitmentB: commitB, CipherText: capsule, EncTxData: payload}
}

func TestScanForZkDvpSwap_ReturnsTheGoodSwapAmongBadEvents(t *testing.T) {
	alice, bob := newWallet(t), newWallet(t)

	short := goodSwapEvent(t, alice, bob, big.NewInt(6))
	short.CipherText = short.CipherText[:len(short.CipherText)-1]
	mismatchedA := goodSwapEvent(t, alice, bob, big.NewInt(7))
	mismatchedA.CommitmentA = big.NewInt(1)
	mismatchedB := goodSwapEvent(t, alice, bob, big.NewInt(8))
	mismatchedB.CommitmentB = big.NewInt(1)
	outOfField := goodSwapEvent(t, alice, bob, core.SNARK_SCALAR_FIELD)
	good := goodSwapEvent(t, alice, bob, big.NewInt(5))

	swaps, err := core.ScanForZkDvpSwap(bob.view.DecapsKey, alice.spend.PublicKey, bob.spend.PublicKey,
		swapAmountIn, swapTokenIn, []core.OnChainZkDvpEvent{short, mismatchedA, mismatchedB, outOfField, good})
	if err != nil {
		t.Fatalf("a bad event stopped the scan: %v", err)
	}
	if len(swaps) != 1 || swaps[0].AmountOut.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("swaps = %+v, want exactly the swap of 5", swaps)
	}
}

// A pkSpend outside the field is the wallet's own fault, not any event's: the
// scan must fail rather than report every note addressed to it as a bad event.
func TestScanners_FailOnAWalletKeyOutsideTheField(t *testing.T) {
	w, alice := newWallet(t), newWallet(t)
	badKey := core.SNARK_SCALAR_FIELD

	if _, err := core.ScanForErc20Notes(w.view.DecapsKey, badKey,
		[]core.OnChainErc20Event{goodErc20Event(t, w, big.NewInt(10), big.NewInt(5))}); err == nil {
		t.Error("ScanForErc20Notes: no error for a pkSpend outside the field")
	}
	if _, err := core.ScanForErc721Notes(w.view.DecapsKey, badKey,
		[]core.OnChainErc721Event{goodErc721Event(t, w, big.NewInt(5))}); err == nil {
		t.Error("ScanForErc721Notes: no error for a pkSpend outside the field")
	}
	if _, err := core.ScanForErc1155Notes(w.view.DecapsKey, badKey,
		[]core.OnChainErc1155Event{goodErc1155Event(t, w, big.NewInt(5))}); err == nil {
		t.Error("ScanForErc1155Notes: no error for a pkSpend outside the field")
	}

	swap := []core.OnChainZkDvpEvent{goodSwapEvent(t, alice, w, big.NewInt(5))}
	if _, err := core.ScanForZkDvpSwap(w.view.DecapsKey, badKey, w.spend.PublicKey, swapAmountIn, swapTokenIn, swap); err == nil {
		t.Error("ScanForZkDvpSwap: no error for an aliceSpendPk outside the field")
	}
	if _, err := core.ScanForZkDvpSwap(w.view.DecapsKey, alice.spend.PublicKey, badKey, swapAmountIn, swapTokenIn, swap); err == nil {
		t.Error("ScanForZkDvpSwap: no error for a bobSpendPk outside the field")
	}
	if _, err := core.ScanForZkDvpSwap(w.view.DecapsKey, alice.spend.PublicKey, w.spend.PublicKey, core.SNARK_SCALAR_FIELD, swapTokenIn, swap); err == nil {
		t.Error("ScanForZkDvpSwap: no error for an amountIn outside the field")
	}
	if _, err := core.ScanForZkDvpSwap(w.view.DecapsKey, alice.spend.PublicKey, w.spend.PublicKey, swapAmountIn, core.SNARK_SCALAR_FIELD, swap); err == nil {
		t.Error("ScanForZkDvpSwap: no error for a tokenIdIn outside the field")
	}
}

func TestWithRejectedVariants_ReportABadCapsule(t *testing.T) {
	w, alice := newWallet(t), newWallet(t)
	want := []core.RejectedEvent{{Index: 0, Reason: core.RejectBadCapsule}}

	bad721 := goodErc721Event(t, w, big.NewInt(5))
	bad721.CipherText = nil
	if _, rejected, err := core.ScanForErc721NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, []core.OnChainErc721Event{bad721}); err != nil || !slices.Equal(rejected, want) {
		t.Errorf("ERC721: rejected = %+v, err = %v", rejected, err)
	}
	bad1155 := goodErc1155Event(t, w, big.NewInt(5))
	bad1155.CipherText = nil
	if _, rejected, err := core.ScanForErc1155NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, []core.OnChainErc1155Event{bad1155}); err != nil || !slices.Equal(rejected, want) {
		t.Errorf("ERC1155: rejected = %+v, err = %v", rejected, err)
	}
	badSwap := goodSwapEvent(t, alice, w, big.NewInt(5))
	badSwap.CipherText = nil
	if _, rejected, err := core.ScanForZkDvpSwapWithRejected(w.view.DecapsKey, alice.spend.PublicKey, w.spend.PublicKey, swapAmountIn, swapTokenIn, []core.OnChainZkDvpEvent{badSwap}); err != nil || !slices.Equal(rejected, want) {
		t.Errorf("ZkDvp: rejected = %+v, err = %v", rejected, err)
	}
}
