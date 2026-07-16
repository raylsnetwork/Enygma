package enygma_test

// TestFullTransactionFlow — end-to-end integration test:
//   register banks → mint → generate ZK proof → submit Transfer → verify balances.
//
// Prerequisites (all must be running before executing):
//   1. Ganache/Hardhat node on localhost:8545, contracts already deployed:
//      cd enygma_payments/run_scripts && python deploy.py  (or equivalent)
//   2. Gnark server on localhost:8080:
//      cd enygma_payments/gnark-server && go run ./cmd/server/main.go
//
// Contract addresses are hardcoded from deploy_receipts.json.
//
// IMPORTANT: Run on a FRESH chain state. After one successful run the sender's
// Pedersen commitment randomness changes — restart the node before re-running.
//
// Run:
//
//	cd go_client/enygma_test && CC=/usr/bin/clang go test -run TestFullTransactionFlow -v -timeout 300s

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── Curve constants (mirrors go_client/internal/curve/curve.go) ──────────────

var (
	curveGx, _ = new(big.Int).SetString("16540640123574156134436876038791482806971768689494387082833631921987005038935", 10)
	curveGy, _ = new(big.Int).SetString("20819045374670962167435360035096875258406992893633759881276124905556507972311", 10)
	curveG     = &babyjub.Point{X: curveGx, Y: curveGy}

	curveHx, _ = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	curveHy, _ = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	curveH     = &babyjub.Point{X: curveHx, Y: curveHy}

	// Baby Jubjub subgroup order
	curveP, _ = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
)

// pedersenCommitment computes v*G + r*H on Baby Jubjub.
func pedersenCommitment(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, curveG)
	rH := babyjub.NewPoint().Mul(r, curveH)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

// addBJPoints adds two Baby Jubjub points.
func addBJPoints(a, b *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(a.Projective(), b.Projective()).Affine()
}

// negMod returns (P - x) mod P (additive inverse in the scalar field).
func negMod(x *big.Int) *big.Int {
	return new(big.Int).Sub(curveP, new(big.Int).Mod(x, curveP))
}

// ── Randomness helpers (mirrors go_client/internal/randomness/operation.go) ──

var hashRandom *big.Int // Poseidon(21) — computed once
var hashTag *big.Int    // Poseidon(12) — computed once

func init() {
	hashRandom, _ = poseidon.Hash([]*big.Int{big.NewInt(21)})
	hashTag, _ = poseidon.Hash([]*big.Int{big.NewInt(12)})
}

// hashArrayGen computes Poseidon(s, s) mod P for each secret s.
func hashArrayGen(secrets []*big.Int) []*big.Int {
	out := make([]*big.Int, len(secrets))
	for i, s := range secrets {
		h, _ := poseidon.Hash([]*big.Int{s, s})
		out[i] = h.Mod(h, curveP)
	}
	return out
}

// tagMessageGen computes Poseidon(HashTag, s[i], blockHash) mod P for each bank.
func tagMessageGen(secrets []*big.Int, blockHash *big.Int) []*big.Int {
	bh := new(big.Int).Mod(blockHash, curveP)
	out := make([]*big.Int, len(secrets))
	for i, s := range secrets {
		h, _ := poseidon.Hash([]*big.Int{hashTag, s, bh})
		out[i] = h.Mod(h, curveP)
	}
	return out
}

// rValue computes Poseidon(HashRandom, s, blockHash) mod P for bank i.
func rValue(s, blockHash *big.Int) *big.Int {
	h, _ := poseidon.Hash([]*big.Int{hashRandom, s, blockHash})
	return h.Mod(h, curveP)
}

