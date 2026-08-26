package enygma_test

// TestC04* reproduces and verifies the fix for C-04
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Critical/LIVE): each participant's
// blinding-factor shift is derived from SharedSecrets[i], a private
// witness the SENDER alone chooses. Before this fix, nothing on-chain ever
// compared FingerPrintofSharedSecrets against anything the recipient
// controls, so any sender could permanently freeze any other registered
// account by fabricating a shared secret the victim never agreed to.
//
// The fix adds a two-party fingerprint registry (registerFingerprint) and
// requires every pairwise fingerprint among a transfer's participants to
// be mutually confirmed AND to match the proof's public signal exactly
// (_verifyFingerprints, wired into transfer()).
//
// Uses MockTransferVerifier (always "verifies") since _verifyFingerprints
// only inspects public_signal content, not proof validity — the real
// transfer circuit's prover route needs go_client's full Poseidon/
// Pedersen witness machinery, disproportionate for testing contract-side
// registry logic. Uses six DISTINCT bank addresses (unlike freshSetup,
// which registers every bank under the one shared owner address) because
// this is inherently a multi-party mechanism: testing it meaningfully
// requires each bank to be able to act as itself on-chain.

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// c04Bank is one of six distinct on-chain identities used by these tests.
type c04Bank struct {
	key       *ecdsa.PrivateKey
	addr      common.Address
	accountID int64
}

// c04Setup deploys fresh Enygma + MockTransferVerifier, registers six
// banks under six DISTINCT addresses (funded from the owner account), and
// mints to bank 0. Returns the bound instance and the six banks.
func c04Setup(t *testing.T, client *ethclient.Client) (*enygma.Enygma, []c04Bank, common.Address) {
	t.Helper()
	ctx := context.Background()

	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))
	ownerAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(ctx)
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
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			t.Fatalf("wait mined: %v", err)
		}
		if r.Status != 1 {
			t.Fatalf("tx reverted: %s", tx.Hash())
		}
		return r
	}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, ownerAuth(),
		artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	mockVerifierAddr := deployFromArtifact(t, client, ownerAuth(),
		artifactBase+"/mocks/MockTransferVerifier.sol/MockTransferVerifier.json")

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}
	waitTx(instance.Initialize(ownerAuth()))
	waitTx(instance.AddVerifier(ownerAuth(), mockVerifierAddr))

	banks := make([]c04Bank, nBanks)
	for i := 0; i < nBanks; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("generate bank %d key: %v", i, err)
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		banks[i] = c04Bank{key: key, addr: addr, accountID: int64(i + 1)}

		// Fund with enough gas for a handful of transactions.
		gasPrice, _ := client.SuggestGasPrice(ctx)
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		fundTx := ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    nonce,
			To:       &addr,
			Value:    new(big.Int).SetUint64(50_000_000_000_000_000), // 0.05 ETH
			Gas:      21000,
			GasPrice: gasPrice,
		})
		signedFundTx, err := ethtypes.SignTx(fundTx, ethtypes.NewEIP155Signer(big.NewInt(chainID)), ownerKey)
		if err != nil {
			t.Fatalf("sign funding tx: %v", err)
		}
		if err := client.SendTransaction(ctx, signedFundTx); err != nil {
			t.Fatalf("fund bank %d: %v", i, err)
		}
		if _, err := bind.WaitMined(ctx, client, signedFundTx); err != nil {
			t.Fatalf("wait funding mined: %v", err)
		}

		pk, err := poseidon.Hash([]*big.Int{big.NewInt(int64(1000 + i)), big.NewInt(int64(1000 + i))})
		if err != nil {
			t.Fatalf("pk[%d]: %v", i, err)
		}
		pk.Mod(pk, curveP)
		// Fix H-02 residual: registerAccount takes a pre-computed
		// commitment now. buildTransferSignal reads balances directly off
		// chain (not by independently recomputing them from a known R),
		// so unlike freshSetup's callers, no specific R value needs to be
		// preserved here — reusing senderPrevR for every bank's
		// registration blinding is fine.
		cx, cy := regCommit(big.NewInt(senderPrevR))
		waitTx(instance.RegisterAccount(ownerAuth(), addr, big.NewInt(banks[i].accountID), pk, cx, cy, []byte{}))
	}

	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	waitTx(instance.MintSupply(ownerAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy))
	return instance, banks, enygmaAddr
}

