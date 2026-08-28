package config

import (
	"fmt"
	"log"
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

	// APIKeys maps each Bearer token to the identifier of the bank it was
	// issued to. Every /relay/* request's Authorization: Bearer <token>
	// header is looked up here; the matching bank identifier is attached to
	// the request's logs and, in the future, per-bank rate limiting.
	//
	// Fix H-06: this used to be a single shared token (APIKey string) — every
	// bank presented the identical string, so the relayer could not
	// attribute a request to a bank (no per-caller accounting, no audit
	// trail, and revoking one bank's access meant rotating the token and
	// locking out every other bank simultaneously). Per-bank tokens let one
	// compromised or retired credential be revoked — simply drop its entry —
	// without touching anyone else's.
	// WARNING: In production, load these from a secrets manager — never commit them.
	APIKeys map[string]string

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
	// Default: 0, meaning "let go-ethereum's bind package estimate it via
	// eth_estimateGas".
	//
	// Fix H-10 (mechanism 1 of 4): a nonzero GasLimit here makes
	// bind.BoundContract skip eth_estimateGas entirely (see
	// go-ethereum/accounts/abi/bind/base.go: `if opts.GasLimit == 0 {
	// estimateGasLimit(...) }`) — which is itself a full simulation against
	// current chain state. Skipping it meant a payload that was guaranteed
	// to revert on-chain (bad proof, stale block number, wrong participant
	// set, ...) still got signed and broadcast, burning the intrinsic gas
	// of a potentially large calldata blob before the revert. Leaving
	// GasLimit at 0 restores that simulation as a free pre-flight check:
	// eth_estimateGas fails locally, with no broadcast and no gas spent,
	// for exactly the same payloads that would have reverted on-chain.
	// An explicit override is still honored (RELAYER_GAS_LIMIT != 0) for
	// chains where gas estimation is known to be unreliable — that is a
	// deliberate, logged opt-out of the pre-flight check, not the default.
	GasLimit uint64
}

// Load reads configuration from environment variables with sensible defaults.
//
// Required env vars (no defaults):
//   - RELAYER_PRIVATE_KEY
//   - RELAYER_API_KEYS (or the deprecated single-token RELAYER_API_KEY)
//
// All other vars are optional and fall back to localhost/local-path defaults
// suitable for running against a local Hardhat node.
func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getenv("RELAYER_RPC_URL", "http://127.0.0.1:8545"),
		RelayerPrivateKeyHex: strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
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

	// Parse gas limit. Default 0 = auto-estimate (see GasLimit doc above).
	gasLimitStr := getenv("RELAYER_GAS_LIMIT", "0")
	gasLimit, ok := new(big.Int).SetString(gasLimitStr, 10)
	if !ok || !gasLimit.IsUint64() {
		return nil, fmt.Errorf("RELAYER_GAS_LIMIT: invalid uint64 %q", gasLimitStr)
	}
	cfg.GasLimit = gasLimit.Uint64()
	if cfg.GasLimit != 0 {
		log.Printf("config: RELAYER_GAS_LIMIT=%d set explicitly — this disables go-ethereum's "+
			"eth_estimateGas pre-flight simulation (Fix H-10), so guaranteed-revert payloads will "+
			"be signed and broadcast instead of rejected locally. Unset it to auto-estimate.",
			cfg.GasLimit)
	}

	// Parse API keys. RELAYER_API_KEYS is "bankId:token,bankId:token,...".
	// The deprecated single-token RELAYER_API_KEY is still accepted as a
	// fallback (mapped to bank id "default") so existing deployments keep
	// working, but it re-introduces H-06's "one token for every bank"
	// problem and logs a warning every time the process starts.
	apiKeys, err := parseAPIKeys(getenv("RELAYER_API_KEYS", ""))
	if err != nil {
		return nil, fmt.Errorf("RELAYER_API_KEYS: %w", err)
	}
	if len(apiKeys) == 0 {
		if legacy := getenv("RELAYER_API_KEY", ""); legacy != "" {
			log.Printf("config: RELAYER_API_KEY is deprecated (Fix H-06) — it grants every bank " +
				"the same credential with no per-bank attribution or independent revocation. " +
				"Set RELAYER_API_KEYS=\"bankId:token,...\" instead.")
			apiKeys = map[string]string{legacy: "default"}
		}
	}
	cfg.APIKeys = apiKeys

	// Validate required fields.
	if cfg.RelayerPrivateKeyHex == "" {
		return nil, fmt.Errorf("RELAYER_PRIVATE_KEY must be set (hex-encoded, no 0x prefix)")
	}
	if len(cfg.APIKeys) == 0 {
		return nil, fmt.Errorf("RELAYER_API_KEYS must be set (\"bankId:token,...\"), or the deprecated RELAYER_API_KEY")
	}

	return cfg, nil
}

// parseAPIKeys parses "bankId:token,bankId:token,..." into a token -> bankId
// map (the lookup direction bearerAuth actually needs). Empty input returns
// an empty, non-nil map. A token containing ':' is fine — SplitN(2) stops at
// the first separator, so the bank id is everything before it.
func parseAPIKeys(raw string) (map[string]string, error) {
	out := map[string]string{}
	if raw == "" {
		return out, nil
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("malformed entry %q, expected \"bankId:token\"", entry)
		}
		bankID, token := parts[0], parts[1]
		if existing, ok := out[token]; ok {
			return nil, fmt.Errorf("token for bank %q is already assigned to bank %q — tokens must be unique", bankID, existing)
		}
		out[token] = bankID
	}
	return out, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
