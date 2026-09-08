package tests

// Tests for the Finding 2 fix in EnygmaAuction.sol (declareWinner access
// control + restored settlement checks).
//
// SCOPE: real end-to-end auction testing (newAuction -> submitBid ->
// opening -> declareWinner) needs a valid ZK proof for the auction
// circuits (VK_ID_AUCTION_INIT_AUDITOR, VK_ID_AUCTION_BID_AUDITOR, etc.).
// The gnark server in this repo has NO route for auction-circuit proof
// generation at all (only /proof/privateMint, /proof/dvpInitiator,
// /proof/dvpDestination exist) — so a real proof-driven test isn't
// currently possible here. EnygmaAuction also isn't part of the standard
// deploy.go/init.go pipeline (it's a separate, not-yet-wired-in contract),
// so these tests deploy and wire up a standalone instance themselves.
//
// What IS directly testable without any proof: the two checks fixed here
// both fire before any proof is ever involved —
//   1. the onlyRole(DEFAULT_OWNER_ROLE) modifier (fires before the function
//      body runs at all)
//   2. the winning-bid-state check (fires immediately after computing
//      winningBlindedBid, before any proof verification)
// The third fix (the not-winning-bids count check) requires driving a real
// bid through submitBid -> opening first, which needs the unavailable
// proof generation — not covered here.

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// hardhatAccount1PrivKeyHex is Account[1] from the well-known local-dev
// mnemonic in hardhat.config.js ("federal unhappy avoid ... link",
// path m/44'/60'/0'/0/1) — derived once and hardcoded here the same way
// hardhatPrivKeyHex (Account[0]) already is in helpers_test.go. Address
// 0xD2C3b34Abae5664986C8cf0F14d1D434Ac894768 matches hardhatRecipientHex.
const hardhatAccount1PrivKeyHex = "69b5623bd1cfe22983c8849d155ca641238c18ab1b2e34c5ae943ed2ce4716b7"

// loadArtifactABIAndBytecode reads a Hardhat artifact and returns both its
// parsed ABI and creation bytecode (loadOnchainABI in helpers_test.go only
// returns the ABI, which isn't enough to deploy a fresh instance).
func loadArtifactABIAndBytecode(t *testing.T, artifactRelPath string) (abi.ABI, []byte) {
	t.Helper()
	data, err := os.ReadFile("../artifacts/contracts/" + artifactRelPath)
	if err != nil {
		t.Fatalf("read artifact %s: %v", artifactRelPath, err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact JSON: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return parsed, common.FromHex(artifact.Bytecode)
}

func TestEnygmaAuction_DeclareWinner_AccessControlAndStateChecks(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("dial hardhat: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	poseidonWrapper, ok := receipts["PoseidonWrapper"]
	if !ok {
		t.Fatal("PoseidonWrapper not found in build/receipts.json — run deploy.sh first")
	}

	ownerAuth := hardhatAuth(t, client) // Account[0]
	ownerPriv, _ := crypto.HexToECDSA("34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154")
	ownerAddr := crypto.PubkeyToAddress(ownerPriv.PublicKey)

	nonOwnerPriv, err := crypto.HexToECDSA(hardhatAccount1PrivKeyHex)
	if err != nil {
		t.Fatalf("parse account[1] key: %v", err)
	}
	nonOwnerAuth, err := bind.NewKeyedTransactorWithChainID(nonOwnerPriv, big.NewInt(hardhatChainID))
	if err != nil {
		t.Fatalf("NewKeyedTransactorWithChainID: %v", err)
	}
	nonOwnerAuth.GasLimit = 6_000_000

	auctionABI, auctionBytecode := loadArtifactABIAndBytecode(t, "core/contracts/EnygmaAuction.sol/EnygmaAuction.json")

	// Deploy a standalone EnygmaAuction with account[0] passed as its own
	// "enygmaDvpContractAddress" — this grants account[0] BOTH
	// DEFAULT_OWNER_ROLE (constructor always grants this to msg.sender) AND
	// DEFAULT_DVP_ROLE (granted to whatever address is passed in), so
	// account[0] can then call the DVP-role-gated initializeEnygmaAuction()
	// itself without needing a real EnygmaDvp contract wired up. Same
	// pattern used for the Echidna harness elsewhere in this branch.
	auctionAddr, tx, _, err := bind.DeployContract(ownerAuth, auctionABI, auctionBytecode, client, ownerAddr)
	if err != nil {
		t.Fatalf("deploy EnygmaAuction: %v", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, tx); err != nil {
		t.Fatalf("wait deploy mined: %v", err)
	}

	auction := bind.NewBoundContract(auctionAddr, auctionABI, client, client, client)

	// verifierContractAddress is never dialed by either test below (both
	// revert before any verifyProof call), so any placeholder address works.
	placeholderVerifier := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	poseidonWrapperAddr := common.HexToAddress(poseidonWrapper.ContractAddress)
	initTx, err := auction.Transact(ownerAuth, "initializeEnygmaAuction", poseidonWrapperAddr, placeholderVerifier)
	if err != nil {
		t.Fatalf("initializeEnygmaAuction: %v", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, initTx); err != nil {
		t.Fatalf("wait init mined: %v", err)
	}

	emptyProofs := []onchainProofReceipt{}

	t.Run("NonOwnerCannotDeclareWinner", func(t *testing.T) {
		callData, err := auctionABI.Pack("declareWinner", big.NewInt(1), big.NewInt(999), big.NewInt(999), emptyProofs)
		if err != nil {
			t.Fatalf("pack declareWinner: %v", err)
		}
		_, err = client.CallContract(context.Background(), ethereum.CallMsg{
			From: nonOwnerAuth.From,
			To:   &auctionAddr,
			Data: callData,
		}, nil)
		if err == nil {
			t.Fatal("expected declareWinner to revert for a non-owner caller, but it succeeded")
		}
		if !strings.Contains(err.Error(), "AccessControl") {
			t.Fatalf("expected an AccessControl revert, got a different error: %v", err)
		}
		t.Logf("non-owner declareWinner correctly reverted: %v", err)
	})

	t.Run("OwnerCannotDeclareWinnerForUnopenedBid", func(t *testing.T) {
		// auctionId/winningBid/winningRandom don't correspond to any real
		// submitted bid, so the computed winningBlindedBid's bidState is
		// the zero value (BID_INACTIVE) — the restored winning-bid-state
		// check must reject this even though the caller IS the owner.
		callData, err := auctionABI.Pack("declareWinner", big.NewInt(1), big.NewInt(999), big.NewInt(999), emptyProofs)
		if err != nil {
			t.Fatalf("pack declareWinner: %v", err)
		}
		_, err = client.CallContract(context.Background(), ethereum.CallMsg{
			From: ownerAuth.From,
			To:   &auctionAddr,
			Data: callData,
		}, nil)
		if err == nil {
			t.Fatal("expected declareWinner to revert for a never-opened bid, but it succeeded")
		}
		if !strings.Contains(err.Error(), "WinningBidOpeningMismatch") {
			t.Fatalf("expected a WinningBidOpeningMismatch revert, got a different error: %v", err)
		}
		t.Logf("owner declareWinner on unopened bid correctly reverted: %v", err)
	})
}
