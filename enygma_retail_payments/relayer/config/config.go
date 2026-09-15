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
	// UsdrVaultAddr — address of the deployed USDr Erc20CoinVault contract
	// (a second, independent vault registered on the same EnygmaDvp — see
	// EnygmaDvp.paymentWithUsdrFee). Used the same way as Erc20VaultAddr,
	// but for the USDr leg of POST /relay/payment_usdr_fee.
	UsdrVaultAddr string
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
	// RelayerFeeSpendPrivateKey — the relayer's own BabyJubJub spend private key
	// (decimal, positive), used ONLY to verify off-chain that a relayer-fee note
	// (PaymentRelayerFeePublic circuit) is actually addressed to this relayer
	// before it's submitted on-chain. Independent of RelayerPrivateKeyHex (which
	// signs Ethereum transactions) — this is a Poseidon-scheme key, matching the
	// scheme NewSpendKeyPair uses for regular payment notes.
	// Optional: stays nil if unset, in which case POST /relay/payment_relayer_fee
	// always returns 503.
	RelayerFeeSpendPrivateKey *big.Int
	// MinFee — minimum acceptable relayer fee (StFee), in token base units.
	// POST /relay/payment_relayer_fee rejects proofs whose fee is below this
	// floor. Defaults to 0 (no floor).
	MinFee *big.Int
}

func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getenv("RELAYER_RPC_URL", "http://localhost:8545"),
		RelayerPrivateKeyHex: strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
		APIKey:               getenv("RELAYER_API_KEY", ""),
		EnygmaDvpAddr:        getenv("RELAYER_DVP_ADDR", ""),
		Erc20VaultAddr:       getenv("RELAYER_ERC20_VAULT_ADDR", ""),
		UsdrVaultAddr:        getenv("RELAYER_USDR_VAULT_ADDR", ""),
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

	// RELAYER_FEE_SPEND_PRIVATE_KEY is optional — /relay/payment_relayer_fee
	// is simply unavailable (503) if it's not configured.
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


