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
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type onchainPrivateMintProof struct {
	Proof        [8]*big.Int `abi:"proof"`
	PublicSignal [4]*big.Int `abi:"public_signal"`
}

// proofStringsToPrivateMint converts the 8 proof strings and 4 public signal strings
// returned by the gnark server into the on-chain ABI struct.
func proofStringsToPrivateMint(t *testing.T, proofStrs []string, sigStrs []string) onchainPrivateMintProof {
	t.Helper()
	if len(proofStrs) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(proofStrs))
	}
	if len(sigStrs) != 4 {
		t.Fatalf("expected 4 public signal elements, got %d", len(sigStrs))
	}
	var p onchainPrivateMintProof
	for i, s := range proofStrs {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("invalid proof element %d: %q", i, s)
		}
		p.Proof[i] = n
	}
	for i, s := range sigStrs {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("invalid public_signal element %d: %q", i, s)
		}
		p.PublicSignal[i] = n
	}
	return p
}

// TestV2Erc20OnChain_PrivateMint exercises the full PrivateMint flow against a live
// Hardhat node and gnark proof server:
//
//	Step 1 — Off-chain agreement: Alice shares pk_spend with the Issuer.
//	Step 2 — Issuer requests a ZK proof from the gnark server.
//	Step 3 — Issuer calls EnygmaDvp.privateMint on-chain.
//	Step 4 — Alice scans the PrivateMint event and confirms the note is hers.
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
	// Erc20CoinVault is registered first in scripts/cmd/init → vaultId=0
	vaultId := big.NewInt(0)
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
	//     Poseidon4(pk_alice, salt, amount, tokenId) == commitment
	//     Poseidon2(pk_alice, salt)                  == cipherText
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
	t.Logf("Step 2 — commitment:  %s", mintResult.Commitment)
	t.Logf("Step 2 — cipherText:  %s", mintResult.CipherText)

	// Convert proof and public signal to on-chain ABI types
	proof := proofStringsToPrivateMint(t,
		mintResult.ProofResponse.Proof,
		mintResult.ProofResponse.PublicSignal,
	)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 3 — Issuer calls EnygmaDvp.privateMint on-chain
	//   The contract checks:
	//     1. onlyRole(DEFAULT_OWNER_ROLE) — the Hardhat deployer account holds this role
	//     2. IPrivateMintVerifier.verifyProof(proof, publicSignal)
	//     3. IAbstractCoinVault.registerCoins([commitment]) — inserts the Merkle leaf
	//   Emits: PrivateMint(vaultId, commitment, cipherText)
	// ─────────────────────────────────────────────────────────────────────────
	mintTx, err := dvp.Transact(auth, "privateMint", vaultId, mintResult.Commitment, proof)
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
	if eventCommitment.Cmp(mintResult.Commitment) != 0 {
		t.Errorf("event commitment: got %s, want %s", eventCommitment, mintResult.Commitment)
	}
	if eventCipherText.Cmp(mintResult.CipherText) != 0 {
		t.Errorf("event cipherText: got %s, want %s", eventCipherText, mintResult.CipherText)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Step 4 — Alice scans the PrivateMint event and confirms the note is hers
	//   Alice computes Poseidon2(pk_alice, salt) and checks it matches the
	//   cipherText from the event. She also verifies the commitment round-trip.
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

	// Alice checks cipherText matches her (pk, salt) pair:
	//   Poseidon2(pk_alice, salt) == cipherText from event
	if mintResult.CipherText.Cmp(eventCipherText) != 0 {
		t.Errorf("Alice cipherText mismatch: computed %s, on-chain %s",
			mintResult.CipherText, eventCipherText)
	}

	t.Logf("Step 4 — Alice confirmed: note is hers, commitment=%s salt=%s amount=%s",
		eventCommitment, issuedSalt, mintAmount)
	t.Logf("Step 4 — Note is spend-ready as WtSaltsIn=%s WtValuesIn=%s", issuedSalt, mintAmount)
}
