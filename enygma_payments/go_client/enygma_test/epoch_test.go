package enygma_test

// TestEpochIntervalFlow — end-to-end test for the epoch-based balance commitment system.
//
// Verifies:
//   1. epochInterval is stored correctly in the deployed contract
//   2. lastBlockNum is epoch-aligned after initialization
//   3. Mining blocks without a tx does NOT advance lastBlockNum
//   4. mintSupply within an epoch writes to the epoch-start slot, NOT block.number
//   5. After crossing an epoch boundary, mintSupply advances lastBlockNum to the new epoch start
//   6. (requires gnark + relayer) A ZK transfer writes to epochStart, not block.number
//   7. (requires gnark + relayer) A proof generated at epoch N is rejected after lastBlockNum
//      advances to epoch N+1 (InvalidBlockNumber revert via relayer 400)
//   8. (requires gnark + relayer) A fresh proof at the new epoch is accepted
//
// Prerequisites:
//  1. Hardhat node on localhost:8545, contracts deployed with EPOCH_INTERVAL=5 (or any small value):
//       EPOCH_INTERVAL=5 python3 run_scripts/deploy_direct.py
//  2. (phases 6-8 only) Gnark server on localhost:8080
//  3. (phases 6-8 only) Relayer on localhost:8082
//       cd enygma_payments/relayer && RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=change-me go run main.go
//
// Run:
//
//	cd go_client/enygma_test && CC=/usr/bin/clang go test -run TestEpochIntervalFlow -v -timeout 600s
//
// IMPORTANT: Run on a FRESH chain. Restart the Hardhat node between runs.

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
	"testing"

	enygma "enygma/contracts"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── Shared ZK + relay helpers ─────────────────────────────────────────────────

