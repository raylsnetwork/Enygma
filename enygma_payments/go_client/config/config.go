package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	ContractAddress string `json:"address"`
	ProofServerURL  string
	RelayerURL      string
}

// addressFile is the structure of address.json
type addressFile struct {
	Address string `json:"address"`
}

// Load loads configuration from file
func Load(addressFilePath string) (*Config, error) {
	// Read address from JSON file
	address, err := readAddress(addressFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read address: %w", err)
	}

	relayerURL := os.Getenv("RELAYER_URL")
	if relayerURL == "" {
		relayerURL = "http://127.0.0.1:8082"
	}

	return &Config{
		ContractAddress: address,
		ProofServerURL:  "http://127.0.0.1:8080/proof/enygma",
		RelayerURL:      relayerURL,
	}, nil
}

func readAddress(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	var addr addressFile
	if err := json.Unmarshal(data, &addr); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return addr.Address, nil
}