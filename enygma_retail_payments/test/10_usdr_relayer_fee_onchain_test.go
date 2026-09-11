package tests

// TestRetailErc20_UsdrRelayerFeeOnChain exercises a genuinely separate
// second-asset relayer fee: a normal payment (main token, main vault) and a
// UsdrFeeCircuit proof (a distinct "USDr" token, its own vault, its own
// circuit — vaultId=1) settled ATOMICALLY in one call
// (EnygmaDvp.paymentWithUsdrFee), via the relayer's
// POST /relay/payment_usdr_fee.
//
// This is the two-independent-proofs design mirroring enygma_payments'
// USDrCircuit, as opposed to the same-token, single-proof
// PaymentRelayerFeePublic mechanism (test 09) already shipped for this repo.
//
// UsdrFeeCircuit is 1-in/2-out with a 9-element public statement:
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0] (relayer fee), StCommitmentsOut[1] (change),
//	 StContractAddress, StFee, StTokenId]
//
// StTokenId is PUBLIC here (private WtTokenId everywhere else in this
// codebase) — that's what gives this circuit's statement a distinct length
// (9) from every other 1-in/2-out circuit's (7 or 8), avoiding a VK-slot
// dispatch collision in Erc20CoinVault.checkReceiptConditions. See
// EnygmaDvp.sol's usdrFixedFeeAmount/usdrTokenId doc comments.
//
// Prerequisites — all three services running, deployed via setup.sh (which
// now deploys a second UsdrERC20/UsdrCoinVault pair and registers a 5th VK):
//
//	Terminal 1: cd ../enygma_dvp && npx hardhat node
//	Terminal 2: bash setup.sh
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: cd relayer && RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=<token> \
//	              RELAYER_FEE_SPEND_PRIVATE_KEY=<positive decimal> go run main.go
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestRetailErc20_UsdrRelayerFeeOnChain -v -timeout 300s

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
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
	usdrPayChangeAmt = 10 // sender's USDr change, arbitrary
)

// ── relay request/response (POST /relay/payment_usdr_fee) ─────────────────────

