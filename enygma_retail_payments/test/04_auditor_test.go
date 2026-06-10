package tests

// TestAuditorViewKeyRecovery verifies the full auditor key-recovery flow:
//
//  1. Generate auditor ML-KEM-768 keypair.
//  2. Alice and Bob generate real spend + view keypairs.
//  3. Encrypt each user's sk_view (DecapsKey seed) for the auditor via
//     EncryptViewKeyForAuditor — mlKemCt is passed as AEAD AAD, binding
//     the two ciphertexts so neither can be swapped independently.
//  4. Register Alice and Bob on-chain with the audit ciphertexts.
//  5. Auditor reads (mlKemCt, aesCt) from UserRegistry via LookupAuditCiphertexts.
//  6. Auditor calls AuditorDecryptViewKey → recovers each user's DecapsKey.
//  7. Verify: recovered EncapsulationKey().Bytes() == original EncapsKey (round-trip).
//  8. Verify AEAD binding: a tampered mlKemCt must cause decryption to fail.
//
// Prerequisites:
//
//	npx hardhat node   (fresh node — registration is not idempotent)
//	deploy + init contracts (build/receipts.json must exist)
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestAuditorViewKeyRecovery -v -timeout 120s

import (
	"bytes"
	"strings"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
)

func TestAuditorViewKeyRecovery(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not available — run: npx hardhat node")
	}

	client := mustDial(t)
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	aliceAuth := hardhatAuth(t, client)
	bobAuth := hardhatBobAuth(t, client)

	// ── Step 1: Auditor keypair ───────────────────────────────────────────────
	t.Log("Step 1 — generating auditor ML-KEM-768 keypair")

	auditorPair, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatalf("NewAuditorKeyPair: %v", err)
	}
	t.Logf("  auditor encapsulation key: %d bytes", len(auditorPair.EncapsKey))

	// ── Step 2: User keypairs ─────────────────────────────────────────────────
	t.Log("Step 2 — generating Alice and Bob's spend and view keypairs")

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair (Alice): %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair (Alice): %v", err)
	}
	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair (Bob): %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair (Bob): %v", err)
	}
	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ── Step 3: Encrypt view keys for auditor ─────────────────────────────────
	t.Log("Step 3 — encrypting view secret keys for auditor (mlKemCt as AEAD AAD)")

	aliceMlKemCt, aliceAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, aliceView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Alice): %v", err)
	}
	bobMlKemCt, bobAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, bobView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Bob): %v", err)
	}
	t.Logf("  Alice: mlKemCt=%d bytes  aesCt=%d bytes", len(aliceMlKemCt), len(aliceAesCt))
	t.Logf("  Bob:   mlKemCt=%d bytes  aesCt=%d bytes", len(bobMlKemCt), len(bobAesCt))

	// ── Step 4: Register on-chain ─────────────────────────────────────────────
	t.Log("Step 4 — registering Alice and Bob on-chain with audit ciphertexts")

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if strings.Contains(err.Error(), "AlreadyRegistered") ||
			strings.Contains(err.Error(), "45ed80e9") {
			t.Skip("requires a fresh Hardhat node — Alice already registered")
		}
		t.Fatalf("Register (Alice): %v", err)
	}
	t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if strings.Contains(err.Error(), "AlreadyRegistered") ||
			strings.Contains(err.Error(), "45ed80e9") {
			t.Skip("requires a fresh Hardhat node — Bob already registered")
		}
		t.Fatalf("Register (Bob): %v", err)
	}
	t.Logf("  Bob registered (%s)", bobAuth.From.Hex())

	// ── Step 5: Auditor reads ciphertexts from chain ──────────────────────────
	t.Log("Step 5 — auditor reads audit ciphertexts from UserRegistry")

	aliceAddr := common.HexToAddress(hardhatAliceAddr)
	bobAddr := common.HexToAddress(hardhatBobAddr)

	gotAliceMlKemCt, gotAliceAesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, aliceAddr)
	if err != nil {
		t.Fatalf("LookupAuditCiphertexts (Alice): %v", err)
	}
	gotBobMlKemCt, gotBobAesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, bobAddr)
	if err != nil {
		t.Fatalf("LookupAuditCiphertexts (Bob): %v", err)
	}
	t.Logf("  Alice: mlKemCt=%d bytes  aesCt=%d bytes", len(gotAliceMlKemCt), len(gotAliceAesCt))
	t.Logf("  Bob:   mlKemCt=%d bytes  aesCt=%d bytes", len(gotBobMlKemCt), len(gotBobAesCt))

	// ── Step 6: Auditor recovers view secret keys ─────────────────────────────
	t.Log("Step 6 — auditor decrypts view secret keys")

	recoveredAliceDK, err := rpcore.AuditorDecryptViewKey(auditorPair.DecapsKey, gotAliceMlKemCt, gotAliceAesCt)
	if err != nil {
		t.Fatalf("AuditorDecryptViewKey (Alice): %v", err)
	}
	recoveredBobDK, err := rpcore.AuditorDecryptViewKey(auditorPair.DecapsKey, gotBobMlKemCt, gotBobAesCt)
	if err != nil {
		t.Fatalf("AuditorDecryptViewKey (Bob): %v", err)
	}

	// ── Step 7: Round-trip verification ──────────────────────────────────────
	t.Log("Step 7 — verifying recovered DecapsKey produces the same EncapsKey")

	recoveredAliceEncKey := recoveredAliceDK.EncapsulationKey().Bytes()
	if !bytes.Equal(recoveredAliceEncKey, aliceView.EncapsKey) {
		t.Errorf("Alice encapsulation key mismatch after auditor recovery")
	}
	t.Logf("  Alice view key round-trip ✓ (%d bytes)", len(recoveredAliceEncKey))

	recoveredBobEncKey := recoveredBobDK.EncapsulationKey().Bytes()
	if !bytes.Equal(recoveredBobEncKey, bobView.EncapsKey) {
		t.Errorf("Bob encapsulation key mismatch after auditor recovery")
	}
	t.Logf("  Bob view key round-trip ✓ (%d bytes)", len(recoveredBobEncKey))

	// ── Step 8: AEAD binding — tampered mlKemCt must fail ────────────────────
	t.Log("Step 8 — verifying AEAD binding rejects a tampered mlKemCt")

	tampered := make([]byte, len(gotAliceMlKemCt))
	copy(tampered, gotAliceMlKemCt)
	tampered[0] ^= 0xFF

	_, err = rpcore.AuditorDecryptViewKey(auditorPair.DecapsKey, tampered, gotAliceAesCt)
	if err == nil {
		t.Error("expected error when mlKemCt is tampered, got nil")
	} else {
		t.Logf("  tampered mlKemCt correctly rejected ✓ (%v)", err)
	}

	t.Log("=== AUDITOR KEY RECOVERY COMPLETE ===")
}
