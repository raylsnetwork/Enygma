package tests

import (
	"testing"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

// The auction prover binds 127.0.0.1 only; a "localhost" default can reach a
// [::1] listener first.
func TestNewAuctionClient_DefaultDialsIPv4Loopback(t *testing.T) {
	if got := core.NewAuctionClient("").BaseURL; got != "http://127.0.0.1:8083" {
		t.Fatalf("default BaseURL = %q, want http://127.0.0.1:8083", got)
	}
}

func TestNewAuctionClient_ExplicitURLUsedAsGiven(t *testing.T) {
	const u = "http://localhost:9999"
	if got := core.NewAuctionClient(u).BaseURL; got != u {
		t.Fatalf("BaseURL = %q, want %q", got, u)
	}
}
