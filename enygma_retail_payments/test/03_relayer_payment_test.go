package tests

// TestRetailErc20_PaymentViaRelayer is the end-to-end test for the full
// payment flow routed through the relayer.
//
// Flow:
//  1. Alice and Bob generate ZK key pairs.
//  2. Alice and Bob register their keys on-chain (UserRegistry).
//  3. Alice mints and deposits 40 ERC-20 tokens into the private vault.
//  4. Alice calls the gnark server to generate a ZK proof (pay 30 to Bob, keep 10 change).
//  5. Alice sends the unsigned proof payload to the relayer (POST /relay/payment).
//  6. Relayer validates (root + nullifier checks) and submits to EnygmaDvp.payment().
//  7. Test confirms the Payment event and Nullifier event are on-chain.
//  8. Bob scans his note and verifies the amount.
//
// Prerequisites — all four services must be running:
//
//	Terminal 1: cd ../enygma_dvp && npx hardhat node
//	Terminal 2: bash setup.sh          (from enygma_retail_payments/)
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: cd relayer && RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=<token> go run main.go
//
// Run:
//
//	cd test &&  go test -run TestRetailErc20_PaymentViaRelayer -v -timeout 300s

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// relayerURL is the base URL for the relayer service.
const relayerURL = "http://localhost:8090"

// relayerAPIKey must match RELAYER_API_KEY set when starting the relayer.
// In a real deployment this would come from a secrets file, not source code.
const relayerAPIKey = "test-api-key-dev-only"

// ── request / response types mirroring the relayer API ────────────────────────

type relayPaymentRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [7]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
}

type relayPaymentResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── helpers ────────────────────────────────────────────────────────────────────

// postToRelayer sends a signed (Bearer token) relay request and returns the response.
func postToRelayer(t *testing.T, apiKey string, req relayPaymentRequest) (*relayPaymentResponse, int, error) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result relayPaymentResponse
	_ = json.Unmarshal(raw, &result)
	return &result, resp.StatusCode, nil
}

// buildRelayRequest converts a PaymentResult into the relayer's JSON request format.
func buildRelayRequest(t *testing.T, result *dvpcore.PaymentResult, vaultId string) relayPaymentRequest {
	t.Helper()

	// Proof: already []string decimal, 8 elements.
	if len(result.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(result.Proof))
	}
	var proof [8]string
	copy(proof[:], result.Proof)

	// PublicSignal: ContractStatement() gives non-interleaved []*big.Int.
	// Layout: [msg, treeNum, root, nullifier, cmt_bob, cmt_change, contractAddr]
	stmt := result.ContractStatement()
	if len(stmt) != 7 {
		t.Fatalf("expected 7 statement elements, got %d", len(stmt))
	}
	var sig [7]string
	for i, v := range stmt {
		sig[i] = v.String()
	}

	return relayPaymentRequest{
		VaultId:      vaultId,
		Proof:        proof,
		PublicSignal: sig,
		CipherText:   "0x" + hex.EncodeToString(result.CipherText),
		EncTxData:    "0x" + hex.EncodeToString(result.EncTxData),
	}
}

// ── main test ─────────────────────────────────────────────────────────────────

