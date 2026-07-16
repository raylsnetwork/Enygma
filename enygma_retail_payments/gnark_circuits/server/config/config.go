package config

type Config struct {
	Port                string
	PaymentPk           string
	PaymentVk           string
	Payment2inPk        string
	Payment2inVk        string
	PaymentFeePk        string
	PaymentFeeVk        string
	PaymentRelayerPk    string
	PaymentRelayerVk    string
	PrivateMintPk       string
	PrivateMintVk       string
}

func Load() *Config {
	return &Config{
		Port:             "8082",
		PaymentPk:        "./scripts/keys/PaymentPK.key",         // 1 input / 2 outputs
		PaymentVk:        "./scripts/keys/PaymentVK.key",
		Payment2inPk:     "./scripts/keys/Payment2inPK.key",      // 2 inputs / 2 outputs
		Payment2inVk:     "./scripts/keys/Payment2inVK.key",
		PaymentFeePk:     "./scripts/keys/PaymentFeePK.key",      // 1 input / 2 outputs + protocol fee
		PaymentFeeVk:     "./scripts/keys/PaymentFeeVK.key",
		PaymentRelayerPk: "./scripts/keys/PaymentRelayerPK.key",  // 1 input / 3 outputs (relayer fee note)
		PaymentRelayerVk: "./scripts/keys/PaymentRelayerVK.key",
		PrivateMintPk:    "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk:    "./scripts/keys/PrivateMintVK.key",
	}
}
