package tests

// 20_v2_dvp_counter_vault_test.go
//
// Each DvP proof names the vault its prover expects to be paid from
// (StCounterVault, the statement's last element), and EnygmaDvp checks it at
// settlement. The commitments do not name a vault, so before this a
// counterparty could pay from another vault in the same asset group — e.g. the
// Enygma ERC-20 vault instead of the plain ERC-20 vault, which share the
// fungible group — and the swap still settled.
//
// Uses the direct path (submitPartialSettlement), whose second leg settles via
// the same _settleOnGroupPair as swap()/exchange() and SwapRelayer.
//
// Prerequisites: fresh Hardhat node + deploy + init, gnark server on :8081.
//
//	cd test && CC=/usr/bin/clang go test -run TestV2DvP_CounterVaultBinding -v -timeout 600s

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestV2DvP_CounterVaultBinding(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	tc := newDvpTestContext(t)
	defer tc.client.Close()
	receipts := loadOnchainReceipts(t)
	d := &directSwap{
		t: t, tc: tc,
		abi: loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"),
		dvp: common.HexToAddress(receipts["EnygmaDvp"].ContractAddress),
	}
	// The other vault in the fungible group.
	enygmaVault := common.HexToAddress(receipts["EnygmaErc20CoinVault"].ContractAddress)
	zero := big.NewInt(0)

	// Both legs come from the plain ERC-20 vault; one side expected to be paid
	// from the Enygma ERC-20 vault, so the settlement must be refused.
	cases := []struct {
		name                     string
		aliceExpects, bobExpects common.Address
	}{
		{"Alice expected another vault", enygmaVault, tc.erc20VaultAddr},
		{"Bob expected another vault", tc.erc20VaultAddr, enygmaVault},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d.t = t
			alice, bob := exchangeLegs(t, tc, c.aliceExpects, c.bobExpects)
			d.mustSubmit("Alice's leg", "submitPartialSettlement", alice, zero, zero, d.deadlineIn(600))
			if e, _ := d.submit("submitPartialSettlement", bob, zero, zero, zero); e != "CounterVaultMismatch" {
				t.Fatalf("Bob's leg from an unexpected vault: got %q, want CounterVaultMismatch", e)
			}
			for who, rc := range map[string]onchainProofReceipt{"Alice": alice, "Bob": bob} {
				if _, spent := d.noteState(tc.erc20Vault, rc); spent {
					t.Fatalf("%s's note spent although the swap was refused", who)
				}
			}
			t.Log("settlement refused with CounterVaultMismatch; no note spent")
		})
	}

	t.Run("matching vaults settle", func(t *testing.T) {
		d.t = t
		alice, bob := exchangeLegs(t, tc, tc.erc20VaultAddr, tc.erc20VaultAddr)
		d.mustSubmit("Alice's leg", "submitPartialSettlement", alice, zero, zero, d.deadlineIn(600))
		d.mustSubmit("Bob's leg", "submitPartialSettlement", bob, zero, zero, zero)
		for who, rc := range map[string]onchainProofReceipt{"Alice": alice, "Bob": bob} {
			if _, spent := d.noteState(tc.erc20Vault, rc); !spent {
				t.Fatalf("%s's note not spent after settlement", who)
			}
		}
		t.Log("settled")
	})
}
