package tests

// End-to-end tests for the DVP relayer (localhost:8091).
//
// Two flows are covered:
//
//   TestDvP_SwapViaRelayer     — Alice swaps 30 ERC-20 for Bob's ERC-721 (POST /relay/swap)
//   TestDvP_ExchangeViaRelayer — Alice and Bob exchange ERC-20 notes (POST /relay/exchange)
//
// NOTE: /relay/payment is not tested here because the DVP gnark server only
// exposes /proof/dvpInitiator and /proof/dvpDestination — it has no
// joinSplitERC20 circuit, so there is no way to generate a payment proof
// from within this test suite.
//
// Exchange protocol note: exchange() requires both receipts to pass the
// ERC-20 vault's checkReceiptConditions which accesses statement[5], so both
// parties must use DvPInitiatorProofFromSalts (7-element statement). Salts are
// pre-agreed so the output commitments cross-reference each other without KEM.
//
// Plus auth/validation error tests that only need the relayer to be running.
//
// Prerequisites — all four services must be running before executing any test:
//
//	Terminal 1: cd enygma_dvp && npx hardhat node
//	Terminal 2: /tmp/deploy_contracts && /tmp/init_contracts  (from enygma_dvp/)
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: RELAYER_PRIVATE_KEY=<key> RELAYER_API_KEY=test-api-key-dev-only \
//	            RELAYER_DVP_ADDR=<EnygmaDvp addr> CC=/usr/bin/clang go run main.go \
//	            (from enygma_dvp/relayer/)
//
// Run all relayer tests:
//
//	cd test && CC=/usr/bin/clang go test -run "TestDvP.*Relayer|TestDvPRelayer" -v -timeout 600s

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── constants ─────────────────────────────────────────────────────────────────

const (
	dvpRelayerURL    = "http://localhost:8091"
	dvpRelayerAPIKey = "test-api-key-dev-only"

	// Vault IDs as registered by init.go: ERC-20 first (0), ERC-721 second (1).
	vaultIdErc20  = "0"
	vaultIdErc721 = "1"
)

// ── shared request/response types mirroring the relayer API ──────────────────

type dvpReceiptPayload struct {
	Proof           [8]string `json:"proof"`
	PublicSignal    []string  `json:"publicSignal"`
	NumberOfInputs  int       `json:"numberOfInputs"`
	NumberOfOutputs int       `json:"numberOfOutputs"`
}

// relayPaymentReq mirrors server.RelayPaymentRequest.
type relayPaymentReq struct {
	VaultId    string            `json:"vaultId"`
	Receipt    dvpReceiptPayload `json:"receipt"`
	CipherText string            `json:"cipherText"` // 0x-prefixed hex: ML-KEM capsule for Bob
	EncTxData  string            `json:"encTxData"`  // 0x-prefixed hex: AES-GCM payload for Bob
}

// relaySwapReq mirrors server.RelaySwapRequest.
type relaySwapReq struct {
	PaymentReceipt  dvpReceiptPayload `json:"paymentReceipt"`
	DeliveryReceipt dvpReceiptPayload `json:"deliveryReceipt"`
	PaymentVaultId  string            `json:"paymentVaultId"`
	DeliveryVaultId string            `json:"deliveryVaultId"`
}

// relayExchangeReq mirrors server.RelayExchangeRequest.
type relayExchangeReq struct {
	Receipt1 dvpReceiptPayload `json:"receipt1"`
	Receipt2 dvpReceiptPayload `json:"receipt2"`
	VaultId1 string            `json:"vaultId1"`
	VaultId2 string            `json:"vaultId2"`
}

type dvpRelayResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func dvpPost(t *testing.T, path, apiKey string, body interface{}) (*dvpRelayResp, int, error) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, dvpRelayerURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(resp.Body)

	var result dvpRelayResp
	_ = json.Unmarshal(respRaw, &result)
	return &result, resp.StatusCode, nil
}

// ── proof payload builders ────────────────────────────────────────────────────

// paymentResultToPayload converts a PaymentResult to a dvpReceiptPayload for /relay/payment.
// ContractStatement() de-interleaves the statement for on-chain submission.
func paymentResultToPayload(t *testing.T, r *core.PaymentResult) dvpReceiptPayload {
	t.Helper()
	stmt := r.ContractStatement()
	sig := make([]string, len(stmt))
	for i, v := range stmt {
		sig[i] = v.String()
	}
	if len(r.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(r.Proof))
	}
	var proof [8]string
	copy(proof[:], r.Proof)
	return dvpReceiptPayload{
		Proof:           proof,
		PublicSignal:    sig,
		NumberOfInputs:  r.NumberOfInputs,
		NumberOfOutputs: r.NumberOfOutputs,
	}
}

