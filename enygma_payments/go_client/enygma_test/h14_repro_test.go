package enygma_test

// TestH14EmptyParticipantArraysRejected reproduces the H-14 finding
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md) against the *fixed* contract and
// confirms the exploit shape is now rejected.
//
// H-14's root cause: withdraw() selected its verifier by
// _withdrawVerifiers[depositParams.length] — a different array from
// participantIds/commitmentDeltas, the ones _verifyPublicInputs and
// _updateBalances actually bind and mutate. That let a caller submit a
// nonzero depositParams (selecting a real, registered verifier) alongside
// EMPTY participantIds/commitmentDeltas: _verifyPublicInputs's entire body
// is a loop over participantIds.length, so an empty array skipped every
// public-key, previous-commitment and tx-commitment check, while
// _executeZkDvpDeposits still minted caller-chosen DvP value with no
// corresponding Enygma-side debit.
//
// Uses MockWithdrawVerifier (always "verifies") and MockZkDvp (mints
// whatever it's asked to and counts it) so the withdraw *circuit* — whose
// prover route is broken independently (M-03) and cannot produce a real
// proof today — is not on the critical path; this test is entirely about
// the array-length/verifier-selection logic in withdraw() itself.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH14 -v -timeout 60s

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestH14EmptyParticipantArraysRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))

	mkAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}
	waitTx := func(tx *ethtypes.Transaction, txErr error) *ethtypes.Receipt {
		t.Helper()
		if txErr != nil {
			t.Fatalf("send tx: %v", txErr)
		}
		r, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			t.Fatalf("wait mined: %v", err)
		}
		return r
	}

	instance, _ := freshSetup(t, client, mkAuth, waitTx) // domain check unreached: InvalidParticipantCount fires first

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	mockZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")

	if r := waitTx(instance.AddZkDvp(mkAuth(), mockZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}
	// Register the mock verifier under DEFAULT_SIZE (6) — the only key the
	// fixed withdraw() can ever look up, per its own doc comment. Also
	// register it under 1, matching len(depositParams) below, so that on
	// the *unmodified* contract (verifier keyed by depositParams.length)
	// the call would reach the same verifier — the fixed contract must
	// reject this on the length checks, not merely on VerifierNotFound.
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockVerifierAddr, big.NewInt(6))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier(6) failed")
	}
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockVerifierAddr, big.NewInt(1))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier(1) failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}

	// Craft a public_signal with only the two fields withdraw() checks when
	// the participant arrays are empty: block number (offset 36) and
	// nullifier (offset 49). Everything else is zero — irrelevant, since
	// _verifyPublicInputs's whole body is skipped for len(participantIds)==0
	// and the mock verifier ignores proof content entirely.
	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	pubSig[36] = blockHash
	pubSig[49] = new(big.Int).SetBytes(crypto.Keccak256([]byte("H14-repro-nullifier")))

	attackProof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	const mintAmount = int64(1_000_000)
	attackDepositParams := []enygma.IEnygmaDepositParams{
		{Amount: big.NewInt(mintAmount), Erc20Adress: common.Address{}, PublicKey: big.NewInt(0)},
	}

	// The H-14 attack: participantIds and commitmentDeltas are EMPTY, so on
	// the unmodified contract _verifyPublicInputs and the debit loop in
	// _updateBalances would be no-ops, while depositParams still instructs
	// the (mock) ZkDvp bridge to mint.
	_, sendErr := instance.Withdraw(mkAuth(),
		[]enygma.IEnygmaPoint{}, attackProof, attackDepositParams, []*big.Int{})
	if sendErr == nil {
		t.Fatal("FAIL (H-14 regressed): withdraw() with empty participant arrays SUCCEEDED — " +
			"the verifier-selection/length gap is back")
	}
	// InvalidParticipantCount() = 0x54fb3045 (commitmentDeltas.length(0) !=
	// DEFAULT_SIZE(6)) — the specific new guard, not just VerifierNotFound
	// (0xe25b142c) or the pre-existing ParticipantIdsLengthMismatch
	// (0x8fe655c0, which wouldn't even fire here since 0 == 0).
	const wantSelector = "0x54fb3045" // InvalidParticipantCount()
	if !strings.Contains(sendErr.Error(), wantSelector) {
		t.Fatalf("withdraw() reverted, but not with InvalidParticipantCount (%s): %v", wantSelector, sendErr)
	}
	t.Logf("withdraw() with empty participant arrays reverted with InvalidParticipantCount() — H-14 correctly rejected: %v", sendErr)

	// ── Control: the mock ZkDvp bridge was never touched ───────────────────
	totalMintedSel := crypto.Keccak256([]byte("totalMinted()"))[:4]
	res, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &mockZkDvpAddr,
		Data: totalMintedSel,
	}, nil)
	if err != nil {
		t.Fatalf("call MockZkDvp.totalMinted(): %v", err)
	}
	totalMinted := new(big.Int).SetBytes(res)
	if totalMinted.Sign() != 0 {
		t.Fatalf("FAIL: MockZkDvp.totalMinted() = %s, want 0 — the reverted withdraw() still minted DvP value", totalMinted)
	}
	t.Log("MockZkDvp.totalMinted() == 0 — the rejected call never reached the ZkDvp bridge")
}