// genCommitmentAndRandom mirrors randomness.GenCommitmentAndRandom.
// Returns (txCommit, txRandom) where txRandom[sender] = sum of receiver randoms,
// txRandom[receiver] = negated random (matches circuit expectation).
func genCommitmentAndRandom(senderId int, transferValue *big.Int, txValues []*big.Int, blockHash *big.Int, secrets []*big.Int) ([]enygma.IEnygmaPoint, []*big.Int) {
	n := len(secrets)
	rValues := make([]*big.Int, n)
	rSum := new(big.Int)

	for i := 0; i < n; i++ {
		r := rValue(secrets[i], blockHash)
		rValues[i] = r
		if i != senderId {
			rSum.Add(rSum, r)
			rSum.Mod(rSum, curveP)
		}
	}
	rValues[senderId] = rSum

	commits := make([]enygma.IEnygmaPoint, n)
	txRandom := make([]*big.Int, n)
	for i := 0; i < n; i++ {
		var pt *babyjub.Point
		if i == senderId {
			// sender: Com(-value, rSum)
			pt = pedersenCommitment(negMod(transferValue), rSum)
			txRandom[i] = rSum
		} else {
			// receiver: Com(txValues[i], -r[i])
			pt = pedersenCommitment(txValues[i], negMod(rValues[i]))
			txRandom[i] = negMod(rValues[i]) // negated for circuit
		}
		commits[i] = enygma.IEnygmaPoint{C1: pt.X, C2: pt.Y}
	}
	return commits, txRandom
}

// ── Test constants ─────────────────────────────────────────────────────────────

// chainURL and chainID are configurable via environment variables so the same
// test suite runs against both a local Hardhat node and Rayls mainnet.
//
// Local Hardhat:
//
//	export ENYGMA_CHAIN_URL=http://127.0.0.1:8545
//	export ENYGMA_CHAIN_ID=31337
//	export MY_KEY=ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
//
// Rayls mainnet (default, no env vars needed):
//
//	export MY_KEY=<your-mainnet-key>
var (
	chainURL = func() string {
		if u := os.Getenv("ENYGMA_CHAIN_URL"); u != "" {
			return u
		}
		return "https://mainnet-rpc.rayls.com"
	}()
	chainID = func() int64 {
		if s := os.Getenv("ENYGMA_CHAIN_ID"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n
			}
		}
		return 72957
	}()
)

const (
	gnarkURL   = "http://127.0.0.1:8080/proof/enygma"
	relayerURL = "http://127.0.0.1:8082"
	relayerKey = "enygma-test-secret" // must match RELAYER_API_KEY

	nBanks      = 6
	senderIdx   = 0
	transferAmt = 100
	mintAmt     = 500

	// Bank 0 credentials. prevR is the randomness used at registration;
	// prevV is the minted amount. These fix the Pedersen commitment to Com(500, 67890).
	senderSk    = 424242
	senderPrevR = 67890
	senderPrevV = mintAmt
)

// ownerPrivKey is loaded from the MY_KEY environment variable at test startup.
// Never hardcode a real key here — set:  export MY_KEY=<your-hex-key>
var ownerPrivKey = func() string {
	if k := os.Getenv("MY_KEY"); k != "" {
		return k
	}
	return "" // tests will fail with a clear "MY_KEY not set" message via mustPrivKey
}()

// receipts holds contract addresses read from deploy_receipts.json.
type receipts struct {
	TOKEN    struct{ ContractAddress string `json:"contractAddress"` } `json:"TOKEN"`
	VERIFIER struct{ ContractAddress string `json:"contractAddress"` } `json:"VERIFIER"`
}

// readReceipts reads deploy_receipts.json from run_scripts/build/enygma/web3/.
func readReceipts(t *testing.T) (tokenAddr, verifierAddr string) {
	t.Helper()
	// Resolve path relative to this file's location.
	_, testFile, _, _ := runtime.Caller(0)
	receiptsPath := filepath.Join(filepath.Dir(testFile), "..", "..", "run_scripts", "build", "enygma", "web3", "deploy_receipts.json")
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		t.Skipf("deploy_receipts.json not found at %s — run the Python deploy scripts first:\n  cd enygma_payments/run_scripts && python deploy_enygma.py ...\n(err: %v)", receiptsPath, err)
	}
	var r receipts
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parse deploy_receipts.json: %v", err)
	}
	if r.TOKEN.ContractAddress == "" || r.VERIFIER.ContractAddress == "" {
		t.Fatal("deploy_receipts.json is missing TOKEN or VERIFIER address — re-run deploy scripts")
	}
	return r.TOKEN.ContractAddress, r.VERIFIER.ContractAddress
}