// proofResultToPayload converts a ProofResult to a dvpReceiptPayload.
// Uses ContractStatement() to de-interleave.
func proofResultToPayload(t *testing.T, r *core.ProofResult) dvpReceiptPayload {
	t.Helper()
	stmt := r.ContractStatement()
	sig := make([]string, len(stmt))
	for i, v := range stmt {
		sig[i] = v.String()
	}
	if len(r.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(r.Proof))
	}
	var proof [8]string
	copy(proof[:], r.Proof)
	return dvpReceiptPayload{
		Proof:           proof,
		PublicSignal:    sig,
		NumberOfInputs:  r.NumberOfInputs,
		NumberOfOutputs: r.NumberOfOutputs,
	}
}

// dvpInitiatorToPayload converts a DvPInitiatorResult to a dvpReceiptPayload.
// Statement is already in non-interleaved form (nIn=1, so interleaved == non-interleaved).
func dvpInitiatorToPayload(t *testing.T, r *core.DvPInitiatorResult) dvpReceiptPayload {
	t.Helper()
	sig := make([]string, len(r.Statement))
	for i, v := range r.Statement {
		sig[i] = v.String()
	}
	if len(r.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(r.Proof))
	}
	var proof [8]string
	copy(proof[:], r.Proof)
	return dvpReceiptPayload{
		Proof:           proof,
		PublicSignal:    sig,
		NumberOfInputs:  r.NumberOfInputs,
		NumberOfOutputs: r.NumberOfOutputs,
	}
}

// dvpDestinationToPayload converts a DvPDestinationResult to a dvpReceiptPayload.
func dvpDestinationToPayload(t *testing.T, r *core.DvPDestinationResult) dvpReceiptPayload {
	t.Helper()
	sig := make([]string, len(r.Statement))
	for i, v := range r.Statement {
		sig[i] = v.String()
	}
	if len(r.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(r.Proof))
	}
	var proof [8]string
	copy(proof[:], r.Proof)
	return dvpReceiptPayload{
		Proof:           proof,
		PublicSignal:    sig,
		NumberOfInputs:  r.NumberOfInputs,
		NumberOfOutputs: r.NumberOfOutputs,
	}
}

// hexBytes encodes bytes with 0x prefix.
func hexBytes(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}



// ══════════════════════════════════════════════════════════════════════════════
// TestDvP_SwapViaRelayer
//
// Alice swaps 30 ERC-20 tokens for Bob's ERC-721 ticket. Both
// sides generate their ZK proofs independently, then the two receipts are
// posted atomically to POST /relay/swap.
//
// Proof layout:
//   Payment receipt (Alice's initiator):  nIn=1, nOut=1, statement has 7 elements
//     [commitA, treeNum, root, nf, commitB, commitA, revertCommitA]
//   Delivery receipt (Bob's destination): nIn=1, nOut=1, statement has 5 elements
//     [commitB, treeNum, root, nf, commitA]
//
// The extra commitA/revertCommitA in Alice's statement are required by the VK
// but not counted as on-chain outputs (NumberOfOutputs=1 so only commitB is inserted).
//
// Verification:
//   - commitB (Bob's USDT) and commitA (Alice's ticket) both appear in Commitment events.
//   - Two Nullifier events (one per side).
// ══════════════════════════════════════════════════════════════════════════════

