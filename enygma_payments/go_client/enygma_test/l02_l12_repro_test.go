package enygma_test

// TestL02_*, TestL12_* reproduce and verify the L-02 and L-12 fixes
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md):
//
//   L-02 — registerAccount was missing whenInitialized (every other
//     participant mutator has it). Calling it before initialize() let
//     the initial commitment be added to totalSupply's pre-initialize
//     (0,0) value — the absorbing element for pointAdd — silently
//     discarding it, and initialize() then overwrites totalSupply
//     regardless, permanently losing it with no recovery function.
//   L-12 — withdraw()/deposit() called out to the external, owner-
//     configured ZkDvp bridge *before* updating their own state, with no
//     reentrancy guard anywhere in the contract. Fixed: state now
//     updates first, and a nonReentrant guard was added regardless (the
//     owner-configured bridge means no hostile callee is reachable in
//     the intended deployment today, but the audit's own remediation
//     says this is "cheap, and should not wait for the bridge to be
//     wired").
//
// Run:
//
//	CC=/usr/bin/clang go test -run "TestL02|TestL12" -v -timeout 60s

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestL02_RegisterAccountRequiresInitialization(t *testing.T) {
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

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	// Deliberately do NOT call Initialize() first.
	cx, cy := regCommit(big.NewInt(1))
	pk := big.NewInt(12345)
	_, sendErr := instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(1), pk, cx, cy, []byte{})
	if sendErr == nil {
		t.Fatal("FAIL (L-02 regressed): registerAccount() succeeded before initialize()")
	}
	if !strings.Contains(sendErr.Error(), "NotInitialized") {
		t.Fatalf("registerAccount() before initialize() reverted, but not with NotInitialized: %v", sendErr)
	}
	t.Logf("registerAccount() correctly rejected before initialize(): %v", sendErr)

	// Control: after Initialize(), the same call succeeds.
	initTx, err := instance.Initialize(mkAuth())
	if err != nil {
		t.Fatalf("initialize(): %v", err)
	}
	if r, err := bind.WaitMined(context.Background(), client, initTx); err != nil || r.Status != 1 {
		t.Fatalf("initialize() failed: err=%v status=%v", err, r)
	}
	tx, sendErr := instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(1), pk, cx, cy, []byte{})
	if sendErr != nil {
		t.Fatalf("registerAccount() after initialize() should succeed: %v", sendErr)
	}
	if r, err := bind.WaitMined(context.Background(), client, tx); err != nil || r.Status != 1 {
		t.Fatalf("registerAccount() after initialize() reverted: err=%v status=%v", err, r)
	}
	t.Log("registerAccount() succeeds after initialize() ✓")
}

// maliciousZkDvpABIJSON is the minimal ABI needed to call setTarget on
// MaliciousZkDvp.sol — hand-written rather than loaded from the compiled
// artifact, since only this one function is ever called from Go.
const maliciousZkDvpABIJSON = `[{"inputs":[{"internalType":"address","name":"_enygma","type":"address"},{"internalType":"bytes","name":"_reentryCalldata","type":"bytes"}],"name":"setTarget","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

func TestL12_WithdrawRejectsReentrantCall(t *testing.T) {
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

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}
	waitTx(instance.Initialize(mkAuth()))

	verifierAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/EnygmaVerifier.sol/Verifier.json")
	if r := waitTx(instance.AddVerifier(mkAuth(), verifierAddr)); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}

	// Register the 6 DEFAULT_SIZE banks (mirrors freshSetup, done inline
	// here so this test keeps enygmaAddr in scope).
	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		r := big.NewInt(senderPrevR)
		if i == senderIdx {
			r = big.NewInt(senderRegR)
		}
		cx, cy := regCommit(r)
		if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], cx, cy, []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}

	mockWithdrawVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockWithdrawVerifierAddr, big.NewInt(nBanks))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}
	maliciousZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MaliciousZkDvp.sol/MaliciousZkDvp.json")
	if r := waitTx(instance.AddZkDvp(mkAuth(), maliciousZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp(malicious) failed")
	}
	// withdraw() is onlyRegistered, checked BEFORE nonReentrant in the
	// modifier list — msg.sender for the reentrant inner call is
	// MaliciousZkDvp's own address, so it must itself be registered or
	// the inner call reverts NotRegistered() before ever reaching the
	// reentrancy guard this test is actually about. Register it as a 7th
	// account purely so the reentrant call gets far enough to prove the
	// point.
	const maliciousAccountId = 7
	mcx, mcy := regCommit(big.NewInt(1))
	if r := waitTx(instance.RegisterAccount(mkAuth(), maliciousZkDvpAddr, big.NewInt(maliciousAccountId), big.NewInt(1), mcx, mcy, []byte{})); r.Status != 1 {
		t.Fatal("registering MaliciousZkDvp as an account failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]

	// Zero-delta withdraw() — the outer call must still pass every real
	// check (domain, block number, nullifier, C-09) to reach
	// _executeZkDvpDeposits, since Fix L-12 moved that external call to
	// the very end of withdraw()'s body; only the reentrant INNER call
	// (same proof, replayed mid-execution) is what nonReentrant actually
	// blocks. This mirrors H-15's "nothing happens" pattern otherwise.
	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		pubSig[24+2*i] = big.NewInt(0)
		pubSig[24+2*i+1] = big.NewInt(1)
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = new(big.Int).SetBytes(crypto.Keccak256([]byte("l12-reentrancy")))
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	depositParams := []enygma.IEnygmaDepositParams{
		{Amount: big.NewInt(0), Erc20Adress: common.Address{}, PublicKey: big.NewInt(0)},
	}

	// Arm the malicious contract with calldata for a second withdraw()
	// call it will attempt mid-execution of the first.
	enygmaABI, err := enygma.EnygmaMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse Enygma ABI: %v", err)
	}
	reentryCalldata, err := enygmaABI.Pack("withdraw", commitmentDeltas, proof, depositParams, participantIds)
	if err != nil {
		t.Fatalf("pack reentrant withdraw() calldata: %v", err)
	}

	maliciousABI, err := abi.JSON(strings.NewReader(maliciousZkDvpABIJSON))
	if err != nil {
		t.Fatalf("parse MaliciousZkDvp ABI: %v", err)
	}
	maliciousBound := bind.NewBoundContract(maliciousZkDvpAddr, maliciousABI, client, client, client)
	if r := waitTx(maliciousBound.Transact(mkAuth(), "setTarget", enygmaAddr, reentryCalldata)); r.Status != 1 {
		t.Fatal("setTarget failed")
	}

	// The outer call: withdraw() -> _executeZkDvpDeposits ->
	// MaliciousZkDvp.depositThroughEnygma -> reenter withdraw() -> must
	// hit nonReentrant and revert, which then reverts the whole outer call.
	_, sendErr := instance.Withdraw(mkAuth(), commitmentDeltas, proof, depositParams, participantIds)
	if sendErr == nil {
		t.Fatal("FAIL (L-12 regressed): a reentrant withdraw() call was accepted")
	}
	if !strings.Contains(sendErr.Error(), "ReentrancyGuardReentrantCall") {
		t.Fatalf("outer withdraw() reverted, but not with ReentrancyGuardReentrantCall: %v", sendErr)
	}
	t.Logf("outer withdraw() correctly reverted on a reentrant inner call: %v", sendErr)
}
