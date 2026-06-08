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
	// The relayer's Ethereum address is derived from this key.
	// WARNING: In production, use a secrets manager. Never commit this value.
	RelayerPrivateKeyHex string
	// APIKey — the Bearer token clients must include in the Authorization header.
	// Every request to /relay/* must carry: Authorization: Bearer <APIKey>
	// WARNING: In production, use a secrets manager. Never commit this value.
	APIKey string
	// EnygmaDvpAddr — address of the deployed EnygmaDvp contract.
	EnygmaDvpAddr string
	// Erc20VaultAddr — address of the deployed Erc20CoinVault contract.
	// Used to check nullifier and root state before relaying.
	Erc20VaultAddr string
	// TagRegistryAddr — address of the deployed TagRegistry contract.
	// Used by POST /relay/tag to publish private messaging tags on-chain.
	TagRegistryAddr string
	// TagChannelRegistryAddr — address of the deployed TagChannelRegistry contract.
	// Used by POST /relay/channel to publish channel setup records on-chain.
	// When routed through the relayer, msg.sender = relayer address (sender privacy).
	TagChannelRegistryAddr string
	// ReceiptsPath — path to build/receipts.json (overrides the addresses above).
	ReceiptsPath string
	// Port — HTTP listen port. Defaults to 8090.
	Port string
}

func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getenv("RELAYER_RPC_URL", "http://localhost:8545"),
		RelayerPrivateKeyHex: strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
		APIKey:               getenv("RELAYER_API_KEY", ""),
		EnygmaDvpAddr:        getenv("RELAYER_DVP_ADDR", ""),
		Erc20VaultAddr:       getenv("RELAYER_ERC20_VAULT_ADDR", ""),
		TagRegistryAddr:        getenv("RELAYER_TAG_REGISTRY_ADDR", ""),
		TagChannelRegistryAddr: getenv("RELAYER_TAG_CHANNEL_REGISTRY_ADDR", ""),
		ReceiptsPath:         getenv("RELAYER_RECEIPTS_PATH", "../build/receipts.json"),
		Port:                 getenv("RELAYER_PORT", "8090"),
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


