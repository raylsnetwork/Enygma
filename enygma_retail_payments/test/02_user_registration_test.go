package tests

// Phase 1 — User Registration
//
// TestUserRegistration verifies the full registration flow:
//  1. Alice and Bob each supply a spend key and a 1184-byte view key.
//  2. Both call UserRegistry.register(pkSpend, pkView) on-chain.
//  3. LookupKeys confirms both keys are readable back from the contract.
//  4. getUserCount, getUserIndex, and getUserAt are verified for round-trip correctness.
//
// Prerequisites:
//   npx hardhat node   (fresh node — registration is not idempotent)
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestUserRegistration -v -timeout 120s

import (
	"math/big"
	"strings"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
)

// mlkemEncapKeySize is the byte length of an ML-KEM-768 encapsulation key.
// Matches mlkem.EncapKeySize — kept as a constant here so this file does not
// need to import lattice_zk/mlkem.
const mlkemEncapKeySize = 1184

// makeViewKey returns a deterministic 1184-byte view key filled with fillByte.
// Used only in tests — real deployments must use actual ML-KEM key generation.
func makeViewKey(fillByte byte) []byte {
	key := make([]byte, mlkemEncapKeySize)
	for i := range key {
		key[i] = fillByte
	}
	return key
}

func TestUserRegistration(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not available — run: npx hardhat node")
	}

	client := mustDial(t)
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	aliceAuth := hardhatAuth(t, client)
	bobAuth   := hardhatBobAuth(t, client)

	alicePkSpend := big.NewInt(0xA11CE)
	bobPkSpend   := big.NewInt(0xB0B)

	// Deterministic view keys — just filled bytes (not real ML-KEM keys).
	aliceViewKey := makeViewKey(0xAA)
	bobViewKey   := makeViewKey(0xBB)

	// ── Step 1: Alice registers ───────────────────────────────────────────────
	t.Log("Step 1 — Alice registers her spend key and view key on-chain")

	if err := rpcore.Register(client, aliceAuth, registryAddr, alicePkSpend, aliceViewKey); err != nil {
		if strings.Contains(err.Error(), "AlreadyRegistered") ||
			strings.Contains(err.Error(), "45ed80e9") {
			t.Skip("requires a fresh Hardhat node — Alice already registered")
		}
		t.Fatalf("Alice Register: %v", err)
	}
	t.Logf("  Alice registered (Ethereum: %s)", aliceAuth.From.Hex())

	// ── Step 2: Bob registers ─────────────────────────────────────────────────
	t.Log("Step 2 — Bob registers his spend key and view key on-chain")

	if err := rpcore.Register(client, bobAuth, registryAddr, bobPkSpend, bobViewKey); err != nil {
		t.Fatalf("Bob Register: %v", err)
	}
	t.Logf("  Bob registered (Ethereum: %s)", bobAuth.From.Hex())

	// ── Step 3: Key lookup ────────────────────────────────────────────────────
	t.Log("Step 3 — Verifying keys are readable back from UserRegistry")

	aliceAddr := common.HexToAddress(hardhatAliceAddr)
	aliceKeys, err := rpcore.LookupKeys(client, registryAddr, aliceAddr)
	if err != nil {
		t.Fatalf("LookupKeys (Alice): %v", err)
	}
	if aliceKeys.SpendKey.Cmp(alicePkSpend) != 0 {
		t.Errorf("Alice pkSpend mismatch: got %s, want %s", aliceKeys.SpendKey, alicePkSpend)
	}
	if len(aliceKeys.ViewKey) != mlkemEncapKeySize {
		t.Errorf("Alice viewKey length: got %d, want %d", len(aliceKeys.ViewKey), mlkemEncapKeySize)
	}
	t.Logf("  Alice: pkSpend=%s viewKey=%d bytes ✓", aliceKeys.SpendKey, len(aliceKeys.ViewKey))

	bobAddr := common.HexToAddress(hardhatBobAddr)
	bobKeys, err := rpcore.LookupKeys(client, registryAddr, bobAddr)
	if err != nil {
		t.Fatalf("LookupKeys (Bob): %v", err)
	}
	if bobKeys.SpendKey.Cmp(bobPkSpend) != 0 {
		t.Errorf("Bob pkSpend mismatch: got %s, want %s", bobKeys.SpendKey, bobPkSpend)
	}
	t.Logf("  Bob: pkSpend=%s viewKey=%d bytes ✓", bobKeys.SpendKey, len(bobKeys.ViewKey))

	// ── Step 4: User count and index ─────────────────────────────────────────
	t.Log("Step 4 — Verifying getUserCount, getUserIndex, getUserAt")

	count, err := rpcore.GetUserCount(client, registryAddr)
	if err != nil {
		t.Fatalf("GetUserCount: %v", err)
	}
	if count < 2 {
		t.Errorf("user count: got %d, want >= 2", count)
	}
	t.Logf("  total registered users: %d", count)

	aliceIdx, err := rpcore.GetUserIndex(client, registryAddr, aliceAddr)
	if err != nil {
		t.Fatalf("GetUserIndex (Alice): %v", err)
	}
	bobIdx, err := rpcore.GetUserIndex(client, registryAddr, bobAddr)
	if err != nil {
		t.Fatalf("GetUserIndex (Bob): %v", err)
	}
	if aliceIdx == bobIdx {
		t.Errorf("Alice and Bob share the same index: %d", aliceIdx)
	}
	t.Logf("  Alice index=%d  Bob index=%d", aliceIdx, bobIdx)

	addrAtAlice, err := rpcore.GetUserAt(client, registryAddr, aliceIdx)
	if err != nil {
		t.Fatalf("GetUserAt (Alice): %v", err)
	}
	if addrAtAlice != aliceAddr {
		t.Errorf("getUserAt(%d): got %s, want %s", aliceIdx, addrAtAlice.Hex(), aliceAddr.Hex())
	}
	addrAtBob, err := rpcore.GetUserAt(client, registryAddr, bobIdx)
	if err != nil {
		t.Fatalf("GetUserAt (Bob): %v", err)
	}
	if addrAtBob != bobAddr {
		t.Errorf("getUserAt(%d): got %s, want %s", bobIdx, addrAtBob.Hex(), bobAddr.Hex())
	}
	t.Logf("  getUserAt round-trip ✓")

	t.Log("=== PHASE 1 COMPLETE: Alice and Bob registered on-chain ===")
}