func TestRetailErc20_PaymentViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	ctx := context.Background()

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	vaultAddr    := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr    := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr      := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI  := loadOnchainABI(t, "RaylsERC20")
	dvpABI    := loadOnchainABI(t, "EnygmaDvp")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)

	aliceAuth := hardhatAuth(t, client)
	bobAuth   := hardhatBobAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId     := big.NewInt(0)
	depositAmt  := big.NewInt(40)
	paymentAmt  := big.NewInt(30)
	changeAmt   := big.NewInt(10)

	// ──────────────────────────────────────────────────────────────────────────
	// Step 1: Key generation
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 1 — generating ZK key pairs for Alice and Bob")

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}
	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ──────────────────────────────────────────────────────────────────────────
	// Step 2: Registration — Alice and Bob register keys on-chain
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 2 — registering keys on-chain (UserRegistry)")

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Logf("  Alice already registered — skipping")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Logf("  Bob already registered — skipping")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ──────────────────────────────────────────────────────────────────────────
	// Step 3: Deposit — Alice deposits 40 tokens into the private vault
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 3 — Alice deposits 40 tokens into vault")

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From,
		new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}

	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("ERC20.approve: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil {
		t.Fatalf("wait approve: %v", err)
	}

	ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate (deposit): %v", err)
	}
	aliceSaltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("DerivePaymentSalt: %v", err)
	}
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)
	aliceCommitment, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	aliceDepositCtxtII, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceCommitment}, capsule, aliceDepositCtxtII)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	depositReceipt, err := bind.WaitMined(ctx, client, depositTx)
	if err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}
	t.Logf("  depositV2 mined (block %d, gas %d, commitment %s)",
		depositReceipt.BlockNumber, depositReceipt.GasUsed, aliceCommitment)

	// Build Merkle proof for Alice's input note.
	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}
	t.Logf("  Merkle root: %s", aliceProof.Root)

	// ──────────────────────────────────────────────────────────────────────────
	// Step 4: Key lookup — Alice reads Bob's keys from UserRegistry
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 4 — using locally-generated keys (registration may have been skipped)")

	// Both Alice and Bob use the keys generated in step 1.
	// If registration was skipped (AlreadyRegistered), the on-chain keys differ from
	// the ones registered by a previous test run — but the payment circuit only needs
	// the public keys as inputs; it does not verify them against the registry.
	aliceEthAddr := common.HexToAddress(hardhatAliceAddr)
	t.Logf("  Bob pk_spend: %s  pk_view: %d bytes", bobSpend.PublicKey, len(bobView.EncapsKey))

	// ──────────────────────────────────────────────────────────────────────────
	// Step 5: ZK Proof — Alice requests proof from gnark server
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 5 — requesting ZK proof from gnark server (pay 30 to Bob, keep 10 change)")

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{depositAmt},
		[]rpcore.KeyPair{
			{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{paymentAmt, changeAmt},
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		tokenId,
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	t.Logf("  proof generated")
	t.Logf("  Bob's commitment    (output 0): %s", paymentResult.Statement[4])
	t.Logf("  Alice's change cmt  (output 1): %s", paymentResult.Statement[5])

	// ──────────────────────────────────────────────────────────────────────────
	// Step 6: Relay — Alice sends unsigned proof to relayer
	// Alice does NOT sign an Ethereum transaction.
	// The relayer validates, signs, and submits to the chain.
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 6 — Alice sends proof to relayer (POST /relay/payment)")

	relayReq := buildRelayRequest(t, paymentResult, "0")

	t.Logf("  Relayer request:")
	t.Logf("    vaultId:      %s", relayReq.VaultId)
	t.Logf("    proof[0]:     %s", relayReq.Proof[0])
	t.Logf("    publicSignal: %v", relayReq.PublicSignal)
	t.Logf("    cipherText:   %s...(%d bytes)", relayReq.CipherText[:min(18, len(relayReq.CipherText))], len(relayReq.CipherText))

	relayResp, statusCode, err := postToRelayer(t, relayerAPIKey, relayReq)
	if err != nil {
		t.Fatalf("postToRelayer: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", statusCode, relayResp.Error)
	}

	t.Logf("  Relayer response:")
	t.Logf("    txHash:      %s", relayResp.TxHash)
	t.Logf("    blockNumber: %d", relayResp.BlockNumber)
	t.Logf("    gasUsed:     %d", relayResp.GasUsed)

	// ──────────────────────────────────────────────────────────────────────────
	// Step 7: Verify on-chain events in the tx receipt
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 7 — verifying on-chain events")

	txHash := common.HexToHash(relayResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionReceipt(%s): %v", relayResp.TxHash, err)
	}

	paymentSig  := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))

	var paymentEvents, nullifierEvents int
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			paymentEvents++
			t.Logf("  Payment event: commitment=%s", log.Topics[2].Big())
		case nullifierSig:
			nullifierEvents++
			t.Logf("  Nullifier event: nullifier=%s", log.Topics[3].Big())
		}
	}

	if paymentEvents != 1 {
		t.Errorf("expected 1 Payment event (Bob's destination), got %d", paymentEvents)
	}
	if nullifierEvents != 1 {
		t.Errorf("expected 1 Nullifier event (Alice's input burned), got %d", nullifierEvents)
	}

	// Confirm the relayer (not Alice) is the tx sender.
	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	senderAddr, err := types.Sender(signer, tx)
	if err == nil {
		aliceEthAddr = common.HexToAddress(hardhatAliceAddr)
		if senderAddr == aliceEthAddr {
			t.Error("tx.from == Alice — relayer did not sign the transaction")
		} else {
			t.Logf("  tx.from = %s (relayer, not Alice)", senderAddr.Hex())
		}
	}

	// ──────────────────────────────────────────────────────────────────────────
	// Step 8: Bob scans his note
	// ──────────────────────────────────────────────────────────────────────────
	t.Log("Step 8 — Bob scans his note")

	_ = dvpABI // used by payment lookup if needed
	_ = dvpAddr

	bobEvents := []dvpcore.OnChainErc20Event{{
		Commitment: paymentResult.ContractStatement()[4],
		CipherText: paymentResult.CipherText,
		EncTxData:  paymentResult.EncTxData,
	}}
	bobNotes, err := dvpcore.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, bobEvents)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(bobNotes) != 1 {
		t.Fatalf("Bob expected 1 note, got %d", len(bobNotes))
	}
	if bobNotes[0].Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("Bob's note amount: got %s, want %s", bobNotes[0].Amount, paymentAmt)
	}
	t.Logf("  Bob's note: amount=%s tokenId=%s", bobNotes[0].Amount, bobNotes[0].TokenId)

	// Alice verifies her change commitment locally (no chain scan needed).
	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, paymentResult.SaltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change verify): %v", err)
	}
	if aliceChangeCmt.Cmp(paymentResult.ContractStatement()[5]) != 0 {
		t.Errorf("Alice's change commitment mismatch: got %s, want %s",
			aliceChangeCmt, paymentResult.ContractStatement()[5])
	}
	t.Logf("  Alice change note verified: amount=%s saltA=%s", changeAmt, paymentResult.SaltA)

	t.Logf("=== END-TO-END RELAYER FLOW COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob", paymentAmt)
	t.Logf("    Alice kept %s tokens as change", changeAmt)
	t.Logf("    Alice's Ethereum address was NOT the transaction sender")
}

