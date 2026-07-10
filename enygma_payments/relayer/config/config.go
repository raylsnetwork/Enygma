package config

import (
	"fmt"
	"math/big"
	"os"
	"strings"
)

// Config holds all relayer configuration loaded from environment variables.
//
// The relayer is the sole party that holds a private key and submits
// transactions to the blockchain. Banks/clients never touch the key or gas.
type Config struct {
	// RPCURL is the Ethereum-compatible JSON-RPC endpoint.
	// Default: http://127.0.0.1:8545 (Hardhat local node).
	RPCURL string

	// ChainID is used for EIP-155 transaction signing.
	// Default: 1337 (Hardhat). Set to the Rayls chain ID in production.
	ChainID *big.Int

	// RelayerPrivateKeyHex is the hex-encoded ECDSA private key (no 0x prefix).
	// The relayer's Ethereum address is derived from this key.
	// WARNING: In production, load this from a secrets manager — never commit it.
	RelayerPrivateKeyHex string

	// APIKey is the Bearer token all clients must include in every /relay/* request.
	//   Authorization: Bearer <APIKey>
	// WARNING: In production, load this from a secrets manager — never commit it.
	APIKey string

	// ContractAddr is the deployed Enygma contract address.
	// Can be set explicitly or left empty to fall back to reading address.json.
	ContractAddr string

	// AddressJSONPath is the path to address.json written by the deploy script.
	// Only read when ContractAddr is not set via env var.
	// Default: ../go_client/address.json
	AddressJSONPath string

	// ABIPath is the path to the compiled Enygma artifact (Hardhat JSON format).
	// Default: ../contracts/enygma/artifacts/contracts/Enygma.sol/Enygma.json
	ABIPath string

	// Port is the HTTP listen port for the relayer server.
	// Default: 8082
	Port string

	// GasLimit is the gas limit applied to every on-chain submission.
	// Default: 300000000
	GasLimit uint64
}

// Load reads configuration from environment variables with sensible defaults.
//
// Required env vars (no defaults):
//   - RELAYER_PRIVATE_KEY
//   - RELAYER_API_KEY
//
// All other vars are optional and fall back to localhost/local-path defaults
// suitable for running against a local Hardhat node.
func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getenv("RELAYER_RPC_URL", "http://127.0.0.1:8545"),
		RelayerPrivateKeyHex: strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
		APIKey:               getenv("RELAYER_API_KEY", ""),
		ContractAddr:         getenv("RELAYER_CONTRACT_ADDR", ""),
		AddressJSONPath:      getenv("RELAYER_ADDRESS_JSON", "../go_client/address.json"),
		ABIPath:              getenv("RELAYER_ABI_PATH", "../contracts/enygma/artifacts/contracts/Enygma.sol/Enygma.json"),
		Port:                 getenv("RELAYER_PORT", "8082"),
	}

	// Parse chain ID.
	chainIDStr := getenv("RELAYER_CHAIN_ID", "1337")
	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		return nil, fmt.Errorf("RELAYER_CHAIN_ID: invalid integer %q", chainIDStr)
	}
	cfg.ChainID = chainID

	// Parse gas limit.
	gasLimitStr := getenv("RELAYER_GAS_LIMIT", "300000000")
	gasLimit, ok := new(big.Int).SetString(gasLimitStr, 10)
	if !ok || !gasLimit.IsUint64() {
		return nil, fmt.Errorf("RELAYER_GAS_LIMIT: invalid uint64 %q", gasLimitStr)
	}
	cfg.GasLimit = gasLimit.Uint64()

	// Validate required fields.
	if cfg.RelayerPrivateKeyHex == "" {
		return nil, fmt.Errorf("RELAYER_PRIVATE_KEY must be set (hex-encoded, no 0x prefix)")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("RELAYER_API_KEY must be set")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