func TestDvP_SwapViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}

	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil { t.Fatalf("ethclient.Dial: %v", err) }
	defer client.Close()

	receipts      := loadOnchainReceipts(t)
	erc20VaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr      := common.HexToAddress(receipts["ERC20"].ContractAddress)
	nftVaultAddr   := common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	erc721Addr     := common.HexToAddress(receipts["ERC721"].ContractAddress)

	erc20VaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	erc20ABI      := loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	nftVaultABI   := loadOnchainABI(t, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json")
	erc721ABI     := loadOnchainABI(t, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")

	erc20Vault := bind.NewBoundContract(erc20VaultAddr, erc20VaultABI, client, client, client)
	erc20      := bind.NewBoundContract(erc20Addr,      erc20ABI,      client, client, client)
	nftVault   := bind.NewBoundContract(nftVaultAddr,   nftVaultABI,   client, client, client)
	erc721     := bind.NewBoundContract(erc721Addr,     erc721ABI,     client, client, client)

	auth := hardhatAuth(t, client)

	gnarkClient := core.NewGnarkClient("http://localhost:8081")
	merkleDepth  := 8
	erc20Amount  := big.NewInt(30)
	erc20TokenId := big.NewInt(0)
	// Use a unique tokenId based on current time to avoid 'already minted' on repeated runs.
	nftTokenId   := big.NewInt(time.Now().UnixNano() % (1 << 32))
	nftAmount    := big.NewInt(1)

	// ── Step 1: Alice deposits 30 ERC-20 ─────────────────────────────────────
	t.Log("Step 1 — Alice deposits 30 ERC-20 tokens")

	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Alice spend key: %v", err) }
	aliceView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Alice view key: %v", err) }

	ssAlice, capsuleAlice, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil { t.Fatalf("Encapsulate (Alice): %v", err) }
	aliceSaltBytes, err := core.DerivePaymentSalt(ssAlice)
	if err != nil { t.Fatalf("DerivePaymentSalt (Alice): %v", err) }
	aliceEncKey, err := core.DerivePaymentKey(ssAlice)
	if err != nil { t.Fatalf("DerivePaymentKey (Alice): %v", err) }
	aliceSaltField := core.SaltBToField(aliceSaltBytes)

	aliceCmt, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltField, erc20Amount, erc20TokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Alice): %v", err) }
	aliceDepositEnc, err := core.EncryptPayload(aliceEncKey, erc20TokenId, erc20Amount)
	if err != nil { t.Fatalf("EncryptPayload (Alice deposit): %v", err) }

	mintErc20Tx, err := erc20.Transact(auth, "mint", auth.From, new(big.Int).Mul(erc20Amount, big.NewInt(10)))
	if err != nil { t.Fatalf("ERC20.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, client, mintErc20Tx); err != nil { t.Fatalf("wait ERC20 mint: %v", err) }
	approveErc20Tx, err := erc20.Transact(auth, "approve", erc20VaultAddr, erc20Amount)
	if err != nil { t.Fatalf("ERC20.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, client, approveErc20Tx); err != nil { t.Fatalf("wait approve: %v", err) }
	// C-1 fix: contract computes commitment on-chain; send key material instead.
	depositErc20Tx, err := erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{erc20Amount, aliceSpend.PublicKey, aliceSaltField, erc20TokenId}, capsuleAlice, aliceDepositEnc)
	if err != nil { t.Fatalf("erc20Vault.depositV2: %v", err) }
	if _, err := bind.WaitMined(ctx, client, depositErc20Tx); err != nil { t.Fatalf("wait ERC20 depositV2: %v", err) }
	t.Logf("  Alice deposited %s USDT (commitment %s)", erc20Amount, aliceCmt)

	// ── Step 2: Bob deposits ERC-721 ticket ──────────────────────────────────
	t.Logf("Step 2 — Bob deposits ERC-721 ticket (tokenId=%s)", nftTokenId)

	bobSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Bob spend key: %v", err) }
	bobView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Bob view key: %v", err) }

	bobNftSalt, err := core.RandomInField()
	if err != nil { t.Fatalf("Bob RandomInField: %v", err) }
	bobNftCmt, err := core.Erc721Commitment(nftTokenId, bobSpend.PublicKey, bobNftSalt)
	if err != nil { t.Fatalf("Erc721Commitment: %v", err) }

	mintNftTx, err := erc721.Transact(auth, "mint", auth.From, nftTokenId)
	if err != nil { t.Fatalf("ERC721.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, client, mintNftTx); err != nil { t.Fatalf("wait NFT mint: %v", err) }
	approveNftTx, err := erc721.Transact(auth, "approve", nftVaultAddr, nftTokenId)
	if err != nil { t.Fatalf("ERC721.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, client, approveNftTx); err != nil { t.Fatalf("wait NFT approve: %v", err) }
	// C-1 fix: contract computes commitment = Poseidon4(pkSpend, salt, 1, tokenId) on-chain.
	depositNftTx, err := nftVault.Transact(auth, "deposit", []*big.Int{nftTokenId, bobSpend.PublicKey, bobNftSalt})
	if err != nil { t.Fatalf("nftVault.deposit: %v", err) }
	if _, err := bind.WaitMined(ctx, client, depositNftTx); err != nil { t.Fatalf("wait NFT deposit: %v", err) }
	t.Logf("  Bob deposited ticket tokenId=%s (commitment %s)", nftTokenId, bobNftCmt)

	// Build Merkle trees for each vault.
	erc20Mt := loadVaultMerkleTree(t, client, erc20VaultAddr, merkleDepth)
	nftMt   := loadVaultMerkleTree(t, client, nftVaultAddr,   merkleDepth)
	aliceProof, err := erc20Mt.GenerateProof(aliceCmt)
	if err != nil { t.Fatalf("GenerateProof (Alice ERC20): %v", err) }
	bobProof, err := nftMt.GenerateProof(bobNftCmt)
	if err != nil { t.Fatalf("GenerateProof (Bob NFT): %v", err) }
	t.Logf("  ERC-20 Merkle root: %s", aliceProof.Root)
	t.Logf("  NFT    Merkle root: %s", bobProof.Root)

	// ── Step 3: Alice — DvPInitiatorProof ────────────────────────────────────
	t.Log("Step 3 — Alice generates DvPInitiatorProof")

	initiator, err := gnarkClient.DvPInitiatorProof(
		core.KeyPair{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		aliceSaltField, erc20Amount, erc20TokenId,
		bobSpend.PublicKey, bobView.EncapsKey,
		nftAmount, nftTokenId,
		big.NewInt(0), aliceProof, merkleDepth,
	)
	if err != nil { t.Fatalf("DvPInitiatorProof: %v", err) }
	t.Logf("  commitB (Bob's USDT):    %s", initiator.CommitB)
	t.Logf("  commitA (Alice's ticket):%s", initiator.CommitA)

	// ── Step 4: Bob scans + verifies commitments ──────────────────────────────
	t.Log("Step 4 — Bob scans and verifies commitments")

	ssBob, err := core.Decapsulate(bobView.DecapsKey, initiator.CipherText)
	if err != nil { t.Fatalf("Bob Decapsulate: %v", err) }
	bobSaltBBytes, err := core.DerivePaymentSalt(ssBob)
	if err != nil { t.Fatalf("Bob DerivePaymentSalt: %v", err) }
	bobSaltABytes, err := core.DeriveDvpSaltInit(ssBob)
	if err != nil { t.Fatalf("Bob DeriveDvpSaltInit: %v", err) }
	bobEncKey, err := core.DerivePaymentKey(ssBob)
	if err != nil { t.Fatalf("Bob DerivePaymentKey: %v", err) }
	saltBField := core.SaltBToField(bobSaltBBytes)
	saltAField := core.SaltBToField(bobSaltABytes)

	decTokenId, decAmount, err := core.DecryptPayload(bobEncKey, initiator.EncTxData)
	if err != nil { t.Fatalf("Bob DecryptPayload: %v", err) }
	t.Logf("  Bob decrypted Alice's note: tokenId=%s amount=%s", decTokenId, decAmount)

	expectedCommitB, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBField, decAmount, decTokenId)
	if err != nil { t.Fatalf("commitB re-derive: %v", err) }
	if expectedCommitB.Cmp(initiator.CommitB) != 0 {
		t.Fatalf("commitB mismatch: got %s want %s", expectedCommitB, initiator.CommitB)
	}
	expectedCommitA, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, saltAField, nftAmount, nftTokenId)
	if err != nil { t.Fatalf("commitA re-derive: %v", err) }
	if expectedCommitA.Cmp(initiator.CommitA) != 0 {
		t.Fatalf("commitA mismatch: got %s want %s", expectedCommitA, initiator.CommitA)
	}
	t.Logf("  commitB and commitA both verified")

	// ── Step 5: Bob — DvPDestinationProof ────────────────────────────────────
	t.Log("Step 5 — Bob generates DvPDestinationProof")

	destination, err := gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey},
		bobNftSalt, nftAmount, nftTokenId,
		aliceSpend.PublicKey, saltAField,
		saltBField,  // Bob's HKDF payment salt (for Bob's ERC20 output commitment)
		decAmount,   // Alice's ERC20 amount Bob receives
		decTokenId,  // Alice's ERC20 tokenId Bob receives
		initiator.CommitA,
		big.NewInt(0), bobProof, merkleDepth,
	)
	if err != nil { t.Fatalf("DvPDestinationProof: %v", err) }
	t.Logf("  destination proof generated")

	// ── Step 6: POST /relay/swap ──────────────────────────────────────────────
	t.Log("Step 6 — POST /relay/swap")

	swapResp, status, err := dvpPost(t, "/relay/swap", dvpRelayerAPIKey, relaySwapReq{
		PaymentReceipt:  dvpInitiatorToPayload(t, initiator),
		DeliveryReceipt: dvpDestinationToPayload(t, destination),
		PaymentVaultId:  vaultIdErc20,
		DeliveryVaultId: vaultIdErc721,
	})
	if err != nil { t.Fatalf("POST /relay/swap: %v", err) }
	if status != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", status, swapResp.Error)
	}
	t.Logf("  txHash=%s block=%d gas=%d", swapResp.TxHash, swapResp.BlockNumber, swapResp.GasUsed)

	// ── Step 7: verify on-chain Commitment events ─────────────────────────────
	t.Log("Step 7 — verifying on-chain Commitment events")

	txHash := common.HexToHash(swapResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil { t.Fatalf("TransactionReceipt: %v", err) }

	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	nullifierSig  := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))

	var foundCommitA, foundCommitB bool
	nullifierCount := 0
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case commitmentSig:
			if len(log.Topics) >= 3 {
				cmt := log.Topics[2].Big()
				switch {
				case cmt.Cmp(initiator.CommitB) == 0:
					foundCommitB = true
					t.Logf("  commitB inserted (Bob's USDT): %s", cmt)
				case cmt.Cmp(initiator.CommitA) == 0:
					foundCommitA = true
					t.Logf("  commitA inserted (Alice's ticket): %s", cmt)
				}
			}
		case nullifierSig:
			nullifierCount++
		}
	}
	if !foundCommitB { t.Error("commitB (Bob's USDT) not found in Commitment events") }
	if !foundCommitA { t.Error("commitA (Alice's ticket) not found in Commitment events") }
	if nullifierCount < 2 {
		t.Errorf("expected ≥2 Nullifier events, got %d", nullifierCount)
	}
	t.Log("=== SWAP VIA RELAYER COMPLETE ===")
}

