package enygma_test

// TestL01_* reproduces and verifies the L-01 fix
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Low): no circuit or contract signal
// ever bound a proof to a specific chain id or deployed Enygma instance.
// The audit's own PoC: "two fresh deployments in the same epoch... the
// identical proof was accepted by BOTH contracts" — any proof valid for one
// deployment (sharing the same pre-state — trivially true for two freshly
// deployed, identically-registered instances) verified against any other.
//
// The fix adds a domain-separator signal to every circuit
// (chainId << 160 | this contract's own address, a single packed field
// element — see Enygma.sol's _expectedDomainId() doc comment for why no
// Poseidon hashing is involved) and checks it first, before any other
// public-input binding, in every proof-consuming entrypoint.
//
// This test deploys TWO fresh Enygma + MockWithdrawVerifier instances with
// byte-for-byte identical registration (six banks, identical keys and
// registration commitments — the exact "same pre-state" precondition the
// audit's PoC required), builds ONE honest zero-delta withdraw() proof
// against instance A's domain, and confirms:
//   - it succeeds against A (the deployment it was built for)
//   - the IDENTICAL proof is rejected by B with InvalidDomain() — the
//     replay the audit demonstrated is no longer possible
//   - a proof rebuilt with B's own domain succeeds against B, and is in
//     turn rejected by A — confirming the check is symmetric, not a
//     one-sided artifact of deployment order
//
// Uses MockWithdrawVerifier (always "verifies") so this test is entirely
// about Enygma.sol's own domain-binding logic — independent of the real
// withdraw circuit's proving pipeline (M-03, broken independently, see
// gnark-server/pkg/circuits/withdraw/handler.go's Fix M-03 comment).
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestL01 -v -timeout 60s

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

// l01Deployment is one fresh, fully-registered Enygma instance.
type l01Deployment struct {
	addr      common.Address
	instance  *enygma.Enygma
	keys6     []*big.Int
	balances6 []enygma.IEnygmaPoint
	blockHash *big.Int
}

// l01DeployAndRegister deploys a fresh Enygma + MockWithdrawVerifier and
// registers the 6 DEFAULT_SIZE banks with IDENTICAL keys/commitments every
// time it's called — the "same pre-state" precondition the audit's PoC
// relied on to make one proof valid against two different deployments.
func l01DeployAndRegister(t *testing.T, client *ethclient.Client, mkAuth func() *bind.TransactOpts,
	waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt, ownerAddr common.Address) l01Deployment {
	t.Helper()

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}
	if r := waitTx(instance.Initialize(mkAuth())); r.Status != 1 {
		t.Fatal("initialize failed")
	}

	mockWithdrawVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockWithdrawVerifierAddr, big.NewInt(nBanks))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}

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
		if rc := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], cx, cy, []byte{})); rc.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}

	return l01Deployment{
		addr:      enygmaAddr,
		instance:  instance,
		keys6:     pubVals.Keys[1:],
		balances6: pubVals.Balances[1:],
		blockHash: blockHash,
	}
}

// l01BuildWithdrawProof builds an honest, zero-delta withdraw() proof
// against dep's own on-chain state, with the domain slot set to
// expectedDomainId(domainAddr) — the caller controls domainAddr
// independently of dep so this can build a proof deliberately carrying the
// WRONG domain (any other deployment's address) to prove the check
// actually rejects it, not just that a correctly-domained proof happens to
// pass.
func l01BuildWithdrawProof(dep l01Deployment, domainAddr common.Address, nullifierSeed string) (enygma.IEnygmaWithdrawProof, []enygma.IEnygmaPoint, []*big.Int) {
	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = dep.keys6[i]
		pubSig[12+2*i] = dep.balances6[i].C1
		pubSig[12+2*i+1] = dep.balances6[i].C2
		pubSig[24+2*i] = big.NewInt(0) // zero-delta: neutral element
		pubSig[24+2*i+1] = big.NewInt(1)
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = dep.blockHash
	pubSig[49] = new(big.Int).SetBytes(crypto.Keccak256([]byte(nullifierSeed)))
	pubSig[51] = expectedDomainId(domainAddr) // Fix L-01

	proof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	return proof, commitmentDeltas, participantIds
}

