package enygma_test

// TestH09_RelayAttribution reproduces and verifies the H-09 (item 4 of 5)
// fix (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/register-High): the
// relayer is the sole possible transaction submitter (every registered
// account is bound to the owner's address, never the bank's own), so
// TransactionSuccessful's senderAddress was always the relayer — the
// chain's own audit trail carried no per-bank attribution for who
// actually asked for a given submission, leaving that fact recoverable
// only from the relayer's own (single-bearer-token, pre-H-06) logs.
//
// transfer()/transferWithFee() now accept an optional, unvalidated
// bankTag string, emitted in the same transaction as
// TransactionSuccessful via a new RelayAttribution event — the relayer
// passes its Fix H-06 per-bank credential identifier; this test confirms
// the event actually reaches the chain with the exact value passed in,
// same-transaction as the transfer it attributes.
//
// Uses the same c04Setup + full pairwise fingerprint confirmation as
// TestC04_HonestTransferSucceedsWithConfirmedFingerprints — the only
// pattern in this suite that reaches a genuinely successful Transfer()
// call (MockTransferVerifier stands in for proof validity; C-04's
// on-chain fingerprint registry is a separate, real requirement this
// test must also satisfy to reach RelayAttribution's emit at all).
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH09 -v -timeout 60s

import (
	"context"
	"math/big"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestH09_RelayAttribution(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)

	// Confirm all C(6,2)=15 pairwise fingerprints, both directions — see
	// TestC04_HonestTransferSucceedsWithConfirmedFingerprints's identical
	// block for the full rationale.
	var fingerprints [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			fingerprints[i][j] = big.NewInt(int64(10000*(lo+1) + (hi + 1)))
		}
	}
	for i := 0; i < nBanks; i++ {
		for j := i + 1; j < nBanks; j++ {
			tx1, err := instance.RegisterFingerprint(bankAuth(t, client, banks[i]), big.NewInt(banks[j].accountID), fingerprints[i][j])
			if err != nil {
				t.Fatalf("register (%d,%d): %v", i, j, err)
			}
			if _, err := bind.WaitMined(context.Background(), client, tx1); err != nil {
				t.Fatalf("wait register (%d,%d): %v", i, j, err)
			}
			tx2, err := instance.RegisterFingerprint(bankAuth(t, client, banks[j]), big.NewInt(banks[i].accountID), fingerprints[j][i])
			if err != nil {
				t.Fatalf("register (%d,%d): %v", j, i, err)
			}
			if _, err := bind.WaitMined(context.Background(), client, tx2); err != nil {
				t.Fatalf("wait register (%d,%d): %v", j, i, err)
			}
		}
	}

	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 333)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	const wantBankTag = "acme-bank-01" // stands in for the relayer's Fix H-06 per-bank credential id
	auth := bankAuth(t, client, banks[0])
	tx, sendErr := instance.Transfer(auth, deltas, proof, participantIds, wantBankTag)
	if sendErr != nil {
		t.Fatalf("transfer with bankTag=%q reverted: %v", wantBankTag, sendErr)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || receipt.Status != 1 {
		t.Fatalf("transfer tx failed: status=%v err=%v", receipt, err)
	}

	var found *enygma.EnygmaRelayAttribution
	for _, l := range receipt.Logs {
		ev, parseErr := instance.ParseRelayAttribution(*l)
		if parseErr == nil {
			found = ev
			break
		}
	}
	if found == nil {
		t.Fatal("FAIL (H-09 regressed): no RelayAttribution event found in the transfer's own receipt logs")
	}
	if found.BankTag != wantBankTag {
		t.Fatalf("FAIL (H-09 regressed): RelayAttribution.bankTag = %q, want %q", found.BankTag, wantBankTag)
	}
	if found.Submitter != auth.From {
		t.Fatalf("RelayAttribution.submitter = %s, want %s (msg.sender, the submitting address)", found.Submitter, auth.From)
	}
	t.Logf("RelayAttribution emitted in the same tx as the transfer: submitter=%s bankTag=%q ✓", found.Submitter, found.BankTag)
}

// TestH09_RelayAttribution_EmptyTagAllowed confirms bankTag is genuinely
// optional — a direct on-chain caller with no bank credential to report
// (e.g. every other test in this suite) must not be forced to supply
// one, and the event still fires with an empty string rather than being
// skipped.
func TestH09_RelayAttribution_EmptyTagAllowed(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)

	var fingerprints [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			fingerprints[i][j] = big.NewInt(int64(20000*(lo+1) + (hi + 1)))
		}
	}
	for i := 0; i < nBanks; i++ {
		for j := i + 1; j < nBanks; j++ {
			tx1, err := instance.RegisterFingerprint(bankAuth(t, client, banks[i]), big.NewInt(banks[j].accountID), fingerprints[i][j])
			if err != nil {
				t.Fatalf("register (%d,%d): %v", i, j, err)
			}
			bind.WaitMined(context.Background(), client, tx1)
			tx2, err := instance.RegisterFingerprint(bankAuth(t, client, banks[j]), big.NewInt(banks[i].accountID), fingerprints[j][i])
			if err != nil {
				t.Fatalf("register (%d,%d): %v", j, i, err)
			}
			bind.WaitMined(context.Background(), client, tx2)
		}
	}

	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 444)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	tx, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "")
	if sendErr != nil {
		t.Fatalf("transfer with empty bankTag reverted: %v", sendErr)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || receipt.Status != 1 {
		t.Fatalf("transfer tx failed: status=%v err=%v", receipt, err)
	}
	for _, l := range receipt.Logs {
		if ev, parseErr := instance.ParseRelayAttribution(*l); parseErr == nil {
			if ev.BankTag != "" {
				t.Fatalf("expected empty bankTag, got %q", ev.BankTag)
			}
			t.Log("empty bankTag accepted and emitted as-is ✓")
			return
		}
	}
	t.Fatal("FAIL: no RelayAttribution event found for an empty-bankTag transfer")
}
