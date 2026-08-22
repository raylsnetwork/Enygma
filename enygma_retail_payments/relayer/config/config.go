package config

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
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

	// MinFee is the minimum acceptable value at publicSignal[8] (the
	// PaymentRelayerFeePublic circuit's public StFee slot) for POST
	// /relay/payment_fee. Requests whose proof commits to a lower fee are
	// rejected with 402 before any on-chain call is made.
	//
	// Default: 0, which disables enforcement — every fee is accepted. Set
	// RELAYER_MIN_FEE explicitly in production; a relayer that never checks
	// the fee sponsors other people's gas for free.
	MinFee *big.Int

	// VKRelayerFeePath is the path to the PaymentRelayerFeePublic circuit's
	// Groth16 verifying key (BN254, gnark's native .key format — the same
	// file gnark_circuits loads to prove) used for local, in-process proof
	// verification before any network call. Verifying keys are public data,
	// safe to ship with the relayer.
	//
	// Default: relative to the gnark_circuits directory, assuming the usual
	// enygma_retail_payments/ layout.
	VKRelayerFeePath string

	// VKPaymentPath is the plain Payment circuit's Groth16 verifying key,
	// used for local, in-process verification on POST /relay/payment —
	// the same role VKRelayerFeePath plays for /relay/payment_fee. Without
	// this, a malformed/forged proof to the fee-free endpoint was only
	// caught by the eth_call dry-run (still correct — checkReceiptConditions
	// reverts on an invalid proof either way — just an RPC round-trip
	// instead of ~1ms of local pairing arithmetic).
	VKPaymentPath string

	// RateLimitRPS / RateLimitBurst bound how fast one caller (identified by
	// its Bearer token) may hit POST /relay/*. Requests over the limit get
	// 429 before touching local verify, the fee check, the dry-run, or the
	// chain — the point is to cap the gas-griefing surface, not just move it
	// one layer down.
	//
	// Default: 5 req/s, burst 10. 0 disables it.
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
	// again. Should comfortably exceed the mined-wait timeout so a
	// slow-but-healthy submission is never mistaken for an abandoned one.
	//
	// Default: 5 minutes.
	DedupStaleness time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:                 getenv("RELAYER_RPC_URL", "http://localhost:8545"),
		RelayerPrivateKeyHex:   strings.TrimPrefix(getenv("RELAYER_PRIVATE_KEY", ""), "0x"),
		APIKey:                 getenv("RELAYER_API_KEY", ""),
		EnygmaDvpAddr:          getenv("RELAYER_DVP_ADDR", ""),
		Erc20VaultAddr:         getenv("RELAYER_ERC20_VAULT_ADDR", ""),
		TagRegistryAddr:        getenv("RELAYER_TAG_REGISTRY_ADDR", ""),
		TagChannelRegistryAddr: getenv("RELAYER_TAG_CHANNEL_REGISTRY_ADDR", ""),
		ReceiptsPath:           getenv("RELAYER_RECEIPTS_PATH", "../build/receipts.json"),
		Port:                   getenv("RELAYER_PORT", "8090"),
		VKRelayerFeePath:       getenv("RELAYER_VK_RELAYER_FEE_PATH", "../gnark_circuits/scripts/keys/PaymentRelayerFeePublicVK.key"),
		VKPaymentPath:          getenv("RELAYER_VK_PAYMENT_PATH", "../gnark_circuits/scripts/keys/PaymentVK.key"),
		StorePath:              getenv("RELAYER_STORE_PATH", "./relayer_state.db"),
	}

	chainIDStr := getenv("RELAYER_CHAIN_ID", "1337")
	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid RELAYER_CHAIN_ID: %q", chainIDStr)
	}
	cfg.ChainID = chainID

	// Parse minimum fee (decimal string, same units as the circuit's StFee signal).
	minFeeStr := getenv("RELAYER_MIN_FEE", "0")
	minFee, ok := new(big.Int).SetString(minFeeStr, 10)
	if !ok || minFee.Sign() < 0 {
		return nil, fmt.Errorf("RELAYER_MIN_FEE: invalid non-negative integer %q", minFeeStr)
	}
	cfg.MinFee = minFee

	// Parse per-caller rate limit.
	rpsStr := getenv("RELAYER_RATE_LIMIT_RPS", "5")
	rps, err := strconv.ParseFloat(rpsStr, 64)
	// ParseFloat accepts "NaN"/"Inf" without error, and NaN fails every
	// ordering comparison (including "< 0"), so the naive check below would
	// silently pass a NaN or infinite rate straight into rate.NewLimiter,
	// where Allow()'s behavior is undefined — most likely silently
	// defeating the rate limit entirely instead of erroring at startup.
	if err != nil || math.IsNaN(rps) || math.IsInf(rps, 0) || rps < 0 {
		return nil, fmt.Errorf("RELAYER_RATE_LIMIT_RPS: invalid non-negative number %q", rpsStr)
	}
	cfg.RateLimitRPS = rps

	burstStr := getenv("RELAYER_RATE_LIMIT_BURST", "10")
	burst, err := strconv.Atoi(burstStr)
	// burst=0 is rejected, not just burst<0: golang.org/x/time/rate.Limiter
	// with a zero burst can never admit a single request (Allow() requires
	// n<=burst, and every request needs n=1), so it wouldn't mean "no burst
	// allowance" as the name might suggest — it would silently 429 every
	// request regardless of RateLimitRPS. Disabling the limiter entirely is
	// already available via RELAYER_RATE_LIMIT_RPS=0; burst has no separate
	// "disabled" value.
	if err != nil || burst < 1 {
		return nil, fmt.Errorf("RELAYER_RATE_LIMIT_BURST: invalid positive integer %q (use RELAYER_RATE_LIMIT_RPS=0 to disable rate limiting)", burstStr)
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
