package enygma_test

// TestCostReport performs the complete Enygma deployment-and-transfer pipeline,
// records the on-chain cost (gas used × gas price) of every operation, and prints
// a detailed formatted report at the end. It deploys fresh contracts so that
// deployment costs are captured alongside setup and transfer costs.
//
// Report sections
//   A. Contract Deployment     — Enygma.sol  +  EnygmaVerifier.sol
//   B. Setup Transactions      — initialize, addVerifier, registerAccount×6, mintSupply
//   C. ZK Proof Generation     — time to generate the Groth16 proof off-chain (gnark)
//   D. Relayer Transfer        — how the HTTP request becomes a mined on-chain tx
//   E. Grand Total             — total gas and native-token cost across all operations
//
// Prerequisites:
//   - chain reachable at mainnet-rpc.rayls.com (export MY_KEY=<hex-private-key>)
//   - gnark server running on :8080
//   - relayer binary built: cd enygma_payments/relayer && ./run.sh
//     The test spawns its own relayer on :8083 → the freshly-deployed contract.
//     The user's relayer on :8082 does NOT need to be running.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestCostReport -v -timeout 300s

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── Cost record ───────────────────────────────────────────────────────────────

// opRecord captures the cost of a single on-chain operation.
type opRecord struct {
	label       string
	txHash      string
	gasUsed     uint64
	gasPriceWei *big.Int // actual gas price from the signed transaction
}

// costWei returns gasUsed × gasPriceWei.
func (r opRecord) costWei() *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(r.gasUsed), r.gasPriceWei)
}

// costETH returns the cost as a human-readable decimal string (18 decimals).
func (r opRecord) costETH() string {
	return weiToETH(r.costWei())
}

// gweiStr returns gas price in Gwei (9 decimals).
func (r opRecord) gweiStr() string {
	gwei := new(big.Int).Div(r.gasPriceWei, big.NewInt(1e9))
	return gwei.String()
}

// weiToETH formats a wei amount as a decimal string with 6 significant figures.
func weiToETH(wei *big.Int) string {
	if wei == nil || wei.Sign() == 0 {
		return "0.000000"
	}
	// integer part = wei / 1e18
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	intPart := new(big.Int).Div(wei, denom)
	// fractional part (6 decimal places)
	remainder := new(big.Int).Mod(wei, denom)
	fracDenom := new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil) // 1e18/1e6
	fracPart := new(big.Int).Div(remainder, fracDenom)
	return fmt.Sprintf("%s.%06d", intPart, fracPart)
}

// ── TestCostReport ────────────────────────────────────────────────────────────

