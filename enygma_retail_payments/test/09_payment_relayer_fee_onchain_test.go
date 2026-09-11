package tests

// TestRetailErc20_PaymentRelayerFeeOnChain finishes what test 08 deliberately
// stopped short of: submitting a PaymentRelayerFeePublic proof through the
// relayer's HTTP API and settling it on-chain via
// EnygmaDvp.paymentWithRelayerFee(), then exercising the two enforcement
// layers that close the gap that existed before this test was added:
//
//   - On-chain: paymentWithRelayerFee() now checks the proof's public StFee
//     signal against the contract-configured relayerFixedFeeAmount (see
//     EnygmaDvp.sol's InvalidRelayerFee revert) — previously ANY fee amount
//     the prover claimed passed verification.
//   - Off-chain: the relayer independently recomputes the fee note's
//     commitment (Erc20CommitmentV2(feeSpendPubKey, feeSalt, StFee, tokenId))
//     and rejects (402) a proof whose fee note isn't actually addressed to
//     its own published spend key — see GET /relay/info's feeSpendPubKey.
//
// Prerequisites — all four services must be running, and the relayer MUST be
// started with a fee spend key configured:
//
//	Terminal 1: cd ../enygma_dvp && npx hardhat node
//	Terminal 2: bash setup.sh          (from enygma_retail_payments/)
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: cd relayer && RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=<token> \
//	              RELAYER_FEE_SPEND_PRIVATE_KEY=<positive decimal> go run main.go
//
// Run:
//
//	cd test && go test -run TestRetailErc20_PaymentRelayerFeeOnChain -v -timeout 300s
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

// ── request / response types mirroring the relayer's fee API ──────────────────

type relayPaymentRelayerFeeRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [9]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
	FeeSalt      string    `json:"feeSalt"`
	TokenId      string    `json:"tokenId"`
}

type relayPaymentRelayerFeeResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

type relayInfoResponse struct {
	RelayerAddr    string `json:"relayerAddr"`
	ChainID        int64  `json:"chainId"`
	FeeSpendPubKey string `json:"feeSpendPubKey"`
}

// fetchRelayerInfo calls GET /relay/info (no auth required).
func fetchRelayerInfo(t *testing.T) relayInfoResponse {
	t.Helper()
	resp, err := http.Get(relayerURL + "/relay/info")
	if err != nil {
		t.Fatalf("GET /relay/info: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /relay/info response: %v", err)
	}
	var out relayInfoResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal /relay/info response: %v", err)
	}
	return out
}

// postRelayerFee sends a signed (Bearer token) relayer-fee request and
// returns the parsed response and status code.
func postRelayerFee(t *testing.T, apiKey string, req relayPaymentRelayerFeeRequest) (*relayPaymentRelayerFeeResponse, int) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment_relayer_fee", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result relayPaymentRelayerFeeResponse
	_ = json.Unmarshal(raw, &result)
	return &result, resp.StatusCode
}

// relayerFeeFlowParams configures one run of the deposit→proof→relay flow
// below, letting the negative-path sub-tests vary the fee amount and/or the
// fee note's recipient key independently of what the circuit itself proves.
type relayerFeeFlowParams struct {
	label          string
	relayerFeeAmt  *big.Int // StFee AND the actual output[2] value (must match — circuit-enforced)
	feeRecipientPk *big.Int // WtSpendPublicKeysOut[2] — who the fee note is addressed to
}

