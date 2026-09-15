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
	// RelayerFeeSpendPrivateKey — the relayer's own BabyJubJub spend private key
	// (decimal, positive), used ONLY to verify off-chain that a relayer-fee note
	// (PaymentRelayerFeePublic or UsdrFee circuit) is actually addressed to this
	// relayer before it's submitted on-chain. Independent of RelayerPrivateKeyHex
	// (which signs Ethereum transactions) — this is a Poseidon-scheme key,
	// matching the scheme SpendKeyPair uses for regular payment notes.
	// Optional: stays nil if unset, in which case the fee-relay routes always
	// return 503.
	RelayerFeeSpendPrivateKey *big.Int
	// MinFee — minimum acceptable relayer fee (StFee), in token base units.
	// The fee-relay routes reject proofs whose fee is below this floor.
	// Defaults to 0 (no floor).
	MinFee *big.Int
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

	// RELAYER_FEE_SPEND_PRIVATE_KEY is optional — the fee-relay routes are
	// simply unavailable (503) if it's not configured.
	if feeKeyStr := getenv("RELAYER_FEE_SPEND_PRIVATE_KEY", ""); feeKeyStr != "" {
		feeKey, ok := new(big.Int).SetString(feeKeyStr, 10)
		if !ok || feeKey.Sign() <= 0 {
			return nil, fmt.Errorf("invalid RELAYER_FEE_SPEND_PRIVATE_KEY: must be a positive decimal integer")
		}
		cfg.RelayerFeeSpendPrivateKey = feeKey
	}

	minFeeStr := getenv("RELAYER_MIN_FEE", "0")
	minFee, ok := new(big.Int).SetString(minFeeStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid RELAYER_MIN_FEE: %q", minFeeStr)
	}
	cfg.MinFee = minFee

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
