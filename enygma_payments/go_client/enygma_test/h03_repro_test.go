package enygma_test

// TestH03_TopAccountSurvivesTransferRollover verifies the H-03 fix
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Low/register-High, LATENT):
// three loops in Enygma.sol walk the 1-based account space
// (1.._totalRegisteredParties). Two were already correctly 1-based
// (check(), _propagateBalancesExcept — shared by mintSupply/burn AND, per
// the C-05 rewrite, by _updateBalancesForTransfer/_updateBalances too);
// the audit's filing was that _updateBalancesForTransfer's OWN copy of
// this loop was still 0-based (0..N-1), wasting a slot on nonexistent id
// 0 and never touching the top registered id — combined with
// pointAdd((0,0), δ) being absorbing, an uninitialized top-account slot
// would silently swallow every future credit on the first transfer of a
// new epoch that omitted it.
//
// Direct code inspection (this session) confirms the fix: the C-05
// rewrite of _updateBalancesForTransfer replaced its own 0-based loop
// with a call to the shared, already-1-based _propagateBalancesExcept(0)
// — the function's own comment says so explicitly ("H-03's off-by-one
// lived in a since-removed 0-based copy of it"). This test is the live
// confirmation: it registers a 7th account beyond transfer()'s fixed
// 6-participant set and confirms that account's balance is NOT silently
// destroyed when a 6-participant transfer crosses an epoch boundary
// without it.
//
// Note on exact reachability: addVerifier (Enygma.sol:748-752) always
// registers under _transferVerifiers[DEFAULT_SIZE] (6) — there is no way
// to register a transfer verifier for any other participant count. So
// the audit's precise precondition ("a transfer omitting a participant")
// is not merely something no shipped client does — it is unreachable via
// the contract's own verifier-registration design, a stronger mitigation
// than the audit itself identified. This test instead exercises the
// SHARED underlying mechanism (_propagateBalancesExcept, used
// identically by transfer/transferWithFee AND withdraw/deposit) via its
// transfer-path caller, with a 7th registered account standing in for
// "the top of the registered range" — the exact boundary condition H-03
// was about — mirroring TestH15NonParticipantSurvivesRollover's already-
// passing equivalent for the withdraw path.
//
// Built on c04Setup (not freshSetup): transfer() also runs C-04's
// _verifyFingerprints, which requires every pairwise fingerprint among
// the 6 PARTICIPANTS mutually confirmed — that requires each of them to
// have its own distinct on-chain address (registerFingerprint resolves
// the caller via addressToAccountId[msg.sender], which collapses to one
// id if every bank shares freshSetup's single owner address). The 7th
// bystander account is never a participant, so its own address doesn't
// need to be distinct.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH03 -v -timeout 60s

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

func TestH03_TopAccountSurvivesTransferRollover(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client) // registers accountIds 1..6 under 6 distinct addresses, MockTransferVerifier wired

	// Register a 7th account beyond transfer()'s fixed 6-participant set
	// — the bystander at the top of the registered range. Never a
	// participant, so it can share the owner's address; only its own
	// balance matters here, not its ability to register a fingerprint.
	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))

	mkAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
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

	const bystanderId = 7
	bystanderSk := big.NewInt(888888)
	bystanderPk, err := poseidon.Hash([]*big.Int{bystanderSk, bystanderSk})
	if err != nil {
		t.Fatalf("bystander pk: %v", err)
	}
	bystanderPk.Mod(bystanderPk, curveP)
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
	if balBefore.X.Sign() == 0 && balBefore.Y.Cmp(big.NewInt(1)) == 0 {
		t.Fatal("test setup bug: bystander balance is already the neutral element before the transfer")
	}

	// Confirm all C(6,2)=15 pairwise fingerprints among the 6 real
	// participants (C-04's requirement — see buildTransferSignal/
	// TestC04_HonestTransferSucceedsWithConfirmedFingerprints for the
	// identical pattern).
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
			fingerprints[i][j] = big.NewInt(int64(30000*(lo+1) + (hi + 1)))
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
	t.Log("all 15 pairwise fingerprints confirmed among the 6 real participants")

	// Mine past the epoch boundary (epochInterval=30, per c04Setup calling
	// Initialize(30)) so the upcoming transfer() call lands in a fresh
	// epoch slot — the exact condition H-03's harm required ("that
	// transfer being the first of a new epoch").
	rpcClient, err := rpc.Dial(chainURL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	mineBlocks(t, rpcClient, 31)

	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 555)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	blockHashBefore := pubSig[36]
	tx, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "")
	if sendErr != nil {
		t.Fatalf("honest 6-participant transfer() reverted — setup problem, not what this test is checking: %v", sendErr)
	}
	if r, err := bind.WaitMined(context.Background(), client, tx); err != nil || r.Status != 1 {
		t.Fatalf("transfer tx failed: status=%v err=%v", r, err)
	}

	newLastBlockNum, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash after: %v", err)
	}
	if newLastBlockNum.Cmp(blockHashBefore) == 0 {
		t.Fatalf("test setup bug: lastBlockNum did not advance (%s) — the call didn't cross an epoch boundary", newLastBlockNum)
	}
	t.Logf("lastBlockNum advanced %s -> %s — epoch rollover confirmed", blockHashBefore, newLastBlockNum)

	balAfter, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(bystanderId))
	if err != nil {
		t.Fatalf("getBalance after: %v", err)
	}
	if balAfter.X.Cmp(balBefore.X) != 0 || balAfter.Y.Cmp(balBefore.Y) != 0 {
		t.Fatalf("FAIL (H-03 regressed): bystander's balance changed across a transfer() that never named accountId=%d — "+
			"before (%s, %s) after (%s, %s)", bystanderId, balBefore.X, balBefore.Y, balAfter.X, balAfter.Y)
	}
	t.Log("bystander's balance (the top of the registered range) is unchanged across the epoch rollover — H-03 correctly fixed")

	if ok, err := instance.Check(&bind.CallOpts{}); err != nil || !ok {
		t.Fatalf("check() invariant broken after the rollover: ok=%v err=%v", ok, err)
	}
	t.Log("check() invariant holds after the rollover")
}