// runRelayerFeeFlow deposits a fresh note for Alice, builds a
// PaymentRelayerFeePublic proof (pay 30 to Bob, 10 change to Alice, a fee
// note per p), and submits it through the relayer's /relay/payment_relayer_fee.
// Returns the relayer's response/status plus the values needed for further
// on-chain / commitment assertions.
func runRelayerFeeFlow(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	vault, erc20 *bind.BoundContract,
	vaultAddr common.Address,
	aliceAuth, bobAuth *bind.TransactOpts,
	bobSpend rpcore.KeyPair,
	bobViewEncapsKey []byte,
	p relayerFeeFlowParams,
) (resp *relayPaymentRelayerFeeResponse, status int, cmtRelayer, feeSalt, tokenId, changeAmt, saltA *big.Int) {
	t.Helper()
	t.Logf("── %s ──", p.label)

	merkleDepth := 8
	tokenId = big.NewInt(0)
	paymentAmt := big.NewInt(int64(rfpPayAmt))
	changeAmt = big.NewInt(int64(rfpChangeAmt))
	// depositAmt must satisfy the circuit's conservation law exactly
	// (valueIn == valOut[0] + valOut[1] + valOut[2]) — computed from the
	// actual amounts used rather than hardcoded, so negative-path sub-tests
	// that vary relayerFeeAmt away from rfpRelayerAmt still produce a
	// provable witness (only the on-chain/off-chain enforcement should
	// reject them, not gnark's own conservation constraint).
	depositAmt := new(big.Int).Add(new(big.Int).Add(paymentAmt, changeAmt), p.relayerFeeAmt)
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("[%s] Alice NewSpendKeyPair: %v", p.label, err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("[%s] Alice NewViewKeyPair: %v", p.label, err)
	}

	// ── Deposit ──
	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("[%s] ERC20.mint: %v", p.label, err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("[%s] wait mint: %v", p.label, err)
	}
	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("[%s] ERC20.approve: %v", p.label, err)
	}
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil {
		t.Fatalf("[%s] wait approve: %v", p.label, err)
	}

	ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		t.Fatalf("[%s] Encapsulate (deposit): %v", p.label, err)
	}
	aliceSaltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("[%s] DerivePaymentSalt: %v", p.label, err)
	}
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("[%s] DerivePaymentKey: %v", p.label, err)
	}
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)
	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("[%s] Erc20CommitmentV2 (deposit): %v", p.label, err)
	}
	aliceDepositCtxtII, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("[%s] EncryptPayload: %v", p.label, err)
	}
	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, aliceDepositCtxtII)
	if err != nil {
		t.Fatalf("[%s] vault.depositV2: %v", p.label, err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("[%s] wait depositV2: %v", p.label, err)
	}

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("[%s] GenerateProof: %v", p.label, err)
	}

	nullifier, err := dvpcore.GetNullifier(aliceSpend.PrivateKey, aliceProof.Indices)
	if err != nil {
		t.Fatalf("[%s] GetNullifier: %v", p.label, err)
	}

	// Output 0: Bob's payment.
	ssBob, ctxtBob, err := rpcore.Encapsulate(bobViewEncapsKey)
	if err != nil {
		t.Fatalf("[%s] Encapsulate (Bob): %v", p.label, err)
	}
	saltBobRaw, err := rpcore.DerivePaymentSalt(ssBob)
	if err != nil {
		t.Fatalf("[%s] DerivePaymentSalt (Bob): %v", p.label, err)
	}
	encKeyBob, err := rpcore.DerivePaymentKey(ssBob)
	if err != nil {
		t.Fatalf("[%s] DerivePaymentKey (Bob): %v", p.label, err)
	}
	ctxtIIBob, err := rpcore.EncryptPayload(encKeyBob, tokenId, paymentAmt)
	if err != nil {
		t.Fatalf("[%s] EncryptPayload (Bob): %v", p.label, err)
	}
	saltBobField := rpcore.SaltBToField(saltBobRaw)
	cmtBob, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, saltBobField, paymentAmt, tokenId)
	if err != nil {
		t.Fatalf("[%s] Erc20CommitmentV2 (Bob): %v", p.label, err)
	}

	// Output 1: Alice's change.
	saltA, err = dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("[%s] RandomInField (change): %v", p.label, err)
	}
	cmtChange, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, saltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("[%s] Erc20CommitmentV2 (change): %v", p.label, err)
	}

	// Output 2: the relayer fee note — amount and recipient per p.
	feeSalt, err = dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("[%s] RandomInField (fee): %v", p.label, err)
	}
	cmtRelayer, err = rpcore.Erc20CommitmentV2(p.feeRecipientPk, feeSalt, p.relayerFeeAmt, tokenId)
	if err != nil {
		t.Fatalf("[%s] Erc20CommitmentV2 (fee): %v", p.label, err)
	}

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
		"stFee":                p.relayerFeeAmt.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [3]string{bobSpend.PublicKey.String(), aliceSpend.PublicKey.String(), p.feeRecipientPk.String()},
		"wtValuesOut":          [3]string{paymentAmt.String(), changeAmt.String(), p.relayerFeeAmt.String()},
		"wtSaltsOut":           [3]string{saltBobField.String(), saltA.String(), feeSalt.String()},
	}

	proofResp, err := postProof(gnarkURL+"/proof/paymentRelayerFeePublic", reqBody)
	if err != nil {
		t.Fatalf("[%s] postProof paymentRelayerFeePublic: %v", p.label, err)
	}
	sig := proofResp.PublicSignal
	if len(sig) != 9 {
		t.Fatalf("[%s] expected 9 public signals, got %d", p.label, len(sig))
	}

	var proofArr [8]string
	for i, v := range proofResp.Proof {
		proofArr[i] = v.String()
	}
	var sigArr [9]string
	for i, v := range sig {
		sigArr[i] = v.String()
	}

	relayReq := relayPaymentRelayerFeeRequest{
		VaultId:      "0",
		Proof:        proofArr,
		PublicSignal: sigArr,
		CipherText:   "0x" + hex.EncodeToString(ctxtBob),
		EncTxData:    "0x" + hex.EncodeToString(ctxtIIBob),
		FeeSalt:      feeSalt.String(),
		TokenId:      tokenId.String(),
	}

	resp, status = postRelayerFee(t, relayerAPIKey, relayReq)
	return resp, status, cmtRelayer, feeSalt, tokenId, changeAmt, saltA
}