// ══════════════════════════════════════════════════════════════════════════════
// TestDvP_ExchangeViaRelayer
//
// Alice and Bob each have ERC-20 notes. They atomically exchange notes through
// the relayer via POST /relay/exchange.
//
//   Alice deposits 20 tokens → Bob gets 20 tokens
//   Bob   deposits 15 tokens → Alice gets 15 tokens
//
// Since exchange() requires both receipts to be ERC-20 Initiator-style proofs
// (the ERC-20 vault's checkReceiptConditions accesses statement[5] which only
// exists in a 7-element DvP Initiator statement), both parties use
// DvPInitiatorProofFromSalts with pre-agreed salts so the output commitments
// cross-reference each other:
//
//   commitForAlice = Poseidon4(aliceSpendPk, saltForAlice, bobAmt, tokenId)
//   commitForBob   = Poseidon4(bobSpendPk,   saltForBob,   aliceAmt, tokenId)
//
//   Alice: stMessage=commitForAlice, commitB=commitForBob,   commitA=commitForAlice
//   Bob:   stMessage=commitForBob,   commitB=commitForAlice, commitA=commitForBob
//
// Cross-reference check (on-chain):
//   receipt1.statement[0] (Alice's stMessage = commitForAlice) == receipt2.statement[4] (Bob's commitB = commitForAlice)
//   receipt2.statement[0] (Bob's   stMessage = commitForBob)   == receipt1.statement[4] (Alice's commitB = commitForBob)
//
// Verification:
//   - commitForBob  (Alice → Bob)  appears in Commitment events
//   - commitForAlice (Bob → Alice) appears in Commitment events
//   - Two Nullifier events (one per side).
// ══════════════════════════════════════════════════════════════════════════════

