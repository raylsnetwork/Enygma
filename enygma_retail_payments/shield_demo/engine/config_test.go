package engine

import "testing"

// The demo's retail prover binds 127.0.0.1 only; a "localhost" GnarkURL can
// reach a [::1] listener first and hand it every hop's witness.
func TestDefaultConfig_GnarkURLDialsIPv4Loopback(t *testing.T) {
	if got := DefaultConfig("").GnarkURL; got != "http://127.0.0.1:8082" {
		t.Fatalf("GnarkURL = %q, want http://127.0.0.1:8082", got)
	}
}
