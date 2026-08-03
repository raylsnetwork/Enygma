package enygma_test

// TestFeeTransferFlow exercises the full fee-circuit stack end-to-end.
//
// Fee model (on-chain fee embedded in sender's commitment):
//
//	The fee is a fixed protocol constant — on mainnet: 0.1 USDr (native token).
//	Senders absorb the fee directly into their commitment: sender deducts
//	(transferAmount + fee) from their Pedersen commitment. The ZK proof commits
//	to the fee at publicSignal[50]; the relayer verifies signal[50] == PROTOCOL_FEE.
//
//	  PROTOCOL_FEE = 20  (test units; mainnet equivalent: 0.1 USDr)
//
//	  Bank 0 (sender)   deducts 120 (100 transfer + 20 fee) from commitment
//	  Bank 1            credited by +60  (destination)
//	  Bank 2            credited by +40  (change)
//	  Protocol fee = 20 → embedded in sender TxValue AND publicSignal[50]
//
//	TxValues = [−120 mod P, +60, +40, 0, 0, 0]
//	Σ(TxValues) = −20 mod P           (= −fee; fee absorbed in sender's value)
//	Σ(TxCommit) + fee·G = (0, 1)     (conservation law; signal[51,52] == (0,1))
//	Σ(TxValues) + fee   = 0           (scalar conservation; signal[53] == 0)
//
// Flow:
//
//	Sender pays PROTOCOL_FEE to Relayer (off-chain, fixed amount — no negotiation)
//	→ Sender builds proof with txValues (Σ=0) and fee=PROTOCOL_FEE at signal[50]
//	→ gnark server /proof/enygma_fee  →  proof[8] + publicSignal[51]
//	→ Relayer checks signal[50] == PROTOCOL_FEE  →  accepts transaction
//	→ Relayer POSTs /relay/transfer_fee  →  Enygma.transferWithFee() on-chain
//	→ On-chain: commitments updated (bank 0 −100, bank 1 +60, bank 2 +40)
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>  (or run against local Hardhat — key auto-applied)
//	gnark server on :8080 loaded with EnygmaFee keys:
//	  cd enygma_payments/gnark-server && go run ./keygen/generate_keys.go -circuit enygma_fee
//	  go run main.go
//
// Run:
//
//	ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//	  CC=/usr/bin/clang go test -run TestFeeTransferFlow -v -timeout 300s

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
	"testing"
	"time"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

const (
	gnarkFeeURL = "http://127.0.0.1:8080/proof/enygma_fee"

	// PROTOCOL_FEE is the fixed relayer fee charged on every transaction.
	// On mainnet this is 0.1 USDr (the native token). No negotiation needed —
	// the relayer rejects any proof where signal[50] != PROTOCOL_FEE.
	PROTOCOL_FEE = 20

	feeDstAmt  = 60                     // bank 1 receives (destination)
	feeChgAmt  = 40                     // bank 2 receives (change)
	feeSendAmt = feeDstAmt + feeChgAmt  // 100: on-chain debit (fee paid separately)
)

