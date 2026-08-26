package enygma_test

// TestH15NonParticipantSurvivesRollover reproduces the H-15 finding
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md) against the *fixed* contract and
// confirms a non-participant's balance survives an epoch rollover.
//
// H-15's root cause: unlike _updateBalancesForTransfer (transfer,
// transferWithFee) and _propagateBalancesExcept (mintSupply, burn),
// _updateBalances (withdraw, deposit) had NO propagation pass at all. Every
// account not named in participantIds simply kept whatever was written at
// balanceCommitments[lastBlockNum][accountId] — a slot getBalance/
// getPublicValues/check() stop reading the instant lastBlockNum advances to
// epochStart, which _updateBalances does unconditionally at its end.
//
// This test registers a 7th bank beyond the fixed 6-account set withdraw()
// operates on (DEFAULT_SIZE) and confirms IT survives a full, honest
// 6-participant withdraw() that crosses an epoch boundary. That's the
// scenario still live once H-14 is fixed alongside this: H-14 (fixed on the
// same combined branch) made withdraw() require commitmentDeltas.length ==
// DEFAULT_SIZE == 6 — with exactly 6 banks registered that leaves no
// "non-participant" a withdraw() call could ever exclude, so H-15's
// original empty-call construction no longer applies to withdraw() at all.
// It remains live the moment a 7th account is registered (registerAccount
// has no upper bound, and DEFAULT_SIZE is fixed at 6), which is exactly
// what this test exercises.
//
// Uses MockWithdrawVerifier (always "verifies") so the withdraw *circuit* —
// whose prover route is broken independently (M-03) and cannot produce a
// real proof today — is not on the critical path; this test needs no
// ZkDvp bridge, since depositParams is empty throughout and the withdraw
// leg of _executeZkDvpDeposits is a no-op for an empty depositParams.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH15 -v -timeout 60s

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestH15NonParticipantSurvivesRollover(t *testing.T) {
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

	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx) // registers accountIds 1..6

	// Register a 7th account — beyond DEFAULT_SIZE, and so beyond any
	// withdraw() call's fixed 6-participant set. This is the bystander.
	const bystanderId = 7
	bystanderSk := big.NewInt(999999)
	bystanderPk, err := poseidon.Hash([]*big.Int{bystanderSk, bystanderSk})
	if err != nil {
		t.Fatalf("bystander pk: %v", err)
	}
	bystanderPk.Mod(bystanderPk, curveP)
	// Fix H-02 residual: this account never proves anything in this test
	// (only its balance commitment's before/after equality is checked), so
	// the exact blinding factors don't need to match any downstream value —
	// senderPrevR/senderMintR are reused here purely as convenient nonzero
	// constants, same as everywhere in this suite that isn't senderIdx.
	bcx, bcy := regCommit(big.NewInt(senderPrevR))
	if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr,
		big.NewInt(bystanderId), bystanderPk, bcx, bcy, []byte{})); r.Status != 1 {
		t.Fatal("registering bystander account failed")
	}
	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(bystanderId), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply to bystander failed")
	}
	t.Logf("registered accountId=%d and minted %d to it — this is the bystander whose balance must survive", bystanderId, mintAmt)

	balBefore, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(bystanderId))
	if err != nil {
		t.Fatalf("getBalance before: %v", err)
	}
	t.Logf("bystander balance before: (%s, %s)", balBefore.X, balBefore.Y)
	if balBefore.X.Sign() == 0 && balBefore.Y.Cmp(big.NewInt(1)) == 0 {
		t.Fatal("test setup bug: bystander balance is already the neutral element before the withdraw call")
	}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	// Fixed withdraw() (H-14) selects its verifier by commitmentDeltas.length,
	// which must be DEFAULT_SIZE (6) — register under that key.
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockVerifierAddr, big.NewInt(6))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	t.Logf("lastBlockNum before rollover: %s", blockHash)

	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys6 := pubVals.Keys[1:]       // accountIds 1..6
	balances6 := pubVals.Balances[1:] // accountIds 1..6

	// Mine past the epoch boundary (epochInterval=30, per freshSetup) so the
	// upcoming withdraw() call lands in a fresh epoch slot.
	rpcClient, err := rpc.Dial(chainURL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	mineBlocks(t, rpcClient, 31)
	newBlockNum, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("block number: %v", err)
	}
	t.Logf("mined to block %d", newBlockNum)

	// Build an honest, zero-delta withdraw() naming all 6 DEFAULT_SIZE
	// accounts (1..6) — required now that H-14 is also fixed. Each delta is
	// the neutral element (0,1): a legitimate no-op for those 6 accounts,
	// so this is a completely ordinary "nothing happens" withdraw() whose
	// only interesting property is that it crosses an epoch boundary.
	// public_signal layout (52-signal): PUBLIC_KEY_OFFSET=6,
	// PREVIOUS_COMMIT_OFFSET=12, TX_COMMIT_OFFSET=24, BLOCK_NUMBER_OFFSET=36,
	// NULLIFIER_OFFSET=49, DOMAIN_OFFSET=51 (Fix L-01; Enygma.sol's own
	// constants).
	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, 6)
	participantIds := make([]*big.Int, 6)
	for i := 0; i < 6; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		pubSig[24+2*i] = big.NewInt(0) // TxCommit = neutral element (0,1): zero delta
		pubSig[24+2*i+1] = big.NewInt(1)
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = new(big.Int).SetBytes(crypto.Keccak256([]byte("H15-repro-nullifier")))
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	if r := waitTx(instance.Withdraw(mkAuth(),
		commitmentDeltas, proof, []enygma.IEnygmaDepositParams{}, participantIds)); r.Status != 1 {
		t.Fatal("honest 6-participant withdraw() reverted — setup problem, not what this test is checking")
	}

	newLastBlockNum, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash after: %v", err)
	}
	if newLastBlockNum.Cmp(blockHash) == 0 {
		t.Fatalf("test setup bug: lastBlockNum did not advance (%s) — the call didn't cross an epoch boundary", newLastBlockNum)
	}
	t.Logf("lastBlockNum advanced %s -> %s — epoch rollover confirmed", blockHash, newLastBlockNum)

	balAfter, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(bystanderId))
	if err != nil {
		t.Fatalf("getBalance after: %v", err)
	}
	t.Logf("bystander balance after:  (%s, %s)", balAfter.X, balAfter.Y)

	if balAfter.X.Cmp(balBefore.X) != 0 || balAfter.Y.Cmp(balBefore.Y) != 0 {
		t.Fatalf("FAIL (H-15 regressed): bystander's balance changed across a withdraw() that never named accountId=%d — "+
			"before (%s, %s) after (%s, %s)", bystanderId, balBefore.X, balBefore.Y, balAfter.X, balAfter.Y)
	}
	t.Log("bystander's balance is unchanged across the epoch rollover — H-15 correctly fixed")

	if ok, err := instance.Check(&bind.CallOpts{}); err != nil || !ok {
		t.Fatalf("check() invariant broken after the rollover: ok=%v err=%v", ok, err)
	}
	t.Log("check() invariant holds after the rollover")
}
