package tests

// TestV2Payment_RelayerFeeAndUsdrFee exercises the dedicated Payment
// deployment (see scripts/deploy_payment.go / scripts/init_payment.go) —
// ported from enygma_retail_payments — against this repo's own relayer,
// which gained two new routes for this pass:
//
//	POST /relay/payment_relayer_fee — same-token relayer fee
//	  (PaymentRelayerFeePublic circuit: 1-in/3-out, fee note is a 3rd output
//	  of the SAME proof, same token as the payment).
//	POST /relay/payment_usdr_fee — genuine second-asset USDr fee
//	  (a normal payment PLUS an independent UsdrFeeCircuit proof over a
//	  SEPARATE token/vault, settled atomically in one call to
//	  EnygmaDvp.paymentWithUsdrFee).
//
// Prerequisites:
//
//	Terminal 1: cd .. && npx hardhat node
//	Terminal 2 (once): CC=/usr/bin/clang go build -o /tmp/deploy_payment scripts/deploy_payment.go && /tmp/deploy_payment
//	Terminal 2 (once): cd gnark_circuits && go run generation.go
//	Terminal 2 (once): cd gnark_circuits && go run ./cmd/export_vk_payment ../build
//	Terminal 2 (once): CC=/usr/bin/clang go build -o /tmp/init_payment scripts/init_payment.go && /tmp/init_payment
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: cd relayer && \
//	              RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \ // gitleaks:allow (public Hardhat demo account, see enygmadvp.config.json)
//	              RELAYER_API_KEY=test-api-key-dev-only \
//	              RELAYER_DVP_ADDR=<from build/payment_receipts.json> \
//	              RELAYER_RECEIPTS_PATH=../build/payment_receipts.json \
//	              RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 \
//	              RELAYER_PORT=8092 \
//	              go run .
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestV2Payment_RelayerFeeAndUsdrFee -v -timeout 300s

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"

	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	paymentRelayerURL = "http://localhost:8092"
	paymentGnarkURL   = "http://localhost:8081"
	paymentAPIKey     = "test-api-key-dev-only"
)

// ── build/payment_receipts.json loader (separate from build/receipts.json) ────

func loadPaymentReceipts(t *testing.T) map[string]onchainReceiptEntry {
	t.Helper()
	data, err := os.ReadFile("../build/payment_receipts.json")
	if err != nil {
		t.Fatalf("read build/payment_receipts.json: %v — run deploy_payment+init_payment first", err)
	}
	var r map[string]onchainReceiptEntry
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parse payment_receipts.json: %v", err)
	}
	return r
}

// ── relayer request/response types (mirror relayer/server/types.go) ───────────

type paymentReceiptPayload struct {
	Proof           [8]string `json:"proof"`
	PublicSignal    []string  `json:"publicSignal"`
	NumberOfInputs  int       `json:"numberOfInputs"`
	NumberOfOutputs int       `json:"numberOfOutputs"`
}

type relayPaymentRelayerFeeReq struct {
	VaultId    string                `json:"vaultId"`
	Receipt    paymentReceiptPayload `json:"receipt"`
	CipherText string                `json:"cipherText"`
	EncTxData  string                `json:"encTxData"`
	FeeSalt    string                `json:"feeSalt"`
	TokenId    string                `json:"tokenId"`
}

type relayPaymentUsdrFeeReq struct {
	VaultId    string                `json:"vaultId"`
	Receipt    paymentReceiptPayload `json:"receipt"`
	CipherText string                `json:"cipherText"`
	EncTxData  string                `json:"encTxData"`

	UsdrVaultId    string                `json:"usdrVaultId"`
	UsdrReceipt    paymentReceiptPayload `json:"usdrReceipt"`
	UsdrCipherText string                `json:"usdrCipherText"`
	UsdrEncTxData  string                `json:"usdrEncTxData"`
	UsdrFeeSalt    string                `json:"usdrFeeSalt"`
}

type relayGenericResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

type relayInfoResp struct {
	RelayerAddr    string `json:"relayerAddr"`
	FeeSpendPubKey string `json:"feeSpendPubKey"`
}

func fetchPaymentRelayerInfo(t *testing.T) relayInfoResp {
	t.Helper()
	resp, err := http.Get(paymentRelayerURL + "/relay/info")
	if err != nil {
		t.Fatalf("GET /relay/info: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /relay/info response: %v", err)
	}
	var out relayInfoResp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal /relay/info response: %v", err)
	}
	return out
}

func postToPaymentRelayer(t *testing.T, path string, body interface{}) (*relayGenericResp, int) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, paymentRelayerURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+paymentAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out relayGenericResp
	_ = json.Unmarshal(raw, &out)
	return &out, resp.StatusCode
}

// ── gnark proof response (raw JSON — no bound-client wrapper exists for
// PaymentRelayerFeePublic/UsdrFee) ──────────────────────────────────────────

type paymentGnarkProofResp struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
	Error        string     `json:"error"`
}

func postPaymentGnarkProof(t *testing.T, path string, body interface{}) *paymentGnarkProofResp {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal gnark request: %v", err)
	}
	resp, err := http.Post(paymentGnarkURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read gnark response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gnark server %d: %s", resp.StatusCode, string(raw))
	}
	var out paymentGnarkProofResp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal gnark response: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("gnark error: %s", out.Error)
	}
	return &out
}

// ── shared deposit helper ──────────────────────────────────────────────────────

type paymentDeposit struct {
	spend       *core.SpendKeyPair
	view        *core.ViewKeyPair
	commitment  *big.Int
	saltBField  *big.Int
	depositAmt  *big.Int
	merkleProof *core.MerkleProof
}

