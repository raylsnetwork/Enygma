package config

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
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

	// MinFee is the minimum acceptable value at publicSignal[50] (the
	// enygma_fee circuit's public Fee slot) for POST /relay/transfer_fee.
	// Requests with a lower embedded fee are rejected with 402 before any
	// on-chain call is made.
	//
	// Default: 0, which disables enforcement — every fee is accepted. Set
	// RELAYER_MIN_FEE explicitly in production; a relayer that never checks
	// the fee will happily subsidize other people's transactions.
	MinFee *big.Int

	// VKTransferPath / VKFeePath are paths to the Groth16 verifying keys
	// (BN254, gnark's native .key format — the same files the gnark server
	// loads to prove) used for local, in-process proof verification before
	// any network call. Verifying keys are public data, safe to ship with
	// the relayer.
	//
	// Default: relative to the gnark-server directory, assuming the usual
	// enygma_payments/ layout.
	VKTransferPath string
	VKFeePath      string

	// RateLimitRPS / RateLimitBurst bound how fast one caller (identified by
	// its Bearer token) may hit POST /relay/*. Requests over the limit get
	// 429 before touching local verify, the fee check, the dry-run, or the
	// chain — the point is to cap the gas-griefing surface, not just move it
	// one layer down.
	//
	// Default: 5 req/s, burst 10 — generous enough that legitimate traffic
	// never notices, but bounded rather than unlimited by default. Unlike
	// RELAYER_MIN_FEE (which changes economics and defaults off to avoid
	// surprising an operator), a sane rate cap doesn't change anyone's
	// behavior, so it defaults on. 0 disables it.
	RateLimitRPS   float64
	RateLimitBurst int

	// MinBalanceWei is the wallet balance floor GET /health/ready checks
	// against, in wei. 0 disables the balance check (connectivity is still
	// checked) — there's no universally sane default across chains with
	// wildly different gas economics, so this is opt-in.
	MinBalanceWei *big.Int

	// StorePath is where the persistent idempotency store (bbolt-backed)
	// lives on disk. Survives restarts — a crash between broadcasting a
	// transaction and responding no longer means the client has no way to
	// learn what happened to their request.
	//
	// Default: ./relayer_state.db (relative to the working directory the
	// relayer is started from).
	StorePath string

	// DedupStaleness is how long a claimed-but-not-yet-terminal ("pending")
	// idempotency record blocks a retry before it's treated as abandoned
	// (e.g. the relayer crashed mid-flight) and the key becomes claimable
	// again. Should comfortably exceed txTimeout so a slow-but-healthy
	// submission is never mistaken for an abandoned one.
	//
	// Default: 5 minutes.
	DedupStaleness time.Duration
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
		VKTransferPath:       getenv("RELAYER_VK_TRANSFER_PATH", "../gnark-server/keys/EnygmaVk.key"),
		VKFeePath:            getenv("RELAYER_VK_FEE_PATH", "../gnark-server/keys/EnygmaFeeVk.key"),
		StorePath:            getenv("RELAYER_STORE_PATH", "./relayer_state.db"),
	}

	// Parse chain ID.
	chainIDStr := getenv("RELAYER_CHAIN_ID", "1337")
	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		return nil, fmt.Errorf("RELAYER_CHAIN_ID: invalid integer %q", chainIDStr)
	}
	cfg.ChainID = chainID

	// Parse gas limit.
	gasLimitStr := getenv("RELAYER_GAS_LIMIT", "5000000")
	gasLimit, ok := new(big.Int).SetString(gasLimitStr, 10)
	if !ok || !gasLimit.IsUint64() {
		return nil, fmt.Errorf("RELAYER_GAS_LIMIT: invalid uint64 %q", gasLimitStr)
	}
	cfg.GasLimit = gasLimit.Uint64()

	// Parse minimum fee (decimal string, same units as the circuit's Fee signal).
	minFeeStr := getenv("RELAYER_MIN_FEE", "0")
	minFee, ok := new(big.Int).SetString(minFeeStr, 10)
	if !ok || minFee.Sign() < 0 {
		return nil, fmt.Errorf("RELAYER_MIN_FEE: invalid non-negative integer %q", minFeeStr)
	}
	cfg.MinFee = minFee

	// Parse per-caller rate limit.
	rpsStr := getenv("RELAYER_RATE_LIMIT_RPS", "5")
	rps, err := strconv.ParseFloat(rpsStr, 64)
	if err != nil || rps < 0 {
		return nil, fmt.Errorf("RELAYER_RATE_LIMIT_RPS: invalid non-negative number %q", rpsStr)
	}
	cfg.RateLimitRPS = rps

	burstStr := getenv("RELAYER_RATE_LIMIT_BURST", "10")
	burst, err := strconv.Atoi(burstStr)
	if err != nil || burst < 0 {
		return nil, fmt.Errorf("RELAYER_RATE_LIMIT_BURST: invalid non-negative integer %q", burstStr)
	}
	cfg.RateLimitBurst = burst

	// Parse minimum balance floor for readiness (wei).
	minBalStr := getenv("RELAYER_MIN_BALANCE_WEI", "0")
	minBal, ok := new(big.Int).SetString(minBalStr, 10)
	if !ok || minBal.Sign() < 0 {
		return nil, fmt.Errorf("RELAYER_MIN_BALANCE_WEI: invalid non-negative integer %q", minBalStr)
	}
	cfg.MinBalanceWei = minBal

	// Parse dedup staleness window.
	stalenessStr := getenv("RELAYER_DEDUP_STALENESS", "5m")
	staleness, err := time.ParseDuration(stalenessStr)
	if err != nil || staleness <= 0 {
		return nil, fmt.Errorf("RELAYER_DEDUP_STALENESS: invalid positive duration %q", stalenessStr)
	}
	cfg.DedupStaleness = staleness

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
