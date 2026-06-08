package tests

import (
	"context"
	"encoding/json"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── Hardhat constants ──────────────────────────────────────────────────────────

const (
	hardhatRPC             = "http://localhost:8545"
	hardhatChainID         = 1337
	// account[0] — owner / Alice's Ethereum signing key (demo only)
	hardhatPrivKeyHex      = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
	hardhatAliceAddr       = "0x0F1013e0e46B97144b25b3131668EF99858BD8D0"
	// account[1] — Bob's Ethereum signing key (demo only)
	hardhatBobPrivKeyHex   = "69b5623bd1cfe22983c8849d155ca641238c18ab1b2e34c5ae943ed2ce4716b7"
	hardhatBobAddr         = "0xD2C3b34Abae5664986C8cf0F14d1D434Ac894768"
	// account[2] — Carol / relayer signing key (demo only)
	hardhatCarolPrivKeyHex = "9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda"
	hardhatCarolAddr       = "0x9E0B331577BB37420231DAc6D199FCb4c7092B87"
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

// ── Connectivity helpers ───────────────────────────────────────────────────────

func serverAvailable(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func chainAvailable() bool {
	return serverAvailable("localhost:8545")
}

func mustDial(t *testing.T) *ethclient.Client {
	t.Helper()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	return client
}

// ── Receipt / ABI helpers ──────────────────────────────────────────────────────

type onchainReceiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

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

// loadOnchainABI reads from the flat contracts/abis/<name>.json layout.
func loadOnchainABI(t *testing.T, contractName string) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("../contracts/abis/" + contractName + ".json")
	if err != nil {
		t.Fatalf("read artifact %s: %v", contractName, err)
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

func hardhatAuth(t *testing.T, client *ethclient.Client) *bind.TransactOpts {
	t.Helper()
	return hardhatAuthFromKey(t, hardhatPrivKeyHex)
}

func hardhatBobAuth(t *testing.T, client *ethclient.Client) *bind.TransactOpts {
	t.Helper()
	return hardhatAuthFromKey(t, hardhatBobPrivKeyHex)
}

func hardhatCarolAuth(t *testing.T, client *ethclient.Client) *bind.TransactOpts {
	t.Helper()
	return hardhatAuthFromKey(t, hardhatCarolPrivKeyHex)
}

func bindContract(t *testing.T, client *ethclient.Client, addr common.Address, a abi.ABI) *bind.BoundContract {
	t.Helper()
	return bind.NewBoundContract(addr, a, client, client, client)
}

func hardhatAuthFromKey(t *testing.T, privKeyHex string) *bind.TransactOpts {
	t.Helper()
	privKey, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(hardhatChainID))
	if err != nil {
		t.Fatalf("NewKeyedTransactorWithChainID: %v", err)
	}
	auth.GasLimit = 8_000_000
	return auth
}

// ── Proof conversion helpers ───────────────────────────────────────────────────

// proofStringsToOnchain converts the 8-element decimal proof strings from the
// gnark server into an onchainSnarkProof ready for ABI encoding.
//
// Gnark output order: [Ax, Ay, BX_A1(imag), BX_A0(real), BY_A1(imag), BY_A0(real), Cx, Cy]
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
			X: [2]*big.Int{vals[2], vals[3]},
			Y: [2]*big.Int{vals[4], vals[5]},
		},
		C: onchainG1Point{X: vals[6], Y: vals[7]},
	}
}

// ── Merkle tree helpers ────────────────────────────────────────────────────────

func loadVaultMerkleTree(t *testing.T, client *ethclient.Client, vaultAddr common.Address, merkleDepth int) *rpcore.MerkleTree {
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
	mt := rpcore.NewMerkleTree(merkleDepth)
	for _, log := range logs {
		if len(log.Topics) < 3 {
			continue
		}
		mt.InsertLeaf(log.Topics[2].Big())
	}
	t.Logf("loadVaultMerkleTree: loaded %d commitment leaves from vault %s", len(logs), vaultAddr.Hex())
	return mt
}