// Pre-shared secrets for banks 1-5 (from initializeSecrets in transaction/main.go).
var baseSecrets = []*big.Int{
	nil, // bank 0: derived at runtime from (prevR, sk)
	big.NewInt(54142),
	big.NewInt(814712),
	big.NewInt(250912012),
	big.NewInt(12312512),
	big.NewInt(12312512),
}

// Secret keys per bank (bank 0 uses senderSk).
var bankSks = []*big.Int{
	big.NewInt(senderSk),
	big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5),
}

func TestFullTransactionFlow(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at localhost:8080 — start gnark-server first")
	}
	if !tcpAvailable("127.0.0.1:8082") {
		t.Skip("relayer not reachable at localhost:8082 — start the relayer first")
	}

	// Read contract addresses from deploy_receipts.json (updated by deploy scripts).
	tokenAddr, verifierAddr := readReceipts(t)
	t.Logf("TOKEN:    %s", tokenAddr)
	t.Logf("VERIFIER: %s", verifierAddr)

	// ── Ethereum client + auth factory ────────────────────────────────────────
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial chain: %v", err)
	}
	defer client.Close()

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	t.Logf("owner: %s", ownerAddr.Hex())

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

	// ── Contract instance ──────────────────────────────────────────────────────
	instance, err := enygma.NewEnygma(common.HexToAddress(tokenAddr), client)
	if err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// ── Setup: initialize (idempotent — AlreadyInitialized is OK) ────────────
	if tx, txErr := instance.Initialize(mkAuth()); txErr == nil {
		if r, _ := bind.WaitMined(context.Background(), client, tx); r != nil && r.Status == 1 {
			t.Log("contract initialized")
		}
	}

	// ── Setup: register verifier ───────────────────────────────────────────────
	if r := waitTx(instance.AddVerifier(mkAuth(), common.HexToAddress(verifierAddr))); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}
	t.Log("verifier registered")

	// ── Setup: compute public keys — Poseidon(sk, sk) mod P ──────────────────
	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, err := poseidon.Hash([]*big.Int{sk, sk})
		if err != nil {
			t.Fatalf("pk[%d]: %v", i, err)
		}
		pks[i] = pk.Mod(pk, curveP)
	}

	// Register banks with accountIds 1-6 (avoids onlyRegistered sentinel=0 bug).
	// All use ownerAddr; addressToAccountId is overwritten each call — last value is 6 ≠ 0.
	for i := 0; i < nBanks; i++ {
		r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR)))
		if r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	t.Logf("registered %d banks (accountIds 1–%d)", nBanks, nBanks)

	// ── Setup: mint to bank 0 (accountId=1) ───────────────────────────────────
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

	// ── Read on-chain state ────────────────────────────────────────────────────
	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	t.Logf("lastBlockNum: %s", blockHash)

	// getPublicValues(nBanks+1) returns indices 0..nBanks; our accounts are at 1..nBanks
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	prevBalances := pubVals.Balances[1:] // accounts 1-6 → circuit banks 0-5
	onChainKeys := pubVals.Keys[1:]       // registered pks for accounts 1-6

	t.Logf("bank 0 initial commitment: (%s, %s)", prevBalances[0].C1, prevBalances[0].C2)

	// ── Compute proof inputs ───────────────────────────────────────────────────
	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)

	senderSecret, err := poseidon.Hash([]*big.Int{prevR, sk})
	if err != nil {
		t.Fatalf("senderSecret: %v", err)
	}
	senderSecret.Mod(senderSecret, curveP)

	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = senderSecret

	hashArray := hashArrayGen(secrets)

	// txValues: bank 0 sends 100, bank 1 receives 60, bank 2 receives 40
	txValues := []*big.Int{
		negMod(big.NewInt(transferAmt)), // −100 mod P
		big.NewInt(60),
		big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}

	// GenCommitmentAndRandom mutates blockHash in original — use a copy
	bh := new(big.Int).Set(blockHash)
	tagMessages := tagMessageGen(secrets, bh)

	bh2 := new(big.Int).Set(blockHash)
	txCommit, txRandom := genCommitmentAndRandom(senderIdx, big.NewInt(transferAmt), txValues, bh2, secrets)

	nullifier, err := poseidon.Hash([]*big.Int{hashArray[senderIdx], blockHash})
	if err != nil {
		t.Fatalf("nullifier: %v", err)
	}

	// ── Build gnark server request ─────────────────────────────────────────────
	toStrs := func(vals []*big.Int) []string {
		s := make([]string, len(vals))
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}

	prevCommitSlice := make([][]string, nBanks)
	for i, pt := range prevBalances {
		prevCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}
	txCommitSlice := make([][]string, nBanks)
	for i, pt := range txCommit {
		txCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}

	// public_keys must match on-chain values to pass _verifyPublicInputs.
	keyStrs := make([]string, nBanks)
	for i, k := range onChainKeys {
		keyStrs[i] = k.String()
	}
	kIndex := make([]*big.Int, nBanks)
	for i := range kIndex {
		kIndex[i] = big.NewInt(int64(i))
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"hashed_shared_secrets":        toStrs(hashArray),
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                toStrs(kIndex),
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"sender_id":                    fmt.Sprintf("%d", senderIdx),
		"shared_secrets":               toStrs(secrets),
		"secret_key":                   sk.String(),
		"previous_sender_balance":      fmt.Sprintf("%d", senderPrevV),
		"previous_sender_random_value": prevR.String(),
		"tx_values":                    toStrs(txValues),
		"tx_random_values":             toStrs(txRandom),
		"sender_tx_value":              fmt.Sprintf("%d", transferAmt),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	// ── Call gnark server ──────────────────────────────────────────────────────
	t.Log("requesting proof (may take ~30s)...")
	httpResp, err := http.Post(gnarkURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("gnark POST: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("gnark %d: %s", httpResp.StatusCode, body)
	}

	var proofResp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&proofResp); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	if len(proofResp.Proof) != 8 || len(proofResp.PublicSignal) != 50 {
		t.Fatalf("bad sizes: proof=%d publicSignal=%d", len(proofResp.Proof), len(proofResp.PublicSignal))
	}
	t.Log("proof received")

	// ── Build relay Transfer request ───────────────────────────────────────────
	// commitmentDeltas are the per-bank Pedersen delta points read from the
	// public signal at TX_COMMIT_OFFSET = 6 (hashes) + 6 (pks) + 12 (prevCommit) = 24.
	const txCommitOffset = 24
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	commFinal := make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		c1 := proofResp.PublicSignal[txCommitOffset+2*i]
		c2 := proofResp.PublicSignal[txCommitOffset+2*i+1]
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: c1, C2: c2}
		commFinal[i] = []string{c1.String(), c2.String()}
	}

	var proof8 [8]string
	for i := 0; i < 8; i++ {
		proof8[i] = proofResp.Proof[i].String()
	}

	pubSigStrs := make([]string, len(proofResp.PublicSignal))
	for i, v := range proofResp.PublicSignal {
		pubSigStrs[i] = v.String()
	}

	// participantIds[i] = i+1 (maps circuit bank i → on-chain accountId i+1)
	kIdx64 := make([]int64, nBanks)
	for i := range kIdx64 {
		kIdx64[i] = int64(i + 1)
	}

	relayReq := struct {
		Proof        [8]string  `json:"proof"`
		PublicSignal []string   `json:"publicSignal"`
		Commitments  [][]string `json:"commitments"`
		KIndex       []int64    `json:"kIndex"`
	}{
		Proof:        proof8,
		PublicSignal: pubSigStrs,
		Commitments:  commFinal,
		KIndex:       kIdx64,
	}

	// ── Submit Transfer via relayer ────────────────────────────────────────────
	t.Log("submitting Transfer via relayer...")
	relayBody, err := json.Marshal(relayReq)
	if err != nil {
		t.Fatalf("marshal relay request: %v", err)
	}

	apiKey := os.Getenv("RELAYER_API_KEY")
	if apiKey == "" {
		apiKey = relayerKey
	}

	relayHTTPReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/transfer", bytes.NewReader(relayBody))
	if err != nil {
		t.Fatalf("build relay request: %v", err)
	}
	relayHTTPReq.Header.Set("Content-Type", "application/json")
	relayHTTPReq.Header.Set("Authorization", "Bearer "+apiKey)

	relayHTTPResp, err := http.DefaultClient.Do(relayHTTPReq)
	if err != nil {
		t.Fatalf("relay POST: %v", err)
	}
	defer relayHTTPResp.Body.Close()

	relayRespBody, _ := io.ReadAll(relayHTTPResp.Body)
	if relayHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", relayHTTPResp.StatusCode, relayRespBody)
	}

	var relayResp struct {
		TxHash      string `json:"txHash"`
		BlockNumber uint64 `json:"blockNumber"`
		GasUsed     uint64 `json:"gasUsed"`
	}
	if err := json.Unmarshal(relayRespBody, &relayResp); err != nil {
		t.Fatalf("parse relay response: %v", err)
	}
	t.Logf("Transfer succeeded via relayer: tx=%s block=%d gas=%d",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)

	// ── Verify: bank 0 balance changed ────────────────────────────────────────
	newBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("getBalance(1): %v", err)
	}
	t.Logf("bank 0 new commitment: (%s, %s)", newBal.X, newBal.Y)

	if newBal.X.Cmp(prevBalances[0].C1) == 0 && newBal.Y.Cmp(prevBalances[0].C2) == 0 {
		t.Error("bank 0 balance unchanged after transfer")
	}

	// Homomorphic check: newBalance = prevBalance + TxCommit[0]
	prevPt := &babyjub.Point{X: prevBalances[0].C1, Y: prevBalances[0].C2}
	deltaPt := &babyjub.Point{X: commitmentDeltas[0].C1, Y: commitmentDeltas[0].C2}
	expectedPt := addBJPoints(prevPt, deltaPt)

	if newBal.X.Cmp(expectedPt.X) != 0 || newBal.Y.Cmp(expectedPt.Y) != 0 {
		t.Errorf("bank 0 homomorphic check FAILED:\n  got      (%s, %s)\n  expected (%s, %s)",
			newBal.X, newBal.Y, expectedPt.X, expectedPt.Y)
	} else {
		t.Log("bank 0 homomorphic check PASSED (prevBal + txDelta = newBal)")
	}

	// Spot-check bank 1 (accountId=2) received 60 tokens.
	bal1, _ := instance.GetBalance(&bind.CallOpts{}, big.NewInt(2))
	prev1 := &babyjub.Point{X: prevBalances[1].C1, Y: prevBalances[1].C2}
	delta1 := &babyjub.Point{X: commitmentDeltas[1].C1, Y: commitmentDeltas[1].C2}
	expected1 := addBJPoints(prev1, delta1)
	if bal1.X.Cmp(expected1.X) != 0 || bal1.Y.Cmp(expected1.Y) != 0 {
		t.Error("bank 1 homomorphic check FAILED")
	} else {
		t.Log("bank 1 homomorphic check PASSED")
	}
}

// tcpAvailable returns true if addr (host:port) is reachable via TCP.
func tcpAvailable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// chainAvailable probes the configured chainURL via TCP.
// Works for both local Hardhat (http://127.0.0.1:8545) and mainnet (https://...).
func chainAvailable() bool {
	u, err := url.Parse(chainURL)
	if err != nil {
		return false
	}
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	return tcpAvailable(host)
}