type relayUsdrFeeReq struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [7]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`

	UsdrVaultId      string    `json:"usdrVaultId"`
	UsdrProof        [8]string `json:"usdrProof"`
	UsdrPublicSignal [9]string `json:"usdrPublicSignal"`
	UsdrCipherText   string    `json:"usdrCipherText"`
	UsdrEncTxData    string    `json:"usdrEncTxData"`
	UsdrFeeSalt      string    `json:"usdrFeeSalt"`
}

type relayUsdrFeeResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── USDr leg: build a UsdrFeeCircuit witness (raw JSON — no bound-client
// helper exists for this circuit shape) ────────────────────────────────────

type usdrFeeWitness struct {
	proof        [8]string
	publicSignal [9]string
	ctxtFee      []byte // opaque note-discovery data for the fee note (relayer already knows its own note; threaded through for symmetry)
	ctxtChange   []byte
	feeSalt      *big.Int
	cmtFee       *big.Int
}

// depositAndBuildUsdrFeeWitness deposits a fresh USDr note for Alice and
// builds a UsdrFeeCircuit proof (fee note of relayerFeeAmt to
// feeRecipientPk, change of usdrPayChangeAmt back to Alice). depositAmt is
// computed from the actual amounts used so the circuit's own conservation
// law always holds, even when a sub-test deliberately picks a
// relayerFeeAmt/tokenId that mismatches the contract's configured values.
func depositAndBuildUsdrFeeWitness(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	usdrVault, usdrErc20 *bind.BoundContract,
	usdrVaultAddr common.Address,
	aliceAuth *bind.TransactOpts,
	relayerFeeAmt, feeRecipientPk, tokenId *big.Int,
) usdrFeeWitness {
	t.Helper()

	merkleDepth := 8
	changeAmt := big.NewInt(usdrPayChangeAmt)
	depositAmt := new(big.Int).Add(relayerFeeAmt, changeAmt)
	vaultAddrBig := new(big.Int).SetBytes(usdrVaultAddr.Bytes())

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	mintTx, err := usdrErc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("UsdrERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approveTx, err := usdrErc20.Transact(aliceAuth, "approve", usdrVaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("UsdrERC20.approve: %v", err)
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
	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}
	depositTx, err := usdrVault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("usdrVault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadVaultMerkleTree(t, client, usdrVaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}

	nullifier, err := dvpcore.GetNullifier(aliceSpend.PrivateKey, aliceProof.Indices)
	if err != nil {
		t.Fatalf("GetNullifier: %v", err)
	}

	// Output 0: relayer fee note.
	feeSalt, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (fee): %v", err)
	}
	cmtFee, err := rpcore.Erc20CommitmentV2(feeRecipientPk, feeSalt, relayerFeeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (fee): %v", err)
	}

	// Output 1: Alice's change.
	changeSalt, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (change): %v", err)
	}
	cmtChange, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, changeSalt, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change): %v", err)
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
		"stCommitmentsOut":     [2]string{cmtFee.String(), cmtChange.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                relayerFeeAmt.String(),
		"stTokenId":            tokenId.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtSpendPublicKeysOut": [2]string{feeRecipientPk.String(), aliceSpend.PublicKey.String()},
		"wtValuesOut":          [2]string{relayerFeeAmt.String(), changeAmt.String()},
		"wtSaltsOut":           [2]string{feeSalt.String(), changeSalt.String()},
	}

	proofResp, err := postProof(gnarkURL+"/proof/usdrFee", reqBody)
	if err != nil {
		t.Fatalf("postProof usdrFee: %v", err)
	}
	if len(proofResp.PublicSignal) != 9 {
		t.Fatalf("expected 9 public signals, got %d", len(proofResp.PublicSignal))
	}

	var proofArr [8]string
	for i, v := range proofResp.Proof {
		proofArr[i] = v.String()
	}
	var sigArr [9]string
	for i, v := range proofResp.PublicSignal {
		sigArr[i] = v.String()
	}

	return usdrFeeWitness{
		proof:        proofArr,
		publicSignal: sigArr,
		ctxtFee:      []byte{},
		ctxtChange:   []byte{},
		feeSalt:      feeSalt,
		cmtFee:       cmtFee,
	}
}

func TestRetailErc20_UsdrRelayerFeeOnChain(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	info := fetchRelayerInfoFor10(t)
	if info.FeeSpendPubKey == "" {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	relayerFeePubKey, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("invalid feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}

	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	usdrErc20Addr := common.HexToAddress(receipts["UsdrERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI := loadOnchainABI(t, "RaylsERC20")
	dvpABI := loadOnchainABI(t, "EnygmaDvp")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	usdrVault := bind.NewBoundContract(usdrVaultAddr, vaultABI, client, client, client)
	usdrErc20 := bind.NewBoundContract(usdrErc20Addr, erc20ABI, client, client, client)
	dvp := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)

	ownerAuth := hardhatAuth(t, client)
	aliceAuth := hardhatAuth(t, client)
	bobAuth := hardhatBobAuth(t, client)

	tokenId := big.NewInt(0) // fixed convention value — see EnygmaDvp.sol's usdrTokenId doc comment
	fixedFee := big.NewInt(7)

	t.Logf("Setup — owner sets usdrFixedFeeAmount = %s (usdrTokenId already 0 from init.go)", fixedFee)
	setFeeTx, err := dvp.Transact(ownerAuth, "setUsdrFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setUsdrFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setUsdrFixedFee: receipt=%+v err=%v", r, err)
	}

	// ── Happy path: main payment + USDr fee, settled atomically ──────────────
	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	mainDepositAmt := big.NewInt(40)
	mainPayAmt := big.NewInt(30)
	mainChangeAmt := big.NewInt(10)

	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}
	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(mainDepositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, mainDepositAmt)
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
	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, mainDepositAmt, big.NewInt(0))
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, big.NewInt(0), mainDepositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}
	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{mainDepositAmt, aliceSpend.PublicKey, aliceSaltBField, big.NewInt(0)}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (main): %v", err)
	}

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{mainDepositAmt},
		[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{mainPayAmt, mainChangeAmt},
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		big.NewInt(0),
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}

	usdrW := depositAndBuildUsdrFeeWitness(t, ctx, client, usdrVault, usdrErc20, usdrVaultAddr, aliceAuth,
		fixedFee, relayerFeePubKey, tokenId)

	var mainProofArr [8]string
	copy(mainProofArr[:], paymentResult.Proof)
	stmt := paymentResult.ContractStatement()
	var mainSigArr [7]string
	for i, v := range stmt {
		mainSigArr[i] = v.String()
	}

	relayReq := relayUsdrFeeReq{
		VaultId:          "0",
		Proof:            mainProofArr,
		PublicSignal:     mainSigArr,
		CipherText:       "0x" + hex.EncodeToString(paymentResult.CipherText),
		EncTxData:        "0x" + hex.EncodeToString(paymentResult.EncTxData),
		UsdrVaultId:      "1",
		UsdrProof:        usdrW.proof,
		UsdrPublicSignal: usdrW.publicSignal,
		UsdrCipherText:   "0x",
		UsdrEncTxData:    "0x",
		UsdrFeeSalt:      usdrW.feeSalt.String(),
	}

	var resp relayUsdrFeeResp
	status := postToRelayerUsdrFee(t, relayReq, &resp)
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

	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	senderAddr, err := types.Sender(signer, tx)
	if err != nil {
		t.Fatalf("types.Sender: %v", err)
	}
	if senderAddr == common.HexToAddress(hardhatAliceAddr) {
		t.Error("tx.from == Alice — relayer did not sign the transaction")
	} else {
		t.Logf("  tx.from = %s (relayer, not Alice) ✓", senderAddr.Hex())
	}

	recomputedFeeCmt, err := rpcore.Erc20CommitmentV2(relayerFeePubKey, usdrW.feeSalt, fixedFee, tokenId)
	if err != nil {
		t.Fatalf("recompute USDr fee commitment: %v", err)
	}
	if recomputedFeeCmt.Cmp(usdrW.cmtFee) != 0 {
		t.Errorf("USDr fee commitment mismatch: got %s, want %s", recomputedFeeCmt, usdrW.cmtFee)
	}
	t.Logf("  USDr fee note (amount=%s, separate token) confirmed spendable ✓ — atomic with the main payment", fixedFee)

	_ = bobAuth // present for symmetry with other tests; not used to sign anything here

	// ── Negative A: on-chain enforcement (fee mismatch) ───────────────────────
	t.Run("negative A: usdrFee mismatches usdrFixedFeeAmount", func(t *testing.T) {
		mismatchedFee := new(big.Int).Add(fixedFee, big.NewInt(1))
		testUsdrFeeRejected(t, ctx, client, vault, erc20, usdrVault, usdrErc20, vaultAddr, usdrVaultAddr,
			aliceAuth, bobSpend.PublicKey, bobView.EncapsKey, gnarkClient, merkleDepth,
			mismatchedFee, relayerFeePubKey, tokenId,
			http.StatusInternalServerError, "InvalidUsdrFee")
	})

	// ── Negative B: on-chain enforcement (tokenId mismatch) ───────────────────
	t.Run("negative B: usdrTokenId mismatches configured value", func(t *testing.T) {
		wrongTokenId := big.NewInt(1)
		testUsdrFeeRejected(t, ctx, client, vault, erc20, usdrVault, usdrErc20, vaultAddr, usdrVaultAddr,
			aliceAuth, bobSpend.PublicKey, bobView.EncapsKey, gnarkClient, merkleDepth,
			fixedFee, relayerFeePubKey, wrongTokenId,
			http.StatusInternalServerError, "InvalidUsdrFee")
	})

	// ── Negative C: off-chain enforcement (wrong recipient key) ───────────────
	t.Run("negative C: usdr fee note addressed to a non-relayer key", func(t *testing.T) {
		strangerSpend, err := rpcore.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("stranger NewSpendKeyPair: %v", err)
		}
		testUsdrFeeRejected(t, ctx, client, vault, erc20, usdrVault, usdrErc20, vaultAddr, usdrVaultAddr,
			aliceAuth, bobSpend.PublicKey, bobView.EncapsKey, gnarkClient, merkleDepth,
			fixedFee, strangerSpend.PublicKey, tokenId,
			http.StatusPaymentRequired, "")
	})

	t.Log("=== USDR RELAYER FEE ON-CHAIN FLOW COMPLETE ===")
}

// testUsdrFeeRejected builds a fresh main-payment leg + a USDr leg with the
// given (possibly invalid) fee/recipient/tokenId, submits through the
// relayer, and asserts the expected failure status (and, when wantErrSubstr
// is non-empty, that the error message contains it — used to confirm the
// exact revert reason rather than just "some failure").
func testUsdrFeeRejected(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	vault, erc20, usdrVault, usdrErc20 *bind.BoundContract,
	vaultAddr, usdrVaultAddr common.Address,
	aliceAuth *bind.TransactOpts,
	bobSpendPk *big.Int,
	bobViewEncapsKey []byte,
	gnarkClient *dvpcore.GnarkClient,
	merkleDepth int,
	usdrFeeAmt, usdrFeeRecipientPk, usdrTokenIdArg *big.Int,
	wantStatus int,
	wantErrSubstr string,
) {
	t.Helper()

	depositAmt := big.NewInt(40)
	payAmt := big.NewInt(30)
	changeAmt := big.NewInt(10)

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
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
	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, big.NewInt(0))
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, big.NewInt(0), depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}
	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, big.NewInt(0)}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (main): %v", err)
	}

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{depositAmt},
		[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{payAmt, changeAmt},
		[]*big.Int{bobSpendPk, aliceSpend.PublicKey},
		[][]byte{bobViewEncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		big.NewInt(0),
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}

	usdrW := depositAndBuildUsdrFeeWitness(t, ctx, client, usdrVault, usdrErc20, usdrVaultAddr, aliceAuth,
		usdrFeeAmt, usdrFeeRecipientPk, usdrTokenIdArg)

	var mainProofArr [8]string
	copy(mainProofArr[:], paymentResult.Proof)
	stmt := paymentResult.ContractStatement()
	var mainSigArr [7]string
	for i, v := range stmt {
		mainSigArr[i] = v.String()
	}

	relayReq := relayUsdrFeeReq{
		VaultId:          "0",
		Proof:            mainProofArr,
		PublicSignal:     mainSigArr,
		CipherText:       "0x" + hex.EncodeToString(paymentResult.CipherText),
		EncTxData:        "0x" + hex.EncodeToString(paymentResult.EncTxData),
		UsdrVaultId:      "1",
		UsdrProof:        usdrW.proof,
		UsdrPublicSignal: usdrW.publicSignal,
		UsdrCipherText:   "0x",
		UsdrEncTxData:    "0x",
		UsdrFeeSalt:      usdrW.feeSalt.String(),
	}

	var resp relayUsdrFeeResp
	status := postToRelayerUsdrFee(t, relayReq, &resp)
	if status != wantStatus {
		t.Fatalf("expected status %d, got %d: %+v", wantStatus, status, resp)
	}
	t.Logf("  relayer responded %d: %s", status, resp.Error)
	if wantErrSubstr != "" && !strings.Contains(resp.Error, wantErrSubstr) {
		t.Errorf("expected error to contain %q, got: %s", wantErrSubstr, resp.Error)
	} else if wantErrSubstr != "" {
		t.Logf("  confirmed the specific %s revert fired ✓", wantErrSubstr)
	}
}

// ── small helpers local to this file ───────────────────────────────────────────

type relayInfoRespFor10 struct {
	RelayerAddr    string `json:"relayerAddr"`
	FeeSpendPubKey string `json:"feeSpendPubKey"`
}

func fetchRelayerInfoFor10(t *testing.T) relayInfoRespFor10 {
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
	var out relayInfoRespFor10
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal /relay/info response: %v", err)
	}
	return out
}

func postToRelayerUsdrFee(t *testing.T, req relayUsdrFeeReq, out *relayUsdrFeeResp) int {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment_usdr_fee", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+relayerAPIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, out)
	return resp.StatusCode
}
