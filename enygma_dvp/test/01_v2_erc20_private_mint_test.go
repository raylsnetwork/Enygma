package tests

// On-chain integration test for the V2 ERC20 PrivateMint flow.
//
// Prerequisites (all must be running/completed before this test):
//
//	1. Hardhat node:      npx hardhat node
//	2. Deploy contracts:  cd scripts &&  go build -o /tmp/deploy ./cmd/deploy && cd .. && /tmp/deploy
//	3. Export VKs:        cd gnark_circuits && go run ./cmd/export_vk_init/ ../build
//	4. Init contracts:    cd scripts &&  go build -o /tmp/init ./cmd/init && cd .. && /tmp/init
//	5. Gnark server:      cd gnark_circuits && go run ./cmd/server
//
// Run with:
//
//	 go test -run TestV2Erc20OnChain_PrivateMint -v -timeout 300s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_dvp/layout"
)

type onchainPrivateMintProof struct {
	Proof        [8]*big.Int                     `abi:"proof"`
	PublicSignal [layout.PrivateMintLen]*big.Int `abi:"public_signal"`
}

// proofStringsToPrivateMint converts the 8 proof strings and the PrivateMint
// statement returned by the gnark server into the on-chain ABI struct.
func proofStringsToPrivateMint(t *testing.T, proofStrs []string, statement []*big.Int) onchainPrivateMintProof {
	t.Helper()
	if len(proofStrs) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(proofStrs))
	}
	if len(statement) != layout.PrivateMintLen {
		t.Fatalf("expected %d public signal elements, got %d", layout.PrivateMintLen, len(statement))
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

// TestV2Erc20OnChain_PrivateMint exercises the full PrivateMint flow against a live
// Hardhat node and gnark proof server:
//
//	Step 1 — Off-chain agreement: Alice shares pk_spend with the Issuer.
//	Step 2 — Issuer requests a ZK proof from the gnark server.
//	Step 3 — Issuer registers a mint-only vault and calls EnygmaDvp.privateMint
//	         onchain.
//	Step 4 — Alice checks the PrivateMint event against the salt and amount the
//	         Issuer sends her offchain, and confirms the note is hers.
//	Step 5 — The same proof is refused a second time.
func TestV2Erc20OnChain_PrivateMint(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	ctx := context.Background()

	// ── Connect to Hardhat ────────────────────────────────────────────────────
	ethClient, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer ethClient.Close()

	// ── Load deployed contract addresses ─────────────────────────────────────
	receipts := loadOnchainReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	// ── Load EnygmaDvp ABI ────────────────────────────────────────────────────
	dvpABI := loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")

	// ── Create bound contract and signer ─────────────────────────────────────
	dvp := bind.NewBoundContract(dvpAddr, dvpABI, ethClient, ethClient, ethClient)
	auth := hardhatAuth(t, ethClient)

	// ── Test parameters ───────────────────────────────────────────────────────
	gnarkClient := core.NewGnarkClient("http://localhost:8081")
	tokenId := big.NewInt(0) // ERC20: tokenId=0
	mintAmount := big.NewInt(100)
	// contractAddress in the ZK public signal must be the EnygmaDvp address as uint256
	contractAddressBig := new(big.Int).SetBytes(dvpAddr.Bytes())

	// ─────────────────────────────────────────────────────────────────────────
	// Step 1 — Off-chain agreement
	//   Alice generates her spend key pair and shares pk_spend with the Issuer.
	//   The Issuer will embed it in the commitment; only Alice (sk_spend holder)
	//   can spend the resulting note.
	// ─────────────────────────────────────────────────────────────────────────
	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	t.Logf("Step 1 — Alice pk_spend: %s", aliceSpend.PublicKey)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 2 — Issuer requests ZK proof from gnark server
	//   The Issuer picks a random salt and calls Erc20PrivateMintProof.
	//   The gnark server proves:
	//     Poseidon4(pk_alice, salt, amount, tokenId)  == commitment
	//     Poseidon(pk_alice, salt, contractAddress)   == cipherText (the note tag)
	// ─────────────────────────────────────────────────────────────────────────
	issuedSalt, err := core.RandomInField()
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
	t.Logf("Step 2 — commitment:  %s", mintResult.Commitment())
	t.Logf("Step 2 — cipherText:  %s", mintResult.CipherText())

	// Convert proof and public signal to on-chain ABI types
	proof := proofStringsToPrivateMint(t,
		mintResult.Proof,
		mintResult.ContractStatement(),
	)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 3 — Issuer registers a mint-only vault and calls EnygmaDvp.privateMint
	//   A minted note has no deposit behind it, so it goes into a vault of its
	//   own, never one that takes deposits: the Issuer registers a fresh
	//   Erc20CoinVault and flags it mintable while it is empty, which closes it
	//   to deposits for good. The contract then checks:
	//     1. onlyRole(DEFAULT_OWNER_ROLE) — the Hardhat deployer account holds this role
	//     2. the vault is flagged mintable (setVaultMintable)
	//     3. IPrivateMintVerifier.verifyProof(proof, publicSignal)
	//     4. the commitment has not been minted before
	//     5. IAbstractCoinVault.registerCoins([commitment]) — inserts the Merkle leaf
	//   Emits: PrivateMint(vaultId, commitment, cipherText)
	// ─────────────────────────────────────────────────────────────────────────
	vaultId := registerMintOnlyVault(t, ethClient, auth, dvp, dvpAddr)
	mintTx, err := dvp.Transact(auth, "privateMint", vaultId, mintResult.Commitment(), proof)
	if err != nil {
		t.Fatalf("EnygmaDvp.privateMint: %v", err)
	}
	mintReceipt, err := bind.WaitMined(ctx, ethClient, mintTx)
	if err != nil {
		t.Fatalf("wait privateMint: %v", err)
	}
	t.Logf("Step 3 — privateMint mined (block %d, gas %d)",
		mintReceipt.BlockNumber, mintReceipt.GasUsed)

	// ── Verify PrivateMint event ──────────────────────────────────────────────
	privateMintSig := crypto.Keccak256Hash([]byte("PrivateMint(uint256,uint256,uint256)"))
	var eventVaultId, eventCommitment, eventCipherText *big.Int
	for _, log := range mintReceipt.Logs {
		if log.Topics[0] == privateMintSig {
			eventVaultId = log.Topics[1].Big()
			eventCommitment = log.Topics[2].Big()
			eventCipherText = log.Topics[3].Big()
			t.Logf("  PrivateMint event: vaultId=%s commitment=%s cipherText=%s",
				eventVaultId, eventCommitment, eventCipherText)
		}
	}
	if eventCommitment == nil {
		t.Fatalf("PrivateMint event not found in transaction receipt")
	}

	if eventVaultId.Cmp(vaultId) != 0 {
		t.Errorf("event vaultId: got %s, want %s", eventVaultId, vaultId)
	}
	if eventCommitment.Cmp(mintResult.Commitment()) != 0 {
		t.Errorf("event commitment: got %s, want %s", eventCommitment, mintResult.Commitment())
	}
	if eventCipherText.Cmp(mintResult.CipherText()) != 0 {
		t.Errorf("event cipherText: got %s, want %s", eventCipherText, mintResult.CipherText())
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Step 4 — Alice confirms the note is hers
	//   The event carries only the commitment and the tag, neither of which
	//   reveals the salt or the amount: the Issuer sends Alice both offchain.
	//   With them she checks the tag, Poseidon(pk_alice, salt, contractAddress),
	//   and the commitment round-trip.
	// ─────────────────────────────────────────────────────────────────────────

	// Alice recomputes the commitment to confirm spend-readiness:
	//   Poseidon4(pk_alice, salt, amount, tokenId) == commitment
	recomputedCommitment, err := core.Erc20CommitmentV2(
		aliceSpend.PublicKey, issuedSalt, mintAmount, tokenId,
	)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 recompute: %v", err)
	}
	if recomputedCommitment.Cmp(eventCommitment) != 0 {
		t.Errorf("Alice commitment round-trip failed: got %s, want %s",
			recomputedCommitment, eventCommitment)
	}

	// Alice checks the tag against her key, the salt and this deployment:
	//   Poseidon(pk_alice, salt, contractAddress) == cipherText from event
	tag, err := poseidon.Hash([]*big.Int{aliceSpend.PublicKey, issuedSalt, contractAddressBig})
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	if tag.Cmp(eventCipherText) != 0 {
		t.Errorf("Alice tag mismatch: computed %s, onchain %s", tag, eventCipherText)
	}

	t.Logf("Step 4 — Alice confirmed: note is hers, commitment=%s salt=%s amount=%s",
		eventCommitment, issuedSalt, mintAmount)
	t.Logf("Step 4 — Note is spend-ready as WtSaltsIn=%s WtValuesIn=%s", issuedSalt, mintAmount)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 5 — The same proof cannot mint the note again, in this vault or any
	//   other: EnygmaDvp keeps every commitment it has minted.
	// ─────────────────────────────────────────────────────────────────────────
	estimate := *auth
	estimate.GasLimit = 0 // estimate, so a revert comes back with its reason
	_, err = dvp.Transact(&estimate, "privateMint", vaultId, mintResult.Commitment(), proof)
	if err == nil || !strings.Contains(err.Error(), "CommitmentAlreadyMinted") {
		t.Fatalf("second privateMint of commitment %s: err = %v, want CommitmentAlreadyMinted", mintResult.Commitment(), err)
	}
	t.Logf("Step 5 — a second mint of the same proof was refused: CommitmentAlreadyMinted")
}

// registerMintOnlyVault deploys a token and an Erc20CoinVault, registers the
// vault with the EnygmaDvp at dvpAddr and flags it mintable while it is empty,
// which closes it to deposits for good. It returns the vault's id.
func registerMintOnlyVault(t *testing.T, client *ethclient.Client, auth *bind.TransactOpts, dvp *bind.BoundContract, dvpAddr common.Address) *big.Int {
	t.Helper()
	tokenABI, tokenBin := loadArtifact(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	tokenAddr, tx, _, err := bind.DeployContract(auth, tokenABI, tokenBin, client, "Minted", "MNT")
	waitOK(t, client, "deploy the mint vault's token", tx, err)
	vaultABI, vaultBin := loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	vaultAddr, tx, _, err := bind.DeployContract(auth, vaultABI, vaultBin, client, dvpAddr)
	waitOK(t, client, "deploy the mint vault", tx, err)
	tx, err = dvp.Transact(auth, "registerVault", vaultAddr, tokenAddr, big.NewInt(1), big.NewInt(8))
	waitOK(t, client, "registerVault", tx, err)

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	var out []interface{}
	if err := vault.Call(&bind.CallOpts{}, &out, "getVaultId"); err != nil {
		t.Fatalf("getVaultId: %v", err)
	}
	vaultId := out[0].(*big.Int)
	tx, err = dvp.Transact(auth, "setVaultMintable", vaultId, true)
	waitOK(t, client, "setVaultMintable", tx, err)
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
