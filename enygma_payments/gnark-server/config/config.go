package config

import "os"

type Config struct {
	// BindHost is the network interface the proving server listens on.
	// Default: 127.0.0.1 (loopback only).
	//
	// Fix H-04: this used to not exist at all — router.Run(":" + cfg.Port)
	// binds every interface (0.0.0.0-equivalent) unconditionally, with no
	// env var or flag to change it. The request body carries a bank's
	// secret_key (its entire spend authority) and every blinding factor
	// in cleartext, over plain HTTP, to an endpoint with no authentication
	// of any kind (M-08). In the documented single-host topology nothing
	// external reaches it — but the moment an operator does the obvious
	// scaling step (moving the prover to a shared or dedicated host), that
	// secret-dense traffic crosses the network in cleartext with no
	// config knob to prevent it. Defaulting to loopback, with an explicit
	// opt-in (GNARK_BIND_HOST) required for anything else, closes the
	// "no way in the code to do it safely" half of that finding; see
	// cmd/server/main.go for the warning logged when a non-loopback host
	// is configured.
	BindHost string

	// Port is the TCP port the proving server listens on.
	// Default: 8080. Previously hardcoded with no override at all.
	Port string

	EnygmaPk    string
	EnygmaVk    string
	EnygmaFeePk string
	EnygmaFeeVk string
	// Fix M-16: was six independent WithdrawPk1..6/WithdrawVk1..6 pairs —
	// one per constraint-system-identical, redundantly-set-up "split"
	// verifier that never actually varied by split count (see
	// keygen/generate_keys.go's generateKeysZkDvpWithdraw doc comment).
	// Only slot 6 (DEFAULT_SIZE) was ever reachable through Enygma.sol's
	// withdraw(); down to one key pair, named for that same slot.
	WithdrawPk6 string
	WithdrawVk6 string
	DepositPk   string
	DepositVk   string

	BurnPk string
	BurnVk string
}

func Load() *Config {
	return &Config{
		BindHost:    getenv("GNARK_BIND_HOST", "127.0.0.1"),
		Port:        getenv("GNARK_PORT", "8080"),
		EnygmaPk:    "./keys/EnygmaPk.key",
		EnygmaVk:    "./keys/EnygmaVk.key",
		EnygmaFeePk: "./keys/EnygmaFeePk.key",
		EnygmaFeeVk: "./keys/EnygmaFeeVk.key",
		WithdrawPk6: "./keys/zkdvp/WithdrawPk6.key",
		WithdrawVk6: "./keys/zkdvp/WithdrawVk6.key",

		DepositPk: "./keys/zkdvp/DepositPk.key",
		DepositVk: "./keys/zkdvp/DepositVk.key", // Fix L-04: was DepositKVk.key (typo) — harmless while vkPath went unused, fatal now that MustLoadKeys actually loads it

		BurnPk: "./keys/BurnPk.key",
		BurnVk: "./keys/BurnVk.key",
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
