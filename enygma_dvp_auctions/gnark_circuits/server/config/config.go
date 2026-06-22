package config

import (
	"encoding/json"
	"os"
)

// Config holds file paths for the six auction circuit proving/verifying keys.
type Config struct {
	AuctionLockPk     string `json:"auctionLockPk"`
	AuctionLockVk     string `json:"auctionLockVk"`
	AuctionBidPk      string `json:"auctionBidPk"`
	AuctionBidVk      string `json:"auctionBidVk"`
	AuctionBatchPk    string `json:"auctionBatchPk"`
	AuctionBatchVk    string `json:"auctionBatchVk"`
	AuctionFinalPk    string `json:"auctionFinalPk"`
	AuctionFinalVk    string `json:"auctionFinalVk"`
	AuctionRevertPk   string `json:"auctionRevertPk"`
	AuctionRevertVk   string `json:"auctionRevertVk"`
	AuctionWithdrawPk string `json:"auctionWithdrawPk"`
	AuctionWithdrawVk string `json:"auctionWithdrawVk"`
}

// Load reads a JSON config file from path. If path is empty it returns defaults
// pointing at ./scripts/keys/.
func Load(path string) (*Config, error) {
	if path == "" {
		return defaults(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaults() *Config {
	return &Config{
		AuctionLockPk:  "./scripts/keys/AuctionLock.pk",
		AuctionLockVk:  "./scripts/keys/AuctionLock.vk",
		AuctionBidPk:   "./scripts/keys/AuctionBid.pk",
		AuctionBidVk:   "./scripts/keys/AuctionBid.vk",
		AuctionBatchPk: "./scripts/keys/AuctionBatch.pk",
		AuctionBatchVk: "./scripts/keys/AuctionBatch.vk",
		AuctionFinalPk:  "./scripts/keys/AuctionFinal.pk",
		AuctionFinalVk:  "./scripts/keys/AuctionFinal.vk",
		AuctionRevertPk:   "./scripts/keys/AuctionRevert.pk",
		AuctionRevertVk:   "./scripts/keys/AuctionRevert.vk",
		AuctionWithdrawPk: "./scripts/keys/AuctionWithdraw.pk",
		AuctionWithdrawVk: "./scripts/keys/AuctionWithdraw.vk",
	}
}