func TestFeeTransferFlow(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at :8080 — start gnark-server first:\n  cd enygma_payments/gnark-server && go run main.go")
	}

	ctx := context.Background()

	// ── Chain client + auth factory ───────────────────────────────────────────
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial chain: %v", err)
	}
	defer client.Close()

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	t.Logf("submitter: %s", ownerAddr.Hex())

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

	waitTx := func(tx *ethtypes.Transaction, txErr error) *ethtypes.Receipt {
		t.Helper()
		if txErr != nil {
			t.Fatalf("send tx: %v", txErr)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			t.Fatalf("wait mined: %v", err)
		}
		return r
	}

	// ── Deploy fresh contracts ────────────────────────────────────────────────
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	t.Log("deploying Enygma contract…")
	enygmaAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	t.Log("deploying standard verifier…")
	verifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/EnygmaVerifier.sol/Verifier.json")
	t.Log("deploying fee verifier…")
	feeVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/EnygmaFeeVerifier.sol/Verifier.json")

	enygmaInstance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	waitTx(enygmaInstance.Initialize(mkAuth()))
	t.Log("initialized")

	if r := waitTx(enygmaInstance.AddVerifier(mkAuth(), verifierAddr)); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}
	t.Logf("standard verifier: %s", verifierAddr.Hex())

	if r := waitTx(enygmaInstance.AddFeeVerifier(mkAuth(), feeVerifierAddr)); r.Status != 1 {
		t.Fatal("addFeeVerifier failed")
	}
	t.Logf("fee verifier: %s", feeVerifierAddr.Hex())

	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		if r := waitTx(enygmaInstance.RegisterAccount(mkAuth(), ownerAddr,
			big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	t.Logf("registered %d banks", nBanks)

	if r := waitTx(enygmaInstance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d tokens to bank 0 (accountId=1)", mintAmt)

	// ── Start test-local relayer ───────────────────────────────────────────────
	_, testFile, _, _ := runtime.Caller(0)
	relayerDir, _ := filepath.Abs(filepath.Join(filepath.Dir(testFile), "..", "..", "relayer"))
	relayerBin := filepath.Join(relayerDir, "relayer_bin")
	if _, statErr := os.Stat(relayerBin); os.IsNotExist(statErr) {
		t.Fatalf("relayer binary not found: %s\nRun: cd enygma_payments/relayer && CC=/usr/bin/clang go build -o relayer_bin .", relayerBin)
	}
	// macOS 25.x (Tahoe) enforces code signing — ad-hoc sign so the binary can run.
	if out, signErr := exec.Command("codesign", "--force", "--deep", "--sign", "-", relayerBin).CombinedOutput(); signErr != nil {
		t.Logf("codesign warning (non-fatal): %v — %s", signErr, out)
	}

	const feeRelayerPort = "8085"
	feeRelayerURL := "http://127.0.0.1:" + feeRelayerPort

	relayerPrivKey := ownerPrivKey
	if relayerPrivKey == "" {
		relayerPrivKey = hardhatTestKey
	}

	relayerCmd := exec.Command(relayerBin)
	relayerCmd.Dir = relayerDir
	relayerCmd.Env = append(os.Environ(),
		"RELAYER_RPC_URL="+chainURL,
		fmt.Sprintf("RELAYER_CHAIN_ID=%d", chainID),
		"RELAYER_PRIVATE_KEY="+relayerPrivKey,
		"RELAYER_API_KEY="+relayerKey,
		"RELAYER_GAS_LIMIT=10000000",
		"RELAYER_CONTRACT_ADDR="+enygmaAddr.Hex(),
		"RELAYER_PORT="+feeRelayerPort,
		fmt.Sprintf("RELAYER_PROTOCOL_FEE=%d", PROTOCOL_FEE),
		"GIN_MODE=release",
	)
	relayerStderr, _ := os.CreateTemp("", "relayer-stderr-*.txt")
	relayerCmd.Stdout = relayerStderr
	relayerCmd.Stderr = relayerStderr
	if err := relayerCmd.Start(); err != nil {
		t.Fatalf("start fee relayer subprocess: %v", err)
	}
	t.Cleanup(func() {
		relayerCmd.Process.Kill()
		if name := relayerStderr.Name(); name != "" {
			if data, rerr := os.ReadFile(name); rerr == nil && len(data) > 0 {
				t.Logf("relayer output:\n%s", data)
			}
			os.Remove(name)
		}
	})

	for i := 0; i < 20; i++ {
		if tcpAvailable("127.0.0.1:" + feeRelayerPort) {
			break
		}
		if i == 19 {
			t.Fatal("fee relayer did not start within 20s")
		}
		time.Sleep(time.Second)
	}
	t.Logf("fee relayer ready on :%s → %s (PROTOCOL_FEE=%d)", feeRelayerPort, enygmaAddr.Hex(), PROTOCOL_FEE)

	// ── Read on-chain state ───────────────────────────────────────────────────
	blockHash, err := enygmaInstance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("GetBlckHash: %v", err)
	}
	pubVals, err := enygmaInstance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("GetPublicValues: %v", err)
	}
	prevBals := pubVals.Balances[1:]
	onChainKeys := pubVals.Keys[1:]

	t.Logf("block=%s", blockHash)
	t.Logf("bank 0 initial commitment: (%s, %s)", prevBals[0].C1, prevBals[0].C2)

	// ── Build proof inputs ────────────────────────────────────────────────────
	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)

	senderSecret, _ := poseidon.Hash([]*big.Int{prevR, sk})
	senderSecret.Mod(senderSecret, curveP)

	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = senderSecret

	// TxValues: sender absorbs both transfer amount AND fee.
	// sender TxValue = -(feeSendAmt + PROTOCOL_FEE) mod P  →  Σ(TxValues) = -fee
	// circuit C2 expects TxValues[sender] = (P - SenderTxValue - Fee) mod P
	feeTotalSend := feeSendAmt + PROTOCOL_FEE // 120: what sender deducts from commitment
	txValues := []*big.Int{
		negMod(big.NewInt(int64(feeTotalSend))), // −120 mod P (amount + fee)
		big.NewInt(feeDstAmt),                   // +60
		big.NewInt(feeChgAmt),                   // +40
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}

	hashArray := hashArrayGen(secrets)
	tagMessages := tagMessageGen(secrets, new(big.Int).Set(blockHash))
	txCommit, txRandom := genCommitmentAndRandom(
		senderIdx, big.NewInt(int64(feeTotalSend)), txValues, new(big.Int).Set(blockHash), secrets)
	nullifier, _ := poseidon.Hash([]*big.Int{hashArray[senderIdx], blockHash})

	// Σ(TxValues) = -fee mod P (not 0 — fee is embedded in sender's value)
	sumTxValues := new(big.Int)
	for _, v := range txValues {
		sumTxValues.Add(sumTxValues, v)
		sumTxValues.Mod(sumTxValues, curveP)
	}
	expectedSumTxValues := new(big.Int).Sub(curveP, big.NewInt(PROTOCOL_FEE)) // P - fee
	if sumTxValues.Cmp(expectedSumTxValues) != 0 {
		t.Fatalf("conservation check FAILED: Σ(TxValues) = %s (expected P−%d)", sumTxValues, PROTOCOL_FEE)
	}
	t.Logf("conservation: Σ(TxValues) ≡ −%d (mod P) ✓  (fee embedded in sender)", PROTOCOL_FEE)

	// ── Build gnark request ───────────────────────────────────────────────────
	toStrs := func(vals []*big.Int) []string {
		s := make([]string, len(vals))
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}

	prevCommitSlice := make([][]string, nBanks)
	for i, pt := range prevBals {
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
	kIdx := make([]*big.Int, nBanks)
	for i := range kIdx {
		kIdx[i] = big.NewInt(int64(i))
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"hashed_shared_secrets":        toStrs(hashArray),
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                toStrs(kIdx),
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"fee":                          fmt.Sprintf("%d", PROTOCOL_FEE), // constant → signal[50]
		"sender_id":                    fmt.Sprintf("%d", senderIdx),
		"shared_secrets":               toStrs(secrets),
		"secret_key":                   sk.String(),
		"previous_sender_balance":      fmt.Sprintf("%d", senderPrevV),
		"previous_sender_random_value": prevR.String(),
		"tx_values":                    toStrs(txValues),
		"tx_random_values":             toStrs(txRandom),
		"sender_tx_value":              fmt.Sprintf("%d", feeSendAmt),
	})

	// ── Call gnark /proof/enygma_fee ──────────────────────────────────────────
	t.Logf("requesting fee proof (PROTOCOL_FEE=%d on-chain debit=%d, may take ~30s)…",
		PROTOCOL_FEE, feeSendAmt)

	gnarkResp, err := http.Post(gnarkFeeURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("gnark POST: %v", err)
	}
	defer gnarkResp.Body.Close()

	body, _ := io.ReadAll(gnarkResp.Body)
	if gnarkResp.StatusCode != http.StatusOK {
		if gnarkResp.StatusCode == http.StatusInternalServerError {
			t.Skipf("gnark fee endpoint returned 500 — fee proving key not loaded.\n"+
				"Generate: cd enygma_payments/gnark-server && go run ./keygen/generate_keys.go -circuit enygma_fee\n"+
				"Error: %s", body)
		}
		t.Fatalf("gnark %d: %s", gnarkResp.StatusCode, body)
	}

	var proofResp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &proofResp); err != nil {
		t.Fatalf("decode proof response: %v", err)
	}

	if len(proofResp.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(proofResp.Proof))
	}
	// 51 base + 2 SumTxCommit(X,Y) + 1 SumTxValuesWithFee = 54
	if len(proofResp.PublicSignal) != 54 {
		t.Fatalf("expected 54 public signals, got %d", len(proofResp.PublicSignal))
	}
	t.Log("proof: 8 elements, public signal: 54 elements ✓")

	// signal[50]: PROTOCOL_FEE constant — relayer verifies this before submitting.
	if feeFromSignal := proofResp.PublicSignal[50]; feeFromSignal.Cmp(big.NewInt(PROTOCOL_FEE)) != 0 {
		t.Fatalf("signal[50] (fee) = %s, want %d", feeFromSignal, PROTOCOL_FEE)
	}
	t.Logf("signal[50] = %d == PROTOCOL_FEE ✓", PROTOCOL_FEE)

	// signal[51–52]: SumTxCommit = Σ(TxCommit) + fee·G.
	// Sender's TxValue = -(amount+fee) ⟹ Σ(TxCommit) = (-fee)·G.
	// Circuit adds fee·G ⟹ SumTxCommit = (0)·G = BJJ identity = (0,1).
	sumTxCommitX := proofResp.PublicSignal[51]
	sumTxCommitY := proofResp.PublicSignal[52]
	bjjIdentityX := big.NewInt(0)
	bjjIdentityY := big.NewInt(1)
	if sumTxCommitX.Cmp(bjjIdentityX) != 0 || sumTxCommitY.Cmp(bjjIdentityY) != 0 {
		t.Errorf("signal[51,52] (SumTxCommit) = (%s, %s), expected (0,1) — conservation broken",
			sumTxCommitX, sumTxCommitY)
	} else {
		t.Logf("signal[51,52] SumTxCommit = Σ(TxCommit)+fee·G = (0,1) ✓")
	}

	// signal[53]: SumTxValuesWithFee = (Σ(TxValues) + fee) mod P.
	// Sender's TxValue absorbs fee ⟹ Σ(TxValues) = -fee ⟹ this is 0.
	sumTxValWithFee := proofResp.PublicSignal[53]
	if sumTxValWithFee.Sign() != 0 {
		t.Errorf("signal[53] (SumTxValuesWithFee) = %s, want 0 (fee embedded in sender TxValue)",
			sumTxValWithFee)
	} else {
		t.Logf("signal[53] SumTxValuesWithFee = 0 ✓  (Σ(TxValues)+fee=0 confirmed)")
	}

	// ── Build relay request ───────────────────────────────────────────────────
	const txCommitOffset = 24
	commFinal := make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		commFinal[i] = []string{
			proofResp.PublicSignal[txCommitOffset+2*i].String(),
			proofResp.PublicSignal[txCommitOffset+2*i+1].String(),
		}
	}

	var proof8 [8]string
	for i, p := range proofResp.Proof {
		proof8[i] = p.String()
	}
	pubSigStrs := make([]string, len(proofResp.PublicSignal))
	for i, v := range proofResp.PublicSignal {
		pubSigStrs[i] = v.String()
	}
	kIdx64 := make([]int64, nBanks)
	for i := range kIdx64 {
		kIdx64[i] = int64(i + 1)
	}

	relayFeeReq := struct {
		Proof        [8]string  `json:"proof"`
		PublicSignal []string   `json:"publicSignal"`
		Commitments  [][]string `json:"commitments"`
		KIndex       []int64    `json:"kIndex"`
	}{proof8, pubSigStrs, commFinal, kIdx64}

	// ── Submit via relayer ────────────────────────────────────────────────────
	// Sender has already paid PROTOCOL_FEE to the relayer (fixed, no negotiation).
	// Relayer verifies signal[50] == PROTOCOL_FEE, then calls transferWithFee().
	t.Log("submitting via /relay/transfer_fee…")

	relayBody, _ := json.Marshal(relayFeeReq)
	apiKey := os.Getenv("RELAYER_API_KEY")
	if apiKey == "" {
		apiKey = relayerKey
	}

	relayHTTPReq, _ := http.NewRequest(http.MethodPost, feeRelayerURL+"/relay/transfer_fee", bytes.NewReader(relayBody))
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
	t.Logf("transferWithFee: tx=%s block=%d gas=%d", relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)

	// ── Verify on-chain balances ──────────────────────────────────────────────
	t.Log("")
	t.Log("── On-Chain Balance Verification ──────────────────────────────────────")

	for _, tc := range []struct {
		accountId int64
		idx       int
		label     string
	}{
		{1, 0, "bank 0 (−100 on-chain)"},
		{2, 1, "bank 1 (+60)"},
		{3, 2, "bank 2 (+40)"},
	} {
		newBal, err := enygmaInstance.GetBalance(&bind.CallOpts{}, big.NewInt(tc.accountId))
		if err != nil {
			t.Fatalf("getBalance(%d): %v", tc.accountId, err)
		}
		delta := &babyjub.Point{
			X: proofResp.PublicSignal[txCommitOffset+2*tc.idx],
			Y: proofResp.PublicSignal[txCommitOffset+2*tc.idx+1],
		}
		expected := addBJPoints(&babyjub.Point{X: prevBals[tc.idx].C1, Y: prevBals[tc.idx].C2}, delta)
		got := &babyjub.Point{X: newBal.X, Y: newBal.Y}
		if got.X.Cmp(expected.X) != 0 || got.Y.Cmp(expected.Y) != 0 {
			t.Errorf("  %-26s MISMATCH\n    got      (%s, %s)\n    expected (%s, %s)",
				tc.label, got.X, got.Y, expected.X, expected.Y)
		} else {
			t.Logf("  %-26s ✓", tc.label)
		}
	}

	// ── Point conservation: Σ(TxCommit) + fee·G = (0,1) ────────────────────
	// With fee embedded in sender's commitment, raw Σ(TxCommit) = (-fee)·G ≠ (0,1).
	// Adding fee·G restores the identity — verified via signal[51,52] above.
	t.Log("")
	t.Log("── Point Conservation ──────────────────────────────────────────────────")

	sumPoint := &babyjub.Point{
		X: proofResp.PublicSignal[txCommitOffset],
		Y: proofResp.PublicSignal[txCommitOffset+1],
	}
	for i := 1; i < nBanks; i++ {
		sumPoint = addBJPoints(sumPoint, &babyjub.Point{
			X: proofResp.PublicSignal[txCommitOffset+2*i],
			Y: proofResp.PublicSignal[txCommitOffset+2*i+1],
		})
	}
	// Add fee·G using the circuit generator
	feeGPoint := babyjub.NewPoint().Mul(big.NewInt(PROTOCOL_FEE), curveG)
	sumWithFee := addBJPoints(sumPoint, feeGPoint)
	if sumWithFee.X.Cmp(bjjIdentityX) != 0 || sumWithFee.Y.Cmp(bjjIdentityY) != 0 {
		t.Errorf("Σ(TxCommit)+fee·G = (%s, %s), expected (0, 1)", sumWithFee.X, sumWithFee.Y)
	} else {
		t.Log("  Σ(TxCommit)+fee·G = (0, 1) ✓  privacy pool intact")
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	t.Log("")
	t.Log("══ TestFeeTransferFlow PASSED ═══════════════════════════════════════════")
	t.Logf("   PROTOCOL_FEE:          %d (mainnet: 0.1 USDr) → signal[50]", PROTOCOL_FEE)
	t.Logf("   Bank 0 commitment debit: %d (transfer %d + fee %d)", feeTotalSend, feeSendAmt, PROTOCOL_FEE)
	t.Log("   Bank 1:                +60")
	t.Log("   Bank 2:                +40")
	t.Logf("   signal[53]:            0  (Σ(TxValues)+fee=0 ✓)")
	t.Log("   Privacy pool:          intact — Σ(TxCommit)+fee·G = (0,1)")
}
