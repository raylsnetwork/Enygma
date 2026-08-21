package tests

// TestRetailErc20_PaymentRelayerFeePublic exercises the combined PaymentRelayerFeePublic circuit.
//
// Fee model:
//
//	Alice pays Bob AND leaves a spendable fee note for the relayer (Carol).
//	  valueIn = paymentAmt + changeAmt + StFee
//	  StFee is a PUBLIC signal — the chain can verify the exact fee Alice paid the relayer.
//	  Relayer receives a real, spendable note commitment (not a burned amount).
//
//	depositAmt    = 45 (payment(30) + change(10) + relayerFee(5))
//	paymentAmt    = 30 (to Bob)
//	changeAmt     = 10 (back to Alice)
//	relayerFeeAmt =  5 (spendable note for relayer / Carol) — PUBLICLY VISIBLE
//
// Signal layout (1-in / 3-out, 9 elements):
//
//	signal[0]  StMessage             = 0
//	signal[1]  StTreeNumbers[0]      = tree index
//	signal[2]  StMerkleRoots[0]      = Merkle root
//	signal[3]  StNullifiers[0]       = nullifier
//	signal[4]  StCommitmentsOut[0]   = Bob's commitment   (output 0)
//	signal[5]  StCommitmentsOut[1]   = Alice's change     (output 1, constrained to senderPk)
//	signal[6]  StCommitmentsOut[2]   = Relayer fee note   (output 2)
//	signal[7]  StContractAddress     = vault address
//	signal[8]  StFee                 = relayer fee amount (NEW — publicly verifiable)
//
// Prerequisites — all four services must be running:
//
//	Terminal 1: cd ../enygma_dvp && npx hardhat node
//	Terminal 2: bash setup.sh                                        (from enygma_retail_payments/)
//	Terminal 3: cd gnark_circuits && go run generation.go && go run main.go
//	Terminal 4: cd relayer && RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=<token> \
//	              RELAYER_MIN_FEE=0 go run main.go
//	            (RELAYER_MIN_FEE must be <= 5, or unset/0, or the relayer
//	             rejects this test's relayerFeeAmt=5 with 402 before ever
//	             touching the chain — see relayer/config's RELAYER_MIN_FEE doc)
//
// Run:
//
//	cd test && go test -run TestRetailErc20_PaymentRelayerFeePublic -v -timeout 300s
//
// On-chain submission:
//
//	EnygmaDvp.paymentWithRelayerFee() and the relayer's POST /relay/payment_fee
//	both exist and are exercised end-to-end below: the proof built in steps 1-10
//	is sent to the relayer, which enforces RELAYER_MIN_FEE against the public
//	StFee signal, dry-runs, and submits paymentWithRelayerFee() on Alice's
//	behalf — she never signs an Ethereum transaction or touches gas.

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

const (
	rfpDeposit    = 45 // paymentAmt(30) + changeAmt(10) + relayerFeeAmt(5)
	rfpPayAmt     = 30
	rfpChangeAmt  = 10
	rfpRelayerAmt = 5
)

// gnarkURL is the base URL for the gnark proof server.
const gnarkURL = "http://localhost:8082"

