package tests

// 24_v2_dvp_swap_key_overwrite_test.go
//
// A direct-path swap is keyed by commitB. Opening one checked neither that
// commitB was already pending nor that it was closed (the closed check read the
// leg's own message, commitA). The counterparty knows commitB's opening, so
// from another ERC-20 vault in the same asset group it could open a second leg
// with the same commitB, overwriting the pending record: the initiator's note
// stayed locked with no record left to time it out (claimSwapTimeout now
// refunds the second leg), and a closed swap could be reopened.
//
// Deploys and registers a second Erc20CoinVault for the attack, so run it on a
// fresh deployment like the rest of the suite.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP_SwapKey -v -timeout 900s

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	core "github.com/raylsnetwork/enygma_dvp/src/core"
)

const swapKeyOverwriteToken = int64(118)

func TestV2DvP_SwapKeyCannotBeOverwrittenOrReopened(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	tc := newDvpTestContext(t)
	defer tc.client.Close()
	receipts := loadOnchainReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)

	// ── A second ERC-20 vault in the fungible group (group 0) ────────────────
	vaultABI, vaultBin := loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	vault2Addr, tx, _, err := bind.DeployContract(tc.auth, vaultABI, vaultBin, tc.client, dvpAddr)
	waitOK(t, tc.client, "deploy second Erc20CoinVault", tx, err)
	tx, err = tc.dvp.Transact(tc.auth, "registerVault", vault2Addr, erc20Addr, big.NewInt(1), big.NewInt(int64(tc.merkleDepth)))
	waitOK(t, tc.client, "registerVault", tx, err)
	vault2Id := int64(-1)
	for i := int64(0); i < 32; i++ {
		var out []interface{}
		if err := tc.dvp.Call(&bind.CallOpts{}, &out, "vaultById", big.NewInt(i)); err != nil {
			t.Fatalf("vaultById(%d): %v", i, err)
		}
		if out[0].(common.Address) == vault2Addr {
			vault2Id = i
			break
		}
	}
	if vault2Id < 0 {
		t.Fatal("second vault not found by vaultById")
	}
	tx, err = tc.dvp.Transact(tc.auth, "addVaultToGroup", big.NewInt(vault2Id), big.NewInt(0))
	waitOK(t, tc.client, "addVaultToGroup", tx, err)
	vault2 := bind.NewBoundContract(vault2Addr, vaultABI, tc.client, tc.client, tc.client)
	t.Logf("second Erc20CoinVault registered as vault %d in group 0", vault2Id)

	// ── Alice opens a swap from vault 0, paying commitB to Bob ───────────────
	d := dvpSetupDeposits(t, tc, swapKeyOverwriteToken)
	saltB, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	saltA, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	alice, err := tc.gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: d.aliceSpend.PrivateKey, PublicKey: d.aliceSpend.PublicKey},
		d.aliceSaltField, d.erc20Amount, d.erc20TokenId,
		d.bobSpend.PublicKey, saltB, d.nftAmount, d.nftTokenId, saltA,
		big.NewInt(0), d.aliceMerkleProof, tc.merkleDepth, vaultInt(tc.erc721VaultAddr),
	)
	if err != nil {
		t.Fatalf("Alice DvPInitiatorProofFromSalts: %v", err)
	}
	aliceLeg := onchainProofReceipt{
		Proof: proofStringsToOnchain(t, alice.Proof), Statement: alice.Statement,
		NumberOfInputs: big.NewInt(1), NumberOfOutputs: big.NewInt(1),
	}
	deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(minSwapDeadline))
	if status, err := submitLeg(t, tc, tc.auth, aliceLeg, 0, 0, deadline); err != nil || status != 1 {
		t.Fatalf("Alice's leg: status=%d err=%v", status, err)
	}
	t.Logf("Alice opened swap commitB=%s from vault 0", alice.CommitB)

	// ── Bob builds a leg with the SAME commitB from his own note in vault 2 ───
	// commitB = Poseidon(pkBob, saltB, amount, tokenId); Bob knows all four.
	bobNote := depositIntoVault(t, tc.ctx, tc.client, vault2, tc.erc20Token, vault2Addr, tc.auth,
		d.erc20Amount, d.erc20TokenId, tc.merkleDepth)
	otherSaltA, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := tc.gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: bobNote.spend.PrivateKey, PublicKey: bobNote.spend.PublicKey},
		bobNote.saltBField, d.erc20Amount, d.erc20TokenId,
		d.bobSpend.PublicKey, saltB, d.nftAmount, d.nftTokenId, otherSaltA,
		big.NewInt(int64(bobNote.merkleProof.TreeNumber)), bobNote.merkleProof, tc.merkleDepth, vaultInt(tc.erc721VaultAddr),
	)
	if err != nil {
		t.Fatalf("Bob DvPInitiatorProofFromSalts: %v", err)
	}
	if bob.CommitB.Cmp(alice.CommitB) != 0 {
		t.Fatalf("test setup: Bob's leg does not reproduce Alice's commitB")
	}
	bobLeg := onchainProofReceipt{
		Proof: proofStringsToOnchain(t, bob.Proof), Statement: bob.Statement,
		NumberOfInputs: big.NewInt(1), NumberOfOutputs: big.NewInt(1),
	}

	// 1. While Alice's swap is pending, Bob's leg must not replace it.
	status, err := submitLeg(t, tc, tc.auth, bobLeg, vault2Id, 0, deadline)
	if err == nil && status == 1 {
		t.Fatal("VULNERABLE: a second leg with the same commitB overwrote Alice's pending swap; " +
			"her note stays locked and claimSwapTimeout now refunds Bob's leg")
	}
	if !isRevert(err, "SwapAlreadyPending") {
		t.Fatalf("Bob's leg was refused, but not with SwapAlreadyPending: status=%d err=%v", status, err)
	}
	t.Log("a second leg on a pending commitB is refused (SwapAlreadyPending)")

	// 2. Once Alice's swap has timed out (and closed), Bob's leg must not reopen it.
	hardhatIncreaseTime(t, tc.client, minSwapDeadline+60)
	tx, err = tc.dvp.Transact(tc.auth, "claimSwapTimeout", alice.CommitB)
	waitOK(t, tc.client, "claimSwapTimeout (Alice)", tx, err)
	deadline2 := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(minSwapDeadline))
	status, err = submitLeg(t, tc, tc.auth, bobLeg, vault2Id, 0, deadline2)
	if err == nil && status == 1 {
		t.Fatal("VULNERABLE: a closed swap (commitB) was reopened by a new leg")
	}
	if !isRevert(err, "SwapClosed") {
		t.Fatalf("reopening was refused, but not with SwapClosed: status=%d err=%v", status, err)
	}
	t.Log("a leg on a closed commitB is refused (SwapClosed)")
}
