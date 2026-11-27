package engine

import (
	"math/big"
	"testing"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// viewKeyNote is the EncryptedNote event of a note of amount and token for a,
// sealed to a's view key as depositNote seals one.
func viewKeyNote(t *testing.T, a *Actor, token, amount int64) dvpcore.OnChainErc20Event {
	t.Helper()
	ss, capsule, err := rpcore.Encapsulate(a.View.EncapsKey)
	if err != nil {
		t.Fatal(err)
	}
	saltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatal(err)
	}
	encKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := rpcore.Erc20CommitmentV2(a.Spend.PublicKey, rpcore.SaltBToField(saltB), big.NewInt(amount), big.NewInt(token))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := rpcore.EncryptPayload(encKey, big.NewInt(token), big.NewInt(amount))
	if err != nil {
		t.Fatal(err)
	}
	return dvpcore.OnChainErc20Event{Commitment: commitment, CipherText: capsule, EncTxData: sealed}
}

// Every spend from a purse proves with the vaults' token id, tokenId. A note of
// another id, sealed to a user, would count in their balance and fail any
// spend that picked it, so neither the user's scan nor the auditor's keeps it.
func TestScansKeepOnlyTheVaultsTokenId(t *testing.T) {
	spend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	view, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bob := &Actor{Name: "Bob", Spend: spend, View: view}
	other, own := viewKeyNote(t, bob, 7, 9), viewKeyNote(t, bob, tokenId.Int64(), 5)

	found := bob.scan([]dvpcore.OnChainErc20Event{other, own}, &bob.notes, assetErc20)
	if len(found) != 1 || len(bob.notes) != 1 || bob.notes[0].commitment.Cmp(own.Commitment) != 0 {
		t.Fatalf("scan found %d notes, purse holds %d; want only the token-%s note", len(found), len(bob.notes), tokenId)
	}
	if got := bob.notes.balance(); got.Cmp(big.NewInt(5)) != 0 {
		t.Errorf("balance %s, want 5", got)
	}

	e := &Engine{auditUsers: []auditUser{{name: bob.Name, spendPk: spend.PublicKey, view: view.DecapsKey}}}
	if owner, _ := e.auditOpen(other); owner != "" {
		t.Errorf("the auditor opened the token-7 note for %s", owner)
	}
	if owner, owned := e.auditOpen(own); owner != bob.Name || owned.Amount.Cmp(big.NewInt(5)) != 0 {
		t.Errorf("the auditor opened Bob's note for %q, amount %v; want Bob, 5", owner, owned.Amount)
	}
}