// TestSybilProtection verifies the registration fee mechanism (paper §6.2).
//
// Flow:
//  1. Owner (Alice = deployer) reads current fee — should be 0 (default).
//  2. Owner sets fee to 1 wei.
//  3. Carol tries to register with 0 ETH → InsufficientFee error.
//  4. Carol registers with 1 wei → success.
//  5. Owner withdraws collected fees.
//  6. Owner resets fee to 0 (clean up for subsequent tests).
func TestSybilProtection(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not available — run: npx hardhat node")
	}

	client := mustDial(t)
	defer client.Close()

	receipts     := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	// Alice is the deployer → owner of UserRegistry.
	aliceAuth := hardhatAuth(t, client)
	// Carol = account[2] — a fresh address for this test.
	carolAuth := hardhatCarolAuth(t, client)

	// ── Step 1: Verify default fee is 0 ──────────────────────────────────────
	t.Log("Step 1 — verifying default registrationFee = 0")

	fee, err := rpcore.GetRegistrationFee(client, registryAddr)
	if err != nil {
		t.Fatalf("GetRegistrationFee: %v", err)
	}
	t.Logf("  current fee = %s wei", fee)
	if fee.Sign() != 0 {
		t.Logf("  fee already set — resetting to 0 first")
		if err := rpcore.SetRegistrationFee(client, aliceAuth, registryAddr, big.NewInt(0)); err != nil {
			t.Fatalf("SetRegistrationFee(0): %v", err)
		}
	}

	// ── Step 2: Owner sets fee to 1 wei ──────────────────────────────────────
	t.Log("Step 2 — owner sets registrationFee to 1 wei")

	if err := rpcore.SetRegistrationFee(client, aliceAuth, registryAddr, big.NewInt(1)); err != nil {
		t.Fatalf("SetRegistrationFee(1): %v", err)
	}
	fee, err = rpcore.GetRegistrationFee(client, registryAddr)
	if err != nil {
		t.Fatalf("GetRegistrationFee after set: %v", err)
	}
	if fee.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected fee=1, got %s", fee)
	}
	t.Logf("  fee set to %s wei ✓", fee)

	// ── Step 3: Carol tries to register without paying → InsufficientFee ─────
	t.Log("Step 3 — Carol tries to register with 0 ETH → expects InsufficientFee")

	carolPkSpend := big.NewInt(0xCA201)
	carolViewKey := makeViewKey(0xCC)

	// Force value=0 to bypass the auto-fee logic and test the contract rejection.
	carolAuthNoFee        := *carolAuth
	carolAuthNoFee.Value  = big.NewInt(0)
	carolAuthNoFeePtr     := &carolAuthNoFee

	// We call the contract directly to test the revert.
	registryABI := loadOnchainABI(t, "UserRegistry")
	registry    := bindContract(t, client, registryAddr, registryABI)
	_, err = registry.Transact(carolAuthNoFeePtr, "register", carolPkSpend, carolViewKey)
	if err == nil {
		t.Fatal("expected InsufficientFee error, got nil")
	}
	if !strings.Contains(err.Error(), "InsufficientFee") &&
		!strings.Contains(err.Error(), "insufficient") &&
		!strings.Contains(err.Error(), "a458261b") { // InsufficientFee(uint256,uint256) selector
		t.Errorf("unexpected error (want InsufficientFee): %v", err)
	}
	t.Logf("  Carol rejected with 0 ETH ✓ (%s)", err)

	// ── Step 4: Carol registers with 1 wei → success ──────────────────────────
	t.Log("Step 4 — Carol registers paying 1 wei → success")

	// rpcore.Register auto-queries the fee and sends it.
	if err := rpcore.Register(client, carolAuth, registryAddr, carolPkSpend, carolViewKey); err != nil {
		if strings.Contains(err.Error(), "AlreadyRegistered") ||
			strings.Contains(err.Error(), "45ed80e9") { // AlreadyRegistered(address) selector
			t.Log("  Carol already registered — skipping registration step")
		} else {
			t.Fatalf("Carol Register: %v", err)
		}
	} else {
		t.Logf("  Carol registered (with fee) ✓")
	}

	// ── Step 5: Owner withdraws collected fees ────────────────────────────────
	t.Log("Step 5 — owner withdraws collected registration fees")

	if err := rpcore.WithdrawFees(client, aliceAuth, registryAddr); err != nil {
		t.Fatalf("WithdrawFees: %v", err)
	}
	t.Log("  fees withdrawn ✓")

	// ── Step 6: Reset fee to 0 for clean test environment ────────────────────
	t.Log("Step 6 — resetting fee to 0")

	if err := rpcore.SetRegistrationFee(client, aliceAuth, registryAddr, big.NewInt(0)); err != nil {
		t.Fatalf("SetRegistrationFee(0) reset: %v", err)
	}
	fee, _ = rpcore.GetRegistrationFee(client, registryAddr)
	t.Logf("  fee reset to %s ✓", fee)

	t.Log("=== SYBIL PROTECTION COMPLETE: fee gate works, owner can withdraw ===")
}
