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

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
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

	// c04Setup registers exactly 6 banks, accountIds 1..6, so
	// _totalRegisteredParties == 6 and _verifyPublicInputsFP's internal
	// getPublicValues(_totalRegisteredParties + 1) array only spans
	// indices 0..6 — id 7 alone would be OUT of that range and revert
	// with a plain array-bounds panic rather than exercising H-07's
	// check at all. Registering one more, unrelated account (id=200)
	// grows _totalRegisteredParties to 7 (array now spans 0..7) while
	// leaving id 7 itself still unregistered — a genuine in-range gap,
	// exactly the audit's "second ownerless slot" scenario, and what
	// actually exercises the new keys[accountId]==0 check instead of
	// short-circuiting on an unrelated bounds panic.
	instance, banks, enygmaAddr := c04Setup(t, client)

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
	fillerTx, err := instance.RegisterAccount(ownerAuth, ownerAddr, big.NewInt(200), big.NewInt(999999), fillerCx, fillerCy, []byte{})
	if err != nil {
		t.Fatalf("register filler account: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, fillerTx); err != nil || r.Status != ethtypes.ReceiptStatusSuccessful {
		t.Fatalf("register filler account not mined: err=%v receipt=%+v", err, r)
	}

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
	for i := 0; i < nBanks-1; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	participantIds[nBanks-1] = big.NewInt(7)

	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (H-07 regressed): transfer() naming an unregistered participant (id=7) was accepted")
	}
	if !strings.Contains(sendErr.Error(), "UnregisteredParticipant") {
		t.Fatalf("transfer() with an unregistered participant reverted, but not with UnregisteredParticipant: %v", sendErr)
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
	participantIds[0] = big.NewInt(0) // the classic sink — never registerable since M-06
	for i := 1; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}

	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (H-07 regressed): transfer() naming accountId=0 as a participant was accepted")
	}
	if !strings.Contains(sendErr.Error(), "UnregisteredParticipant") {
		t.Fatalf("transfer() with accountId=0 reverted, but not with UnregisteredParticipant: %v", sendErr)
	}
	t.Logf("transfer() correctly rejected accountId=0: %v", sendErr)
}