func TestRetailErc20_PaymentRelayerFeePublic(t *testing.T) {
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
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	erc20ABI := loadOnchainABI(t, "RaylsERC20")
	vaultABI  := loadOnchainABI(t, "Erc20CoinVault")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)

	aliceAuth := hardhatAuth(t, client)
	bobAuth   := hardhatBobAuth(t, client)

	merkleDepth := 8
	tokenId     := big.NewInt(0)
	depositAmt  := big.NewInt(int64(rfpDeposit))
	paymentAmt  := big.NewInt(int64(rfpPayAmt))
	changeAmt   := big.NewInt(int64(rfpChangeAmt))
	relayerAmt  := big.NewInt(int64(rfpRelayerAmt))
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

	// ── Step 1: Key generation ────────────────────────────────────────────────
	t.Log("Step 1 — generating ZK key pairs for Alice, Bob, and relayer (Carol)")

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
	relayerSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Relayer NewSpendKeyPair: %v", err)
	}

	t.Logf("  Alice   pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob     pk_spend: %s", bobSpend.PublicKey)
	t.Logf("  Relayer pk_spend: %s", relayerSpend.PublicKey)

	auditorPair, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatalf("NewAuditorKeyPair: %v", err)
	}
	aliceMlKemCt, aliceAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, aliceView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Alice): %v", err)
	}
	bobMlKemCt, bobAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, bobView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Bob): %v", err)
	}

	// ── Step 2: Registration ─────────────────────────────────────────────────
	t.Log("Step 2 — registering keys on-chain (UserRegistry)")

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Log("  Alice already registered")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Log("  Bob already registered")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ── Step 3: Deposit — Alice deposits 45 tokens ───────────────────────────
	t.Logf("Step 3 — Alice deposits %d tokens (payment=%d + change=%d + relayerFee=%d)",
		rfpDeposit, rfpPayAmt, rfpChangeAmt, rfpRelayerAmt)

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
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, aliceDepositCtxtII)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	depositReceipt, err := bind.WaitMined(ctx, client, depositTx)
	if err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}
	t.Logf("  depositV2 mined (block %d, gas %d, commitment %s)",
		depositReceipt.BlockNumber, depositReceipt.GasUsed, aliceCommitment)

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}
	t.Logf("  Merkle root: %s", aliceProof.Root)

	// ── Step 4: Build PaymentRelayerFeePublic proof request ──────────────────
	t.Logf("Step 4 — building PaymentRelayerFeePublic proof request (pay=%d to Bob, change=%d to Alice, fee=%d to relayer, fee is PUBLIC)",
		rfpPayAmt, rfpChangeAmt, rfpRelayerAmt)

	// Compute nullifier: Poseidon(sk, pathIndices)
	nullifier, err := dvpcore.GetNullifier(aliceSpend.PrivateKey, aliceProof.Indices)
	if err != nil {
		t.Fatalf("GetNullifier: %v", err)
	}
	t.Logf("  nullifier: %s", nullifier)

	// Output 0: Bob's payment — ML-KEM delivery
	ssBob, ctxtBob, err := rpcore.Encapsulate(bobView.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate (Bob output): %v", err)
	}
	saltBobRaw, err := rpcore.DerivePaymentSalt(ssBob)
	if err != nil {
		t.Fatalf("DerivePaymentSalt (Bob): %v", err)
	}
	encKeyBob, err := rpcore.DerivePaymentKey(ssBob)
	if err != nil {
		t.Fatalf("DerivePaymentKey (Bob): %v", err)
	}
	ctxtIIBob, err := rpcore.EncryptPayload(encKeyBob, tokenId, paymentAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (Bob): %v", err)
	}
	saltBobField := rpcore.SaltBToField(saltBobRaw)
	cmtBob, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, saltBobField, paymentAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob): %v", err)
	}

	// Output 1: Alice's change — random salt (SaltA)
	saltA, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (change): %v", err)
	}
	cmtChange, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, saltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change): %v", err)
	}

	// Output 2: Relayer fee note — random salt (SaltRelayer)
	saltRelayer, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (relayer): %v", err)
	}
	cmtRelayer, err := rpcore.Erc20CommitmentV2(relayerSpend.PublicKey, saltRelayer, relayerAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (relayer): %v", err)
	}

	// Build path elements as [8]string
	var pathElements [8]string
	for j, elem := range aliceProof.Elements {
		if j >= 8 {
			break
		}
		pathElements[j] = elem.String()
	}

	reqBody := map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{"0"},
		"stMerkleRoots":        [1]string{aliceProof.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [3]string{cmtBob.String(), cmtChange.String(), cmtRelayer.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                relayerAmt.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [3]string{bobSpend.PublicKey.String(), aliceSpend.PublicKey.String(), relayerSpend.PublicKey.String()},
		"wtValuesOut":          [3]string{paymentAmt.String(), changeAmt.String(), relayerAmt.String()},
		"wtSaltsOut":           [3]string{saltBobField.String(), saltA.String(), saltRelayer.String()},
	}

	// ── Step 5: Request proof from gnark server ───────────────────────────────
	t.Log("Step 5 — requesting PaymentRelayerFeePublic proof from gnark server")

	proofResp, err := postProof(gnarkURL+"/proof/paymentRelayerFeePublic", reqBody)
	if err != nil {
		t.Fatalf("postProof paymentRelayerFeePublic: %v", err)
	}
	t.Log("  PaymentRelayerFeePublic proof generated ✓")

	// ── Step 6: Verify public signals ────────────────────────────────────────
	t.Log("Step 6 — verifying public signals (9 elements)")

	sig := proofResp.PublicSignal
	if len(sig) != 9 {
		t.Fatalf("expected 9 public signals, got %d", len(sig))
	}
	t.Logf("  signal[0] msg           = %s", sig[0])
	t.Logf("  signal[1] treeNum       = %s", sig[1])
	t.Logf("  signal[2] root          = %s", sig[2])
	t.Logf("  signal[3] nullifier     = %s", sig[3])
	t.Logf("  signal[4] cmt_bob       = %s", sig[4])
	t.Logf("  signal[5] cmt_change    = %s", sig[5])
	t.Logf("  signal[6] cmt_relayer   = %s", sig[6])
	t.Logf("  signal[7] contractAddr  = %s", sig[7])
	t.Logf("  signal[8] StFee         = %s", sig[8])

	checkBig := func(label string, got, want *big.Int) {
		t.Helper()
		if got.Cmp(want) != 0 {
			t.Errorf("%s: got %s, want %s", label, got, want)
		} else {
			t.Logf("  %s ✓", label)
		}
	}

	checkBig("signal[2] merkle root", sig[2], aliceProof.Root)
	checkBig("signal[3] nullifier",   sig[3], nullifier)
	checkBig("signal[4] cmt_bob",     sig[4], cmtBob)
	checkBig("signal[5] cmt_change",  sig[5], cmtChange)
	checkBig("signal[6] cmt_relayer", sig[6], cmtRelayer)
	checkBig("signal[7] contractAddr",sig[7], vaultAddrBig)
	checkBig("signal[8] StFee",       sig[8], relayerAmt)

	// ── Step 7: Bob scans his note via ML-KEM ────────────────────────────────
	t.Log("Step 7 — Bob scans his note via ML-KEM")

	bobEvents := []dvpcore.OnChainErc20Event{{
		Commitment: cmtBob,
		CipherText: ctxtBob,
		EncTxData:  ctxtIIBob,
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
	t.Logf("  Bob's note: amount=%s tokenId=%s ✓", bobNotes[0].Amount, bobNotes[0].TokenId)

	// ── Step 8: Alice verifies her change commitment ──────────────────────────
	t.Log("Step 8 — Alice verifies her change commitment locally")

	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, saltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change verify): %v", err)
	}
	if aliceChangeCmt.Cmp(sig[5]) != 0 {
		t.Errorf("Alice's change commitment mismatch: got %s, want %s", aliceChangeCmt, sig[5])
	}
	t.Logf("  Alice's change note: amount=%s saltA=%s ✓", changeAmt, saltA)

	// ── Step 9: Relayer verifies fee commitment ───────────────────────────────
	t.Log("Step 9 — relayer verifies fee commitment locally (using SaltRelayer)")

	relayerFeeCmt, err := rpcore.Erc20CommitmentV2(relayerSpend.PublicKey, saltRelayer, relayerAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (relayer verify): %v", err)
	}
	if relayerFeeCmt.Cmp(sig[6]) != 0 {
		t.Errorf("Relayer fee commitment mismatch: got %s, want %s", relayerFeeCmt, sig[6])
	}
	t.Logf("  Relayer fee note: amount=%s saltRelayer=%s ✓", relayerAmt, saltRelayer)

	// ── Step 10: Confirm public fee is visible ────────────────────────────────
	t.Log("Step 10 — confirming public fee signal equals relayer fee amount")

	if sig[8].Cmp(relayerAmt) != 0 {
		t.Errorf("public StFee signal mismatch: got %s, want %s", sig[8], relayerAmt)
	}
	t.Logf("  StFee=%s is publicly visible on-chain ✓ (anyone can verify Alice paid the relayer exactly %s tokens)",
		sig[8], relayerAmt)

	// ── Step 11: Relay — Alice sends the proof to the relayer ────────────────
	// Alice does NOT sign an Ethereum transaction. The relayer enforces
	// RELAYER_MIN_FEE against sig[8] (StFee), dry-runs, and submits
	// paymentWithRelayerFee() on her behalf.
	t.Log("Step 11 — Alice sends proof to relayer (POST /relay/payment_fee)")

	relayReq := relayPaymentFeeRequest{
		VaultId:      "0",
		Proof:        bigsToArray8(t, proofResp.Proof),
		PublicSignal: bigsToArray9(t, sig),
		CipherText:   "0x" + hex.EncodeToString(ctxtBob),
		EncTxData:    "0x" + hex.EncodeToString(ctxtIIBob),
	}

	relayResp, statusCode, err := postPaymentFeeToRelayer(t, relayerAPIKey, relayReq)
	if err != nil {
		t.Fatalf("postPaymentFeeToRelayer: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", statusCode, relayResp.Error)
	}
	t.Logf("  Relayer response: txHash=%s block=%d gasUsed=%d",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)

	// ── Step 12: Verify on-chain events and that the relayer, not Alice, signed ─
	t.Log("Step 12 — verifying on-chain Payment + Nullifier events")

	txHash := common.HexToHash(relayResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionReceipt(%s): %v", relayResp.TxHash, err)
	}

	paymentEventSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierEventSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	var paymentEvents, nullifierEvents int
	for _, lg := range txReceipt.Logs {
		switch lg.Topics[0] {
		case paymentEventSig:
			paymentEvents++
			t.Logf("  Payment event: commitment=%s", lg.Topics[2].Big())
		case nullifierEventSig:
			nullifierEvents++
			t.Logf("  Nullifier event: nullifier=%s", lg.Topics[3].Big())
		}
	}
	if paymentEvents != 1 {
		t.Errorf("expected 1 Payment event (Bob's destination), got %d", paymentEvents)
	}
	if nullifierEvents != 1 {
		t.Errorf("expected 1 Nullifier event (Alice's input burned), got %d", nullifierEvents)
	}

	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	if senderAddr, err := types.Sender(signer, tx); err == nil {
		if senderAddr == common.HexToAddress(hardhatAliceAddr) {
			t.Error("tx.from == Alice — relayer did not sign the transaction")
		} else {
			t.Logf("  tx.from = %s (relayer, not Alice)", senderAddr.Hex())
		}
	}

	t.Logf("=== PAYMENT RELAYER FEE PUBLIC FLOW COMPLETE (on-chain) ===")
	t.Logf("    Alice paid %s tokens to Bob", paymentAmt)
	t.Logf("    Alice kept %s tokens as change", changeAmt)
	t.Logf("    Relayer earned spendable note of %s tokens (publicly verifiable via StFee)", relayerAmt)
	t.Log("    Alice's Ethereum address was NOT the transaction sender")
}

