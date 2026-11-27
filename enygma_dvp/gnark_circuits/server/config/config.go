package config

import "os"

type Config struct {
	Port string

	// APIKey, if non-empty, requires every request to carry a matching
	// X-Gnark-Key header. Set GNARK_API_KEY in production to prevent unauthorized use
	// of the proving keys by any process that can reach the server's loopback
	// address.
	APIKey string

	PrivateMintPk string
	PrivateMintVk string

	DvPInitiatorPk   string
	DvPInitiatorVk   string
	DvPDestinationPk string
	DvPDestinationVk string

	// Payment-family circuits — dedicated Payment deployment: retail's keys,
	// which make dvp-keygen copies from enygma_retail_payments.
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
}

func Load() *Config {
	return &Config{
		Port:   "8081",
		APIKey: os.Getenv("GNARK_API_KEY"),

		PrivateMintPk:    "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk:    "./scripts/keys/PrivateMintVK.key",
		DvPInitiatorPk:   "./scripts/keys/DvPInitiatorPK.key",
		DvPInitiatorVk:   "./scripts/keys/DvPInitiatorVK.key",
		DvPDestinationPk: "./scripts/keys/DvPDestinationPK.key",
		DvPDestinationVk: "./scripts/keys/DvPDestinationVK.key",

		PaymentPk:                 "./scripts/keys/PaymentPK.key",
		PaymentVk:                 "./scripts/keys/PaymentVK.key",
		Payment2inPk:              "./scripts/keys/Payment2inPK.key",
		Payment2inVk:              "./scripts/keys/Payment2inVK.key",
		PaymentFeePk:              "./scripts/keys/PaymentFeePK.key",
		PaymentFeeVk:              "./scripts/keys/PaymentFeeVK.key",
		PaymentRelayerFeePublicPk: "./scripts/keys/PaymentRelayerFeePublicPK.key",
		PaymentRelayerFeePublicVk: "./scripts/keys/PaymentRelayerFeePublicVK.key",
		UsdrFeePk:                 "./scripts/keys/UsdrFeePK.key",
		UsdrFeeVk:                 "./scripts/keys/UsdrFeeVK.key",
	}
}