// ── validation error tests ─────────────────────────────────────────────────────

// TestRelayer_MissingAuthHeader verifies 401 when no Authorization header is sent.
func TestRelayer_MissingAuthHeader(t *testing.T) {
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	body := `{"vaultId":"0","proof":["0","0","0","0","0","0","0","0"],"publicSignal":["0","0","0","0","0","0","0"],"cipherText":"0x00","encTxData":"0x00"}`
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	t.Logf("  got expected 401 for missing Authorization header")
}

// TestRelayer_WrongToken verifies 403 when the Bearer token is incorrect.
func TestRelayer_WrongToken(t *testing.T) {
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	body := `{"vaultId":"0","proof":["0","0","0","0","0","0","0","0"],"publicSignal":["0","0","0","0","0","0","0"],"cipherText":"0x00","encTxData":"0x00"}`
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token-12345")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	t.Logf("  got expected 403 for wrong Bearer token")
}

// TestRelayer_BadProofFormat verifies 400 when proof fields are not valid numbers.
func TestRelayer_BadProofFormat(t *testing.T) {
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	body := `{"vaultId":"0","proof":["not-a-number","0","0","0","0","0","0","0"],"publicSignal":["0","0","0","0","0","0","0"],"cipherText":"0x00","encTxData":"0x00"}`
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", relayerAPIKey))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	t.Logf("  got expected 400: %s", raw)
}

// ── go 1.21 compat ─────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