func depositIntoVault(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	vault, erc20 *bind.BoundContract,
	vaultAddr common.Address,
	auth *bind.TransactOpts,
	depositAmt *big.Int,
	tokenId *big.Int,
	merkleDepth int,
) paymentDeposit {
	t.Helper()

	spend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	view, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}

	mintTx, err := erc20.Transact(auth, "mint", auth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approveTx, err := erc20.Transact(auth, "approve", vaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("ERC20.approve: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil {
		t.Fatalf("wait approve: %v", err)
	}

	ss, capsule, err := core.Encapsulate(view.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	saltB, err := core.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("DerivePaymentSalt: %v", err)
	}
	encKey, err := core.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	saltBField := core.SaltBToField(saltB)
	commitment, err := core.Erc20CommitmentV2(spend.PublicKey, saltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	depositCtxt, err := core.EncryptPayload(encKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}
	depositTx, err := vault.Transact(auth, "depositV2",
		[]*big.Int{depositAmt, spend.PublicKey, saltBField, tokenId}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	proof, err := mt.GenerateProof(commitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}

	return paymentDeposit{
		spend:       spend,
		view:        view,
		commitment:  commitment,
		saltBField:  saltBField,
		depositAmt:  depositAmt,
		merkleProof: proof,
	}
}

func TestV2Payment_RelayerFeeAndUsdrFee(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	if !serverAvailable("localhost:8092") {
		t.Skip("dedicated Payment relayer not running on localhost:8092 — skipping")
	}

	info := fetchPaymentRelayerInfo(t)
	if info.FeeSpendPubKey == "" {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	relayerFeePubKey, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("invalid feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}
	t.Logf("relayer: %s  feeSpendPubKey: %s", info.RelayerAddr, relayerFeePubKey)

	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadPaymentReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	usdrErc20Addr := common.HexToAddress(receipts["UsdrERC20"].ContractAddress)

	dvpABI := loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	vaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	erc20ABI := loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")

	dvp := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)
	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	usdrVault := bind.NewBoundContract(usdrVaultAddr, vaultABI, client, client, client)
	usdrErc20 := bind.NewBoundContract(usdrErc20Addr, erc20ABI, client, client, client)

	owner := hardhatAuth(t, client) // account[0] — deployer, holds DEFAULT_OWNER_ROLE

	merkleDepth := 8
	tokenId := big.NewInt(0)
	gnarkClient := core.NewGnarkClient(paymentGnarkURL)

	relayerFixedFee := big.NewInt(5)
	usdrFixedFee := big.NewInt(7)

	t.Logf("Setup — owner sets relayerFixedFeeAmount=%s, usdrFixedFeeAmount=%s", relayerFixedFee, usdrFixedFee)
	if tx, err := dvp.Transact(owner, "setRelayerFixedFee", relayerFixedFee); err != nil {
		t.Fatalf("setRelayerFixedFee: %v", err)
	} else if r, err := bind.WaitMined(ctx, client, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setRelayerFixedFee: receipt=%+v err=%v", r, err)
	}
	if tx, err := dvp.Transact(owner, "setUsdrFixedFee", usdrFixedFee); err != nil {
		t.Fatalf("setUsdrFixedFee: %v", err)
	} else if r, err := bind.WaitMined(ctx, client, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setUsdrFixedFee: receipt=%+v err=%v", r, err)
	}

	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	// ═══════════════════════════════════════════════════════════════════════
	// Test A — same-token relayer fee (PaymentRelayerFeePublic, 1-in/3-out)
	// ═══════════════════════════════════════════════════════════════════════
	t.Run("relayer fee (same token) via /relay/payment_relayer_fee", func(t *testing.T) {
		payAmt := big.NewInt(30)
		changeAmt := big.NewInt(10)
		depositAmt := new(big.Int).Add(new(big.Int).Add(payAmt, changeAmt), relayerFixedFee)

		d := depositIntoVault(t, ctx, client, vault, erc20, vaultAddr, owner, depositAmt, tokenId, merkleDepth)

		// GetNullifierBoundTree (not GetNullifier) — binds the proof to this
		// vault AND tree, matching the circuit-side NullifierBoundTree fix
		// (StContractAddress and StTreeNumbers were both previously
		// unconstrained).
		nullifier, err := core.GetNullifierBoundTree(d.spend.PrivateKey, big.NewInt(int64(d.merkleProof.TreeNumber)), d.merkleProof.Indices, merkleDepth, new(big.Int).SetBytes(vaultAddr.Bytes()))
		if err != nil {
			t.Fatalf("GetNullifierBound: %v", err)
		}

		ssBob, ctxtBob, err := core.Encapsulate(bobView.EncapsKey)
		if err != nil {
			t.Fatalf("Encapsulate (Bob): %v", err)
		}
		saltBobRaw, err := core.DerivePaymentSalt(ssBob)
		if err != nil {
			t.Fatalf("DerivePaymentSalt (Bob): %v", err)
		}
		encKeyBob, err := core.DerivePaymentKey(ssBob)
		if err != nil {
			t.Fatalf("DerivePaymentKey (Bob): %v", err)
		}
		ctxtIIBob, err := core.EncryptPayload(encKeyBob, tokenId, payAmt)
		if err != nil {
			t.Fatalf("EncryptPayload (Bob): %v", err)
		}
		saltBobField := core.SaltBToField(saltBobRaw)
		cmtBob, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBobField, payAmt, tokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (Bob): %v", err)
		}

		saltChange, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField (change): %v", err)
		}
		cmtChange, err := core.Erc20CommitmentV2(d.spend.PublicKey, saltChange, changeAmt, tokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (change): %v", err)
		}

		feeSalt, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField (fee): %v", err)
		}
		cmtRelayer, err := core.Erc20CommitmentV2(relayerFeePubKey, feeSalt, relayerFixedFee, tokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (fee): %v", err)
		}

		var pathElements [8]string
		for j, e := range d.merkleProof.Elements {
			if j >= 8 {
				break
			}
			pathElements[j] = e.String()
		}
		vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

		reqBody := map[string]interface{}{
			"stMessage":            "0",
			"stTreeNumbers":        [1]string{"0"},
			"stMerkleRoots":        [1]string{d.merkleProof.Root.String()},
			"stNullifiers":         [1]string{nullifier.String()},
			"stCommitmentsOut":     [3]string{cmtBob.String(), cmtChange.String(), cmtRelayer.String()},
			"stContractAddress":    vaultAddrBig.String(),
			"stFee":                relayerFixedFee.String(),
			"wtPrivateKeysIn":      [1]string{d.spend.PrivateKey.String()},
			"wtValuesIn":           [1]string{d.depositAmt.String()},
			"wtSaltsIn":            [1]string{d.saltBField.String()},
			"wtPathElements":       [1][8]string{pathElements},
			"wtPathIndices":        [1]string{d.merkleProof.Indices.String()},
			"wtTokenId":            tokenId.String(),
			"wtSpendPublicKeysOut": [3]string{bobSpend.PublicKey.String(), d.spend.PublicKey.String(), relayerFeePubKey.String()},
			"wtValuesOut":          [3]string{payAmt.String(), changeAmt.String(), relayerFixedFee.String()},
			"wtSaltsOut":           [3]string{saltBobField.String(), saltChange.String(), feeSalt.String()},
		}
		proof := postPaymentGnarkProof(t, "/proof/paymentRelayerFeePublic", reqBody)
		if len(proof.PublicSignal) != 9 {
			t.Fatalf("expected 9 public signals, got %d", len(proof.PublicSignal))
		}

		var proofArr [8]string
		for i, v := range proof.Proof {
			proofArr[i] = v.String()
		}
		sigStrs := make([]string, len(proof.PublicSignal))
		for i, v := range proof.PublicSignal {
			sigStrs[i] = v.String()
		}

		resp, status := postToPaymentRelayer(t, "/relay/payment_relayer_fee", relayPaymentRelayerFeeReq{
			VaultId: "0",
			Receipt: paymentReceiptPayload{
				Proof: proofArr, PublicSignal: sigStrs, NumberOfInputs: 1, NumberOfOutputs: 3,
			},
			CipherText: "0x" + hex.EncodeToString(ctxtBob),
			EncTxData:  "0x" + hex.EncodeToString(ctxtIIBob),
			FeeSalt:    feeSalt.String(),
			TokenId:    tokenId.String(),
		})
		if status != http.StatusOK {
			t.Fatalf("POST /relay/payment_relayer_fee: status=%d error=%s", status, resp.Error)
		}
		t.Logf("  paymentWithRelayerFee mined: block=%d gas=%d tx=%s", resp.BlockNumber, resp.GasUsed, resp.TxHash)

		sender := txSenderFor12(t, client, resp.TxHash)
		if sender.Hex() != info.RelayerAddr {
			t.Errorf("tx.from = %s, want relayer %s", sender.Hex(), info.RelayerAddr)
		} else {
			t.Logf("  tx.from = %s (relayer) ✓", sender.Hex())
		}

		recomputed, err := core.Erc20CommitmentV2(relayerFeePubKey, feeSalt, relayerFixedFee, tokenId)
		if err != nil {
			t.Fatalf("recompute fee commitment: %v", err)
		}
		if recomputed.Cmp(cmtRelayer) != 0 {
			t.Errorf("fee commitment mismatch: got %s, want %s", recomputed, cmtRelayer)
		}
		t.Logf("  relayer fee note (amount=%s) confirmed spendable ✓", relayerFixedFee)
	})

	// ═══════════════════════════════════════════════════════════════════════
	// Test B — genuine second-asset USDr fee, atomic dual-proof settlement
	// ═══════════════════════════════════════════════════════════════════════
	buildUsdrWitness := func(t *testing.T, feeAmt, feeRecipientPk, usdrTokenId *big.Int) (proof *paymentGnarkProofResp, feeSalt, cmtFee *big.Int) {
		t.Helper()
		changeAmt := big.NewInt(10)
		depositAmt := new(big.Int).Add(feeAmt, changeAmt)

		d := depositIntoVault(t, ctx, client, usdrVault, usdrErc20, usdrVaultAddr, owner, depositAmt, usdrTokenId, merkleDepth)

		// GetNullifierBoundTree (not GetNullifier) — binds the proof to this
		// vault AND tree, matching the circuit-side NullifierBoundTree fix.
		nullifier, err := core.GetNullifierBoundTree(d.spend.PrivateKey, big.NewInt(int64(d.merkleProof.TreeNumber)), d.merkleProof.Indices, merkleDepth, new(big.Int).SetBytes(usdrVaultAddr.Bytes()))
		if err != nil {
			t.Fatalf("GetNullifierBound (usdr): %v", err)
		}

		fSalt, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField (usdr fee): %v", err)
		}
		fCmt, err := core.Erc20CommitmentV2(feeRecipientPk, fSalt, feeAmt, usdrTokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (usdr fee): %v", err)
		}

		changeSalt, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField (usdr change): %v", err)
		}
		cmtChange, err := core.Erc20CommitmentV2(d.spend.PublicKey, changeSalt, changeAmt, usdrTokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (usdr change): %v", err)
		}

		var pathElements [8]string
		for j, e := range d.merkleProof.Elements {
			if j >= 8 {
				break
			}
			pathElements[j] = e.String()
		}
		usdrVaultAddrBig := new(big.Int).SetBytes(usdrVaultAddr.Bytes())

		reqBody := map[string]interface{}{
			"stMessage":            "0",
			"stTreeNumbers":        [1]string{"0"},
			"stMerkleRoots":        [1]string{d.merkleProof.Root.String()},
			"stNullifiers":         [1]string{nullifier.String()},
			"stCommitmentsOut":     [2]string{fCmt.String(), cmtChange.String()},
			"stContractAddress":    usdrVaultAddrBig.String(),
			"stFee":                feeAmt.String(),
			"stTokenId":            usdrTokenId.String(),
			"wtPrivateKeysIn":      [1]string{d.spend.PrivateKey.String()},
			"wtValuesIn":           [1]string{d.depositAmt.String()},
			"wtSaltsIn":            [1]string{d.saltBField.String()},
			"wtPathElements":       [1][8]string{pathElements},
			"wtPathIndices":        [1]string{d.merkleProof.Indices.String()},
			"wtSpendPublicKeysOut": [2]string{feeRecipientPk.String(), d.spend.PublicKey.String()},
			"wtValuesOut":          [2]string{feeAmt.String(), changeAmt.String()},
			"wtSaltsOut":           [2]string{fSalt.String(), changeSalt.String()},
		}
		p := postPaymentGnarkProof(t, "/proof/usdrFee", reqBody)
		if len(p.PublicSignal) != 9 {
			t.Fatalf("expected 9 public signals, got %d", len(p.PublicSignal))
		}
		return p, fSalt, fCmt
	}

	buildMainLeg := func(t *testing.T) (*core.PaymentResult, [8]string, []string) {
		t.Helper()
		payAmt := big.NewInt(30)
		changeAmt := big.NewInt(10)
		depositAmt := new(big.Int).Add(payAmt, changeAmt)

		d := depositIntoVault(t, ctx, client, vault, erc20, vaultAddr, owner, depositAmt, tokenId, merkleDepth)

		vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
		result, err := gnarkClient.BoundPaymentProof(
			vaultAddrBig,
			big.NewInt(0),
			[]*big.Int{d.depositAmt},
			[]core.KeyPair{{PrivateKey: d.spend.PrivateKey, PublicKey: d.spend.PublicKey}},
			[]*big.Int{d.saltBField},
			[]*big.Int{payAmt, changeAmt},
			[]*big.Int{bobSpend.PublicKey, d.spend.PublicKey},
			[][]byte{bobView.EncapsKey, d.view.EncapsKey},
			merkleDepth,
			[]*core.MerkleProof{d.merkleProof},
			[]*big.Int{big.NewInt(0)},
			tokenId,
		)
		if err != nil {
			t.Fatalf("BoundPaymentProof: %v", err)
		}
		var proofArr [8]string
		copy(proofArr[:], result.Proof)
		stmt := result.ContractStatement()
		sig := make([]string, len(stmt))
		for i, v := range stmt {
			sig[i] = v.String()
		}
		return result, proofArr, sig
	}

	t.Run("USDr fee (second asset) via /relay/payment_usdr_fee", func(t *testing.T) {
		mainResult, mainProofArr, mainSig := buildMainLeg(t)
		usdrProof, usdrFeeSalt, cmtFee := buildUsdrWitness(t, usdrFixedFee, relayerFeePubKey, tokenId)

		var usdrProofArr [8]string
		for i, v := range usdrProof.Proof {
			usdrProofArr[i] = v.String()
		}
		usdrSig := make([]string, len(usdrProof.PublicSignal))
		for i, v := range usdrProof.PublicSignal {
			usdrSig[i] = v.String()
		}

		resp, status := postToPaymentRelayer(t, "/relay/payment_usdr_fee", relayPaymentUsdrFeeReq{
			VaultId: "0",
			Receipt: paymentReceiptPayload{
				Proof: mainProofArr, PublicSignal: mainSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			CipherText:  "0x" + hex.EncodeToString(mainResult.CipherText),
			EncTxData:   "0x" + hex.EncodeToString(mainResult.EncTxData),
			UsdrVaultId: "1",
			UsdrReceipt: paymentReceiptPayload{
				Proof: usdrProofArr, PublicSignal: usdrSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			UsdrCipherText: "0x",
			UsdrEncTxData:  "0x",
			UsdrFeeSalt:    usdrFeeSalt.String(),
		})
		if status != http.StatusOK {
			t.Fatalf("POST /relay/payment_usdr_fee: status=%d error=%s", status, resp.Error)
		}
		t.Logf("  paymentWithUsdrFee mined: block=%d gas=%d tx=%s", resp.BlockNumber, resp.GasUsed, resp.TxHash)

		txHash := common.HexToHash(resp.TxHash)
		txReceipt, err := client.TransactionReceipt(ctx, txHash)
		if err != nil {
			t.Fatalf("TransactionReceipt: %v", err)
		}
		paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
		nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
		var payEvents, nfEvents int
		for _, log := range txReceipt.Logs {
			switch log.Topics[0] {
			case paymentSig:
				payEvents++
			case nullifierSig:
				nfEvents++
			}
		}
		if payEvents != 2 {
			t.Errorf("expected 2 Payment events (main + USDr), got %d", payEvents)
		}
		if nfEvents != 2 {
			t.Errorf("expected 2 Nullifier events (main + USDr), got %d", nfEvents)
		}

		sender := txSenderFor12(t, client, resp.TxHash)
		if sender.Hex() != info.RelayerAddr {
			t.Errorf("tx.from = %s, want relayer %s", sender.Hex(), info.RelayerAddr)
		} else {
			t.Logf("  tx.from = %s (relayer) ✓", sender.Hex())
		}

		recomputed, err := core.Erc20CommitmentV2(relayerFeePubKey, usdrFeeSalt, usdrFixedFee, tokenId)
		if err != nil {
			t.Fatalf("recompute USDr fee commitment: %v", err)
		}
		if recomputed.Cmp(cmtFee) != 0 {
			t.Errorf("USDr fee commitment mismatch: got %s, want %s", recomputed, cmtFee)
		}
		t.Logf("  USDr fee note (amount=%s, separate token) confirmed spendable ✓ — atomic with the main payment", usdrFixedFee)
	})

	t.Run("negative: usdrFee mismatches usdrFixedFeeAmount", func(t *testing.T) {
		mainResult, mainProofArr, mainSig := buildMainLeg(t)
		mismatched := new(big.Int).Add(usdrFixedFee, big.NewInt(1))
		usdrProof, usdrFeeSalt, _ := buildUsdrWitness(t, mismatched, relayerFeePubKey, tokenId)

		var usdrProofArr [8]string
		for i, v := range usdrProof.Proof {
			usdrProofArr[i] = v.String()
		}
		usdrSig := make([]string, len(usdrProof.PublicSignal))
		for i, v := range usdrProof.PublicSignal {
			usdrSig[i] = v.String()
		}

		resp, status := postToPaymentRelayer(t, "/relay/payment_usdr_fee", relayPaymentUsdrFeeReq{
			VaultId: "0",
			Receipt: paymentReceiptPayload{
				Proof: mainProofArr, PublicSignal: mainSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			CipherText:  "0x" + hex.EncodeToString(mainResult.CipherText),
			EncTxData:   "0x" + hex.EncodeToString(mainResult.EncTxData),
			UsdrVaultId: "1",
			UsdrReceipt: paymentReceiptPayload{
				Proof: usdrProofArr, PublicSignal: usdrSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			UsdrCipherText: "0x",
			UsdrEncTxData:  "0x",
			UsdrFeeSalt:    usdrFeeSalt.String(),
		})
		if status != http.StatusInternalServerError {
			t.Fatalf("expected 500 (on-chain revert surfaced), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded %d: %s", status, resp.Error)
		if !strings.Contains(resp.Error, "InvalidUsdrFee") {
			t.Errorf("expected error to contain InvalidUsdrFee, got: %s", resp.Error)
		} else {
			t.Log("  confirmed the specific InvalidUsdrFee revert fired ✓")
		}
	})

	t.Run("negative: usdr fee note addressed to a non-relayer key", func(t *testing.T) {
		mainResult, mainProofArr, mainSig := buildMainLeg(t)
		stranger, err := core.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("stranger NewSpendKeyPair: %v", err)
		}
		usdrProof, usdrFeeSalt, _ := buildUsdrWitness(t, usdrFixedFee, stranger.PublicKey, tokenId)

		var usdrProofArr [8]string
		for i, v := range usdrProof.Proof {
			usdrProofArr[i] = v.String()
		}
		usdrSig := make([]string, len(usdrProof.PublicSignal))
		for i, v := range usdrProof.PublicSignal {
			usdrSig[i] = v.String()
		}

		resp, status := postToPaymentRelayer(t, "/relay/payment_usdr_fee", relayPaymentUsdrFeeReq{
			VaultId: "0",
			Receipt: paymentReceiptPayload{
				Proof: mainProofArr, PublicSignal: mainSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			CipherText:  "0x" + hex.EncodeToString(mainResult.CipherText),
			EncTxData:   "0x" + hex.EncodeToString(mainResult.EncTxData),
			UsdrVaultId: "1",
			UsdrReceipt: paymentReceiptPayload{
				Proof: usdrProofArr, PublicSignal: usdrSig, NumberOfInputs: 1, NumberOfOutputs: 2,
			},
			UsdrCipherText: "0x",
			UsdrEncTxData:  "0x",
			UsdrFeeSalt:    usdrFeeSalt.String(),
		})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402 (off-chain ownership check), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded 402 as expected: %s", resp.Error)
	})

	t.Log("=== DEDICATED PAYMENT DEPLOYMENT — RELAYER FEE + USDR FEE COMPLETE ===")
	fmt.Println()
}

func txSenderFor12(t *testing.T, client *ethclient.Client, txHashHex string) common.Address {
	t.Helper()
	tx, _, err := client.TransactionByHash(context.Background(), common.HexToHash(txHashHex))
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	sender, err := types.Sender(signer, tx)
	if err != nil {
		t.Fatalf("types.Sender: %v", err)
	}
	return sender
}
