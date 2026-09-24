package tests

// Can a DvP swap's first leg be settled on its own, bypassing the swap?
//
// Erc20CoinVault.transfer() is public and only checks the proof, the roots and
// the nullifiers. A DvP Initiator receipt (1 input, numberOfOutputs = 1, VK
// slot 23) goes through the same checkReceiptConditions dispatch, so if
// transfer() accepts it, anyone holding Alice's receipt can spend her input
// note and insert commitB (the note that pays Bob) without Bob delivering
// anything. The receipt is public from the moment Alice broadcasts
// submitPartialSettlement, and the counterparty has it off-chain.
//
// A failing sub-test message starts with "VULNERABLE:".
//
// Prerequisites are the same as 13_v2_dvp_frontrun_test.go.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP_StandaloneInitiatorLeg -v -timeout 900s

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

const (
	standaloneLegToken      = int64(115)
	standaloneDeliveryToken = int64(116)
)

func TestV2DvP_StandaloneInitiatorLeg(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	tc := newDvpTestContext(t)
	defer tc.client.Close()
	attacker := attackerAuth(t, tc)

	d := dvpSetupDeposits(t, tc, standaloneLegToken)
	p := dvpGenerateProofs(t, tc, d)

	// The attacker holds Alice's receipt and calls the vault directly instead
	// of going through EnygmaDvp.submitPartialSettlement.
	tx, err := tc.erc20Vault.Transact(attacker, "transfer", p.aliceReceipt)
	if err != nil {
		t.Logf("SAFE: vault.transfer rejected Alice's initiator receipt: %v", err)
		return
	}
	mined, err := bind.WaitMined(tc.ctx, tc.client, tx)
	if err != nil {
		t.Fatalf("wait vault.transfer: %v", err)
	}
	if mined.Status != 1 {
		t.Log("SAFE: vault.transfer reverted on Alice's initiator receipt")
		return
	}

	insertedB := false
	for _, log := range mined.Logs {
		if log.Topics[0] == tc.commitmentSig && len(log.Topics) >= 3 && log.Topics[2].Big().Cmp(p.commitB) == 0 {
			insertedB = true
		}
	}
	t.Errorf("VULNERABLE: vault.transfer() settled Alice's DvP initiator receipt on its own (commitB inserted=%v): "+
		"her payment leg executed with no delivery from Bob", insertedB)

	// Alice's honest submission is now dead: her nullifier is already spent.
	deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
	status, subErr := submitLeg(t, tc, tc.auth, p.aliceReceipt, 0, 0, deadline)
	if subErr == nil && status == 1 {
		t.Log("  (Alice's own submitPartialSettlement still succeeded)")
	} else {
		t.Logf("  Alice's own submitPartialSettlement now fails (status=%d err=%v)", status, subErr)
	}
}

// The mirror case: Bob's DvP Destination receipt (his delivery leg) run alone
// through Erc721CoinVault.transfer(). If it is accepted, Alice, who needs Bob's
// receipt to see the swap through, can force Bob's ticket out of his hands
// without paying.
func TestV2DvP_StandaloneDeliveryLeg(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	tc := newDvpTestContext(t)
	defer tc.client.Close()
	attacker := attackerAuth(t, tc)

	d := dvpSetupDeposits(t, tc, standaloneDeliveryToken)
	p := dvpGenerateProofs(t, tc, d)

	tx, err := tc.erc721Vault.Transact(attacker, "transfer", p.bobReceipt)
	if err != nil {
		t.Logf("SAFE: erc721Vault.transfer rejected Bob's delivery receipt: %v", err)
		return
	}
	mined, err := bind.WaitMined(tc.ctx, tc.client, tx)
	if err != nil {
		t.Fatalf("wait erc721Vault.transfer: %v", err)
	}
	if mined.Status != 1 {
		t.Log("SAFE: erc721Vault.transfer reverted on Bob's delivery receipt")
		return
	}
	insertedA := false
	for _, log := range mined.Logs {
		if log.Topics[0] == tc.commitmentSig && len(log.Topics) >= 3 && log.Topics[2].Big().Cmp(p.commitA) == 0 {
			insertedA = true
		}
	}
	t.Errorf("VULNERABLE: erc721Vault.transfer() settled Bob's DvP delivery receipt on its own (commitA inserted=%v): "+
		"his ticket was delivered with no payment from Alice", insertedA)
}
