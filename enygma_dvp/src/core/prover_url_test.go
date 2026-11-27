package core

import "testing"

// The DvP prover binds 127.0.0.1 only. A default naming "localhost" can
// resolve to ::1 first (macOS), where another local account's listener would
// receive the witness before the client fell back to 127.0.0.1.
func TestNewGnarkClient_DefaultDialsIPv4Loopback(t *testing.T) {
	if got := NewGnarkClient("").BaseURL; got != "http://127.0.0.1:8081" {
		t.Fatalf("default BaseURL = %q, want http://127.0.0.1:8081", got)
	}
}

func TestNewGnarkClient_ExplicitURLUsedAsGiven(t *testing.T) {
	const u = "http://localhost:9999"
	if got := NewGnarkClient(u).BaseURL; got != u {
		t.Fatalf("BaseURL = %q, want %q", got, u)
	}
}