func TestRetailErc20_PaymentRelayerFeeOnChain(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	info := fetchRelayerInfo(t)
	if info.FeeSpendPubKey == "" {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	relayerFeePubKey, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("invalid feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}
	t.Logf("relayer fee spend pubkey: %s", relayerFeePubKey)

	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI := loadOnchainABI(t, "RaylsERC20")
	dvpABI := loadOnchainABI(t, "EnygmaDvp")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	dvp := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)

	ownerAuth := hardhatAuth(t, client) // account[0] — also the deployer, holds DEFAULT_OWNER_ROLE
	aliceAuth := hardhatAuth(t, client) // Alice reuses account[0], matching tests 03/08's convention
	bobAuth := hardhatBobAuth(t, client)

	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	fixedFee := big.NewInt(int64(rfpRelayerAmt)) // 5, matching test 08's relayerFeeAmt

	// ── Setup: owner configures the fixed relayer fee on-chain ────────────────
	t.Logf("Setup — owner sets relayerFixedFeeAmount = %s", fixedFee)
	setFeeTx, err := dvp.Transact(ownerAuth, "setRelayerFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setRelayerFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setRelayerFixedFee: receipt=%+v err=%v", r, err)
	}

	// ── Happy path ──────────────────────────────────────────────────────────
	resp, status, cmtRelayer, feeSalt, tokenId, changeAmt, saltA := runRelayerFeeFlow(
		t, ctx, client, vault, erc20, vaultAddr, aliceAuth, bobAuth,
		rpcore.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey}, bobView.EncapsKey,
		relayerFeeFlowParams{
			label:          "happy path: correct fee, addressed to relayer",
			relayerFeeAmt:  fixedFee,
			feeRecipientPk: relayerFeePubKey,
		},
	)
	if status != http.StatusOK {
		t.Fatalf("happy path: relayer returned %d: %s", status, resp.Error)
	}
	t.Logf("  txHash=%s blockNumber=%d gasUsed=%d", resp.TxHash, resp.BlockNumber, resp.GasUsed)

	txHash := common.HexToHash(resp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionReceipt(%s): %v", resp.TxHash, err)
	}
	paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	var paymentEvents, nullifierEvents int
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			paymentEvents++
		case nullifierSig:
			nullifierEvents++
		}
	}
	if paymentEvents != 1 {
		t.Errorf("happy path: expected 1 Payment event, got %d", paymentEvents)
	}
	if nullifierEvents != 1 {
		t.Errorf("happy path: expected 1 Nullifier event, got %d", nullifierEvents)
	}

	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	senderAddr, err := types.Sender(signer, tx)
	if err != nil {
		t.Fatalf("types.Sender: %v", err)
	}
	aliceEthAddr := common.HexToAddress(hardhatAliceAddr)
	if senderAddr == aliceEthAddr {
		t.Error("happy path: tx.from == Alice — relayer did not sign the transaction")
	} else {
		t.Logf("  tx.from = %s (relayer, not Alice)", senderAddr.Hex())
	}

	recomputed, err := rpcore.Erc20CommitmentV2(relayerFeePubKey, feeSalt, fixedFee, tokenId)
	if err != nil {
		t.Fatalf("recompute relayer fee commitment: %v", err)
	}
	if recomputed.Cmp(cmtRelayer) != 0 {
		t.Errorf("happy path: recomputed fee commitment %s != %s", recomputed, cmtRelayer)
	}
	t.Logf("  relayer fee note (amount=%s) confirmed spendable ✓", fixedFee)
	_, _ = changeAmt, saltA // consumed only for logging symmetry with test 08's flow

	// ── Negative A: on-chain enforcement (InvalidRelayerFee) ──────────────────
	// Same flow, but the proof's StFee (and its correspondingly-consistent
	// output[2] value — the circuit itself requires WtValuesOut[2] == StFee)
	// no longer matches the contract's configured relayerFixedFeeAmount. The
	// note is still correctly addressed to the relayer, so the relayer's own
	// off-chain ownership check passes and the proof reaches chain, where
	// paymentWithRelayerFee() must now reject it.
	mismatchedFee := new(big.Int).Add(fixedFee, big.NewInt(1))
	respA, statusA, _, _, _, _, _ := runRelayerFeeFlow(
		t, ctx, client, vault, erc20, vaultAddr, aliceAuth, bobAuth,
		rpcore.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey}, bobView.EncapsKey,
		relayerFeeFlowParams{
			label:          "negative A: fee mismatches relayerFixedFeeAmount",
			relayerFeeAmt:  mismatchedFee,
			feeRecipientPk: relayerFeePubKey,
		},
	)
	if statusA == http.StatusOK {
		t.Fatalf("negative A: expected the relay to fail (mismatched fee should revert on-chain), got 200: %+v", respA)
	}
	t.Logf("  negative A: relayer responded %d: %s", statusA, respA.Error)
	// Hardhat Network simulates a transaction before accepting it into the
	// mempool, so a reverting on-chain call fails at dvp.Transact() itself
	// (500 from the relayer) rather than being mined with a failed receipt —
	// unlike a plain network's "submit first, fail on-chain later" behavior.
	// The important assertion is the *reason*: this must be the specific
	// InvalidRelayerFee revert this test exists to catch, not some other
	// unrelated failure.
	if statusA != http.StatusInternalServerError {
		t.Errorf("negative A: expected 500 (on-chain revert surfaced from Transact), got %d", statusA)
	}
	if !strings.Contains(respA.Error, "InvalidRelayerFee") {
		t.Errorf("negative A: expected the revert reason to be InvalidRelayerFee, got: %s", respA.Error)
	} else {
		t.Log("  negative A: confirmed the specific InvalidRelayerFee revert fired ✓")
	}

	// ── Negative B: off-chain enforcement (fee note not addressed to relayer) ─
	// Correct fee amount, but the fee note is addressed to an unrelated key —
	// the relayer's Poseidon ownership recomputation must reject this before
	// ever touching chain.
	strangerSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("stranger NewSpendKeyPair: %v", err)
	}
	respB, statusB, _, _, _, _, _ := runRelayerFeeFlow(
		t, ctx, client, vault, erc20, vaultAddr, aliceAuth, bobAuth,
		rpcore.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey}, bobView.EncapsKey,
		relayerFeeFlowParams{
			label:          "negative B: fee note addressed to a non-relayer key",
			relayerFeeAmt:  fixedFee,
			feeRecipientPk: strangerSpend.PublicKey,
		},
	)
	if statusB != http.StatusPaymentRequired {
		t.Fatalf("negative B: expected 402 (off-chain ownership check), got %d: %+v", statusB, respB)
	}
	t.Logf("  negative B: relayer responded 402 as expected: %s", respB.Error)

	t.Logf("=== RELAYER FEE ON-CHAIN FLOW COMPLETE ===")
	fmt.Println() // spacing before go test's own summary line
}