func TestDvP_ExchangeViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}

	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil { t.Fatalf("ethclient.Dial: %v", err) }
	defer client.Close()

	receipts       := loadOnchainReceipts(t)
	erc20VaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr      := common.HexToAddress(receipts["ERC20"].ContractAddress)

	erc20VaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	erc20ABI      := loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")

	erc20Vault := bind.NewBoundContract(erc20VaultAddr, erc20VaultABI, client, client, client)
	erc20      := bind.NewBoundContract(erc20Addr,      erc20ABI,      client, client, client)

	auth := hardhatAuth(t, client)

	gnarkClient := core.NewGnarkClient("http://localhost:8081")
	merkleDepth := 8
	tokenId     := big.NewInt(0)
	aliceAmt    := big.NewInt(20) // Alice gives 20 tokens to Bob
	bobAmt      := big.NewInt(15) // Bob   gives 15 tokens to Alice

	// ── Step 1: generate key pairs ────────────────────────────────────────────
	t.Log("Step 1 — generating key pairs")

	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Alice spend key: %v", err) }
	aliceView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Alice view key: %v", err) }
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Bob spend key: %v", err) }
	bobView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Bob view key: %v", err) }
	_ = aliceView
	_ = bobView

	// ── Step 2: Alice deposits 20 ERC-20 tokens ───────────────────────────────
	t.Log("Step 2 — Alice deposits 20 ERC-20 tokens")

	ssAlice, capsuleAlice, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil { t.Fatalf("Encapsulate (Alice): %v", err) }
	aliceSaltBytes, err := core.DerivePaymentSalt(ssAlice)
	if err != nil { t.Fatalf("DerivePaymentSalt (Alice): %v", err) }
	aliceEncKey, err := core.DerivePaymentKey(ssAlice)
	if err != nil { t.Fatalf("DerivePaymentKey (Alice): %v", err) }
	aliceSaltField := core.SaltBToField(aliceSaltBytes)
	aliceCmt, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltField, aliceAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Alice): %v", err) }
	aliceDepositEnc, err := core.EncryptPayload(aliceEncKey, tokenId, aliceAmt)
	if err != nil { t.Fatalf("EncryptPayload (Alice): %v", err) }

	mintTx, err := erc20.Transact(auth, "mint", auth.From,
		new(big.Int).Mul(new(big.Int).Add(aliceAmt, bobAmt), big.NewInt(10)))
	if err != nil { t.Fatalf("ERC20.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil { t.Fatalf("wait mint: %v", err) }

	approveTx, err := erc20.Transact(auth, "approve", erc20VaultAddr,
		new(big.Int).Add(aliceAmt, bobAmt))
	if err != nil { t.Fatalf("ERC20.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil { t.Fatalf("wait approve: %v", err) }

	// C-1 fix: contract computes commitment on-chain from key material.
	depositAliceTx, err := erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{aliceAmt, aliceSpend.PublicKey, aliceSaltField, tokenId}, capsuleAlice, aliceDepositEnc)
	if err != nil { t.Fatalf("Alice depositV2: %v", err) }
	if _, err := bind.WaitMined(ctx, client, depositAliceTx); err != nil { t.Fatalf("wait Alice deposit: %v", err) }
	t.Logf("  Alice deposited %s tokens (commitment %s)", aliceAmt, aliceCmt)

	// ── Step 3: Bob deposits 15 ERC-20 tokens ────────────────────────────────
	t.Log("Step 3 — Bob deposits 15 ERC-20 tokens")

	ssBob, capsuleBob, err := core.Encapsulate(bobView.EncapsKey)
	if err != nil { t.Fatalf("Encapsulate (Bob): %v", err) }
	bobSaltBytes, err := core.DerivePaymentSalt(ssBob)
	if err != nil { t.Fatalf("DerivePaymentSalt (Bob): %v", err) }
	bobEncKey, err := core.DerivePaymentKey(ssBob)
	if err != nil { t.Fatalf("DerivePaymentKey (Bob): %v", err) }
	bobSaltField := core.SaltBToField(bobSaltBytes)
	bobCmt, err := core.Erc20CommitmentV2(bobSpend.PublicKey, bobSaltField, bobAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Bob): %v", err) }
	bobDepositEnc, err := core.EncryptPayload(bobEncKey, tokenId, bobAmt)
	if err != nil { t.Fatalf("EncryptPayload (Bob): %v", err) }

	// C-1 fix: contract computes commitment on-chain from key material.
	depositBobTx, err := erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{bobAmt, bobSpend.PublicKey, bobSaltField, tokenId}, capsuleBob, bobDepositEnc)
	if err != nil { t.Fatalf("Bob depositV2: %v", err) }
	if _, err := bind.WaitMined(ctx, client, depositBobTx); err != nil { t.Fatalf("wait Bob deposit: %v", err) }
	t.Logf("  Bob deposited %s tokens (commitment %s)", bobAmt, bobCmt)

	// ── Step 4: pre-agree on output salts ────────────────────────────────────
	//
	// Both parties pre-agree on saltForAlice and saltForBob so their respective
	// DvPInitiatorProofs produce cross-referencing commitments without KEM.
	//
	//   commitForAlice = Poseidon4(aliceSpendPk, saltForAlice, bobAmt, tokenId)
	//   commitForBob   = Poseidon4(bobSpendPk,   saltForBob,   aliceAmt, tokenId)
	//
	//   Alice's proof: stMessage=commitForAlice, commitA=commitForAlice, commitB=commitForBob
	//   Bob's proof:   stMessage=commitForBob,   commitA=commitForBob,   commitB=commitForAlice
	t.Log("Step 4 — pre-computing output salts and commitments")

	saltForAliceBytes, err := core.GenerateRandomValue(32)
	if err != nil { t.Fatalf("GenerateRandomValue (saltForAlice): %v", err) }
	saltForBobBytes, err := core.GenerateRandomValue(32)
	if err != nil { t.Fatalf("GenerateRandomValue (saltForBob): %v", err) }
	saltForAlice := core.SaltBToField(saltForAliceBytes)
	saltForBob   := core.SaltBToField(saltForBobBytes)

	commitForAlice, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, saltForAlice, bobAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (commitForAlice): %v", err) }
	commitForBob, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltForBob, aliceAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (commitForBob): %v", err) }
	t.Logf("  commitForAlice = %s", commitForAlice)
	t.Logf("  commitForBob   = %s", commitForBob)

	// ── Step 5: build Merkle proofs ────────────────────────────────────────────
	t.Log("Step 5 — building Merkle proofs")

	mt := loadVaultMerkleTree(t, client, erc20VaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCmt)
	if err != nil { t.Fatalf("GenerateProof (Alice): %v", err) }
	bobProof, err := mt.GenerateProof(bobCmt)
	if err != nil { t.Fatalf("GenerateProof (Bob): %v", err) }

	// ── Step 6: Alice generates DvPInitiatorProofFromSalts ────────────────────
	//
	// Alice spends her 20-token note.
	//   saltB = saltForBob   → commitB = commitForBob   (Alice's output for Bob)
	//   saltA = saltForAlice → commitA = commitForAlice  (= stMessage; what Alice expects)
	t.Log("Step 6 — Alice generates DvPInitiatorProofFromSalts")

	aliceResult, err := gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		aliceSaltField,
		aliceAmt, tokenId,
		bobSpend.PublicKey,
		saltForBob,  // saltB: Alice's output salt for Bob's commitment
		bobAmt, tokenId,
		saltForAlice, // saltA: Alice's expected incoming commitment salt
		big.NewInt(0),
		aliceProof,
		merkleDepth,
	)
	if err != nil { t.Fatalf("DvPInitiatorProofFromSalts (Alice): %v", err) }
	t.Logf("  Alice commitB (for Bob):   %s", aliceResult.CommitB)
	t.Logf("  Alice commitA (for Alice): %s", aliceResult.CommitA)

	// ── Step 7: Bob generates DvPInitiatorProofFromSalts ─────────────────────
	//
	// Bob spends his 15-token note.
	//   saltB = saltForAlice → commitB = commitForAlice (Bob's output for Alice)
	//   saltA = saltForBob   → commitA = commitForBob   (= stMessage; what Bob expects)
	t.Log("Step 7 — Bob generates DvPInitiatorProofFromSalts")

	bobResult, err := gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey},
		bobSaltField,
		bobAmt, tokenId,
		aliceSpend.PublicKey,
		saltForAlice, // saltB: Bob's output salt for Alice's commitment
		aliceAmt, tokenId,
		saltForBob,   // saltA: Bob's expected incoming commitment salt
		big.NewInt(0),
		bobProof,
		merkleDepth,
	)
	if err != nil { t.Fatalf("DvPInitiatorProofFromSalts (Bob): %v", err) }
	t.Logf("  Bob commitB (for Alice): %s", bobResult.CommitB)
	t.Logf("  Bob commitA (for Bob):   %s", bobResult.CommitA)

	// ── Step 8: verify cross-references locally ───────────────────────────────
	// receipt1.statement[0] (=commitForAlice) must equal receipt2.statement[4] (=commitForAlice)
	// receipt2.statement[0] (=commitForBob)   must equal receipt1.statement[4] (=commitForBob)
	if aliceResult.Statement[0].Cmp(bobResult.Statement[4]) != 0 {
		t.Fatalf("cross-ref A: alice.stmt[0]=%s != bob.stmt[4]=%s",
			aliceResult.Statement[0], bobResult.Statement[4])
	}
	if bobResult.Statement[0].Cmp(aliceResult.Statement[4]) != 0 {
		t.Fatalf("cross-ref B: bob.stmt[0]=%s != alice.stmt[4]=%s",
			bobResult.Statement[0], aliceResult.Statement[4])
	}
	t.Log("  cross-reference check passed")

	// ── Step 9: POST /relay/exchange ──────────────────────────────────────────
	t.Log("Step 9 — POST /relay/exchange")

	exchResp, status, err := dvpPost(t, "/relay/exchange", dvpRelayerAPIKey, relayExchangeReq{
		Receipt1: dvpInitiatorToPayload(t, aliceResult),
		Receipt2: dvpInitiatorToPayload(t, bobResult),
		VaultId1: vaultIdErc20,
		VaultId2: vaultIdErc20,
	})
	if err != nil { t.Fatalf("POST /relay/exchange: %v", err) }
	if status != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", status, exchResp.Error)
	}
	t.Logf("  txHash=%s block=%d gas=%d", exchResp.TxHash, exchResp.BlockNumber, exchResp.GasUsed)

	// ── Step 10: verify on-chain Commitment events ────────────────────────────
	t.Log("Step 10 — verifying on-chain Commitment events")

	txHash := common.HexToHash(exchResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil { t.Fatalf("TransactionReceipt: %v", err) }

	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	nullifierSig  := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))

	var foundForBob, foundForAlice bool
	nullifierCount := 0
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case commitmentSig:
			if len(log.Topics) >= 3 {
				cmt := log.Topics[2].Big()
				switch {
				case cmt.Cmp(commitForBob) == 0:
					foundForBob = true
					t.Logf("  commitForBob inserted: %s", cmt)
				case cmt.Cmp(commitForAlice) == 0:
					foundForAlice = true
					t.Logf("  commitForAlice inserted: %s", cmt)
				}
			}
		case nullifierSig:
			nullifierCount++
		}
	}
	if !foundForBob   { t.Error("commitForBob not found in Commitment events") }
	if !foundForAlice { t.Error("commitForAlice not found in Commitment events") }
	if nullifierCount < 2 {
		t.Errorf("expected ≥2 Nullifier events, got %d", nullifierCount)
	}
	t.Logf("=== EXCHANGE VIA RELAYER COMPLETE ===")
	t.Logf("    Alice gave %s tokens → Bob  (commitForBob=%s)", aliceAmt, commitForBob)
	t.Logf("    Bob   gave %s tokens → Alice (commitForAlice=%s)", bobAmt, commitForAlice)
}

