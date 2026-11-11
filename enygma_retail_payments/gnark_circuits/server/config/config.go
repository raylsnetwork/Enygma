package config

import (
	"os"
	"runtime"
	"strconv"
)

type Config struct {
	// BindHost is the interface the server listens on: loopback unless
	// GNARK_BIND_HOST says otherwise. Proof requests carry the witness
	// (secret keys and blinding factors) in cleartext.
	BindHost string
	// AuthToken, when set (GNARK_AUTH_TOKEN), is the bearer token every
	// request must carry in its Authorization header. Empty means no check.
	AuthToken string
	// MaxConcurrentProofs bounds how many proofs run at once
	// (GNARK_MAX_CONCURRENT_PROOFS, default GOMAXPROCS).
	MaxConcurrentProofs int

	Port                      string
	PaymentPk                 string
	PaymentVk                 string
	Payment2inPk              string
	Payment2inVk              string
	PaymentFeePk              string
	PaymentFeeVk              string
	PaymentRelayerFeePublicPk string
	PaymentRelayerFeePublicVk string
	UsdrFeePk                 string
	UsdrFeeVk                 string
	PrivateMintPk             string
	PrivateMintVk             string
}

func Load() *Config {
	return &Config{
		BindHost:            getenv("GNARK_BIND_HOST", "127.0.0.1"),
		AuthToken:           os.Getenv("GNARK_AUTH_TOKEN"),
		MaxConcurrentProofs: positiveInt(os.Getenv("GNARK_MAX_CONCURRENT_PROOFS"), runtime.GOMAXPROCS(0)),

		Port:                      "8082",
		PaymentPk:                 "./scripts/keys/PaymentPK.key", // 1 input / 2 outputs
		PaymentVk:                 "./scripts/keys/PaymentVK.key",
		Payment2inPk:              "./scripts/keys/Payment2inPK.key", // 2 inputs / 2 outputs
		Payment2inVk:              "./scripts/keys/Payment2inVK.key",
		PaymentFeePk:              "./scripts/keys/PaymentFeePK.key", // 1 input / 2 outputs, fee absorbed into sender's input
		PaymentFeeVk:              "./scripts/keys/PaymentFeeVK.key",
		PaymentRelayerFeePublicPk: "./scripts/keys/PaymentRelayerFeePublicPK.key", // 1 input / 3 outputs (relayer fee note, public fee)
		PaymentRelayerFeePublicVk: "./scripts/keys/PaymentRelayerFeePublicVK.key",
		UsdrFeePk:                 "./scripts/keys/UsdrFeePK.key", // 1 input / 2 outputs, second asset, public token id
		UsdrFeeVk:                 "./scripts/keys/UsdrFeeVK.key",
		PrivateMintPk:             "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk:             "./scripts/keys/PrivateMintVK.key",
	}
}

// getenv returns the environment variable key, or fallback when it is unset or empty.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// positiveInt parses v as a positive integer, or returns fallback.
func positiveInt(v string, fallback int) int {
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return fallback
}
