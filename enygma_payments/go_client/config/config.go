package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	ContractAddress string `json:"address"`
	CommitChainURL  string
	ProofServerURL  string
	PrivateKey      string
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

	// TODO: Load from environment variables or config file
	return &Config{
		ContractAddress: address,
		CommitChainURL:  "http://127.0.0.1:8545",
		ProofServerURL:  "http://127.0.0.1:8080/proof/enygma",
		PrivateKey:      "Input Private Key",
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