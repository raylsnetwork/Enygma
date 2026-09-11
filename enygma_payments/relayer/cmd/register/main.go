// register is a one-time setup tool that maps the relayer's Ethereum address
// to a participant accountId in the deployed Enygma contract.
//
// The Enygma contract requires msg.sender to have a non-zero accountId (via
// addressToAccountId[msg.sender] != 0) before it will accept Deposit, Withdraw,
// or Transfer calls. Run this once after deployment, before starting the relayer.
//
// As of the USDr fee-proof feature, the relayer must also be a genuine
// k=6 anonymity-set participant (see Enygma.sol's transfer(): the USDr
// proof shares the same participantIds as the main transfer proof), not
// just an identity-check bypass — so this tool now derives and registers
// a REAL spend key (publicKey = Poseidon(sk,sk) mod P, the same derivation
// every other account uses) instead of the previous dummy publicKey=1.
// It also initializes the relayer's USDr balance via
// initializeUsdrBalance(), required before checkUsdr() will pass for it.
//
// Note: viewKey stays empty — it's write-only on-chain (used for off-chain
// note discovery in note-based systems), never read by any circuit or
// contract check here. The FingerPrint/SharedSecrets/MessageTags values a
// sender uses for the relayer's slot are entirely prover-chosen and never
// independently verified against the relayer's identity on-chain (only
// PublicKey/PreviousCommit/TxCommit are cross-checked) — so no real
// ML-KEM key agreement with the relayer is needed for a valid proof;
// callers building a witness that includes the relayer can use any
// self-consistent placeholder for that slot (matching how demo/main.go
// already falls back to demoDefaults[i] for banks without a run
// key-agreement step).
//
// Usage:
//
//	OWNER_PRIVATE_KEY=<hex>   \
//	RELAYER_PRIVATE_KEY=<hex> \
//	RELAYER_SPEND_KEY=<decimal secret scalar> \
//	go run ./cmd/register --account-id 6
//
// Required env vars:
//
//	OWNER_PRIVATE_KEY   — hex ECDSA key authorised to call registerAccount
//	RELAYER_PRIVATE_KEY — hex ECDSA key whose address will be registered
//	RELAYER_SPEND_KEY   — decimal secret scalar (sk); publicKey is derived
//	                      as Poseidon(sk,sk) mod P. Pick any nonzero value
//	                      for local/demo use — never reuse a production key.
//
// Optional env vars (same defaults as the relayer server):
//
//	RELAYER_RPC_URL       — default http://127.0.0.1:8545
//	RELAYER_CHAIN_ID      — default 1337
//	RELAYER_CONTRACT_ADDR — explicit contract address; overrides address.json
//	RELAYER_ADDRESS_JSON  — default ../go_client/address.json
//	RELAYER_RANDOMNESS       — decimal randomness for the main-balance
//	                           initial commitment; default: derived from
//	                           RELAYER_SPEND_KEY (not a real random source —
//	                           fine for local/demo use only)
//	RELAYER_USDR_RANDOMNESS  — decimal randomness for the USDr initial
//	                           commitment; same default derivation, offset
//	                           so it's distinct from RELAYER_RANDOMNESS
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	enygma "enygma_payments_relayer/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// curveP is the Baby Jubjub subgroup order — same constant used throughout
// this project (circuits, demo, tests) for mod-reducing derived values.
var curveP, _ = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