// gnarkProofBody builds the JSON request body for the gnark proof server.
// blockHash is the on-chain lastBlockNum to embed in the proof.
// prevBalances and onChainKeys are read from getPublicValues.
func gnarkProofBody(t *testing.T, blockHash *big.Int, prevBalances []enygma.IEnygmaPoint, onChainKeys []*big.Int) []byte {
	t.Helper()
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

	bh := new(big.Int).Set(blockHash)
	tagMessages := tagMessageGen(secrets, bh)
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
	body, _ := json.Marshal(map[string]interface{}{
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
	return body
}

// callGnarkServer posts to the gnark proof server and returns (proof[8], pubSignal[50], deltas).
func callGnarkServer(t *testing.T, reqBody []byte) ([8]string, []string, []enygma.IEnygmaPoint) {
	t.Helper()
	t.Log("requesting ZK proof (may take ~30s)...")
	httpResp, e := http.Post(gnarkURL, "application/json", bytes.NewReader(reqBody))
	if e != nil {
		t.Fatalf("gnark POST: %v", e)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("gnark %d: %s", httpResp.StatusCode, raw)
	}
	var resp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if e := json.NewDecoder(httpResp.Body).Decode(&resp); e != nil {
		t.Fatalf("decode proof: %v", e)
	}
	if len(resp.Proof) != 8 || len(resp.PublicSignal) != 80 {
		t.Fatalf("unexpected sizes: proof=%d publicSignal=%d", len(resp.Proof), len(resp.PublicSignal))
	}
	t.Log("proof received ✓")

	var proof8 [8]string
	for i := 0; i < 8; i++ {
		proof8[i] = resp.Proof[i].String()
	}
	pubSigStrs := make([]string, 80)
	for i, v := range resp.PublicSignal {
		pubSigStrs[i] = v.String()
	}
	// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (pks) + 12 (prevCommit) = 54.
	const txCommitOffset = 54
	deltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := 0; i < nBanks; i++ {
		deltas[i] = enygma.IEnygmaPoint{
			C1: resp.PublicSignal[txCommitOffset+2*i],
			C2: resp.PublicSignal[txCommitOffset+2*i+1],
		}
	}
	return proof8, pubSigStrs, deltas
}

// relayTransferBody builds the JSON body for POST /relay/transfer.
func relayTransferBody(proof8 [8]string, pubSigStrs []string, deltas []enygma.IEnygmaPoint) interface{} {
	commFinal := make([][]string, nBanks)
	for i, d := range deltas {
		commFinal[i] = []string{d.C1.String(), d.C2.String()}
	}
	kIdx64 := make([]int64, nBanks)
	for i := range kIdx64 {
		kIdx64[i] = int64(i + 1)
	}
	return struct {
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
}

// postRelayTransfer POSTs a transfer to the relayer and returns (statusCode, responseBody).
func postRelayTransfer(t *testing.T, apiKey string, req interface{}) (int, []byte) {
	t.Helper()
	data, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/transfer", bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("relay POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// ── Epoch helpers ─────────────────────────────────────────────────────────────

// epochStartFor mirrors Enygma.sol: floor(blockNum / interval) * interval.
func epochStartFor(blockNum, interval *big.Int) *big.Int {
	q := new(big.Int).Div(blockNum, interval)
	return new(big.Int).Mul(q, interval)
}

// blocksToNextEpoch returns how many blocks to mine to reach the next epoch start.
func blocksToNextEpoch(blockNum, interval *big.Int) uint64 {
	next := new(big.Int).Add(epochStartFor(blockNum, interval), interval)
	delta := new(big.Int).Sub(next, blockNum)
	return delta.Uint64()
}

// readEpochInterval calls epochInterval() view function on the contract via raw ABI.
// The Go bindings pre-date the immutable, so we call it via crypto.Keccak256 selector.
func readEpochInterval(t *testing.T, ctx context.Context, client *ethclient.Client, addr common.Address) *big.Int {
	t.Helper()
	sel := crypto.Keccak256([]byte("epochInterval()"))[:4]
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: sel}, nil)
	if err != nil {
		t.Fatalf("call epochInterval(): %v", err)
	}
	if len(out) == 0 {
		t.Fatal("epochInterval() returned empty — is this the updated contract?")
	}
	return new(big.Int).SetBytes(out)
}

// mineBlocks mines n blocks on the Hardhat node via the hardhat_mine RPC extension.
func mineBlocks(t *testing.T, rpcClient *rpc.Client, n uint64) {
	t.Helper()
	var result json.RawMessage
	if err := rpcClient.Call(&result, "hardhat_mine", fmt.Sprintf("0x%x", n)); err != nil {
		t.Fatalf("hardhat_mine(%d): %v", n, err)
	}
}

// currentBlock returns the current chain block number as *big.Int.
func currentBlock(t *testing.T, ctx context.Context, client *ethclient.Client) *big.Int {
	t.Helper()
	bn, err := client.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("blockNumber: %v", err)
	}
	return new(big.Int).SetUint64(bn)
}

// ── Test ───────────────────────────────────────────────────────────────────────

func TestEpochIntervalFlow(t *testing.T) {
	if !tcpAvailable("127.0.0.1:8545") {
		t.Skip("chain not reachable at 127.0.0.1:8545 — start Hardhat and deploy:\n  EPOCH_INTERVAL=5 python3 run_scripts/deploy_direct.py")
	}

	ctx := context.Background()

	// Create a raw RPC client (for hardhat_mine) and wrap it as ethclient.
	rpcClient, err := rpc.Dial(chainURL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	defer rpcClient.Close()
	client := ethclient.NewClient(rpcClient)

	tokenAddr, verifierAddr := readReceipts(t)
	t.Logf("TOKEN:    %s", tokenAddr)
	t.Logf("VERIFIER: %s", verifierAddr)

	contractAddr := common.HexToAddress(tokenAddr)
	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		t.Fatalf("create contract: %v", err)
	}

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))

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
		r, e := bind.WaitMined(ctx, client, tx)
		if e != nil {
			t.Fatalf("wait mined: %v", e)
		}
		return r
	}

	// ── Phase 1: Read epochInterval from contract ─────────────────────────────
	t.Log("=== Phase 1: epoch configuration ===")

	interval := readEpochInterval(t, ctx, client, contractAddr)
	t.Logf("epochInterval = %s blocks", interval)
	if interval.Sign() == 0 {
		t.Fatal("epochInterval = 0 — contract was deployed without epoch parameter; re-deploy with EPOCH_INTERVAL=5")
	}

	// ── Phase 2: Contract setup ───────────────────────────────────────────────
	t.Log("=== Phase 2: setup ===")

	if tx, txErr := instance.Initialize(mkAuth()); txErr == nil {
		if r, _ := bind.WaitMined(ctx, client, tx); r != nil && r.Status == 1 {
			t.Log("contract initialized")
		}
	}
	if r := waitTx(instance.AddVerifier(mkAuth(), common.HexToAddress(verifierAddr))); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}

	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, e := poseidon.Hash([]*big.Int{sk, sk})
		if e != nil {
			t.Fatalf("pk[%d]: %v", i, e)
		}
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	t.Logf("registered %d banks (accountIds 1–%d)", nBanks, nBanks)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("initial mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

	// ── Phase 3: Verify lastBlockNum is epoch-aligned ─────────────────────────
	t.Log("=== Phase 3: epoch alignment ===")

	lastBlockNum, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("GetBlckHash: %v", err)
	}
	rem := new(big.Int).Mod(lastBlockNum, interval)
	if rem.Sign() != 0 {
		t.Errorf("lastBlockNum=%s is NOT a multiple of epochInterval=%s (remainder=%s)",
			lastBlockNum, interval, rem)
	} else {
		t.Logf("lastBlockNum=%s is epoch-aligned (mod %s = 0) ✓", lastBlockNum, interval)
	}

	blockAfterSetup := currentBlock(t, ctx, client)
	t.Logf("block.number=%s, lastBlockNum=%s", blockAfterSetup, lastBlockNum)
	if blockAfterSetup.Cmp(lastBlockNum) > 0 {
		t.Logf("block.number (%s) > lastBlockNum (%s) — confirms epoch slot is used, not raw block.number ✓",
			blockAfterSetup, lastBlockNum)
	}

	// ── Phase 4: Mining blocks without a tx must NOT advance lastBlockNum ─────
	t.Log("=== Phase 4: no-tx block mining ===")

	snapshot4 := new(big.Int).Set(lastBlockNum)
	mineBlocks(t, rpcClient, 2)
	blockAfterMine := currentBlock(t, ctx, client)

	lastAfterMine, _ := instance.GetBlckHash(&bind.CallOpts{})
	if lastAfterMine.Cmp(snapshot4) != 0 {
		t.Errorf("lastBlockNum changed without a tx: %s → %s (mined 2 blocks)", snapshot4, lastAfterMine)
	} else {
		t.Logf("mined 2 blocks (block.number now %s), lastBlockNum still %s — no-tx advance correctly prevented ✓",
			blockAfterMine, lastAfterMine)
	}

	// ── Phase 5: mintSupply within epoch — slot must be epochStart, not block.number
	t.Log("=== Phase 5: mint within epoch ===")

	blockBeforeMint5 := currentBlock(t, ctx, client)
	expectedSlot5 := epochStartFor(blockBeforeMint5, interval)
	// Mint to bank 2 (accountId=2), NOT bank 0 (sender), so senderPrevV=500 stays correct for phase 7.
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(50), big.NewInt(2))); r.Status != 1 {
		t.Fatal("mintSupply (phase 5) failed")
	}
	blockAfterMint5 := currentBlock(t, ctx, client)
	lastAfterMint5, _ := instance.GetBlckHash(&bind.CallOpts{})

	t.Logf("blockBefore=%s, blockAfter=%s, epochStart=%s, lastBlockNum=%s",
		blockBeforeMint5, blockAfterMint5, expectedSlot5, lastAfterMint5)

	if lastAfterMint5.Cmp(expectedSlot5) != 0 {
		t.Errorf("lastBlockNum=%s ≠ epochStart=%s — expected epoch-start slot to be used",
			lastAfterMint5, expectedSlot5)
	} else {
		t.Logf("lastBlockNum == epochStart=%s ✓ (block.number=%s was NOT used)", lastAfterMint5, blockAfterMint5)
	}
	if blockAfterMint5.Cmp(lastAfterMint5) != 0 {
		t.Logf("confirmed: block.number (%s) ≠ lastBlockNum (%s) ✓", blockAfterMint5, lastAfterMint5)
	}

	// ── Phase 6: Cross epoch boundary — lastBlockNum must advance ─────────────
	t.Log("=== Phase 6: epoch boundary crossing ===")

	epochStart6pre := new(big.Int).Set(lastAfterMint5)
	blockNow := currentBlock(t, ctx, client)
	toMine := blocksToNextEpoch(blockNow, interval)
	t.Logf("mining %d blocks to cross epoch boundary (current block=%s, epochInterval=%s)",
		toMine, blockNow, interval)
	mineBlocks(t, rpcClient, toMine)

	blockAtBoundary := currentBlock(t, ctx, client)
	expectedSlot6 := epochStartFor(blockAtBoundary, interval)
	t.Logf("at boundary: block.number=%s, expectedNewEpochStart=%s", blockAtBoundary, expectedSlot6)

	// Mint to bank 3 (accountId=3), not sender, to keep senderPrevV=500 intact.
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(10), big.NewInt(3))); r.Status != 1 {
		t.Fatal("mintSupply at epoch boundary failed")
	}

	blockAfterMint6 := currentBlock(t, ctx, client)
	lastAfterMint6, _ := instance.GetBlckHash(&bind.CallOpts{})
	newEpochSlot := epochStartFor(blockAfterMint6, interval)

	t.Logf("after crossing mint: block=%s, lastBlockNum=%s, expectedEpochStart=%s",
		blockAfterMint6, lastAfterMint6, newEpochSlot)

	if lastAfterMint6.Cmp(newEpochSlot) != 0 {
		t.Errorf("lastBlockNum=%s ≠ newEpochStart=%s after epoch crossing", lastAfterMint6, newEpochSlot)
	} else {
		t.Logf("lastBlockNum advanced to new epochStart=%s ✓", lastAfterMint6)
	}
	if lastAfterMint6.Cmp(epochStart6pre) == 0 {
		t.Error("lastBlockNum did NOT advance after epoch boundary — epoch crossing not detected")
	} else {
		t.Logf("epoch advance confirmed: %s → %s ✓", epochStart6pre, lastAfterMint6)
	}

	// ── Phases 7-8: ZK transfer + stale proof rejection ───────────────────────
	if !tcpAvailable("127.0.0.1:8080") {
		t.Log("=== Phases 7-8: SKIPPED — gnark server not available at 127.0.0.1:8080 ===")
		return
	}
	if !tcpAvailable("127.0.0.1:8082") {
		t.Log("=== Phases 7-8: SKIPPED — relayer not available at 127.0.0.1:8082 ===")
		return
	}

	apiKey := os.Getenv("RELAYER_API_KEY")
	if apiKey == "" {
		apiKey = relayerKey
	}

	// ── Phase 7: Generate ZK proof at current epoch, verify lastBlockNum = epochStart ──
	t.Log("=== Phase 7: ZK transfer — verify epoch slot usage ===")

	epochStart7, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("GetBlckHash (phase 7): %v", err)
	}
	t.Logf("proof will be generated at blockNum=%s (epoch start)", epochStart7)

	pubVals7, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("GetPublicValues (phase 7): %v", err)
	}
	prevBalances7 := pubVals7.Balances[1:]
	onChainKeys7 := pubVals7.Keys[1:]

	// Generate proof — saved, NOT submitted yet (will be the stale proof in phase 8).
	proof8_7, pubSig7, deltas7 := callGnarkServer(t, gnarkProofBody(t, new(big.Int).Set(epochStart7), prevBalances7, onChainKeys7))

	// ── Phase 8: Advance to the next epoch WITHOUT changing balances ───────────
	// mintSupply(0) adds the Baby Jubjub identity point — a no-op for commitments.
	// This advances lastBlockNum without altering any bank's balance, so
	// _verifyPublicInputs still passes for the stale proof but _verifyBlockNumber fails.
	t.Log("=== Phase 8: advance epoch (mintSupply(0) = identity — no balance change) ===")

	blockBeforeAdvance := currentBlock(t, ctx, client)
	toMine8 := blocksToNextEpoch(blockBeforeAdvance, interval)
	t.Logf("mining %d blocks to next epoch boundary", toMine8)
	mineBlocks(t, rpcClient, toMine8)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(0), big.NewInt(3))); r.Status != 1 {
		t.Fatal("identity mint (phase 8) failed")
	}

	epochStart8, _ := instance.GetBlckHash(&bind.CallOpts{})
	t.Logf("after identity mint: lastBlockNum=%s (was %s)", epochStart8, epochStart7)
	if epochStart8.Cmp(epochStart7) == 0 {
		t.Fatal("lastBlockNum did NOT advance after epoch boundary + mint")
	}
	t.Logf("epoch advanced: %s → %s ✓", epochStart7, epochStart8)

	pubVals8, _ := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	prevBalances8 := pubVals8.Balances[1:]
	for i := 0; i < nBanks; i++ {
		if prevBalances8[i].C1.Cmp(prevBalances7[i].C1) != 0 || prevBalances8[i].C2.Cmp(prevBalances7[i].C2) != 0 {
			t.Errorf("bank %d balance changed after identity mint (should be unchanged)", i)
		}
	}
	t.Log("all balances unchanged after identity mint ✓")

	// ── Submit stale proof — must be rejected with InvalidBlockNumber ─────────
	// Hardhat pre-simulates reverts → relayer gets a send error (HTTP 500).
	// On a real node the tx mines with Status=0 → relayer returns HTTP 400.
	// Both are valid rejection signals; we accept either.
	t.Log("submitting stale proof (proof.blockNum=epochStart7, chain.lastBlockNum=epochStart8) — expect 400/500...")
	staleReq := relayTransferBody(proof8_7, pubSig7, deltas7)
	code, body8 := postRelayTransfer(t, apiKey, staleReq)
	if code != http.StatusBadRequest && code != http.StatusInternalServerError {
		t.Errorf("stale proof: got HTTP %d, want 400 or 500\nbody: %s", code, body8)
	} else if !bytes.Contains(body8, []byte("InvalidBlockNumber")) {
		t.Errorf("stale proof rejected (HTTP %d) but error missing InvalidBlockNumber:\nbody: %s", code, body8)
	} else {
		t.Logf("stale proof correctly rejected (HTTP %d, InvalidBlockNumber) ✓\nmessage: %s", code, body8)
	}

	lastAfterStale, _ := instance.GetBlckHash(&bind.CallOpts{})
	if lastAfterStale.Cmp(epochStart8) != 0 {
		t.Errorf("lastBlockNum changed after reverted tx: %s → %s", epochStart8, lastAfterStale)
	} else {
		t.Logf("lastBlockNum unchanged after reverted tx (%s) ✓", lastAfterStale)
	}

	// ── Phase 9: Fresh proof at new epoch — must succeed ──────────────────────
	t.Log("=== Phase 9: fresh proof at new epoch — verify accepted ===")

	proof8_9, pubSig9, deltas9 := callGnarkServer(t, gnarkProofBody(t, new(big.Int).Set(epochStart8), prevBalances8, onChainKeys7))

	blockBeforeTransfer9 := currentBlock(t, ctx, client)
	expectedSlot9 := epochStartFor(blockBeforeTransfer9, interval)

	code9, body9 := postRelayTransfer(t, apiKey, relayTransferBody(proof8_9, pubSig9, deltas9))
	if code9 != http.StatusOK {
		t.Fatalf("fresh proof rejected (HTTP %d): %s", code9, body9)
	}
	t.Logf("fresh proof accepted (HTTP 200) ✓\nrelayer response: %s", body9)

	blockAfterTransfer9 := currentBlock(t, ctx, client)
	lastAfterTransfer9, _ := instance.GetBlckHash(&bind.CallOpts{})
	actualSlot9 := epochStartFor(blockAfterTransfer9, interval)

	t.Logf("after transfer: block=%s, lastBlockNum=%s, expectedEpochStart=%s",
		blockAfterTransfer9, lastAfterTransfer9, actualSlot9)

	if lastAfterTransfer9.Cmp(actualSlot9) != 0 {
		t.Errorf("lastBlockNum=%s ≠ epochStart=%s after transfer", lastAfterTransfer9, actualSlot9)
	} else {
		t.Logf("lastBlockNum=%s == epochStart ✓ (block.number=%s)", lastAfterTransfer9, blockAfterTransfer9)
		_ = expectedSlot9
	}

	newBal, e := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if e != nil {
		t.Fatalf("getBalance(1): %v", e)
	}
	if newBal.X.Cmp(prevBalances8[0].C1) == 0 && newBal.Y.Cmp(prevBalances8[0].C2) == 0 {
		t.Error("bank 0 balance unchanged after phase-9 transfer")
	} else {
		t.Logf("bank 0 balance updated after transfer ✓")
	}
}