// bankAuth builds a TransactOpts for one of the six distinct bank keys.
func bankAuth(t *testing.T, client *ethclient.Client, bank c04Bank) *bind.TransactOpts {
	t.Helper()
	ctx := context.Background()
	nonce, err := client.PendingNonceAt(ctx, bank.addr)
	if err != nil {
		t.Fatalf("nonce for %s: %v", bank.addr, err)
	}
	gasPrice, _ := client.SuggestGasPrice(ctx)
	auth, err := bind.NewKeyedTransactorWithChainID(bank.key, big.NewInt(chainID))
	if err != nil {
		t.Fatalf("auth for %s: %v", bank.addr, err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = 16_000_000
	auth.GasPrice = gasPrice
	return auth
}

func TestC04_RegisterFingerprint_RequiresBothParties(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, _ := c04Setup(t, client) // domain separator unused: no transfer proof submitted
	a, b := banks[0], banks[1]
	fp := big.NewInt(424242)

	// Unilateral submission from A alone must not confirm.
	txA, err := instance.RegisterFingerprint(bankAuth(t, client, a), big.NewInt(b.accountID), fp)
	if err != nil {
		t.Fatalf("A registerFingerprint: %v", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, txA); err != nil {
		t.Fatalf("wait A tx: %v", err)
	}
	confirmed, err := instance.FingerprintConfirmed(&bind.CallOpts{}, big.NewInt(a.accountID), big.NewInt(b.accountID))
	if err != nil {
		t.Fatalf("check confirmed: %v", err)
	}
	if confirmed {
		t.Fatal("FAIL: unilateral fingerprint submission confirmed with no counterpart")
	}
	t.Log("A's solo submission correctly leaves the pair unconfirmed")

	// A mismatched submission from B must not confirm either.
	wrongFp := big.NewInt(999999)
	txBWrong, err := instance.RegisterFingerprint(bankAuth(t, client, b), big.NewInt(a.accountID), wrongFp)
	if err != nil {
		t.Fatalf("B registerFingerprint (wrong): %v", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, txBWrong); err != nil {
		t.Fatalf("wait B wrong tx: %v", err)
	}
	confirmed, err = instance.FingerprintConfirmed(&bind.CallOpts{}, big.NewInt(a.accountID), big.NewInt(b.accountID))
	if err != nil {
		t.Fatalf("check confirmed after mismatch: %v", err)
	}
	if confirmed {
		t.Fatal("FAIL: mismatched fingerprint claims confirmed")
	}
	t.Log("mismatched claims (A=424242, B=999999) correctly leave the pair unconfirmed")

	// B resubmitting the matching value must confirm.
	txBRight, err := instance.RegisterFingerprint(bankAuth(t, client, b), big.NewInt(a.accountID), fp)
	if err != nil {
		t.Fatalf("B registerFingerprint (matching): %v", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, txBRight); err != nil {
		t.Fatalf("wait B matching tx: %v", err)
	}
	confirmed, err = instance.FingerprintConfirmed(&bind.CallOpts{}, big.NewInt(a.accountID), big.NewInt(b.accountID))
	if err != nil {
		t.Fatalf("check confirmed after match: %v", err)
	}
	if !confirmed {
		t.Fatal("FAIL: matching mutual submissions did not confirm")
	}
	onChainFp, err := instance.ConfirmedFingerprint(&bind.CallOpts{}, big.NewInt(a.accountID), big.NewInt(b.accountID))
	if err != nil || onChainFp.Cmp(fp) != 0 {
		t.Fatalf("confirmed fingerprint mismatch: got %v err %v, want %v", onChainFp, err, fp)
	}
	// Symmetric: confirmedFingerprint[b][a] must also read back correctly.
	onChainFpRev, err := instance.ConfirmedFingerprint(&bind.CallOpts{}, big.NewInt(b.accountID), big.NewInt(a.accountID))
	if err != nil || onChainFpRev.Cmp(fp) != 0 {
		t.Fatalf("confirmed fingerprint not symmetric: got %v err %v, want %v", onChainFpRev, err, fp)
	}
	t.Log("matching mutual submissions correctly confirm, symmetrically, with the agreed value")
}

// buildTransferSignal constructs an 80-signal public_signal array plus
// matching zero-delta commitmentDeltas for a transfer among all six banks,
// with the FingerPrintofSharedSecrets matrix filled from fingerprints
// (a full 6x6, values ignored on the diagonal). Everything else
// (PublicKey/PreviousCommit read from real on-chain state, BlockNumber
// from lastBlockNum, a fresh Nullifier) is honestly constructed — the mock
// verifier means this test is entirely about _verifyFingerprints, not
// proof validity, so TxCommit is left at the neutral element throughout.
func buildTransferSignal(t *testing.T, instance *enygma.Enygma, enygmaAddr common.Address, fingerprints [nBanks][nBanks]*big.Int, nullifierSeed int64) ([81]*big.Int, []enygma.IEnygmaPoint) {
	t.Helper()
	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys := pubVals.Keys[1:]
	balances := pubVals.Balances[1:]

	var pubSig [81]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i != j {
				pubSig[i*nBanks+j] = fingerprints[i][j]
			}
		}
		pubSig[36+i] = keys[i]
		pubSig[42+2*i] = balances[i].C1
		pubSig[42+2*i+1] = balances[i].C2
		pubSig[54+2*i] = big.NewInt(0) // TxCommit = neutral element (0,1): zero delta
		pubSig[54+2*i+1] = big.NewInt(1)
	}
	pubSig[66] = blockHash
	pubSig[79] = new(big.Int).SetInt64(nullifierSeed)
	pubSig[80] = expectedDomainId(enygmaAddr) // Fix L-01

	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := 0; i < nBanks; i++ {
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
	}
	return pubSig, commitmentDeltas
}

func TestC04_TransferRejectsUnconfirmedFingerprint(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)
	// Deliberately register NO fingerprints at all — the attacker (bank 0)
	// has never run key agreement with anyone, exactly the scenario C-04
	// exploited: a victim who never agreed to transact with the sender.
	var fingerprints [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			fingerprints[i][j] = big.NewInt(int64(1000*i + j)) // attacker's own fabricated values
		}
	}
	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 111)

	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (C-04 regressed): transfer() succeeded with zero confirmed fingerprints")
	}
	const wantSelector = "0x4364d19c" // FingerprintNotConfirmed()
	if !strings.Contains(sendErr.Error(), wantSelector) {
		t.Fatalf("transfer() reverted, but not with FingerprintNotConfirmed (%s): %v", wantSelector, sendErr)
	}
	t.Logf("transfer() with zero confirmed fingerprints correctly reverted with FingerprintNotConfirmed: %v", sendErr)
}

func TestC04_HonestTransferSucceedsWithConfirmedFingerprints(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)

	// Confirm all C(6,2)=15 pairwise fingerprints, both directions.
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
			fingerprints[i][j] = big.NewInt(int64(10000*(lo+1) + (hi + 1))) // symmetric per unordered pair
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

	pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fingerprints, 222)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
	}
	proof := enygma.IEnygmaProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	tx, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas, proof, participantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr != nil {
		t.Fatalf("FAIL: honest transfer with all 15 pairs confirmed was rejected: %v", sendErr)
	}
	r, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || r.Status != 1 {
		t.Fatalf("transfer tx failed: status=%v err=%v", r, err)
	}
	t.Log("honest transfer with all pairwise fingerprints confirmed succeeds")
}