// ══════════════════════════════════════════════════════════════════════════════
// Auth / validation error tests
// These only need the DVP relayer to be running — no chain or gnark server.
// ══════════════════════════════════════════════════════════════════════════════

func TestDvPRelayer_HealthCheck(t *testing.T) {
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}
	resp, err := http.Get(dvpRelayerURL + "/health")
	if err != nil { t.Fatalf("GET /health: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	t.Logf("  /health: %s", raw)
}

func TestDvPRelayer_MissingAuthHeader(t *testing.T) {
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}
	for _, path := range []string{"/relay/payment", "/relay/swap", "/relay/exchange"} {
		req, _ := http.NewRequest(http.MethodPost, dvpRelayerURL+path, bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil { t.Fatalf("%s: %v", path, err) }
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", path, resp.StatusCode)
		} else {
			t.Logf("  %s → 401 ✓", path)
		}
	}
}

func TestDvPRelayer_WrongToken(t *testing.T) {
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}
	for _, path := range []string{"/relay/payment", "/relay/swap", "/relay/exchange"} {
		req, _ := http.NewRequest(http.MethodPost, dvpRelayerURL+path, bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer wrong-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil { t.Fatalf("%s: %v", path, err) }
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", path, resp.StatusCode)
		} else {
			t.Logf("  %s → 403 ✓", path)
		}
	}
}

