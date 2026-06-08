package config

type Config struct {
	Port              string
	PaymentPk         string
	PaymentVk         string
	Payment2inPk      string
	Payment2inVk      string
	PrivateMintPk     string
	PrivateMintVk     string
}

func Load() *Config {
	return &Config{
		Port:          "8082",
		PaymentPk:     "./scripts/keys/PaymentPK.key",    // Design for NINPUTS=1;NOUTPUTS=2;
		PaymentVk:     "./scripts/keys/PaymentVK.key",
		Payment2inPk:  "./scripts/keys/Payment2inPK.key", // Design for NINPUTS=2;NOUTPUTS=2;
		Payment2inVk:  "./scripts/keys/Payment2inVK.key",
		PrivateMintPk: "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk: "./scripts/keys/PrivateMintVK.key",
	}
}
