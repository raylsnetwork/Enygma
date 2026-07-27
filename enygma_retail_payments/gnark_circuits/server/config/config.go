package config

type Config struct {
	Port                      string
	PaymentPk                 string
	PaymentVk                 string
	Payment2inPk              string
	Payment2inVk              string
	PaymentRelayerFeePublicPk string
	PaymentRelayerFeePublicVk string
	PrivateMintPk             string
	PrivateMintVk             string
}

func Load() *Config {
	return &Config{
		Port:             "8082",
		PaymentPk:                 "./scripts/keys/PaymentPK.key",                 // 1 input / 2 outputs
		PaymentVk:                 "./scripts/keys/PaymentVK.key",
		Payment2inPk:              "./scripts/keys/Payment2inPK.key",              // 2 inputs / 2 outputs
		Payment2inVk:              "./scripts/keys/Payment2inVK.key",
		PaymentRelayerFeePublicPk: "./scripts/keys/PaymentRelayerFeePublicPK.key", // 1 input / 3 outputs (relayer fee note, public fee)
		PaymentRelayerFeePublicVk: "./scripts/keys/PaymentRelayerFeePublicVK.key",
		PrivateMintPk:             "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk:             "./scripts/keys/PrivateMintVK.key",
	}
}