func TestDvPRelayer_BadProof(t *testing.T) {
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}
	// payment: malformed proof element
	body := `{"vaultId":"0","receipt":{"proof":["not-a-number","0","0","0","0","0","0","0"],"publicSignal":["0","0","0","0","0"],"numberOfInputs":1,"numberOfOutputs":1},"cipherText":"0x00","encTxData":"0x00"}`
	relayResp, status, err := dvpPost(t, "/relay/payment", dvpRelayerAPIKey, json.RawMessage(body))
	if err != nil { t.Fatalf("POST /relay/payment: %v", err) }
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", status, relayResp.Error)
	} else {
		t.Logf("  /relay/payment with bad proof → 400 ✓ (%s)", relayResp.Error)
	}
}

func TestDvPRelayer_SignalTooShort(t *testing.T) {
	if !serverAvailable("localhost:8091") {
		t.Skip("DVP relayer not running on localhost:8091 — skipping")
	}
	// numberOfInputs=2 but only 5 signal elements — needs at least 1+3*2+1=8
	body := `{"vaultId":"0","receipt":{"proof":["0","0","0","0","0","0","0","0"],"publicSignal":["0","0","0","0","0"],"numberOfInputs":2,"numberOfOutputs":1},"cipherText":"0x00","encTxData":"0x00"}`
	relayResp, status, err := dvpPost(t, "/relay/payment", dvpRelayerAPIKey, json.RawMessage(body))
	if err != nil { t.Fatalf("POST /relay/payment: %v", err) }
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", status, relayResp.Error)
	} else {
		t.Logf("  /relay/payment with short signal → 400 ✓ (%s)", relayResp.Error)
	}
}