func main() {
	accountID := flag.Int64("account-id", 100,
		"accountId to register the relayer under (must be non-zero, unique across all participants)")
	flag.Parse()

	rpcURL := envOr("RELAYER_RPC_URL", "http://127.0.0.1:8545")
	chainIDStr := envOr("RELAYER_CHAIN_ID", "1337")
	contractAddrStr := os.Getenv("RELAYER_CONTRACT_ADDR")
	addressJSON := envOr("RELAYER_ADDRESS_JSON", "../go_client/address.json")

	ownerKeyHex := requireEnv("OWNER_PRIVATE_KEY")
	relayerKeyHex := strings.TrimPrefix(requireEnv("RELAYER_PRIVATE_KEY"), "0x")

	spendKeyStr := requireEnv("RELAYER_SPEND_KEY")
	sk, ok := new(big.Int).SetString(spendKeyStr, 10)
	if !ok {
		log.Fatalf("invalid RELAYER_SPEND_KEY: %q (expected a decimal integer)", spendKeyStr)
	}

	randomnessStr := os.Getenv("RELAYER_RANDOMNESS")
	var randomness *big.Int
	if randomnessStr != "" {
		randomness, ok = new(big.Int).SetString(randomnessStr, 10)
		if !ok {
			log.Fatalf("invalid RELAYER_RANDOMNESS: %q", randomnessStr)
		}
	} else {
		randomness = new(big.Int).Mod(sk, curveP)
	}

	usdrRandomnessStr := os.Getenv("RELAYER_USDR_RANDOMNESS")
	var usdrRandomness *big.Int
	if usdrRandomnessStr != "" {
		usdrRandomness, ok = new(big.Int).SetString(usdrRandomnessStr, 10)
		if !ok {
			log.Fatalf("invalid RELAYER_USDR_RANDOMNESS: %q", usdrRandomnessStr)
		}
	} else {
		// Offset from randomness so the two initial commitments differ even
		// when RELAYER_RANDOMNESS/RELAYER_USDR_RANDOMNESS are both left at
		// their derived defaults.
		usdrRandomness = new(big.Int).Mod(new(big.Int).Add(randomness, big.NewInt(1)), curveP)
	}

	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		log.Fatalf("invalid RELAYER_CHAIN_ID: %q", chainIDStr)
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial %s: %v", rpcURL, err)
	}
	defer client.Close()

	// Owner key signs the registerAccount call.
	ownerKey, err := crypto.HexToECDSA(strings.TrimPrefix(ownerKeyHex, "0x"))
	if err != nil {
		log.Fatalf("parse OWNER_PRIVATE_KEY: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(ownerKey, chainID)
	if err != nil {
		log.Fatalf("build transactor: %v", err)
	}
	auth.Value = big.NewInt(0)
	auth.GasLimit = 300_000_000

	// Derive the relayer's address from its private key.
	relayerKey, err := crypto.HexToECDSA(relayerKeyHex)
	if err != nil {
		log.Fatalf("parse RELAYER_PRIVATE_KEY: %v", err)
	}
	relayerAddr := crypto.PubkeyToAddress(relayerKey.PublicKey)

	// Resolve the deployed contract address.
	if contractAddrStr == "" {
		contractAddrStr, err = readAddressJSON(addressJSON)
		if err != nil {
			log.Fatalf("resolve contract address: %v", err)
		}
	}

	instance, err := enygma.NewEnygma(common.HexToAddress(contractAddrStr), client)
	if err != nil {
		log.Fatalf("bind contract at %s: %v", contractAddrStr, err)
	}

	// publicKey = Poseidon(sk,sk) mod P — the same derivation every other
	// account uses (see EnygmaCircuit/USDrCircuit's own knowledge-of-secret-
	// key check), not the previous dummy publicKey=1.
	publicKey, err := poseidon.Hash([]*big.Int{sk, sk})
	if err != nil {
		log.Fatalf("derive publicKey: %v", err)
	}
	publicKey.Mod(publicKey, curveP)

	log.Printf("Registering relayer")
	log.Printf("  Address:          %s", relayerAddr.Hex())
	log.Printf("  AccountId:        %d", *accountID)
	log.Printf("  PublicKey:        %s", publicKey.String())
	log.Printf("  Contract:         %s", contractAddrStr)

	tx, err := instance.RegisterAccount(auth, relayerAddr, big.NewInt(*accountID), publicKey, randomness, []byte{})
	if err != nil {
		log.Fatalf("registerAccount(): %v", err)
	}
	log.Printf("  Transaction:      %s", tx.Hash().Hex())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		log.Fatalf("wait mined: %v", err)
	}
	if receipt.Status != 1 {
		log.Fatalf("registerAccount reverted in block %d", receipt.BlockNumber.Uint64())
	}
	log.Printf("  Block:            %d (gas used: %d)", receipt.BlockNumber.Uint64(), receipt.GasUsed)

	// Fresh nonce for the next transaction from the same owner key — auth's
	// Nonce isn't set explicitly (go-ethereum fetches it per-call), so this
	// is safe as long as nothing else races the owner key between the two
	// calls, same assumption the rest of this tool already makes.
	usdrTx, err := instance.InitializeUsdrBalance(auth, big.NewInt(*accountID), usdrRandomness)
	if err != nil {
		log.Fatalf("initializeUsdrBalance(): %v", err)
	}
	log.Printf("  USDr Transaction: %s", usdrTx.Hash().Hex())

	usdrCtx, usdrCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer usdrCancel()
	usdrReceipt, err := bind.WaitMined(usdrCtx, client, usdrTx)
	if err != nil {
		log.Fatalf("wait mined (usdr): %v", err)
	}
	if usdrReceipt.Status != 1 {
		log.Fatalf("initializeUsdrBalance reverted in block %d", usdrReceipt.BlockNumber.Uint64())
	}
	log.Printf("  USDr Block:       %d (gas used: %d)", usdrReceipt.BlockNumber.Uint64(), usdrReceipt.GasUsed)

	log.Printf("Registration successful — relayer is ready to submit transactions and receive USDr fees.")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func readAddressJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var f struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Address == "" {
		return "", fmt.Errorf("%s: address field is empty", path)
	}
	return f.Address, nil
}
