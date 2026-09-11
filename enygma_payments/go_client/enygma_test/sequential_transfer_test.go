package enygma_test

// TestSequentialTransfers runs two back-to-back ZK confidential transfers from
// bank 0 through the full stack: gnark prover → relayer subprocess → chain.
// Each round now also builds and relays a second, independent USDr fee proof
// alongside the main transfer proof — the relayer's dual-proof /relay/transfer
// contract, settled atomically in one on-chain transfer() call per round.
//
// Scenario (fresh contracts, 6 banks, bank 0 minted 500 tokens + 200 USDr):
//
//	Transfer 1: Bank 0 sends 100  →  Bank 1: +60, Bank 2: +40   (+ USDr fee 10 → bank 1)
//	Transfer 2: Bank 0 sends 200  →  Bank 3: +120, Bank 4: +80  (+ USDr fee 10 → bank 1)
//
// Between the two transfers the test derives bank 0's updated Pedersen commitment
// randomness from the first transfer's output, reads the new on-chain block number,
// and generates a fresh proof against the updated state.
//
// Final assertions:
//   - All updated bank commitments match the expected homomorphic sums.
//   - check() confirms Σ(bank commitments) == totalSupply after both transfers.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>
//	gnark server running on :8080
//	relayer binary built: cd enygma_payments/relayer && ./run.sh
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestSequentialTransfers -v -timeout 600s

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

