package tests

import (
	"math/big"
	"slices"
	"testing"

	"github.com/iden3/go-iden3-crypto/poseidon"
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

// A pkSpend outside the field is the wallet's own fault, not any event's: the
// scan must fail rather than report every note addressed to it as a bad event.
func TestScanners_FailOnAWalletKeyOutsideTheField(t *testing.T) {
	w := newWallet(t)
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
}

func TestWithRejectedVariants_ReportABadCapsule(t *testing.T) {
	w := newWallet(t)
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
}

// An event lacking a value the scan reads (a nil commitment, or an ERC1155
// event's nil contract address) can be built by hand but never decoded from a
// chain log. Each scanner reports it as RejectMissingValue, rather than
// panicking, and still finds the good note after it.
func TestScanners_RejectAnEventMissingAValue(t *testing.T) {
	w := newWallet(t)
	want := []core.RejectedEvent{{Index: 0, Reason: core.RejectMissingValue}}

	t.Run("ERC20 commitment", func(t *testing.T) {
		bad := goodErc20Event(t, w, big.NewInt(10), big.NewInt(6))
		bad.Commitment = nil
		events := []core.OnChainErc20Event{bad, goodErc20Event(t, w, big.NewInt(10), big.NewInt(5))}
		owned, rejected, err := core.ScanForErc20NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, events)
		if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 || !slices.Equal(rejected, want) {
			t.Fatalf("owned = %+v, rejected = %+v, err = %v; want the note of 5 and %+v", owned, rejected, err, want)
		}
		// Every wallet's scan reports it, not only the one its payload opens for.
		other := newWallet(t)
		if _, rejected, err := core.ScanForErc20NotesWithRejected(other.view.DecapsKey, other.spend.PublicKey, events[:1]); err != nil || !slices.Equal(rejected, want) {
			t.Fatalf("another wallet's scan: rejected = %+v, err = %v; want %+v", rejected, err, want)
		}
	})
	t.Run("ERC721 commitment", func(t *testing.T) {
		bad := goodErc721Event(t, w, big.NewInt(6))
		bad.Commitment = nil
		owned, rejected, err := core.ScanForErc721NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey,
			[]core.OnChainErc721Event{bad, goodErc721Event(t, w, big.NewInt(5))})
		if err != nil || len(owned) != 1 || owned[0].TokenId.Cmp(big.NewInt(5)) != 0 || !slices.Equal(rejected, want) {
			t.Fatalf("owned = %+v, rejected = %+v, err = %v; want token 5 and %+v", owned, rejected, err, want)
		}
	})
	for _, c := range []struct {
		name  string
		unset func(*core.OnChainErc1155Event)
	}{
		{"ERC1155 commitment", func(ev *core.OnChainErc1155Event) { ev.Commitment = nil }},
		{"ERC1155 contract address", func(ev *core.OnChainErc1155Event) { ev.ContractAddress = nil }},
	} {
		t.Run(c.name, func(t *testing.T) {
			bad := goodErc1155Event(t, w, big.NewInt(6))
			c.unset(&bad)
			owned, rejected, err := core.ScanForErc1155NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey,
				[]core.OnChainErc1155Event{bad, goodErc1155Event(t, w, big.NewInt(5))})
			if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 || !slices.Equal(rejected, want) {
				t.Fatalf("owned = %+v, rejected = %+v, err = %v; want the note of 5 and %+v", owned, rejected, err, want)
			}
		})
	}
}

// A change note, published as a client publishes a withdrawal's (a capsule to
// the owner's own view key, the change sealed under it), opens for the owner
// but is not the capsule's note: its salt is Poseidon(sk, nullifier0), as the
// Payment-family circuits require of a withdrawal's change. The owner's scan
// reports it as RejectCommitmentMismatch, as scan.go documents, and
// WithdrawChangeNote rebuilds it from the first nullifier and the change its
// payload opens to.
func TestScanForErc20NotesWithRejected_ReportsAChangeNoteAsAMismatch(t *testing.T) {
	owner := newWallet(t)
	token, change := big.NewInt(0), big.NewInt(35)
	sk, pk := owner.spend.PrivateKey, owner.spend.PublicKey
	nf0, err := core.GetNullifierWithTree(sk, big.NewInt(0), big.NewInt(3), 8)
	if err != nil {
		t.Fatalf("GetNullifierWithTree: %v", err)
	}
	salt, err := poseidon.Hash([]*big.Int{sk, nf0})
	if err != nil {
		t.Fatalf("Poseidon(sk, nf0): %v", err)
	}
	commitment, err := poseidon.Hash([]*big.Int{pk, salt, change, token})
	if err != nil {
		t.Fatalf("change commitment: %v", err)
	}
	capsule, payload, _ := seal(t, owner, token, change)
	ev := core.OnChainErc20Event{Commitment: commitment, CipherText: capsule, EncTxData: payload}

	owned, rejected, err := core.ScanForErc20NotesWithRejected(owner.view.DecapsKey, pk, []core.OnChainErc20Event{ev})
	want := []core.RejectedEvent{{Index: 0, Reason: core.RejectCommitmentMismatch}}
	if err != nil || len(owned) != 0 || !slices.Equal(rejected, want) {
		t.Fatalf("owned = %+v, rejected = %+v, err = %v; want no note and %+v", owned, rejected, err, want)
	}

	ss, err := core.Decapsulate(owner.view.DecapsKey, capsule)
	if err != nil {
		t.Fatalf("Decapsulate: %v", err)
	}
	key, err := core.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	gotToken, gotChange, err := core.DecryptPayload(key, payload)
	if err != nil {
		t.Fatalf("the owner's view key does not open the change payload: %v", err)
	}
	_, rebuilt, err := core.WithdrawChangeNote(sk, nf0, gotChange, gotToken)
	if err != nil || rebuilt.Cmp(commitment) != 0 {
		t.Fatalf("WithdrawChangeNote rebuilt %v (err %v), want the change commitment %s", rebuilt, err, commitment)
	}
}
