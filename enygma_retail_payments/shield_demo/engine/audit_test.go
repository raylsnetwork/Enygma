package engine

import (
	"crypto/mlkem"
	"math/big"
	"slices"
	"strings"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
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

// The auditor checks every registration's audit ciphertexts, which the
// contract accepts on their lengths alone (#434). A run's own user whose
// ciphertexts do not open, or open to a view key other than the registered
// one, stops the run; any other account's is named, not counted: the auditor
// cannot tell an earlier run's user, encrypted to another auditor, from an
// opt-out.
func TestTheAuditorNamesEveryRegistrationItCannotRead(t *testing.T) {
	sender, bob := common.HexToAddress("0x5e"), common.HexToAddress("0xb0")
	junk, mismatch := common.HexToAddress("0x01"), common.HexToAddress("0x02")
	e := &Engine{actors: []*Actor{{Name: "Issuer", Address: common.HexToAddress("0x15")}, {Name: "Sender", Address: sender}, {Name: "Bob", Address: bob}}}
	view := func() *mlkem.DecapsulationKey768 {
		k, err := mlkem.GenerateKey768()
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	reg := func(user common.Address, check rpcore.AuditCheck) rpcore.AuditedRegistration {
		return rpcore.AuditedRegistration{User: user, Keys: &rpcore.UserKeys{SpendKey: big.NewInt(1)}, AuditCheck: check}
	}
	readable := func(user common.Address) rpcore.AuditedRegistration {
		return reg(user, rpcore.AuditCheck{ViewKey: view(), Matches: true})
	}
	unopened := func(user common.Address) rpcore.AuditedRegistration { return reg(user, rpcore.AuditCheck{}) }
	mismatched := func(user common.Address) rpcore.AuditedRegistration {
		return reg(user, rpcore.AuditCheck{ViewKey: view()})
	}

	t.Run("other accounts' unreadable registrations are named", func(t *testing.T) {
		users, others, err := e.sortAuditChecks([]rpcore.AuditedRegistration{unopened(junk), readable(sender), mismatched(mismatch), readable(bob)})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, u := range users {
			names = append(names, u.name)
		}
		if !slices.Equal(names, []string{"Sender", "Bob"}) {
			t.Errorf("recovered the view keys of %v, want Sender and Bob", names)
		}
		if !slices.Equal(others.unopened, []string{junk.Hex()}) {
			t.Errorf("unopened %v, want %s", others.unopened, junk.Hex())
		}
		if !slices.Equal(others.mismatched, []string{mismatch.Hex()}) {
			t.Errorf("mismatched %v, want %s", others.mismatched, mismatch.Hex())
		}
	})

	for _, tc := range []struct {
		name string
		reg  rpcore.AuditedRegistration
		want string
	}{
		{"a run user's ciphertexts that do not open stop the run", unopened(bob), "Bob's audit ciphertexts do not open"},
		{"a run user's ciphertexts of another view key stop the run", mismatched(bob), "Bob registered a view key other than"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := e.sortAuditChecks([]rpcore.AuditedRegistration{readable(sender), tc.reg})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want one containing %q", err, tc.want)
			}
		})
	}
}
