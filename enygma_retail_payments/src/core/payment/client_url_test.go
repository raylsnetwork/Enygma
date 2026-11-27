package payment

import "testing"

// The retail prover binds 127.0.0.1 only; a "localhost" default can reach a
// [::1] listener first (see enygma_dvp core's TestNewGnarkClient_DefaultDialsIPv4Loopback).
func TestNewPaymentClient_DefaultDialsIPv4Loopback(t *testing.T) {
	if got := NewPaymentClient("").BaseURL; got != "http://127.0.0.1:8082" {
		t.Fatalf("default BaseURL = %q, want http://127.0.0.1:8082", got)
	}
}

func TestNewPaymentClient_ExplicitURLUsedAsGiven(t *testing.T) {
	const u = "http://localhost:9999"
	if got := NewPaymentClient(u).BaseURL; got != u {
		t.Fatalf("BaseURL = %q, want %q", got, u)
	}
}
