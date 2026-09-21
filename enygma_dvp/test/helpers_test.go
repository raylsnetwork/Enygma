package tests

import (
	"context"
	"encoding/json"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── Hardhat constants ──────────────────────────────────────────────────────────

const (
	hardhatRPC     = "http://localhost:8545"
	hardhatChainID = 1337
	// Account[0] from enygmadvp.config.json — the deployer and ERC20 owner
	hardhatPrivKeyHex = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
)

// ── ABI struct types mirroring IEnygmaDvp.sol ──────────────────────────────────

type onchainG1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

type onchainG2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

type onchainSnarkProof struct {
	A onchainG1Point `abi:"a"`
	B onchainG2Point `abi:"b"`
	C onchainG1Point `abi:"c"`
}

type onchainProofReceipt struct {
	Proof           onchainSnarkProof `abi:"proof"`
	Statement       []*big.Int        `abi:"statement"`
	NumberOfInputs  *big.Int          `abi:"numberOfInputs"`
	NumberOfOutputs *big.Int          `abi:"numberOfOutputs"`
}

// ── Connectivity helpers ───────────────────────────────────────────────────────

// serverAvailable checks whether something is listening on addr.
func serverAvailable(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// chainAvailable checks whether the Hardhat node is running on localhost:8545.
func chainAvailable() bool {
	return serverAvailable("localhost:8545")
}

// ── Receipt / ABI helpers ──────────────────────────────────────────────────────

type onchainReceiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

// loadOnchainReceipts reads build/receipts.json and returns the contract address map.
func loadOnchainReceipts(t *testing.T) map[string]onchainReceiptEntry {
	t.Helper()
	data, err := os.ReadFile("../build/receipts.json")
	if err != nil {
		t.Fatalf("read build/receipts.json: %v — run deploy+init first", err)
	}
	var r map[string]onchainReceiptEntry
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parse receipts.json: %v", err)
	}
	return r
}

// loadOnchainABI reads a Hardhat artifact JSON and parses the ABI field.
func loadOnchainABI(t *testing.T, artifactRelPath string) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("../artifacts/contracts/" + artifactRelPath)
	if err != nil {
		t.Fatalf("read artifact %s: %v", artifactRelPath, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact JSON: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return parsed
}

// hardhatAuth returns a transactor keyed to Hardhat account[0].
func hardhatAuth(t *testing.T, client *ethclient.Client) *bind.TransactOpts {
	t.Helper()
	privKey, err := crypto.HexToECDSA(hardhatPrivKeyHex)
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(hardhatChainID))
	if err != nil {
		t.Fatalf("NewKeyedTransactorWithChainID: %v", err)
	}
	auth.GasLimit = 6_000_000
	return auth
}

// ── Proof conversion helpers ───────────────────────────────────────────────────

// proofStringsToOnchain converts the 8-element decimal proof strings from the
// gnark server into an onchainSnarkProof ready for ABI encoding.
//
// Gnark handler output order (proofRemix):
//
//	[Ax, Ay, BX_A1(imag), BX_A0(real), BY_A1(imag), BY_A0(real), Cx, Cy]
//
// EIP-197 / IEnygmaDvp G2Point convention: x[0]=imaginary, x[1]=real.
func proofStringsToOnchain(t *testing.T, proof []string) onchainSnarkProof {
	t.Helper()
	if len(proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(proof))
	}
	vals := make([]*big.Int, 8)
	for i, s := range proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("invalid proof element %d: %q", i, s)
		}
		vals[i] = n
	}
	return onchainSnarkProof{
		A: onchainG1Point{X: vals[0], Y: vals[1]},
		B: onchainG2Point{
			X: [2]*big.Int{vals[2], vals[3]}, // [imag (A1), real (A0)]
			Y: [2]*big.Int{vals[4], vals[5]},
		},
		C: onchainG1Point{X: vals[6], Y: vals[7]},
	}
}

// ── Merkle tree helpers ────────────────────────────────────────────────────────

// loadVaultMerkleTree queries all historical Commitment events emitted by the
// vault contract and reconstructs the full Merkle tree, matching on-chain state.
func loadVaultMerkleTree(t *testing.T, client *ethclient.Client, vaultAddr common.Address, merkleDepth int) *core.MerkleTree {
	t.Helper()
	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentSig}},
	}
	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		t.Fatalf("FilterLogs (vault Commitment events): %v", err)
	}
	mt := core.NewMerkleTree(merkleDepth)
	for _, log := range logs {
		if len(log.Topics) < 3 {
			continue
		}
		mt.InsertLeaf(log.Topics[2].Big())
	}
	t.Logf("loadVaultMerkleTree: loaded %d commitment leaves from vault %s", len(logs), vaultAddr.Hex())
	return mt
}