// TestL01_CrossDeploymentReplayRejected reproduces the audit's own PoC
// shape directly: a proof built and valid for deployment A must be
// rejected by deployment B, even though B shares A's exact pre-state
// (identical registration, identical balances/keys, identical block hash —
// the scenario that let the pre-fix contract accept the identical proof on
// both).
func TestL01_CrossDeploymentReplayRejected(t *testing.T) {
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

	// Two fresh, byte-for-byte identically-registered deployments — the
	// audit's own "same pre-state" precondition.
	depA := l01DeployAndRegister(t, client, mkAuth, waitTx, ownerAddr)
	depB := l01DeployAndRegister(t, client, mkAuth, waitTx, ownerAddr)
	if depA.addr == depB.addr {
		t.Fatal("test setup bug: A and B deployed to the same address")
	}
	for i := range depA.keys6 {
		if depA.keys6[i].Cmp(depB.keys6[i]) != 0 ||
			depA.balances6[i].C1.Cmp(depB.balances6[i].C1) != 0 ||
			depA.balances6[i].C2.Cmp(depB.balances6[i].C2) != 0 {
			t.Fatalf("test setup bug: A and B pre-state diverges at bank %d — not the scenario this test needs", i)
		}
	}
	t.Logf("A=%s B=%s — identical pre-state confirmed (keys, balances, block hash)", depA.addr.Hex(), depB.addr.Hex())

	// A proof built and domain-tagged for A...
	proofA, deltasA, idsA := l01BuildWithdrawProof(depA, depA.addr, "l01-replay-a")

	// ...succeeds against A.
	if r := waitTx(depA.instance.Withdraw(mkAuth(), deltasA, proofA, []enygma.IEnygmaDepositParams{}, idsA)); r.Status != 1 {
		t.Fatal("honest withdraw against A (correct domain) reverted — setup problem, not what this test checks")
	}
	t.Log("proof domain-tagged for A succeeds against A ✓")

	// The IDENTICAL proof, replayed against B: pre-fix, this is exactly
	// what the audit demonstrated succeeding. Post-fix it must not.
	_, sendErr := depB.instance.Withdraw(mkAuth(), deltasA, proofA, []enygma.IEnygmaDepositParams{}, idsA)
	if sendErr == nil {
		t.Fatal("FAIL (L-01 regressed): a proof built for deployment A was accepted by deployment B")
	}
	if !strings.Contains(sendErr.Error(), "InvalidDomain") {
		t.Fatalf("cross-deployment replay reverted, but not with InvalidDomain: %v", sendErr)
	}
	t.Logf("the IDENTICAL proof replayed against B correctly reverted with InvalidDomain(): %v", sendErr)

	// Symmetry check: this isn't a one-sided artifact of deployment
	// order — a proof domain-tagged for B must equally be rejected by A.
	proofB, deltasB, idsB := l01BuildWithdrawProof(depB, depB.addr, "l01-replay-b")
	if r := waitTx(depB.instance.Withdraw(mkAuth(), deltasB, proofB, []enygma.IEnygmaDepositParams{}, idsB)); r.Status != 1 {
		t.Fatal("honest withdraw against B (correct domain) reverted — setup problem, not what this test checks")
	}
	t.Log("proof domain-tagged for B succeeds against B ✓")

	_, sendErr2 := depA.instance.Withdraw(mkAuth(), deltasB, proofB, []enygma.IEnygmaDepositParams{}, idsB)
	if sendErr2 == nil {
		t.Fatal("FAIL (L-01 regressed): a proof built for deployment B was accepted by deployment A")
	}
	if !strings.Contains(sendErr2.Error(), "InvalidDomain") {
		t.Fatalf("reverse cross-deployment replay reverted, but not with InvalidDomain: %v", sendErr2)
	}
	t.Logf("the reverse replay (B's proof against A) correctly reverted with InvalidDomain(): %v — domain check confirmed symmetric", sendErr2)
}
