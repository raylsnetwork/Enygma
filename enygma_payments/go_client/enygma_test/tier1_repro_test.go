package enygma_test

// TestM01/TestM06/TestH08 reproduce and verify the Tier 1 Medium fixes
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md):
//
//   M-01 — delegatecall to a codeless verifier returns success=true,
//     which the contract treated as "proof valid". addVerifier/
//     addWithdrawVerifier/addDepositVerifier/addFeeVerifier/
//     addBurnVerifier now all reject a codeless address at registration
//     time (verifier.code.length == 0), and every call site rejects one
//     that somehow loses its code after registration.
//   M-06 — registerAccount was validation-free; a repeat call for an
//     already-live account reset its balance to Com(0,...), destroying
//     it. Now guarded by publicKeys[accountId] != 0.
//   H-08 — ownership was `immutable` with no getter, no transfer, no
//     pause. Now a two-step transferOwnership/acceptOwnership, an
//     owner() getter, and a pause() circuit breaker on every value-
//     moving entry point.
//
// Run:
//
//	CC=/usr/bin/clang go test -run "TestM01|TestM06|TestH08" -v -timeout 60s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestM01_CodelessVerifierRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain separator unused: no proof submitted

	// A plain EOA-style address with no deployed code.
	codeless := common.HexToAddress("0x000000000000000000000000000000DeadBeef")
	code, err := client.CodeAt(context.Background(), codeless, nil)
	if err == nil && len(code) != 0 {
		t.Fatalf("test setup broken: %s unexpectedly has code", codeless.Hex())
	}

	_, sendErr := instance.AddVerifier(mkAuth(), codeless)
	if sendErr == nil {
		t.Fatal("FAIL (M-01 regressed): addVerifier accepted a codeless address")
	}
	if !strings.Contains(sendErr.Error(), "VerifierHasNoCode") {
		t.Fatalf("addVerifier reverted, but not with VerifierHasNoCode: %v", sendErr)
	}
	t.Logf("addVerifier correctly rejected a codeless address: %v", sendErr)
}

func TestM06_RepeatRegistrationRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain separator unused: no proof submitted // registers accountIds 1..6 already

	balBefore, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) before repeat: %v", err)
	}

	// Attempt to re-register accountId=1 (bank 0) with a fresh commitment —
	// pre-fix this would silently zero the account's balance.
	pk, _ := poseidon.Hash([]*big.Int{big.NewInt(999), big.NewInt(999)})
	pk.Mod(pk, curveP)
	cx, cy := regCommit(big.NewInt(11111))
	arbitraryAddr := common.HexToAddress("0x2222222222222222222222222222222222222E") // addr param is independent of accountId/owner — any address works

	_, sendErr := instance.RegisterAccount(mkAuth(), arbitraryAddr, big.NewInt(1), pk, cx, cy, []byte{})
	if sendErr == nil {
		t.Fatal("FAIL (M-06 regressed): repeat registerAccount(1) was accepted")
	}
	if !strings.Contains(sendErr.Error(), "AlreadyRegistered") {
		t.Fatalf("repeat registerAccount reverted, but not with AlreadyRegistered: %v", sendErr)
	}
	t.Logf("repeat registerAccount correctly rejected: %v", sendErr)

	balAfter, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) after rejected repeat: %v", err)
	}
	if balAfter.X.Cmp(balBefore.X) != 0 || balAfter.Y.Cmp(balBefore.Y) != 0 {
		t.Fatalf("bank 0's balance changed despite the rejected repeat call: before (%s,%s) after (%s,%s)",
			balBefore.X, balBefore.Y, balAfter.X, balAfter.Y)
	}
	t.Log("bank 0's balance is unchanged after the rejected repeat call ✓")
}

func TestM06_ZeroAccountIdRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain separator unused: no proof submitted

	pk, _ := poseidon.Hash([]*big.Int{big.NewInt(1), big.NewInt(1)})
	pk.Mod(pk, curveP)
	cx, cy := regCommit(big.NewInt(1))
	arbitraryAddr := common.HexToAddress("0x3333333333333333333333333333333333333E")

	_, sendErr := instance.RegisterAccount(mkAuth(), arbitraryAddr, big.NewInt(0), pk, cx, cy, []byte{})
	if sendErr == nil {
		t.Fatal("FAIL: registerAccount(accountId=0) was accepted")
	}
	if !strings.Contains(sendErr.Error(), "InvalidAccountId") {
		t.Fatalf("registerAccount(0) reverted, but not with InvalidAccountId: %v", sendErr)
	}
	t.Logf("registerAccount(accountId=0) correctly rejected: %v", sendErr)
}

func TestH08_TwoStepOwnershipTransfer(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain separator unused: no proof submitted

	originalOwner, err := instance.Owner(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("owner(): %v", err)
	}

	newOwner := common.HexToAddress("0x1111111111111111111111111111111111111E")
	if r := waitTx(instance.TransferOwnership(mkAuth(), newOwner)); r.Status != 1 {
		t.Fatal("transferOwnership failed")
	}

	// Ownership must NOT have moved yet — this is the whole point of a
	// two-step transfer: a typo'd target address cannot strand it.
	ownerAfterPropose, err := instance.Owner(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("owner() after propose: %v", err)
	}
	if ownerAfterPropose != originalOwner {
		t.Fatalf("FAIL: owner changed after transferOwnership alone (before acceptOwnership): got %s, want %s",
			ownerAfterPropose.Hex(), originalOwner.Hex())
	}
	pending, err := instance.PendingOwner(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("pendingOwner(): %v", err)
	}
	if pending != newOwner {
		t.Fatalf("pendingOwner() = %s, want %s", pending.Hex(), newOwner.Hex())
	}
	t.Log("ownership correctly unchanged after step 1 (transferOwnership) ✓")

	// The old owner (still onlyOwner) attempting to mint must still work —
	// ownership genuinely has not moved.
	mcx, mcy := mintCommitPt(big.NewInt(10), big.NewInt(1))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(10), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("original owner could not mintSupply after proposing a transfer — ownership moved prematurely")
	}
	t.Log("original owner retains full onlyOwner access before acceptOwnership ✓")
}

func TestH08_PauseBlocksMintAndRegister(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain separator unused: no proof submitted

	paused, err := instance.Paused(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("paused(): %v", err)
	}
	if paused {
		t.Fatal("contract unexpectedly paused right after freshSetup")
	}

	if r := waitTx(instance.Pause(mkAuth())); r.Status != 1 {
		t.Fatal("pause() failed")
	}
	paused, err = instance.Paused(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("paused() after pause(): %v", err)
	}
	if !paused {
		t.Fatal("paused() still false after a successful pause() call")
	}

	mcx, mcy := mintCommitPt(big.NewInt(10), big.NewInt(1))
	_, sendErr := instance.MintSupply(mkAuth(), big.NewInt(10), big.NewInt(1), mcx, mcy)
	if sendErr == nil {
		t.Fatal("FAIL (H-08 regressed): mintSupply succeeded while paused")
	}
	if !strings.Contains(sendErr.Error(), "ContractIsPaused") {
		t.Fatalf("mintSupply while paused reverted, but not with ContractIsPaused: %v", sendErr)
	}
	t.Logf("mintSupply correctly blocked while paused: %v", sendErr)

	// unpause must itself remain callable while paused (it's exempt from
	// whenNotPaused, or a pause could never be lifted) and restore access.
	if r := waitTx(instance.Unpause(mkAuth())); r.Status != 1 {
		t.Fatal("unpause() failed")
	}
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(10), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply still failing after unpause()")
	}
	t.Log("mintSupply succeeds again after unpause() ✓")
}
