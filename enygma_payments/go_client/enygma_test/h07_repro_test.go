package enygma_test

// TestH07_UnregisteredParticipantRejected reproduces H-07
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/LIVE): accountId 0, and any
// in-range id that was never actually registered, used to pass every
// on-chain check by simply setting the corresponding public_signal slot to
// 0 — keys[unregisteredId] is 0 too (publicKeys defaults to 0 until
// registerAccount sets it), so the public-key comparison matched trivially.
// Value routed to such a slot is destroyed (its key is unrecoverable),
// making this a griefing/value-destruction primitive rather than theft.
//
// registerAccount now requires ids to be assigned sequentially (1, 2, 3, ...
// with no gaps), so the "in-range but never registered" slot H-07 describes
// can no longer be created at all: every id in 1.._totalRegisteredParties is
// registered by construction, and an id above that range is out of the
// getPublicValues array and reverts before any per-participant check. The
// keys[accountId]==0 check in _verifyPublicInputsFP stays as defense in
// depth. This test therefore asserts both halves of that guarantee: the gap
// cannot be constructed, and a transfer naming an unregistered id is still
// rejected.
//
// Uses c04Setup/buildTransferSignal/bankAuth (MockTransferVerifier —
// _verifyPublicInputsFP's own logic is what's under test here, not proof
// validity) — the same infrastructure C-04's tests already established.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH07 -v -timeout 60s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestH07_UnregisteredParticipantRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)

	// Registering a non-adjacent id (200) used to grow _totalRegisteredParties
	// to 7 while leaving id 7 unregistered — the exact in-range gap H-07
	// needed. It must now be refused.
	ctx := context.Background()
	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		t.Fatalf("owner nonce: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	ownerAuth, err := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
	if err != nil {
		t.Fatalf("owner auth: %v", err)
	}
	ownerAuth.Nonce = big.NewInt(int64(nonce))
	ownerAuth.Value = big.NewInt(0)
	ownerAuth.GasLimit = 8_000_000
	ownerAuth.GasPrice = gasPrice

	fillerCx, fillerCy := regCommit(big.NewInt(424242))
	_, fillerErr := instance.RegisterAccount(ownerAuth, ownerAddr, big.NewInt(200), big.NewInt(999999), fillerCx, fillerCy, []byte{})
	if fillerErr == nil {
		t.Fatal("FAIL: registerAccount(id=200) was accepted while only ids 1..6 exist — an in-range gap can be created again")
	}
	if !strings.Contains(fillerErr.Error(), "InvalidAccountId") {
		t.Fatalf("registerAccount(id=200) reverted, but not with InvalidAccountId: %v", fillerErr)
	}
	t.Logf("registerAccount(id=200) correctly refused (ids must be sequential): %v", fillerErr)

	var fingerprints [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			fingerprints[i][j] = big.NewInt(0)
		}
	}
	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 777)

	// Sorted, strictly increasing (matching the C-05 fix's own requirement)
	// with the last slot replaced by the unregistered id 7 instead of
	// bank 5's real accountID (6), so this test is unambiguously about
	// H-07's check rather than incidentally tripping the sort-order one.
	participantIds := make([]*big.Int, nBanks)
	accountIds := make([]int64, nBanks)
	for i := 0; i < nBanks-1; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
		accountIds[i] = banks[i].accountID
	}
	participantIds[nBanks-1] = big.NewInt(7)
	accountIds[nBanks-1] = 7

	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds)

	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, usdrDeltas, usdrProof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (H-07 regressed): transfer() naming an unregistered participant (id=7) was accepted")
	}
	// id 7 is above _totalRegisteredParties (6), so it is now rejected by the
	// out-of-range array access before the keys[accountId]==0 check is reached.
	// Either revert proves the transfer is refused.
	msg := sendErr.Error()
	if !strings.Contains(msg, "UnregisteredParticipant") && !strings.Contains(msg, "panic") && !strings.Contains(msg, "0x32") {
		t.Fatalf("transfer() with an unregistered participant reverted, but not with UnregisteredParticipant or an out-of-range panic: %v", sendErr)
	}
	t.Logf("transfer() correctly rejected an unregistered participant id: %v", sendErr)
}

func TestH07_AccountIdZeroRejected(t *testing.T) {
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
			fingerprints[i][j] = big.NewInt(0)
		}
	}
	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 778)

	participantIds := make([]*big.Int, nBanks)
	accountIds := make([]int64, nBanks)
	participantIds[0] = big.NewInt(0) // the classic sink — never registerable since M-06
	accountIds[0] = 0
	for i := 1; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
		accountIds[i] = banks[i].accountID
	}

	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds)

	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, usdrDeltas, usdrProof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (H-07 regressed): transfer() naming accountId=0 as a participant was accepted")
	}
	if !strings.Contains(sendErr.Error(), "UnregisteredParticipant") {
		t.Fatalf("transfer() with accountId=0 reverted, but not with UnregisteredParticipant: %v", sendErr)
	}
	t.Logf("transfer() correctly rejected accountId=0: %v", sendErr)
}
