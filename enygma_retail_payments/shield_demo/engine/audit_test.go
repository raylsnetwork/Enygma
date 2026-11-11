package engine

import (
	"math/big"
	"testing"
)

// A walkthrough with the USDr fee on and tags off: no payer is named, so the
// auditor infers balances by conservation, and the Sender, the only payer,
// loses each relayer fee from the USDr they were granted (#250).
func TestConservationTakesTheRelayerFeesFromTheShielder(t *testing.T) {
	n := big.NewInt
	a := &Audit{
		Users:    []string{"Sender", "Bruno", "Carla"},
		Received: map[string]*big.Int{"Sender": n(100), "Bruno": n(30), "Carla": n(20)},
		Entries: []AuditEntry{
			{Kind: AuditShield, Asset: assetErc20, From: "Sender", To: "Sender", Amount: n(100)},
			{Kind: AuditUsdrGrant, Asset: assetUsdr, From: "Issuer", To: "Sender", Amount: n(20)},
			{Kind: AuditTransfer, Asset: assetErc20, To: "Bruno", Amount: n(30)},
			{Kind: AuditTransfer, Asset: assetErc20, To: "Carla", Amount: n(20)},
			{Kind: AuditUsdrFee, Asset: assetUsdr, To: "Relayer", Amount: n(1)},
			{Kind: AuditUsdrFee, Asset: assetUsdr, To: "Relayer", Amount: n(1)},
		},
	}
	a.conserve()

	if !a.Inferred {
		t.Fatal("balances not inferred")
	}
	want := map[string]struct{ erc20, usdr int64 }{"Sender": {50, 18}, "Bruno": {30, 0}, "Carla": {20, 0}}
	for u, w := range want {
		if got := a.Balances[u]; got.Cmp(n(w.erc20)) != 0 {
			t.Errorf("%s RaylsERC20 = %v, want %d", u, got, w.erc20)
		}
		if got := a.UsdrBalances[u]; got.Cmp(n(w.usdr)) != 0 {
			t.Errorf("%s USDr = %v, want %d", u, got, w.usdr)
		}
	}
}
