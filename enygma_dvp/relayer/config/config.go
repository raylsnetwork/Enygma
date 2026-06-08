package config

import (
	"fmt"
	"math/big"
	"os"
	"strings"
)

// Config holds all relayer configuration loaded from environment variables.
type Config struct {
	// Ethereum RPC endpoint — defaults to localhost:8545 (Hardhat).
	RPCURL string
	// ChainID — used for EIP-155 transaction signing.
	ChainID *big.Int
	// RelayerPrivateKeyHex — hex-encoded ECDSA private key (no 0x prefix).
	// WARNING: In production, use a secrets manager. Never commit this value.
	RelayerPrivateKeyHex string
	// APIKey — Bearer token required on all /relay/* routes.
	// WARNING: In production, use a secrets manager. Never commit this value.
	APIKey string
	// EnygmaDvpAddr — address of the deployed EnygmaDvp contract.
	// If empty, read from ReceiptsPath.
	EnygmaDvpAddr string
	// ReceiptsPath — path to build/receipts.json.
	ReceiptsPath string
	// Port — HTTP listen port. Defaults to 8091.
	Port string
}

func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getenv("RELAYER_RPC_URL", "http://localhost:8545"),
		RelayerPrivateKeyHex: strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
		APIKey:               getenv("RELAYER_API_KEY", ""),
		EnygmaDvpAddr:        getenv("RELAYER_DVP_ADDR", ""),
		ReceiptsPath:         getenv("RELAYER_RECEIPTS_PATH", "../build/receipts.json"),
		Port:                 getenv("RELAYER_PORT", "8091"),
	}

	chainIDStr := getenv("RELAYER_CHAIN_ID", "1337")
	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid RELAYER_CHAIN_ID: %q", chainIDStr)
	}
	cfg.ChainID = chainID

	if cfg.RelayerPrivateKeyHex == "" {
		return nil, fmt.Errorf("RELAYER_PRIVATE_KEY must be set")
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