// ── relayer request/response types (POST /relay/payment_fee) ──────────────────
//
// Mirrors relayer/server.RelayPaymentFeeRequest/RelayPaymentResponse — this
// test module doesn't import the relayer's Go module, so the wire shape is
// duplicated here rather than shared, same as relayPaymentRequest in
// 03_relayer_payment_test.go.

type relayPaymentFeeRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [9]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
}

// postPaymentFeeToRelayer sends a signed (Bearer token) relay request to
// POST /relay/payment_fee and returns the response — see postToRelayer in
// 03_relayer_payment_test.go for the plain-payment equivalent.
func postPaymentFeeToRelayer(t *testing.T, apiKey string, req relayPaymentFeeRequest) (*relayPaymentResponse, int, error) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment_fee", bytes.NewReader(body))
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

// bigsToArray8 converts a []*big.Int of length 8 (the gnark proof response's
// Proof field) into the [8]string decimal array the relayer expects.
func bigsToArray8(t *testing.T, in []*big.Int) [8]string {
	t.Helper()
	if len(in) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(in))
	}
	var out [8]string
	for i, v := range in {
		out[i] = v.String()
	}
	return out
}

// bigsToArray9 converts a []*big.Int of length 9 (the PaymentRelayerFeePublic
// circuit's public signal) into the [9]string decimal array the relayer expects.
func bigsToArray9(t *testing.T, in []*big.Int) [9]string {
	t.Helper()
	if len(in) != 9 {
		t.Fatalf("expected 9 public signal elements, got %d", len(in))
	}
	var out [9]string
	for i, v := range in {
		out[i] = v.String()
	}
	return out
}

// proofResponse mirrors the JSON response from /proof/paymentRelayerFeePublic.
type proofResponse struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
	Error        string     `json:"error"`
}

// postProof POSTs a JSON payload to the gnark server and returns the parsed response.
func postProof(url string, body interface{}) (*proofResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http.Post: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnark server %d: %s", resp.StatusCode, string(raw))
	}
	var out proofResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("gnark error: %s", out.Error)
	}
	return &out, nil
}
