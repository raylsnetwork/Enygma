package tests

import (
	"math/big"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
)

// TestScanForErc20Notes_FullFlow exercises the complete Alice→Bob flow:
//
//  1. Alice holds a note (sk_spendA, saltA, amount=5, tokenId=10).
//  2. Alice sends 5 USDT to Bob non-interactively:
//     a. Encapsulate(pk_viewB) → saltB, cipherText
//     b. EncryptPayload(saltB, tokenId, amount) → encTxData
//     c. Commitment = Poseidon(pk_spendB, saltB_field, amount, tokenId)
//  3. Bob scans the event and recovers the note without prior interaction.
func TestScanForErc20Notes_FullFlow(t *testing.T) {
	tokenId := big.NewInt(10)
	amount := big.NewInt(5)

	// --- Bob generates his key pairs ---
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}

	// --- Alice builds the output note for Bob ---
	ss, cipherText, err := core.Encapsulate(bobView.EncapsKey)
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

	encTxData, err := core.EncryptPayload(encKey, tokenId, amount)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}

	saltBField := core.SaltBToField(saltB)
	commitment, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBField, amount, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}

	// --- Chain event (what gets posted on-chain) ---
	events := []core.OnChainErc20Event{
		{
			Commitment:  commitment,
			CipherText: cipherText,
			EncTxData: encTxData,
		},
	}

	// --- Bob scans ---
	owned, err := core.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, events)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}

	if len(owned) != 1 {
		t.Fatalf("expected 1 owned note, got %d", len(owned))
	}
	note := owned[0]

	if note.TokenId.Cmp(tokenId) != 0 {
		t.Errorf("TokenId: got %s, want %s", note.TokenId, tokenId)
	}
	if note.Amount.Cmp(amount) != 0 {
		t.Errorf("Amount: got %s, want %s", note.Amount, amount)
	}
	if note.Commitment.Cmp(commitment) != 0 {
		t.Errorf("Commitment: got %s, want %s", note.Commitment, commitment)
	}
	if note.SaltBField.Cmp(saltBField) != 0 {
		t.Errorf("SaltBField: got %s, want %s", note.SaltBField, saltBField)
	}
}

// TestScanForErc20Notes_IgnoresOtherRecipients verifies that notes addressed to
// a different view key are silently skipped.
func TestScanForErc20Notes_IgnoresOtherRecipients(t *testing.T) {
	// Alice's note is sent to Carol, not Bob.
	carolView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair carol: %v", err)
	}
	carolSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair carol: %v", err)
	}

	ss, cipherText, err := core.Encapsulate(carolView.EncapsKey)
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
	tokenId := big.NewInt(10)
	amount := big.NewInt(5)

	encTxData, err := core.EncryptPayload(encKey, tokenId, amount)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}
	saltBField := core.SaltBToField(saltB)
	commitment, err := core.Erc20CommitmentV2(carolSpend.PublicKey, saltBField, amount, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}

	events := []core.OnChainErc20Event{
		{Commitment: commitment, CipherText: cipherText, EncTxData: encTxData},
	}

	// Bob tries to scan — should find nothing.
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair bob: %v", err)
	}
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair bob: %v", err)
	}

	owned, err := core.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, events)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(owned) != 0 {
		t.Errorf("expected 0 owned notes for wrong view key, got %d", len(owned))
	}
}

// TestScanForErc20Notes_MultipleEvents verifies scanning across a mix of events:
// some addressed to Bob, some to others.
func TestScanForErc20Notes_MultipleEvents(t *testing.T) {
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair bob: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair bob: %v", err)
	}

	carolView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair carol: %v", err)
	}
	carolSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair carol: %v", err)
	}

	makeEvent := func(viewEncapKey []byte, spendPk *big.Int, tokenId, amount *big.Int) core.OnChainErc20Event {
		t.Helper()
		ss, ctI, err := core.Encapsulate(viewEncapKey)
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
		ctII, err := core.EncryptPayload(encKey, tokenId, amount)
		if err != nil {
			t.Fatalf("EncryptPayload: %v", err)
		}
		saltBField := core.SaltBToField(saltB)
		cmt, err := core.Erc20CommitmentV2(spendPk, saltBField, amount, tokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2: %v", err)
		}
		return core.OnChainErc20Event{Commitment: cmt, CipherText: ctI, EncTxData: ctII}
	}

	events := []core.OnChainErc20Event{
		makeEvent(bobView.EncapsKey, bobSpend.PublicKey, big.NewInt(10), big.NewInt(5)),   // Bob: 5 USDT
		makeEvent(carolView.EncapsKey, carolSpend.PublicKey, big.NewInt(10), big.NewInt(3)), // Carol: skip
		makeEvent(bobView.EncapsKey, bobSpend.PublicKey, big.NewInt(10), big.NewInt(7)),   // Bob: 7 USDT
	}

	owned, err := core.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, events)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(owned) != 2 {
		t.Fatalf("expected 2 owned notes, got %d", len(owned))
	}

	// Verify amounts (order preserved)
	expectedAmounts := []*big.Int{big.NewInt(5), big.NewInt(7)}
	for i, note := range owned {
		if note.Amount.Cmp(expectedAmounts[i]) != 0 {
			t.Errorf("note[%d] amount: got %s, want %s", i, note.Amount, expectedAmounts[i])
		}
	}
}

// TestScanForErc20Notes_SaltBFieldUsableAsWitness verifies that the SaltBField
// returned in OwnedErc20Note matches what Erc20CommitmentV2 expects when Bob
// later wants to spend the note.
func TestScanForErc20Notes_SaltBFieldUsableAsWitness(t *testing.T) {
	tokenId := big.NewInt(10)
	amount := big.NewInt(5)

	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}

	ss, cipherText, err := core.Encapsulate(bobView.EncapsKey)
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
	encTxData, err := core.EncryptPayload(encKey, tokenId, amount)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}
	saltBField := core.SaltBToField(saltB)
	commitment, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBField, amount, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}

	events := []core.OnChainErc20Event{
		{Commitment: commitment, CipherText: cipherText, EncTxData: encTxData},
	}

	owned, err := core.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, events)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(owned) != 1 {
		t.Fatalf("expected 1 note, got %d", len(owned))
	}

	// Bob can recompute the commitment from recovered data — this is what the
	// circuit verifies when he spends the note.
	note := owned[0]
	recomputed, err := core.Erc20CommitmentV2(bobSpend.PublicKey, note.SaltBField, note.Amount, note.TokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 recompute: %v", err)
	}
	if recomputed.Cmp(note.Commitment) != 0 {
		t.Errorf("recomputed commitment does not match: got %s, want %s", recomputed, note.Commitment)
	}
}