func TestSequentialTransfers(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at localhost:8080 — start gnark-server first")
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

	waitTxOK := func(tx *ethtypes.Transaction, txErr error) *ethtypes.Receipt {
		t.Helper()
		if txErr != nil {
			t.Fatalf("send tx: %v", txErr)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			t.Fatalf("wait mined: %v", err)
		}
		if r.Status != 1 {
			t.Fatalf("tx reverted Status=0 tx=%s", r.TxHash.Hex())
		}
		return r
	}

	// ── Deploy fresh contracts ────────────────────────────────────────────────
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	t.Log("deploying Enygma.sol…")
	enygmaAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	t.Log("deploying EnygmaVerifier.sol…")
	verifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/EnygmaVerifier.sol/Verifier.json")

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}
	t.Logf("contracts: enygma=%s verifier=%s", enygmaAddr.Hex(), verifierAddr.Hex())

	// ── Setup ─────────────────────────────────────────────────────────────────
	waitTxOK(instance.Initialize(mkAuth()))
	waitTxOK(instance.AddVerifier(mkAuth(), verifierAddr))

	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		waitTxOK(instance.RegisterAccount(mkAuth(), ownerAddr,
			big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{}))
	}
	waitTxOK(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1)))
	t.Logf("setup: %d banks registered, %d tokens minted to bank 0 (accountId=1)", nBanks, mintAmt)

	// ── USDr setup: every transfer() call now also settles a second,
	// independent USDr fee proof in the same atomic call (see
	// TestNullifierReuseProtection for the single-round version of this).
	// usdrRecipientIdx stands in for "the relayer" — see the same note in
	// scenario_test.go: no separate relayer identity is needed here since
	// proveAndRelay talks to a real relayer HTTP service, but that service
	// just forwards whatever proof it's given; it doesn't need to BE one
	// of the 6 accounts for this test.
	const usdrRecipientIdx = (senderIdx + 1) % nBanks // bank 1
	t.Log("deploying UsdrVerifier.sol…")
	usdrVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/UsdrVerifier.sol/Verifier.json")
	waitTxOK(instance.AddUsdrVerifier(mkAuth(), usdrVerifierAddr))
	waitTxOK(instance.SetUsdrFixedFee(mkAuth(), big.NewInt(usdrFeeAmt)))
	for i := 0; i < nBanks; i++ {
		waitTxOK(instance.InitializeUsdrBalance(mkAuth(),
			big.NewInt(int64(i+1)), big.NewInt(usdrPrevR)))
	}
	waitTxOK(instance.MintUsdrSupply(mkAuth(), big.NewInt(usdrMintAmt), big.NewInt(senderIdx+1)))
	t.Logf("setup: usdr verifier registered, %d banks USDr-initialized, %d USDr minted to bank 0", nBanks, usdrMintAmt)

	// ── Start test-local relayer on :8084 ─────────────────────────────────────
	// A fresh relayer instance is needed because the user's relayer on :8082 is
	// pointed at a different (older) contract address.
	_, testFile, _, _ := runtime.Caller(0)
	relayerDir, _ := filepath.Abs(filepath.Join(filepath.Dir(testFile), "..", "..", "relayer"))
	relayerBin := filepath.Join(relayerDir, "relayer_bin")
	if _, statErr := os.Stat(relayerBin); os.IsNotExist(statErr) {
		t.Fatalf("relayer binary not found: %s\nRun: cd enygma_payments/relayer && ./run.sh", relayerBin)
	}
	// macOS 25.x (Tahoe) enforces code signing — ad-hoc sign so the binary can run
	// (same workaround as fee_transfer_test.go).
	if out, signErr := exec.Command("codesign", "--force", "--deep", "--sign", "-", relayerBin).CombinedOutput(); signErr != nil {
		t.Logf("codesign warning (non-fatal): %v — %s", signErr, out)
	}

	const seqRelayerPort = "8084"
	seqRelayerURL := "http://127.0.0.1:" + seqRelayerPort

	relayerCmd := exec.Command(relayerBin)
	relayerCmd.Dir = relayerDir
	relayerCmd.Env = append(os.Environ(),
		"RELAYER_RPC_URL="+chainURL,
		fmt.Sprintf("RELAYER_CHAIN_ID=%d", chainID),
		"RELAYER_PRIVATE_KEY="+ownerPrivKey,
		"RELAYER_API_KEY="+relayerKey,
		"RELAYER_GAS_LIMIT=10000000",
		"RELAYER_CONTRACT_ADDR="+enygmaAddr.Hex(),
		"RELAYER_PORT="+seqRelayerPort,
	)
	if err := relayerCmd.Start(); err != nil {
		t.Fatalf("start relayer subprocess: %v", err)
	}
	t.Cleanup(func() { relayerCmd.Process.Kill() })

	for i := 0; i < 20; i++ {
		if tcpAvailable("127.0.0.1:" + seqRelayerPort) {
			break
		}
		if i == 19 {
			t.Fatal("test-local relayer did not start within 20s")
		}
		time.Sleep(time.Second)
	}
	t.Logf("test-local relayer ready on :%s → %s", seqRelayerPort, enygmaAddr.Hex())

	// ── proveAndRelay: gnark proof → relayer → mined tx ───────────────────────
	// Returns (txRandom, commitmentDeltas). txRandom[senderIdx] is the randomness
	// used in the sender's delta commitment — callers use it to derive the sender's
	// updated prevR for the next transfer.
	proveAndRelay := func(
		label string,
		blockHash *big.Int,
		prevBals []enygma.IEnygmaPoint, // [nBanks] from GetPublicValues (offset by 1)
		onChainKeys []*big.Int,
		secrets []*big.Int,
		txVals []*big.Int,
		txAmt, prevV int64,
		prevR *big.Int,
		usdrPrevBals []enygma.IEnygmaPoint, // [nBanks] from GetUsdrPublicValues (offset by 1)
		usdrFeeAmt, usdrPrevV int64,
		usdrPrevR *big.Int,
	) ([]*big.Int, []enygma.IEnygmaPoint, []*big.Int) {
		t.Helper()
		t.Logf("  [%s] proving: block=%s senderBalance=%d", label, blockHash, prevV)

		sk := big.NewInt(senderSk)
		fp := fingerPrintGen(secrets, senderIdx)
		tagMessages := tagMessageGen(secrets, new(big.Int).Set(blockHash))
		txCommit, txRand := genCommitmentAndRandom(
			senderIdx, big.NewInt(txAmt), txVals, new(big.Int).Set(blockHash), secrets)
		nullifier, _ := poseidon.Hash([]*big.Int{secrets[senderIdx], blockHash})

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
		kIdx0 := make([]*big.Int, nBanks)
		for i := range kIdx0 {
			kIdx0[i] = big.NewInt(int64(i))
		}

		reqBody, _ := json.Marshal(map[string]interface{}{
			"fingerprint_shared_secrets":   fp2Strs(fp),
			"public_keys":                  keyStrs,
			"previous_commits":             prevCommitSlice,
			"tx_commits":                   txCommitSlice,
			"block_number":                 blockHash.String(),
			"anonymity_set":                toStrs(kIdx0),
			"message_tags":                 toStrs(tagMessages),
			"nullifier":                    nullifier.String(),
			"sender_id":                    fmt.Sprintf("%d", senderIdx),
			"shared_secrets":               toStrs(secrets),
			"secret_key":                   sk.String(),
			"previous_sender_balance":      fmt.Sprintf("%d", prevV),
			"previous_sender_random_value": prevR.String(),
			"tx_values":                    toStrs(txVals),
			"tx_random_values":             toStrs(txRand),
			"sender_tx_value":              fmt.Sprintf("%d", txAmt),
		})

		gnarkResp, err := http.Post(gnarkURL, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("[%s] gnark POST: %v", label, err)
		}
		defer gnarkResp.Body.Close()
		if gnarkResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(gnarkResp.Body)
			t.Fatalf("[%s] gnark %d: %s", label, gnarkResp.StatusCode, body)
		}
		var proofResp struct {
			Proof        []*big.Int `json:"proof"`
			PublicSignal []*big.Int `json:"publicSignal"`
		}
		if err := json.NewDecoder(gnarkResp.Body).Decode(&proofResp); err != nil {
			t.Fatalf("[%s] decode proof: %v", label, err)
		}
		if len(proofResp.Proof) != 8 || len(proofResp.PublicSignal) != 80 {
			t.Fatalf("[%s] unexpected sizes: proof=%d signal=%d",
				label, len(proofResp.Proof), len(proofResp.PublicSignal))
		}
		t.Logf("  [%s] proof received", label)

		// ── USDr fee proof — independently proves the same sender paying
		// usdrFeeAmt to usdrRecipientIdx over the second (USDr) balance
		// ledger, same k=6 participants/block number as the main proof
		// above. See scenario_test.go's TestNullifierReuseProtection for
		// the single-round version of this witness construction.
		usdrSenderSecret, _ := poseidon.Hash([]*big.Int{usdrPrevR, sk})
		usdrSenderSecret.Mod(usdrSenderSecret, curveP)
		usdrSecrets := make([]*big.Int, nBanks)
		copy(usdrSecrets, baseSecrets)
		usdrSecrets[senderIdx] = usdrSenderSecret

		usdrFp := fingerPrintGen(usdrSecrets, senderIdx)
		usdrTagMessages := tagMessageGenUsdr(usdrSecrets, new(big.Int).Set(blockHash))

		usdrTxValues := make([]*big.Int, nBanks)
		for i := range usdrTxValues {
			usdrTxValues[i] = big.NewInt(0)
		}
		usdrTxValues[senderIdx] = negMod(big.NewInt(usdrFeeAmt))
		usdrTxValues[usdrRecipientIdx] = big.NewInt(usdrFeeAmt)

		usdrTxCommit, usdrTxRand := genCommitmentAndRandomUsdr(
			senderIdx, big.NewInt(usdrFeeAmt), usdrTxValues, new(big.Int).Set(blockHash), usdrSecrets)
		usdrNullifier, _ := poseidon.Hash([]*big.Int{usdrSenderSecret, blockHash})

		usdrPrevCommitSlice := make([][]string, nBanks)
		for i, pt := range usdrPrevBals {
			usdrPrevCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
		}
		usdrTxCommitSlice := make([][]string, nBanks)
		for i, pt := range usdrTxCommit {
			usdrTxCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
		}

		usdrReqBody, _ := json.Marshal(map[string]interface{}{
			"fingerprint_shared_secrets":   fp2Strs(usdrFp),
			"public_keys":                  keyStrs,
			"previous_commits":             usdrPrevCommitSlice,
			"tx_commits":                   usdrTxCommitSlice,
			"block_number":                 blockHash.String(),
			"anonymity_set":                toStrs(kIdx0),
			"message_tags":                 toStrs(usdrTagMessages),
			"nullifier":                    usdrNullifier.String(),
			"sender_id":                    fmt.Sprintf("%d", senderIdx),
			"shared_secrets":               toStrs(usdrSecrets),
			"secret_key":                   sk.String(),
			"previous_sender_balance":      fmt.Sprintf("%d", usdrPrevV),
			"previous_sender_random_value": usdrPrevR.String(),
			"tx_values":                    toStrs(usdrTxValues),
			"tx_random_values":             toStrs(usdrTxRand),
			"sender_tx_value":              fmt.Sprintf("%d", usdrFeeAmt),
		})

		usdrGnarkResp, err := http.Post(gnarkUsdrURL, "application/json", bytes.NewReader(usdrReqBody))
		if err != nil {
			t.Fatalf("[%s] gnark usdr POST: %v", label, err)
		}
		defer usdrGnarkResp.Body.Close()
		if usdrGnarkResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(usdrGnarkResp.Body)
			t.Fatalf("[%s] gnark usdr %d: %s", label, usdrGnarkResp.StatusCode, body)
		}
		var usdrProofResp struct {
			Proof        []*big.Int `json:"proof"`
			PublicSignal []*big.Int `json:"publicSignal"`
		}
		if err := json.NewDecoder(usdrGnarkResp.Body).Decode(&usdrProofResp); err != nil {
			t.Fatalf("[%s] decode usdr proof: %v", label, err)
		}
		// 81 = the main proof's 80-signal layout + FeeAmount (public,
		// appended last — see USDrCircuit.Define / IEnygma.UsdrProof).
		if len(usdrProofResp.Proof) != 8 || len(usdrProofResp.PublicSignal) != 81 {
			t.Fatalf("[%s] unexpected usdr sizes: proof=%d signal=%d",
				label, len(usdrProofResp.Proof), len(usdrProofResp.PublicSignal))
		}
		t.Logf("  [%s] usdr fee proof received", label)

		// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (pks) + 12 (prevCommit) = 54.
		const txCommitOffset = 54
		deltas := make([]enygma.IEnygmaPoint, nBanks)
		commStrs := make([][]string, nBanks)
		for i := 0; i < nBanks; i++ {
			deltas[i] = enygma.IEnygmaPoint{
				C1: proofResp.PublicSignal[txCommitOffset+2*i],
				C2: proofResp.PublicSignal[txCommitOffset+2*i+1],
			}
			commStrs[i] = []string{deltas[i].C1.String(), deltas[i].C2.String()}
		}
		usdrCommStrs := make([][]string, nBanks)
		for i := 0; i < nBanks; i++ {
			usdrCommStrs[i] = []string{
				usdrProofResp.PublicSignal[txCommitOffset+2*i].String(),
				usdrProofResp.PublicSignal[txCommitOffset+2*i+1].String(),
			}
		}

		var proof8 [8]string
		for i := 0; i < 8; i++ {
			proof8[i] = proofResp.Proof[i].String()
		}
		pubSigStrs := make([]string, len(proofResp.PublicSignal))
		for i, v := range proofResp.PublicSignal {
			pubSigStrs[i] = v.String()
		}
		var usdrProof8 [8]string
		for i := 0; i < 8; i++ {
			usdrProof8[i] = usdrProofResp.Proof[i].String()
		}
		usdrPubSigStrs := make([]string, len(usdrProofResp.PublicSignal))
		for i, v := range usdrProofResp.PublicSignal {
			usdrPubSigStrs[i] = v.String()
		}
		kIdx64 := make([]int64, nBanks)
		for i := range kIdx64 {
			kIdx64[i] = int64(i + 1)
		}

		relayReq := struct {
			Proof            [8]string  `json:"proof"`
			PublicSignal     []string   `json:"publicSignal"`
			Commitments      [][]string `json:"commitments"`
			UsdrProof        [8]string  `json:"usdrProof"`
			UsdrPublicSignal []string   `json:"usdrPublicSignal"`
			UsdrCommitments  [][]string `json:"usdrCommitments"`
			KIndex           []int64    `json:"kIndex"`
		}{proof8, pubSigStrs, commStrs, usdrProof8, usdrPubSigStrs, usdrCommStrs, kIdx64}
		relayBody, _ := json.Marshal(relayReq)

		relayHTTPReq, _ := http.NewRequest(http.MethodPost,
			seqRelayerURL+"/relay/transfer", bytes.NewReader(relayBody))
		relayHTTPReq.Header.Set("Content-Type", "application/json")
		relayHTTPReq.Header.Set("Authorization", "Bearer "+relayerKey)

		relayHTTPResp, err := http.DefaultClient.Do(relayHTTPReq)
		if err != nil {
			t.Fatalf("[%s] relay POST: %v", label, err)
		}
		defer relayHTTPResp.Body.Close()
		respBody, _ := io.ReadAll(relayHTTPResp.Body)
		if relayHTTPResp.StatusCode != http.StatusOK {
			t.Fatalf("[%s] relayer %d: %s", label, relayHTTPResp.StatusCode, respBody)
		}
		var relayResp struct {
			TxHash      string `json:"txHash"`
			BlockNumber uint64 `json:"blockNumber"`
			GasUsed     uint64 `json:"gasUsed"`
		}
		if err := json.Unmarshal(respBody, &relayResp); err != nil {
			t.Fatalf("[%s] parse relay response: %v", label, err)
		}
		t.Logf("  [%s] mined: tx=%s block=%d gas=%d",
			label, relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)
		return txRand, deltas, usdrTxRand
	}

	// ══════════════════════════════════════════════════════════════════════════
	// Transfer 1: Bank 0 sends 100 → Bank 1 (+60), Bank 2 (+40)
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("── Transfer 1: Bank 0 sends 100 → [Bank 1: +60, Bank 2: +40] ──────────────────")

	blockHash1, _ := instance.GetBlckHash(&bind.CallOpts{})
	pubVals1, _ := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	prevBals1 := pubVals1.Balances[1:] // accountId 1..6 → slice indices 0..5
	keys1 := pubVals1.Keys[1:]
	usdrPubVals1, _ := instance.GetUsdrPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	usdrPrevBals1 := usdrPubVals1.Balances[1:]
	t.Logf("  block=%s", blockHash1)
	t.Logf("  bank 0 balance: (%s, %s)", prevBals1[0].C1, prevBals1[0].C2)

	prevR1 := big.NewInt(senderPrevR)
	senderSk1 := big.NewInt(senderSk)
	secret1, _ := poseidon.Hash([]*big.Int{prevR1, senderSk1})
	secret1.Mod(secret1, curveP)
	secrets1 := make([]*big.Int, nBanks)
	copy(secrets1, baseSecrets)
	secrets1[senderIdx] = secret1

	txVals1 := []*big.Int{
		negMod(big.NewInt(100)),
		big.NewInt(60), big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}
	usdrPrevR1 := big.NewInt(usdrPrevR)
	txRand1, deltas1, usdrTxRand1 := proveAndRelay("T1", blockHash1, prevBals1, keys1, secrets1, txVals1, 100, senderPrevV, prevR1,
		usdrPrevBals1, usdrFeeAmt, usdrPrevV, usdrPrevR1)

	// Verify T1 balances homomorphically: newBal[i] == prevBal[i] + delta[i].
	for _, tc := range []struct {
		accountId int64
		balsIdx   int
		label     string
	}{
		{1, 0, "bank 0 (−100)"},
		{2, 1, "bank 1 (+60)"},
		{3, 2, "bank 2 (+40)"},
	} {
		bal, _ := instance.GetBalance(&bind.CallOpts{}, big.NewInt(tc.accountId))
		prev := &babyjub.Point{X: prevBals1[tc.balsIdx].C1, Y: prevBals1[tc.balsIdx].C2}
		delta := &babyjub.Point{X: deltas1[tc.balsIdx].C1, Y: deltas1[tc.balsIdx].C2}
		exp := addBJPoints(prev, delta)
		if bal.X.Cmp(exp.X) != 0 || bal.Y.Cmp(exp.Y) != 0 {
			t.Errorf("T1: %s homomorphic check FAILED\n  got      (%s,%s)\n  expected (%s,%s)",
				tc.label, bal.X, bal.Y, exp.X, exp.Y)
		} else {
			t.Logf("  T1 %-20s commitment correct ✓", tc.label)
		}
	}

	// ── Derive bank 0's Pedersen randomness after T1 ──────────────────────────
	// T1 delta for bank 0 = Com(−100, rSum)  where rSum = txRand1[senderIdx].
	// Com(500, 67890) + Com(−100, rSum)  =  Com(400, (67890 + rSum) mod P).
	prevR2 := new(big.Int).Add(big.NewInt(senderPrevR), txRand1[senderIdx])
	prevR2.Mod(prevR2, curveP)
	const senderBalAfterT1 = mintAmt - 100 // 400

	secret2, _ := poseidon.Hash([]*big.Int{prevR2, senderSk1})
	secret2.Mod(secret2, curveP)
	secrets2 := make([]*big.Int, nBanks)
	copy(secrets2, baseSecrets)
	secrets2[senderIdx] = secret2

	// Same derivation for bank 0's USDr randomness after paying the T1 fee:
	// Com(0, usdrPrevR) + Com(200,0) + Com(−usdrFeeAmt, rSum) = Com(200−fee, (usdrPrevR + rSum) mod P).
	usdrPrevR2 := new(big.Int).Add(usdrPrevR1, usdrTxRand1[senderIdx])
	usdrPrevR2.Mod(usdrPrevR2, curveP)
	const usdrBalAfterT1 = usdrMintAmt - usdrFeeAmt

	// ══════════════════════════════════════════════════════════════════════════
	// Transfer 2: Bank 0 sends 200 → Bank 3 (+120), Bank 4 (+80)
	// ══════════════════════════════════════════════════════════════════════════
	t.Log("")
	t.Log("── Transfer 2: Bank 0 sends 200 → [Bank 3: +120, Bank 4: +80] ─────────────────")

	// Re-read on-chain state: lastBlockNum advances after T1 is mined.
	blockHash2, _ := instance.GetBlckHash(&bind.CallOpts{})
	pubVals2, _ := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	prevBals2 := pubVals2.Balances[1:]
	keys2 := pubVals2.Keys[1:]
	usdrPubVals2, _ := instance.GetUsdrPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	usdrPrevBals2 := usdrPubVals2.Balances[1:]
	t.Logf("  block=%s (advanced from %s)", blockHash2, blockHash1)
	t.Logf("  bank 0 balance: (%s, %s)", prevBals2[0].C1, prevBals2[0].C2)

	txVals2 := []*big.Int{
		negMod(big.NewInt(200)),
		big.NewInt(0), big.NewInt(0),
		big.NewInt(120), big.NewInt(80), big.NewInt(0),
	}
	_, deltas2, _ := proveAndRelay("T2", blockHash2, prevBals2, keys2, secrets2, txVals2, 200, senderBalAfterT1, prevR2,
		usdrPrevBals2, usdrFeeAmt, usdrBalAfterT1, usdrPrevR2)

	// ── Final balance verification ────────────────────────────────────────────
	t.Log("")
	t.Log("── Final Balance Verification ───────────────────────────────────────────────────")

	for _, tc := range []struct {
		accountId int64
		balsIdx   int
		label     string
	}{
		{1, 0, "bank 0 (net −300, holds 200)"},
		{4, 3, "bank 3 (+120 in T2)"},
		{5, 4, "bank 4 (+80 in T2)"},
	} {
		bal, _ := instance.GetBalance(&bind.CallOpts{}, big.NewInt(tc.accountId))
		prev := &babyjub.Point{X: prevBals2[tc.balsIdx].C1, Y: prevBals2[tc.balsIdx].C2}
		delta := &babyjub.Point{X: deltas2[tc.balsIdx].C1, Y: deltas2[tc.balsIdx].C2}
		exp := addBJPoints(prev, delta)
		if bal.X.Cmp(exp.X) != 0 || bal.Y.Cmp(exp.Y) != 0 {
			t.Errorf("final: %s homomorphic check FAILED\n  got      (%s,%s)\n  expected (%s,%s)",
				tc.label, bal.X, bal.Y, exp.X, exp.Y)
		} else {
			t.Logf("  %-38s commitment correct ✓", tc.label)
		}
	}

	// check() invariant: Σ(all bank commitments) == totalSupply.
	// Transfers move value between banks but never create or destroy tokens,
	// so the totalSupply commitment must equal the sum of all bank commitments.
	ok, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() reverted — totalSupply invariant VIOLATED after both transfers: %v", err)
	}
	if !ok {
		t.Fatal("check() returned false — totalSupply invariant violated")
	}
	t.Log("  check() PASSED ✓ — Σ(all bank commitments) == totalSupply after both transfers")

	// checkUsdr() invariant: same as check() but for the USDr ledger — both
	// rounds' fee proofs settled atomically alongside the main transfer.
	usdrOK, err := instance.CheckUsdr(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("checkUsdr() reverted — USDr totalSupply invariant VIOLATED after both transfers: %v", err)
	}
	if !usdrOK {
		t.Fatal("checkUsdr() returned false — USDr totalSupply invariant violated")
	}
	t.Log("  checkUsdr() PASSED ✓ — Σ(all bank USDr commitments) == usdrTotalSupply after both transfers")

	// usdrRecipientIdx (bank 1, accountId 2) stood in for "the relayer" —
	// it must have collected usdrFeeAmt in each of the two rounds.
	relayerUsdrBal, err := instance.GetUsdrBalance(&bind.CallOpts{}, big.NewInt(usdrRecipientIdx+1))
	if err != nil {
		t.Fatalf("getUsdrBalance(relayer): %v", err)
	}
	initialUsdrCommit := pedersenCommitment(big.NewInt(0), big.NewInt(usdrPrevR))
	if relayerUsdrBal.X.Cmp(initialUsdrCommit.X) == 0 && relayerUsdrBal.Y.Cmp(initialUsdrCommit.Y) == 0 {
		t.Error("FAIL: relayer's USDr commitment unchanged after two rounds of fees — fees not credited")
	} else {
		t.Log("  relayer's USDr commitment changed after both rounds — fees credited ✓")
	}

	t.Log("")
	t.Log("══ TestSequentialTransfers PASSED ══════════════════════════════════════════════")
	t.Log("   Transfer 1: bank 0 sent 100  →  bank 1 (+60), bank 2 (+40)  +  USDr fee 10 → bank 1")
	t.Log("   Transfer 2: bank 0 sent 200  →  bank 3 (+120), bank 4 (+80)  +  USDr fee 10 → bank 1")
	t.Log("   Bank 0 final plaintext balance: 200  (500 minted − 100 − 200)")
	t.Log("   Bank 0 final USDr balance: 180  (200 minted − 10 − 10)")
}
