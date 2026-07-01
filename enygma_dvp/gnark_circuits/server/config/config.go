package config

type Config struct {
	Port string

	// APIKey, if non-empty, requires every request to carry a matching
	// X-Gnark-Key header. Set in production to prevent unauthorized use
	// of the proving keys by any process that can reach the server's loopback
	// address.
	APIKey string

	PrivateMintPk string
	PrivateMintVk string

	DvPInitiatorPk   string
	DvPInitiatorVk   string
	DvPDestinationPk string
	DvPDestinationVk string
}

func Load() *Config {
	return &Config{
		Port: "8081",

		PrivateMintPk:    "./scripts/keys/PrivateMintPK.key",
		PrivateMintVk:    "./scripts/keys/PrivateMintVK.key",
		DvPInitiatorPk:   "./scripts/keys/DvPInitiatorPK.key",
		DvPInitiatorVk:   "./scripts/keys/DvPInitiatorVK.key",
		DvPDestinationPk: "./scripts/keys/DvPDestinationPK.key",
		DvPDestinationVk: "./scripts/keys/DvPDestinationVK.key",
	}
}

