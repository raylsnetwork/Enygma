package utils

// Regression tests for the SSRF fix in merkle_status.go: rpcUrl in both
// MerkleStatusRequest and MerkleVaultRequest comes directly from the request
// body, feeds an outbound HTTP request (doRPC), and the fetched result is
// echoed back to the caller — the exact shape that turns SSRF into a way to
// read cloud metadata / internal-only services through this server.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── resolveToSafeIP: the classification decision itself ─────────────────────

func TestResolveToSafeIP_RejectsCloudMetadataAddress(t *testing.T) {
	// 169.254.169.254 is the metadata endpoint on AWS, Azure, GCP, and
	// several other clouds alike — the single highest-value SSRF target
	// this fix exists to block.
	if _, err := resolveToSafeIP("169.254.169.254"); err == nil {
		t.Fatal("FAIL: cloud metadata address 169.254.169.254 was accepted")
	}
}

func TestResolveToSafeIP_RejectsPrivateNetworkAddresses(t *testing.T) {
	for _, host := range []string{"10.0.0.5", "172.16.0.1", "192.168.1.1"} {
		if _, err := resolveToSafeIP(host); err == nil {
			t.Fatalf("FAIL: private-network address %s was accepted", host)
		}
	}
}

func TestResolveToSafeIP_RejectsIPv6LinkLocalAndULA(t *testing.T) {
	for _, host := range []string{"fe80::1", "fd00::1"} {
		if _, err := resolveToSafeIP(host); err == nil {
			t.Fatalf("FAIL: IPv6 internal address %s was accepted", host)
		}
	}
}

func TestResolveToSafeIP_AllowsLoopback(t *testing.T) {
	// The tool's own documented default (http://127.0.0.1:8545) — must stay
	// usable. This handler is itself bound to 127.0.0.1 only, so allowing
	// loopback as a *target* grants no reach an attacker able to call this
	// endpoint at all doesn't already have.
	for _, host := range []string{"127.0.0.1", "::1"} {
		ip, err := resolveToSafeIP(host)
		if err != nil {
			t.Fatalf("loopback host %s should be allowed: %v", host, err)
		}
		if !ip.IsLoopback() {
			t.Fatalf("expected a loopback IP back for %s, got %s", host, ip)
		}
	}
}

func TestResolveToSafeIP_AllowsOrdinaryPublicAddress(t *testing.T) {
	ip, err := resolveToSafeIP("8.8.8.8")
	if err != nil {
		t.Fatalf("public address should be allowed: %v", err)
	}
	if ip.String() != "8.8.8.8" {
		t.Fatalf("expected 8.8.8.8 back, got %s", ip)
	}
}

// ── newSafeRPCClient: the end-to-end behavior ───────────────────────────────

func TestNewSafeRPCClient_RejectsNonHTTPScheme(t *testing.T) {
	// Regression for the ORIGINAL mitigation (scheme allowlist) — must still
	// hold after this fix, not just the new IP checks.
	if _, err := newSafeRPCClient("file:///etc/passwd"); err == nil {
		t.Fatal("FAIL: file:// scheme was accepted")
	}
	if _, err := newSafeRPCClient("gopher://169.254.169.254/"); err == nil {
		t.Fatal("FAIL: gopher:// scheme was accepted")
	}
}

func TestNewSafeRPCClient_RejectsCloudMetadataURL(t *testing.T) {
	if _, err := newSafeRPCClient("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("FAIL: a full cloud-metadata URL was accepted")
	}
}

// TestNewSafeRPCClient_RoundTripsToLoopback proves the happy path actually
// works end to end: a real local HTTP server, a real client built by this
// function, a real request that reaches it and gets the real response back —
// not just "validation returned no error".
func TestNewSafeRPCClient_RoundTripsToLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x2a"}`)
	}))
	defer srv.Close()

	client, err := newSafeRPCClient(srv.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", srv.URL, err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("round trip through the pinned client failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestNewSafeRPCClient_RefusesRedirects: a same-address response that then
// redirects elsewhere must not be followed — otherwise the initial
// validation is bypassable by a 302.
func TestNewSafeRPCClient_RefusesRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("FAIL: the redirect target was reached — CheckRedirect did not stop it")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client, err := newSafeRPCClient(redirector.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", redirector.URL, err)
	}
	resp, err := client.Get(redirector.URL)
	// A refused redirect surfaces as a client error (net/http wraps the
	// CheckRedirect error), not a followed 200 from the target.
	if err == nil {
		defer resp.Body.Close()
		t.Fatalf("FAIL: request succeeded with status %d — redirect was followed", resp.StatusCode)
	}
}

// TestNewSafeRPCClient_PinsConnection_IgnoresWhatTheHostWouldReresolveTo is
// the direct proof for the DNS-rebinding gap: even if the *http.Client's own
// internal DialContext were handed a different address to dial (exactly
// what a second, independent resolution at connect time could produce),
// the connection still lands on the address newSafeRPCClient validated —
// because DialContext ignores the addr argument entirely and dials the
// pinned pair directly.
func TestNewSafeRPCClient_PinsConnection_IgnoresWhatTheHostWouldReresolveTo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	client, err := newSafeRPCClient(srv.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient: %v", err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	// Call DialContext with an address that does NOT match the real server —
	// simulating exactly what a rebound second DNS answer would hand the
	// transport. If pinning works, the connection still succeeds (it goes
	// to the real, validated server, not to whatever bogus addr is passed).
	conn, err := tr.DialContext(context.Background(), "tcp", "203.0.113.1:9999")
	if err != nil {
		t.Fatalf("pinned dial failed even though it should ignore the given addr: %v", err)
	}
	conn.Close()

	// And confirm 203.0.113.1:9999 (TEST-NET-3, guaranteed unroutable) was
	// never actually where it connected — dialing straight to *that*
	// address on its own must fail (quickly, since this repo's own
	// sandbox has no route to it), proving the successful dial above went
	// somewhere else (the pinned, validated target).
	if _, err := net.DialTimeout("tcp", "203.0.113.1:9999", 500*time.Millisecond); err == nil {
		t.Fatal("test invariant broken: 203.0.113.1:9999 unexpectedly reachable in this environment")
	}
}