// ── TestBlockNumberMismatch ────────────────────────────────────────────────────

// TestBlockNumberMismatch is a focused negative test that verifies the contract's
// _verifyBlockNumber guard: a ZK proof whose embedded blockNum does not match the
// contract's current lastBlockNum is ALWAYS rejected, regardless of whether the
// proof's Pedersen inputs are otherwise valid.
//
// Scenario:
//
//	Generate proof  → blockNum = proofEpoch  (e.g. 10)
//	Advance chain   → lastBlockNum = chainEpoch (e.g. 15, via mine + mintSupply(0))
//	Submit proof    → proofEpoch ≠ chainEpoch → InvalidBlockNumber() revert
//
// mintSupply(0) adds the Baby Jubjub identity point to the recipient commitment,
// which is a cryptographic no-op (C + identity = C). This advances lastBlockNum to
// the new epoch without altering any bank's balance, so _verifyPublicInputs still
// passes and only _verifyBlockNumber fires as the rejection cause.
//
// Prerequisites: fresh Hardhat node, gnark server on :8080, relayer on :8082.
//
// Run alone (requires fresh chain):
//
//	CC=/usr/bin/clang go test -run TestBlockNumberMismatch -v -timeout 300s
func TestBlockNumberMismatch(t *testing.T) {
	if !tcpAvailable("127.0.0.1:8545") {
		t.Skip("chain not reachable at :8545 — start Hardhat with EPOCH_INTERVAL=5")
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at :8080")
	}
	if !tcpAvailable("127.0.0.1:8082") {
		t.Skip("relayer not reachable at :8082")
	}

	ctx := context.Background()

	rpcClient, err := rpc.Dial(chainURL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	defer rpcClient.Close()
	client := ethclient.NewClient(rpcClient)

	tokenAddr, verifierAddr := readReceipts(t)
	contractAddr := common.HexToAddress(tokenAddr)
	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		t.Fatalf("new contract: %v", err)
	}

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))

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
		r, e := bind.WaitMined(ctx, client, tx)
		if e != nil {
			t.Fatalf("wait mined: %v", e)
		}
		return r
	}

	apiKey := os.Getenv("RELAYER_API_KEY")
	if apiKey == "" {
		apiKey = relayerKey
	}

	// ── Setup ─────────────────────────────────────────────────────────────────
	if tx, txErr := instance.Initialize(mkAuth()); txErr == nil {
		bind.WaitMined(ctx, client, tx)
	}
	if r := waitTx(instance.AddVerifier(mkAuth(), common.HexToAddress(verifierAddr))); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}
	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr, big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("setup complete: 6 banks registered, %d minted to bank 0", mintAmt)

	interval := readEpochInterval(t, ctx, client, contractAddr)

	// ── Step 1: capture current epoch and generate proof ─────────────────────
	proofEpoch, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("GetBlckHash: %v", err)
	}
	t.Logf("step 1: proofEpoch = %s (will embed this blockNum in the proof)", proofEpoch)

	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("GetPublicValues: %v", err)
	}

	proof8, pubSig, deltas := callGnarkServer(t,
		gnarkProofBody(t, new(big.Int).Set(proofEpoch), pubVals.Balances[1:], pubVals.Keys[1:]))

	// ── Step 2: advance blockchain epoch without touching balances ────────────
	// Mine to the next epoch boundary, then mintSupply(0) which adds the
	// Baby Jubjub identity element — a cryptographic no-op for all commitments.
	// After this, lastBlockNum = chainEpoch ≠ proofEpoch.
	blockNow := currentBlock(t, ctx, client)
	toMine := blocksToNextEpoch(blockNow, interval)
	t.Logf("step 2: mining %d blocks to next epoch boundary...", toMine)
	mineBlocks(t, rpcClient, toMine)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(0), big.NewInt(2))); r.Status != 1 {
		t.Fatal("identity mint to advance epoch failed")
	}

	chainEpoch, _ := instance.GetBlckHash(&bind.CallOpts{})
	if chainEpoch.Cmp(proofEpoch) == 0 {
		t.Fatalf("epoch did not advance: expected chainEpoch > %s", proofEpoch)
	}
	t.Logf("step 2: blockchain epoch advanced %s → %s", proofEpoch, chainEpoch)
	t.Logf("        MISMATCH: proof.blockNum=%s ≠ chain.lastBlockNum=%s", proofEpoch, chainEpoch)

	// ── Step 3: submit proof — must be rejected ───────────────────────────────
	// The ZK proof is cryptographically valid (correct Pedersen commitments,
	// valid snark), but its embedded blockNum no longer matches lastBlockNum.
	// _verifyBlockNumber catches this and reverts with InvalidBlockNumber().
	//
	// Hardhat pre-simulates the revert at the RPC layer → relayer gets a send
	// error and returns HTTP 500. On a real chain the tx mines with Status=0
	// and the relayer returns HTTP 400. Both confirm rejection.
	t.Logf("step 3: submitting proof (blockNum=%s) to chain (lastBlockNum=%s)...", proofEpoch, chainEpoch)
	code, respBody := postRelayTransfer(t, apiKey, relayTransferBody(proof8, pubSig, deltas))

	t.Logf("relayer response: HTTP %d — %s", code, respBody)

	if code != http.StatusBadRequest && code != http.StatusInternalServerError {
		t.Errorf("FAIL: expected HTTP 400 or 500 (InvalidBlockNumber revert), got %d", code)
	} else if !bytes.Contains(respBody, []byte("InvalidBlockNumber")) {
		t.Errorf("FAIL: rejection occurred (HTTP %d) but error does not name InvalidBlockNumber\nbody: %s", code, respBody)
	} else {
		t.Logf("PASS: proof correctly rejected — proof.blockNum=%s ≠ chain.lastBlockNum=%s → InvalidBlockNumber() ✓",
			proofEpoch, chainEpoch)
	}

	// ── Step 4: confirm chain state is unchanged ──────────────────────────────
	// A reverted transaction must not modify lastBlockNum.
	lastAfter, _ := instance.GetBlckHash(&bind.CallOpts{})
	if lastAfter.Cmp(chainEpoch) != 0 {
		t.Errorf("lastBlockNum mutated after reverted tx: %s → %s", chainEpoch, lastAfter)
	} else {
		t.Logf("step 4: lastBlockNum still %s after reverted tx — chain state intact ✓", lastAfter)
	}
}
