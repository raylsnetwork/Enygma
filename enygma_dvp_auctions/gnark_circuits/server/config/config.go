package config

import (
	"encoding/json"
	"os"
)

// Config holds file paths for the three auction circuit proving/verifying keys.
type Config struct {
	AuctionLockPk       string `json:"auctionLockPk"`
	AuctionLockVk       string `json:"auctionLockVk"`
	AuctionBidPk        string `json:"auctionBidPk"`
	AuctionBidVk        string `json:"auctionBidVk"`
	AuctionSettlePk     string `json:"auctionSettlePk"`
	AuctionSettleVk     string `json:"auctionSettleVk"`
	AuctionNotWinningPk string `json:"auctionNotWinningPk"`
	AuctionNotWinningVk string `json:"auctionNotWinningVk"`
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
		AuctionLockPk:       "./scripts/keys/AuctionLock.pk",
		AuctionLockVk:       "./scripts/keys/AuctionLock.vk",
		AuctionBidPk:        "./scripts/keys/AuctionBid.pk",
		AuctionBidVk:        "./scripts/keys/AuctionBid.vk",
		AuctionSettlePk:     "./scripts/keys/AuctionSettle.pk",
		AuctionSettleVk:     "./scripts/keys/AuctionSettle.vk",
		AuctionNotWinningPk: "./scripts/keys/AuctionNotWinning.pk",
		AuctionNotWinningVk: "./scripts/keys/AuctionNotWinning.vk",
	}
}
