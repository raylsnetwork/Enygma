package config_test

import (
	"testing"

	"enygma_dvp/gnark_circuits/server/config"
)

func TestLoadAPIKey(t *testing.T) {
	t.Setenv("GNARK_API_KEY", "")
	if config.Load().APIKey != "" {
		t.Fatal("authentication must be off by default")
	}

	t.Setenv("GNARK_API_KEY", "test-key")
	if config.Load().APIKey != "test-key" {
		t.Fatal("Load did not read GNARK_API_KEY")
	}
}
