package tests

// On-chain integration tests for the ERC20 retail payment flow.
//
// TestRetailErc20_PrivateMint: exercises the PrivateMint flow.
//   - The deployment ships no PrivateMintVerifier (its config's "private-mint"
//     switch is off), so the Issuer registers one and a mint-only vault.
//   - Issuer mints a private note directly to Alice's spend key, once.
//
// TestRetailErc20_Payment: full register → deposit → payment → scan cycle.
//   - Alice and Bob each register their spend key and view key on-chain.
//   - Alice deposits 40 tokens.
//   - Alice looks up Bob's keys from the registry and pays him 30, keeps 10 as change.
//   - Bob and Alice scan their respective notes.
//
// Prerequisites:
//   1. Hardhat node (keep running):     cd ../enygma_dvp && npx hardhat node
//   2. Deploy + init contracts:         bash setup.sh   (from enygma_retail_payments/ root)
//   3. Gnark server (port 8082, keep running): cd gnark_circuits && go run ./cmd/server
//
// Run with:
//   cd test && go test ./... -v -timeout 600s

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── PrivateMint ABI types ──────────────────────────────────────────────────────

type onchainPrivateMintProof struct {
	Proof        [8]*big.Int `abi:"proof"`
	PublicSignal [4]*big.Int `abi:"public_signal"`
}

func proofStringsToPrivateMint(t *testing.T, proofStrs []string, statement []*big.Int) onchainPrivateMintProof {
	t.Helper()
	if len(proofStrs) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(proofStrs))
	}
	if len(statement) != 4 {
		t.Fatalf("expected 4 public signal elements, got %d", len(statement))
	}
	var p onchainPrivateMintProof
	for i, s := range proofStrs {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("invalid proof element %d: %q", i, s)
		}
		p.Proof[i] = n
	}
	copy(p.PublicSignal[:], statement)
	return p
}

// ── TestRetailErc20_PrivateMint ────────────────────────────────────────────────

func TestRetailErc20_PrivateMint(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping on-chain test")
	}

	ctx := context.Background()

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	dvpABI := loadOnchainABI(t, "EnygmaDvp")
	dvp := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)
	auth := hardhatAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	tokenId := big.NewInt(0)
	mintAmount := big.NewInt(100)
	contractAddressBig := new(big.Int).SetBytes(dvpAddr.Bytes())

	// Step 1 — Alice generates her spend key pair.
	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	t.Logf("Step 1 — Alice pk_spend: %s", aliceSpend.PublicKey)

	// Step 2 — Issuer requests ZK proof from gnark server.
	issuedSalt, err := rpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (salt): %v", err)
	}

	mintResult, err := gnarkClient.Erc20PrivateMintProof(
		aliceSpend.PublicKey,
		issuedSalt,
		mintAmount,
		tokenId,
		contractAddressBig,
	)
	if err != nil {
		t.Fatalf("Erc20PrivateMintProof: %v", err)
	}
	t.Logf("Step 2 — commitment: %s", mintResult.Commitment())

	proof := proofStringsToPrivateMint(t,
		mintResult.Proof,
		mintResult.ContractStatement(),
	)

	// Step 3 — Issuer calls EnygmaDvp.privateMint onchain. A minted note has
	// no deposit behind it, and both of this deployment's vaults take
	// deposits, so the deployment ships no PrivateMintVerifier: the Issuer
	// registers one, and a mint-only vault of its own.
	if pm, ok := receipts["PrivateMintVerifier"]; ok {
		t.Fatalf("the deploy shipped a PrivateMintVerifier (%s), though enygmapayment.config.json leaves private-mint off", pm.ContractAddress)
	}
	verifierAddr := deployFromAbis(t, "PrivateMintVerifier", client, auth)
	regTx, err := dvp.Transact(auth, "registerPrivateMintVerifier", verifierAddr)
	waitMinted(t, client, "registerPrivateMintVerifier", regTx, err)
	vaultId := registerMintOnlyVault(t, client, auth, dvp, dvpAddr)
	mintTx, err := dvp.Transact(auth, "privateMint", vaultId, mintResult.Commitment(), proof)
	if err != nil {
		t.Fatalf("EnygmaDvp.privateMint: %v", err)
	}
	mintReceipt, err := bind.WaitMined(ctx, client, mintTx)
	if err != nil {
		t.Fatalf("wait privateMint: %v", err)
	}
	t.Logf("Step 3 — privateMint mined (block %d, gas %d)",
		mintReceipt.BlockNumber, mintReceipt.GasUsed)

	privateMintSig := crypto.Keccak256Hash([]byte("PrivateMint(uint256,uint256,uint256)"))
	var eventCommitment *big.Int
	for _, log := range mintReceipt.Logs {
		if log.Topics[0] == privateMintSig {
			eventCommitment = log.Topics[2].Big()
			t.Logf("  PrivateMint event: commitment=%s", eventCommitment)
		}
	}
	if eventCommitment == nil {
		t.Fatalf("PrivateMint event not found in transaction receipt")
	}
	if eventCommitment.Cmp(mintResult.Commitment()) != 0 {
		t.Errorf("event commitment: got %s, want %s", eventCommitment, mintResult.Commitment())
	}

	// Step 4 — Alice confirms the note is hers.
	recomputedCommitment, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, issuedSalt, mintAmount, tokenId,
	)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 recompute: %v", err)
	}
	if recomputedCommitment.Cmp(eventCommitment) != 0 {
		t.Errorf("Alice commitment round-trip failed: got %s, want %s",
			recomputedCommitment, eventCommitment)
	}
	t.Logf("Step 4 — Alice confirmed note: commitment=%s amount=%s salt=%s",
		eventCommitment, mintAmount, issuedSalt)

	// Step 5 — The same proof cannot mint the note again: EnygmaDvp keeps
	// every commitment it has minted.
	estimate := *auth
	estimate.GasLimit = 0 // estimate, so a revert comes back with its reason
	_, err = dvp.Transact(&estimate, "privateMint", vaultId, mintResult.Commitment(), proof)
	if err == nil || !strings.Contains(err.Error(), "CommitmentAlreadyMinted") {
		t.Fatalf("second privateMint of commitment %s: err = %v, want CommitmentAlreadyMinted", mintResult.Commitment(), err)
	}
	t.Logf("Step 5 — a second mint of the same proof was refused: CommitmentAlreadyMinted")
}

