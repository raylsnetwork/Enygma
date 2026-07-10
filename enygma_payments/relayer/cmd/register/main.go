// register is a one-time setup tool that maps the relayer's Ethereum address
// to a participant accountId in the deployed Enygma contract.
//
// The Enygma contract requires msg.sender to have a non-zero accountId (via
// addressToAccountId[msg.sender] != 0) before it will accept Deposit, Withdraw,
// or Transfer calls. Run this once after deployment, before starting the relayer.
//
// Usage:
//
//	OWNER_PRIVATE_KEY=<hex>   \
//	RELAYER_PRIVATE_KEY=<hex> \
//	go run ./cmd/register --account-id 100
//
// Required env vars:
//
//	OWNER_PRIVATE_KEY   — hex ECDSA key authorised to call registerAccount
//	RELAYER_PRIVATE_KEY — hex ECDSA key whose address will be registered
//
// Optional env vars (same defaults as the relayer server):
//
//	RELAYER_RPC_URL       — default http://127.0.0.1:8545
//	RELAYER_CHAIN_ID      — default 1337
//	RELAYER_CONTRACT_ADDR — explicit contract address; overrides address.json
//	RELAYER_ADDRESS_JSON  — default ../go_client/address.json
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
)

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

	log.Printf("Registering relayer")
	log.Printf("  Address:          %s", relayerAddr.Hex())
	log.Printf("  AccountId:        %d", *accountID)
	log.Printf("  Contract:         %s", contractAddrStr)

	// publicKey and randomness are dummy values: the relayer is only the
	// msg.sender for on-chain submissions and does not participate in ZK
	// circuits as a bank. The contract only checks accountId != 0.
	tx, err := instance.RegisterAccount(auth, relayerAddr, big.NewInt(*accountID), big.NewInt(1), big.NewInt(0))
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
	log.Printf("Registration successful — relayer is ready to submit transactions.")
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
