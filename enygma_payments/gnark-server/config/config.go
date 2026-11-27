package config

import "os"

type Config struct {
	// Plain HTTP is restricted to loopback. Other binds require mutual TLS.
	BindHost string
	Port     string

	AllowRemote bool
	TLSCert     string
	TLSKey      string
	TLSClientCA string

	EnygmaPk    string
	EnygmaVk    string
	EnygmaFeePk string
	EnygmaFeeVk string
	UsdrPk      string
	UsdrVk      string
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
		AllowRemote: os.Getenv("GNARK_ALLOW_REMOTE") == "1",
		TLSCert:     os.Getenv("GNARK_TLS_CERT"),
		TLSKey:      os.Getenv("GNARK_TLS_KEY"),
		TLSClientCA: os.Getenv("GNARK_TLS_CLIENT_CA"),
		EnygmaPk:    "./keys/EnygmaPk.key",
		EnygmaVk:    "./keys/EnygmaVk.key",
		EnygmaFeePk: "./keys/EnygmaFeePk.key",
		EnygmaFeeVk: "./keys/EnygmaFeeVk.key",
		UsdrPk:      "./keys/UsdrPk.key",
		UsdrVk:      "./keys/UsdrVk.key",
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
