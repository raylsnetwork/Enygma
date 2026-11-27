package tests

import (
	"math/big"
	"slices"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
)

// An Erc20CoinVault's notes all carry its TOKEN_ID, 0. A note sealed to the
// wallet with another id opens and matches its commitment, but no withdrawal
// from the vault can name that id, and every note paid from it would carry
// it, so the vault's scan refuses it.
func TestScanForErc20NotesWithRejected_RefusesAnotherTokenId(t *testing.T) {
	w := newWallet(t)
	vaultToken := big.NewInt(0)
	events := []core.OnChainErc20Event{
		goodErc20Event(t, w, big.NewInt(7), big.NewInt(9)),
		goodErc20Event(t, w, vaultToken, big.NewInt(5)),
	}

	owned, rejected, err := core.ScanForErc20NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, vaultToken, events)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(owned) != 1 || owned[0].Amount.Cmp(big.NewInt(5)) != 0 || owned[0].TokenId.Sign() != 0 {
		t.Fatalf("owned = %+v, want exactly the token-0 note of 5", owned)
	}
	want := []core.RejectedEvent{{Index: 0, Reason: core.RejectWrongTokenId}}
	if !slices.Equal(rejected, want) {
		t.Fatalf("rejected = %+v, want %+v", rejected, want)
	}

	// ScanForErc20Notes keeps both: it filters by no token id.
	all, err := core.ScanForErc20Notes(w.view.DecapsKey, w.spend.PublicKey, events)
	if err != nil || len(all) != 2 {
		t.Fatalf("ScanForErc20Notes kept %d notes (err %v), want both", len(all), err)
	}
}

// The token id to keep is the wallet's own input: one that is no field
// element fails the scan rather than keeping notes of any id.
func TestScanForErc20NotesWithRejected_NeedsATokenId(t *testing.T) {
	w := newWallet(t)
	events := []core.OnChainErc20Event{goodErc20Event(t, w, big.NewInt(0), big.NewInt(5))}
	for name, tokenId := range map[string]*big.Int{"nil": nil, "the field modulus": core.SNARK_SCALAR_FIELD} {
		if owned, _, err := core.ScanForErc20NotesWithRejected(w.view.DecapsKey, w.spend.PublicKey, tokenId, events); err == nil {
			t.Errorf("token id %s: scan kept %d notes, want an error", name, len(owned))
		}
	}
}