func TestCostReport(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at localhost:8080 — start gnark-server first")
	}
	// No pre-check for :8082 — this test spawns its own relayer on :8083.

	ctx := context.Background()

	// ── Chain client ──────────────────────────────────────────────────────────
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial chain: %v", err)
	}
	defer client.Close()

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	t.Logf("Submitter address: %s", ownerAddr.Hex())

	// mkAuth builds a new TransactOpts with current nonce and suggested gas price.
	mkAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(ctx)
		auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}

	// record waits for a submitted transaction to be mined, fetches its actual
	// gas price, logs a one-line summary, and returns an opRecord.
	// Go prohibits mixing a string literal with a multi-return call in one
	// argument list, so callers explicitly unpack (tx, txErr) before calling.
	record := func(label string, tx *ethtypes.Transaction, txErr error) opRecord {
		t.Helper()
		if txErr != nil {
			t.Fatalf("[%s] send tx: %v", label, txErr)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			t.Fatalf("[%s] wait mined: %v", label, err)
		}
		if r.Status != 1 {
			t.Fatalf("[%s] tx reverted (Status=0, tx=%s)", label, r.TxHash.Hex())
		}
		onChainTx, _, err := client.TransactionByHash(ctx, r.TxHash)
		if err != nil {
			t.Fatalf("[%s] fetch tx: %v", label, err)
		}
		rec := opRecord{
			label:       label,
			txHash:      r.TxHash.Hex(),
			gasUsed:     r.GasUsed,
			gasPriceWei: onChainTx.GasPrice(),
		}
		t.Logf("  %-32s gas=%7d  @%s Gwei  cost=%s native", label, rec.gasUsed, rec.gweiStr(), rec.costETH())
		return rec
	}

	var allRecords []opRecord
	addRec := func(r opRecord) { allRecords = append(allRecords, r) }

	// ══════════════════════════════════════════════════════════════════════════
	// SECTION A: Contract Deployment
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  SECTION A — Contract Deployment")
	t.Log("════════════════════════════════════════════════════════")

	const artifactBase = "../../contracts/enygma/artifacts/contracts"

	// Deploy Enygma.sol ───────────────────────────────────────────────────────
	// The Enygma contract holds all bank balance commitments, the totalSupply
	// commitment point, and the nullifier set. It is the core settlement layer.
	t.Log("[A1] Deploying Enygma.sol (epochInterval=30)…")
	enygmaAuth := mkAuth()
	enygmaABIStr, enygmaBytecode := artifactJSON(t, artifactBase+"/Enygma.sol/Enygma.json")
	parsedEnygmaABI, _ := abi.JSON(strings.NewReader(enygmaABIStr))
	enygmaAddr, enygmaTx, _, err := bind.DeployContract(enygmaAuth, parsedEnygmaABI, enygmaBytecode, client, big.NewInt(30))
	if err != nil {
		t.Fatalf("deploy Enygma: %v", err)
	}
	enygmaR, err := bind.WaitMined(ctx, client, enygmaTx)
	if err != nil || enygmaR.Status != 1 {
		t.Fatalf("deploy Enygma failed: status=%d err=%v", enygmaR.Status, err)
	}
	{
		onChainTx, _, _ := client.TransactionByHash(ctx, enygmaR.TxHash)
		r := opRecord{"Deploy Enygma.sol", enygmaR.TxHash.Hex(), enygmaR.GasUsed, onChainTx.GasPrice()}
		t.Logf("  %-32s gas=%7d  @%s Gwei  cost=%s native", r.label, r.gasUsed, r.gweiStr(), r.costETH())
		t.Logf("  → deployed at %s", enygmaAddr.Hex())
		addRec(r)
	}

	// Deploy EnygmaVerifier.sol ───────────────────────────────────────────────
	// The Verifier is a standalone Groth16 pairing-check contract generated from
	// the gnark proving key. Enygma calls it to verify every Transfer proof.
	t.Log("[A2] Deploying EnygmaVerifier.sol…")
	verifierAuth := mkAuth()
	verifierABIStr, verifierBytecode := artifactJSON(t, artifactBase+"/EnygmaVerifier.sol/Verifier.json")
	parsedVerifierABI, _ := abi.JSON(strings.NewReader(verifierABIStr))
	verifierAddr, verifierTx, _, err := bind.DeployContract(verifierAuth, parsedVerifierABI, verifierBytecode, client)
	if err != nil {
		t.Fatalf("deploy Verifier: %v", err)
	}
	verifierR, err := bind.WaitMined(ctx, client, verifierTx)
	if err != nil || verifierR.Status != 1 {
		t.Fatalf("deploy Verifier failed: status=%d err=%v", verifierR.Status, err)
	}
	{
		onChainTx, _, _ := client.TransactionByHash(ctx, verifierR.TxHash)
		r := opRecord{"Deploy EnygmaVerifier.sol", verifierR.TxHash.Hex(), verifierR.GasUsed, onChainTx.GasPrice()}
		t.Logf("  %-32s gas=%7d  @%s Gwei  cost=%s native", r.label, r.gasUsed, r.gweiStr(), r.costETH())
		t.Logf("  → deployed at %s", verifierAddr.Hex())
		addRec(r)
	}

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// SECTION B: Setup Transactions
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  SECTION B — Setup Transactions")
	t.Log("════════════════════════════════════════════════════════")

	// initialize() ────────────────────────────────────────────────────────────
	// Sets the contract status to INITIALIZED and records the deployer as _owner.
	// Must be called exactly once before any bank registration or minting.
	t.Log("[B1] initialize()…")
	{
		tx, txErr := instance.Initialize(mkAuth())
		addRec(record("initialize()", tx, txErr))
	}

	// addVerifier() ───────────────────────────────────────────────────────────
	// Registers the Groth16 verifier contract address so Transfer can call it.
	// Enygma stores _transferVerifier; Transfer() reverts with VerifierNotFound
	// until this call succeeds.
	t.Log("[B2] addVerifier()…")
	{
		tx, txErr := instance.AddVerifier(mkAuth(), verifierAddr)
		addRec(record("addVerifier()", tx, txErr))
	}

	// registerAccount() × nBanks ──────────────────────────────────────────────
	// Each call stores (address → accountId) mapping and an initial Pedersen
	// commitment Com(0, randomness) = randomness·H for that bank.
	// With the totalSupply fix, each call also adds this commitment to
	// (totalSupplyX, totalSupplyY) so check() stays valid.
	t.Log("[B3] registerAccount() × 6…")
	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	var regTotalGas uint64
	for i := 0; i < nBanks; i++ {
		tx, txErr := instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})
		r := record(fmt.Sprintf("registerAccount(bank %d)", i+1), tx, txErr)
		addRec(r)
		regTotalGas += r.gasUsed
	}
	t.Logf("  total gas for all 6 registerAccount calls: %d", regTotalGas)

	// mintSupply() ────────────────────────────────────────────────────────────
	// Adds Com(amount, 0) = amount·G to bank 0's balance commitment and to
	// (totalSupplyX, totalSupplyY). Does NOT reveal the amount on-chain; only
	// the Pedersen commitment point is stored.
	t.Log("[B4] mintSupply(500, accountId=1)…")
	{
		tx, txErr := instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))
		addRec(record("mintSupply(500, 1)", tx, txErr))
	}

	// ══════════════════════════════════════════════════════════════════════════
	// SECTION C: ZK Proof Generation (off-chain, gnark server)
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  SECTION C — ZK Proof Generation (off-chain)")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  The gnark server re-derives the circuit witness from the request,")
	t.Log("  runs Groth16 proving on the Baby JubJub circuit, and returns")
	t.Log("  an 8-element proof + 50-element public signal (no gas cost).")

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	t.Logf("  contract lastBlockNum = %s (embedded in proof as block_number)", blockHash)

	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	prevBalances := pubVals.Balances[1:]
	onChainKeys := pubVals.Keys[1:]

	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)
	senderSecret, _ := poseidon.Hash([]*big.Int{prevR, sk})
	senderSecret.Mod(senderSecret, curveP)

	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = senderSecret

	fp := fingerPrintGen(secrets, senderIdx)
	txValues := []*big.Int{
		negMod(big.NewInt(transferAmt)),
		big.NewInt(60), big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}
	tagMessages := tagMessageGen(secrets, new(big.Int).Set(blockHash))
	txCommit, txRandom := genCommitmentAndRandom(senderIdx, big.NewInt(transferAmt), txValues, new(big.Int).Set(blockHash), secrets)
	nullifier, _ := poseidon.Hash([]*big.Int{senderSecret, blockHash})

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
	keyStrs := make([]string, nBanks)
	for i, k := range onChainKeys {
		keyStrs[i] = k.String()
	}
	kIndex := make([]*big.Int, nBanks)
	for i := range kIndex {
		kIndex[i] = big.NewInt(int64(i))
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"fingerprint_shared_secrets":   fp2Strs(fp),
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

	t.Log("  Sending proof request to gnark server (may take ~30s)…")
	proofStart := time.Now()
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
	proofDuration := time.Since(proofStart)
	t.Logf("  Proof generated in %s (8 elements + 80 public signals)", proofDuration.Round(time.Millisecond))
	t.Log("  Cost: $0.00 (proof generation is off-chain, runs on your server)")

	// ══════════════════════════════════════════════════════════════════════════
	// SECTION D: Relayer Transfer Flow
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  SECTION D — Relayer Transfer")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  HOW THE RELAYER SENDS A TRANSACTION:")
	t.Log("")
	t.Log("  1. Test  → POST /relay/transfer  (HTTP, Bearer token auth)")
	t.Log("             payload: {proof[8], publicSignal[50], commitments[6][2], kIndex[6]}")
	t.Log("")
	t.Log("  2. Relayer parses JSON, validates field counts, zero-pads publicSignal to [50].")
	t.Log("")
	t.Log("  3. Relayer holds a txMu mutex to prevent nonce races across concurrent requests.")
	t.Log("     It also deduplicates identical in-flight requests via proof[0] as the key.")
	t.Log("")
	t.Log("  4. Relayer calls  instance.Transfer(auth, commitmentDeltas, proof, participantIds)")
	t.Log("     using its own private key (RELAYER_PRIVATE_KEY) and the gas limit from config.")
	t.Log("     The go-ethereum bind layer fetches the current nonce and submits the raw tx.")
	t.Log("")
	t.Log("  5. On-chain, Enygma.transfer() runs:")
	t.Log("     a. _verifyBlockNumber()  — proof.publicSignal[blockNumIdx] == lastBlockNum")
	t.Log("     b. _verifyPublicInputs() — verify hashes, PKs, prev-commitments match chain state")
	t.Log("     c. Groth16 pairing check — calls EnygmaVerifier.verifyProof() (most gas)")
	t.Log("     d. _consumeNullifier()   — mark nullifier as used (replay protection)")
	t.Log("     e. _updateBalances()     — add each txCommit[i] to bank[i]'s commitment")
	t.Log("")
	t.Log("  6. Relayer waits for the receipt (up to 45s), checks Status==1,")
	t.Log("     returns {txHash, blockNumber, gasUsed} to the test as HTTP 200.")
	t.Log("")

	// ── Start a test-local relayer on :8083 ──────────────────────────────────
	// The user's relayer on :8082 is configured with a different contract address.
	// We spawn a fresh relayer instance on :8083 pointed at the contract we just
	// deployed in Section A. The binary is built by run.sh (CGO_ENABLED=0).
	_, testFile, _, _ := runtime.Caller(0)
	relayerDir, _ := filepath.Abs(filepath.Join(filepath.Dir(testFile), "..", "..", "relayer"))
	relayerBin := filepath.Join(relayerDir, "relayer_bin")
	if _, statErr := os.Stat(relayerBin); os.IsNotExist(statErr) {
		t.Fatalf("relayer binary not found at %s\nBuild it first:\n  cd enygma_payments/relayer && ./run.sh", relayerBin)
	}

	const testRelayerPort = "8083"
	testRelayerURL := "http://127.0.0.1:" + testRelayerPort

	relayerCmd := exec.Command(relayerBin)
	relayerCmd.Dir = relayerDir // so relative ABI/address paths resolve correctly
	relayerCmd.Env = append(os.Environ(),
		"RELAYER_RPC_URL="+chainURL,
		fmt.Sprintf("RELAYER_CHAIN_ID=%d", chainID),
		"RELAYER_PRIVATE_KEY="+ownerPrivKey,
		"RELAYER_API_KEY="+relayerKey,
		"RELAYER_GAS_LIMIT=10000000",
		"RELAYER_CONTRACT_ADDR="+enygmaAddr.Hex(),
		"RELAYER_PORT="+testRelayerPort,
	)
	if err := relayerCmd.Start(); err != nil {
		t.Fatalf("start relayer subprocess: %v", err)
	}
	t.Cleanup(func() { relayerCmd.Process.Kill() })
	t.Logf("  started test-local relayer pid=%d on :%s → %s",
		relayerCmd.Process.Pid, testRelayerPort, enygmaAddr.Hex())

	// Poll until the relayer is accepting TCP connections (up to 20s).
	for i := 0; i < 20; i++ {
		if tcpAvailable("127.0.0.1:" + testRelayerPort) {
			break
		}
		if i == 19 {
			t.Fatal("test-local relayer did not become ready within 20s")
		}
		time.Sleep(time.Second)
	}
	t.Log("  test-local relayer ready ✓")

	// ── Build relay request and submit ────────────────────────────────────────
	// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (pks) + 12 (prevCommit) = 54.
	const txCommitOffset = 54
	commitmentDeltas := make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		commitmentDeltas[i] = []string{
			proofResp.PublicSignal[txCommitOffset+2*i].String(),
			proofResp.PublicSignal[txCommitOffset+2*i+1].String(),
		}
	}
	var proof8Strs [8]string
	for i := 0; i < 8; i++ {
		proof8Strs[i] = proofResp.Proof[i].String()
	}
	pubSigStrs := make([]string, len(proofResp.PublicSignal))
	for i, v := range proofResp.PublicSignal {
		pubSigStrs[i] = v.String()
	}
	kIdx64 := make([]int64, nBanks)
	for i := range kIdx64 {
		kIdx64[i] = int64(i + 1)
	}
	relayReq := struct {
		Proof        [8]string  `json:"proof"`
		PublicSignal []string   `json:"publicSignal"`
		Commitments  [][]string `json:"commitments"`
		KIndex       []int64    `json:"kIndex"`
	}{proof8Strs, pubSigStrs, commitmentDeltas, kIdx64}
	relayBody, _ := json.Marshal(relayReq)

	t.Logf("  POST %s/relay/transfer", testRelayerURL)
	transferStart := time.Now()
	relayHTTPReq, _ := http.NewRequest(http.MethodPost, testRelayerURL+"/relay/transfer", bytes.NewReader(relayBody))
	relayHTTPReq.Header.Set("Content-Type", "application/json")
	relayHTTPReq.Header.Set("Authorization", "Bearer "+relayerKey)
	relayHTTPResp, err := http.DefaultClient.Do(relayHTTPReq)
	if err != nil {
		t.Fatalf("relay POST: %v", err)
	}
	defer relayHTTPResp.Body.Close()
	relayRespBody, _ := io.ReadAll(relayHTTPResp.Body)
	if relayHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", relayHTTPResp.StatusCode, relayRespBody)
	}
	transferDuration := time.Since(transferStart)

	var relayResp struct {
		TxHash      string `json:"txHash"`
		BlockNumber uint64 `json:"blockNumber"`
		GasUsed     uint64 `json:"gasUsed"`
	}
	if err := json.Unmarshal(relayRespBody, &relayResp); err != nil {
		t.Fatalf("parse relay response: %v", err)
	}

	// Fetch the actual gas price from the relayer's signed transaction.
	relayTx, _, err := client.TransactionByHash(ctx, common.HexToHash(relayResp.TxHash))
	if err != nil {
		t.Fatalf("fetch relayer tx %s: %v", relayResp.TxHash, err)
	}
	transferRecord := opRecord{
		label:       "Transfer (via relayer)",
		txHash:      relayResp.TxHash,
		gasUsed:     relayResp.GasUsed,
		gasPriceWei: relayTx.GasPrice(),
	}
	t.Logf("  relayer round-trip: %s", transferDuration.Round(time.Millisecond))
	t.Logf("  %-36s gas=%7d  @%s Gwei  cost=%s native",
		transferRecord.label, transferRecord.gasUsed, transferRecord.gweiStr(), transferRecord.costETH())
	t.Logf("  tx=%s  block=%d", transferRecord.txHash, relayResp.BlockNumber)
	addRec(transferRecord)

	// Verify bank 0 balance updated (sanity check).
	newBal, _ := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if newBal.X.Cmp(prevBalances[0].C1) == 0 && newBal.Y.Cmp(prevBalances[0].C2) == 0 {
		t.Error("bank 0 balance unchanged after transfer — something went wrong")
	} else {
		t.Logf("  bank 0 commitment updated ✓ (100 tokens sent: 60 to bank 1, 40 to bank 2)")
	}

	// ══════════════════════════════════════════════════════════════════════════
	// SECTION E: Grand Total Cost Report
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("  SECTION E — Grand Total Cost Report")
	t.Log("════════════════════════════════════════════════════════")
	t.Log("")

	// Column widths: operation | tx hash (short) | gas used | gas price | cost
	hdr := fmt.Sprintf("  %-36s  %-12s  %10s  %8s  %16s",
		"Operation", "TxHash", "Gas Used", "Gwei", "Cost (native)")
	t.Log(hdr)
	t.Log("  " + strings.Repeat("─", len(hdr)-2))

	totalGas := uint64(0)
	totalCostWei := new(big.Int)

	sections := []struct {
		title string
		start int
		end   int
	}{
		{"A  DEPLOYMENT", 0, 2},
		{"B  SETUP", 2, 2 + 1 + 1 + nBanks + 1},
		{"D  TRANSFER", 2 + 1 + 1 + nBanks + 1, len(allRecords)},
	}

	for _, sec := range sections {
		t.Logf("")
		t.Logf("  ── %s ──", sec.title)
		secGas := uint64(0)
		secCost := new(big.Int)
		for _, rec := range allRecords[sec.start:sec.end] {
			shortHash := rec.txHash
			if len(shortHash) > 12 {
				shortHash = shortHash[:6] + "…" + shortHash[len(shortHash)-4:]
			}
			row := fmt.Sprintf("  %-36s  %-12s  %10d  %8s  %16s",
				rec.label, shortHash, rec.gasUsed, rec.gweiStr(), rec.costETH())
			t.Log(row)
			secGas += rec.gasUsed
			secCost.Add(secCost, rec.costWei())
		}
		t.Logf("  %-36s  %-12s  %10d  %8s  %16s",
			"  subtotal", "", secGas, "", weiToETH(secCost))
		totalGas += secGas
		totalCostWei.Add(totalCostWei, secCost)
	}

	t.Log("")
	t.Log("  " + strings.Repeat("═", len(hdr)-2))
	t.Logf("  %-36s  %-12s  %10d  %8s  %16s",
		"GRAND TOTAL", "", totalGas, "", weiToETH(totalCostWei))
	t.Log("")
	t.Logf("  ZK proof generation time : %s (off-chain, no gas)", proofDuration.Round(time.Millisecond))
	t.Logf("  Transfer submission time : %s (submit → mined)", transferDuration.Round(time.Millisecond))
	t.Log("")
	t.Logf("  Note: 'native' = chain native gas token (chainID %d, RPC %s).", chainID, chainURL)
	t.Log("  Deployment costs (A) are one-time. Ongoing cost per")
	t.Log("  confidential transfer = Section D only (~1.8M gas).")
	t.Log("════════════════════════════════════════════════════════")
}
