package tests

// TestAuditRegistrations registers three fresh accounts whose audit
// ciphertexts UserRegistry.register accepts alike, as it checks only their
// lengths, and has the auditor check every registration in the registry
// (#434): one opens to the view key its user registered, one is well-sized
// junk and does not open, and one opens to a view key other than the
// registered one.
//
// Run (on the shield_demo/run.sh stack, any chain):
//
//	cd test && CC=/usr/bin/clang go test -run TestAuditRegistrations -v -timeout 120s

import (
	"bytes"
	"context"
	"crypto/mlkem"
	"math/big"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestAuditRegistrations(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not available — run: npx hardhat node")
	}
	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()
	registryAddr := common.HexToAddress(loadOnchainReceipts(t)["UserRegistry"].ContractAddress)

	auditor, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	// register registers a fresh, funded account with a fresh spend key, the
	// view key pkView and the audit ciphertexts mlKemCt and aesCt.
	register := func(pkView, mlKemCt, aesCt []byte) common.Address {
		t.Helper()
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		fundAccount(t, ctx, client, addr)
		auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		spend, err := rpcore.NewSpendKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		if err := rpcore.Register(client, auth, registryAddr, spend.PublicKey, pkView, mlKemCt, aesCt); err != nil {
			t.Fatalf("register %s: %v", addr.Hex(), err)
		}
		return addr
	}
	encrypt := func(view *mlkem.DecapsulationKey768) (mlKemCt, aesCt []byte) {
		t.Helper()
		mlKemCt, aesCt, err := rpcore.EncryptViewKeyForAuditor(auditor.EncapsKey, view)
		if err != nil {
			t.Fatal(err)
		}
		return mlKemCt, aesCt
	}

	honestView, otherView := newTestViewKey(t), newTestViewKey(t)
	honestKem, honestAes := encrypt(honestView)
	honest := register(honestView.EncapsulationKey().Bytes(), honestKem, honestAes)
	junk := register(newTestViewKey(t).EncapsulationKey().Bytes(), bytes.Repeat([]byte{0xab}, 1088), bytes.Repeat([]byte{0xcd}, 92))
	otherKem, otherAes := encrypt(otherView)
	mismatch := register(newTestViewKey(t).EncapsulationKey().Bytes(), otherKem, otherAes)

	regs, err := rpcore.AuditRegistrations(client, registryAddr, auditor.DecapsKey)
	if err != nil {
		t.Fatalf("AuditRegistrations: %v", err)
	}
	count, err := rpcore.GetUserCount(client, registryAddr)
	if err != nil {
		t.Fatal(err)
	}
	if len(regs) != count {
		t.Fatalf("%d registrations audited, the registry holds %d", len(regs), count)
	}
	got := map[common.Address]rpcore.AuditedRegistration{}
	for _, r := range regs {
		got[r.User] = r
	}
	for _, tc := range []struct {
		name           string
		user           common.Address
		opens, matches bool
		view           *mlkem.DecapsulationKey768
	}{
		{"honest", honest, true, true, honestView},
		{"junk", junk, false, false, nil},
		{"another view key", mismatch, true, false, otherView},
	} {
		r, ok := got[tc.user]
		if !ok {
			t.Errorf("%s registration %s not audited", tc.name, tc.user.Hex())
			continue
		}
		if r.Opens() != tc.opens || r.Matches != tc.matches {
			t.Errorf("%s registration: opens %v, matches %v; want %v, %v", tc.name, r.Opens(), r.Matches, tc.opens, tc.matches)
			continue
		}
		if tc.view != nil && !bytes.Equal(r.ViewKey.Bytes(), tc.view.Bytes()) {
			t.Errorf("%s registration opened to a view key other than the one encrypted", tc.name)
		}
	}
}

func newTestViewKey(t *testing.T) *mlkem.DecapsulationKey768 {
	t.Helper()
	k, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}
	return k
}