// deployFromAbis deploys the contracts/abis/<name>.json artifact with args and
// returns its address.
func deployFromAbis(t *testing.T, name string, client *ethclient.Client, auth *bind.TransactOpts, args ...interface{}) common.Address {
	t.Helper()
	data, err := os.ReadFile("../contracts/abis/" + name + ".json")
	if err != nil {
		t.Fatalf("read %s artifact: %v", name, err)
	}
	var artifact struct {
		Bytecode string `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse %s artifact: %v", name, err)
	}
	addr, tx, _, err := bind.DeployContract(auth, loadOnchainABI(t, name), common.FromHex(artifact.Bytecode), client, args...)
	waitMinted(t, client, "deploy "+name, tx, err)
	return addr
}

// waitMinted fails the test unless tx was sent (err nil) and mined successfully.
func waitMinted(t *testing.T, client *ethclient.Client, what string, tx *types.Transaction, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	rcpt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("%s: not mined successfully (err=%v)", what, err)
	}
}

// registerMintOnlyVault deploys a token and an Erc20CoinVault, registers the
// vault with the EnygmaDvp at dvpAddr, at the deployment's tree depth, and
// flags it mintable while it is empty, which closes it to deposits for good.
// It returns the vault's id.
func registerMintOnlyVault(t *testing.T, client *ethclient.Client, auth *bind.TransactOpts, dvp *bind.BoundContract, dvpAddr common.Address) *big.Int {
	t.Helper()
	data, err := os.ReadFile("../enygmapayment.config.json")
	if err != nil {
		t.Fatalf("read enygmapayment.config.json: %v", err)
	}
	var config struct {
		Circom struct {
			MetaParameters struct {
				TreeDepth int64 `json:"tree-depth"`
			} `json:"meta-parameters"`
		} `json:"circom"`
	}
	if err := json.Unmarshal(data, &config); err != nil || config.Circom.MetaParameters.TreeDepth == 0 {
		t.Fatalf("enygmapayment.config.json: no circom.meta-parameters.tree-depth (err %v)", err)
	}

	tokenAddr := deployFromAbis(t, "RaylsERC20", client, auth, "Minted", "MNT")
	vaultAddr := deployFromAbis(t, "Erc20CoinVault", client, auth, dvpAddr)
	tx, err := dvp.Transact(auth, "registerVault", vaultAddr, tokenAddr, big.NewInt(1), big.NewInt(config.Circom.MetaParameters.TreeDepth))
	waitMinted(t, client, "registerVault", tx, err)

	vault := bind.NewBoundContract(vaultAddr, loadOnchainABI(t, "Erc20CoinVault"), client, client, client)
	var out []interface{}
	if err := vault.Call(&bind.CallOpts{}, &out, "getVaultId"); err != nil {
		t.Fatalf("getVaultId: %v", err)
	}
	vaultId := out[0].(*big.Int)
	tx, err = dvp.Transact(auth, "setVaultMintable", vaultId, true)
	waitMinted(t, client, "setVaultMintable", tx, err)
	out = nil
	if err := vault.Call(&bind.CallOpts{}, &out, "depositsClosed"); err != nil {
		t.Fatalf("depositsClosed: %v", err)
	}
	if !out[0].(bool) {
		t.Fatalf("vault %s takes deposits after setVaultMintable(%s, true)", vaultAddr.Hex(), vaultId)
	}
	t.Logf("Step 3 — mint-only vault %s registered as vault %s, closed to deposits", vaultAddr.Hex(), vaultId)
	return vaultId
}

// ── TestRetailErc20_Payment ────────────────────────────────────────────────────

func TestRetailErc20_Payment(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping on-chain test")
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
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI := loadOnchainABI(t, "RaylsERC20")
	dvpABI := loadOnchainABI(t, "EnygmaDvp")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)

	aliceAuth := hardhatAuth(t, client)
	bobAuth := hardhatBobAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId := big.NewInt(0)
	depositAmt := big.NewInt(40)
	paymentAmt := big.NewInt(30)
	changeAmt := big.NewInt(10)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 1: Key generation — Alice and Bob each generate their ZK key pairs.
	// ─────────────────────────────────────────────────────────────────────────
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

	t.Logf("Step 1 — Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("Step 1 — Bob   pk_spend: %s", bobSpend.PublicKey)

	// Generate auditor keypair and encrypt each user's view secret key for the auditor.
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

	// ─────────────────────────────────────────────────────────────────────────
	// Step 2: Registration — Alice and Bob publish their keys on-chain.
	//   - pkSpend stored in contract state (uint256).
	//   - pkView  stored in contract state (bytes, 1184 bytes ML-KEM-768 key).
	//   - mlKemAuditCt + aesAuditCt stored for auditor key recovery.
	// ─────────────────────────────────────────────────────────────────────────
	t.Log("Step 2 — Alice registers her keys on-chain...")
	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Logf("Step 2 — Alice already registered (%s)", aliceAuth.From.Hex())
	} else {
		t.Logf("Step 2 — Alice registered (Ethereum addr: %s)", aliceAuth.From.Hex())
	}

	t.Log("Step 2 — Bob registers his keys on-chain...")
	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Logf("Step 2 — Bob already registered (%s)", bobAuth.From.Hex())
	} else {
		t.Logf("Step 2 — Bob registered (Ethernet addr: %s)", bobAuth.From.Hex())
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Step 3: Deposit — Alice deposits 40 tokens into the private vault.
	// ─────────────────────────────────────────────────────────────────────────
	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From,
		new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	t.Logf("Step 3 — minted tokens to Alice (%s)", aliceAuth.From.Hex())

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
		t.Fatalf("DerivePaymentSalt (deposit): %v", err)
	}
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey (deposit): %v", err)
	}
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)

	aliceCommitment, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}

	aliceDepositCtxtII, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
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
	t.Logf("Step 3 — depositV2 mined (block %d, gas %d, commitment %s)",
		depositReceipt.BlockNumber, depositReceipt.GasUsed, aliceCommitment)

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof for Alice: %v", err)
	}
	t.Logf("Step 3 — Merkle root: %s", aliceProof.Root)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 4: Key lookup — verify registry returns keys for Bob and Alice.
	//   The lookup is tested here; the actual payment uses local keys so the
	//   test is self-consistent regardless of what prior tests registered.
	// ─────────────────────────────────────────────────────────────────────────
	bobEthAddr := common.HexToAddress(hardhatBobAddr)
	bobKeys, err := rpcore.LookupKeys(client, registryAddr, bobEthAddr)
	if err != nil {
		t.Fatalf("LookupKeys (Bob): %v", err)
	}
	t.Logf("Step 4 — registry lookup OK (on-chain pk_spend=%s, view=%d bytes)",
		bobKeys.SpendKey, len(bobKeys.ViewKey))

	aliceEthAddr := common.HexToAddress(hardhatAliceAddr)
	if _, err := rpcore.LookupKeys(client, registryAddr, aliceEthAddr); err != nil {
		t.Fatalf("LookupKeys (Alice): %v", err)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Step 5: Payment — Alice pays 30 to Bob, keeps 10 as change.
	//   Inputs:  [Alice's 40-token note]
	//   Outputs: [30 to Bob, 10 change to Alice]
	//   Local keys are used so Bob can decrypt and Alice can verify the change,
	//   independent of what keys are currently stored in the registry.
	// ─────────────────────────────────────────────────────────────────────────
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
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey}, // local keys
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},     // local keys
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		tokenId,
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	t.Logf("Step 5 — gnark Payment proof generated")
	t.Logf("  Bob's commitment   (output 0): %s", paymentResult.Statement[layout.PaymentCommitmentsOutOffset])
	t.Logf("  Alice's change cmt (output 1): %s", paymentResult.Statement[layout.PaymentCommitmentsOutOffset+1])

	snarkProof := proofStringsToOnchain(t, paymentResult.Proof)
	onchainReceipt := ProofReceipt{
		Proof: SnarkProof{
			A: G1Point{X: snarkProof.A.X, Y: snarkProof.A.Y},
			B: G2Point{X: snarkProof.B.X, Y: snarkProof.B.Y},
			C: G1Point{X: snarkProof.C.X, Y: snarkProof.C.Y},
		},
		Statement:       paymentResult.ContractStatement(),
		NumberOfInputs:  big.NewInt(int64(paymentResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(paymentResult.NumberOfOutputs)),
	}

	vaultId := big.NewInt(0)
	onchainTx, err := submitPayment(
		client, aliceAuth, dvpABI, dvpAddr,
		onchainReceipt,
		vaultId,
		paymentResult.CipherText, // Bob's ML-KEM capsule only
		paymentResult.EncTxData,  // Bob's AES-GCM ciphertext only
	)
	if err != nil {
		t.Fatalf("SubmitPayment: %v", err)
	}
	t.Logf("Step 5 — payment() mined (block %d, gas %d)",
		onchainTx.BlockNumber, onchainTx.GasUsed)

	paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	var paymentEvents int
	for _, log := range onchainTx.Logs {
		if log.Topics[0] == paymentSig {
			paymentEvents++
			t.Logf("  Payment event: commitment=%s", log.Topics[2].Big())
		}
	}
	// Only Bob's destination commitment (output 0) emits a Payment event.
	// Alice's change commitment is inserted into the tree but has no on-chain ciphertext.
	if paymentEvents != 1 {
		t.Errorf("expected 1 Payment event (Bob's destination), got %d", paymentEvents)
	}

	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	foundNullifier := false
	for _, log := range onchainTx.Logs {
		if log.Topics[0] == nullifierSig {
			foundNullifier = true
			t.Logf("  Nullifier event: nullifier=%s", log.Topics[3].Big())
		}
	}
	if !foundNullifier {
		t.Errorf("Nullifier event not found in payment receipt")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Step 6: Scanning — Bob and Alice scan Payment events for their notes.
	// ─────────────────────────────────────────────────────────────────────────
	bobEvents := []dvpcore.OnChainErc20Event{{
		Commitment: paymentResult.Statement[layout.PaymentCommitmentsOutOffset],
		CipherText: paymentResult.CipherText,
		EncTxData:  paymentResult.EncTxData,
	}}
	bobNotes, err := dvpcore.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, bobEvents)
	if err != nil {
		t.Fatalf("ScanForErc20Notes (Bob): %v", err)
	}
	if len(bobNotes) != 1 {
		t.Fatalf("Bob expected 1 note, got %d", len(bobNotes))
	}
	if bobNotes[0].Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("Bob's note amount: got %s, want %s", bobNotes[0].Amount, paymentAmt)
	}
	t.Logf("Step 6 — Bob scanned his note: amount=%s tokenId=%s",
		bobNotes[0].Amount, bobNotes[0].TokenId)

	// Alice's change salt is random (protocol §"Deriving a change salt") and returned
	// directly in paymentResult.SaltA — she doesn't need to scan the chain.
	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, paymentResult.SaltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Alice change verify): %v", err)
	}
	if aliceChangeCmt.Cmp(paymentResult.Statement[layout.PaymentCommitmentsOutOffset+1]) != 0 {
		t.Errorf("Alice's change commitment mismatch: got %s, want %s",
			aliceChangeCmt, paymentResult.Statement[layout.PaymentCommitmentsOutOffset+1])
	}
	t.Logf("Step 6 — Alice verified her change note: amount=%s tokenId=%s saltA=%s",
		changeAmt, tokenId, paymentResult.SaltA)

	t.Logf("=== PAYMENT COMPLETE: Alice paid %s tokens to Bob, kept %s tokens as change ===",
		paymentAmt, changeAmt)
}
