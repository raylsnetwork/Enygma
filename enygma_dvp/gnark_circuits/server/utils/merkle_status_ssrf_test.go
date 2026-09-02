package utils

// Regression tests for the SSRF fix in merkle_status.go: rpcUrl in both
// MerkleStatusRequest and MerkleVaultRequest comes directly from the request
// body, feeds an outbound HTTP request (doRPC), and the fetched result is
// echoed back to the caller — the exact shape that turns SSRF into a way to
// read cloud metadata / internal-only services through this server.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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

// TestNewSafeRPCClient_DialContextValidatesWhateverHostItsAskedToDial is
// the direct proof for the DNS-rebinding gap under the current design:
// DialContext re-resolves and re-validates the *specific* host:port it's
// asked to dial, every single time, rather than trusting one up-front
// resolution forever. That means there's never a window where an unchecked
// resolution gets used — a disallowed target fails right here, and a
// legitimate one succeeds, regardless of which "hop" (original request or
// an allowed same-host redirect) is asking.
func TestNewSafeRPCClient_DialContextValidatesWhateverHostItsAskedToDial(t *testing.T) {
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

	// The real server's own address must dial successfully.
	conn, err := tr.DialContext(context.Background(), "tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dialing the real server's own address failed: %v", err)
	}
	conn.Close()

	// A disallowed host must be refused at DialContext itself — this is
	// what protects both the original request and any redirect target,
	// without needing any state shared between them.
	if _, err := tr.DialContext(context.Background(), "tcp", "169.254.169.254:80"); err == nil {
		t.Fatal("FAIL: DialContext dialed a disallowed (link-local metadata) address")
	}
}

// ── doRPC: real redirect-following behavior (the shape every production
// RPC call actually goes through) ───────────────────────────────────────

// jsonRPCReq is a shared minimal request for the doRPC tests below — the
// specific method only matters for TestDoRPC_UsesLongerTimeoutForGetLogs.
var jsonRPCReq = jsonRPCRequest{Jsonrpc: "2.0", Method: "eth_call", ID: 1}

// TestDoRPC_FollowsSameHostRedirect_PreservingPOSTBody proves doRPC — not
// a bare client.Get, which the original version of this test used and
// which doesn't exercise the bug this covers — correctly follows a
// same-host redirect while preserving the POST method and JSON body. Go's
// built-in redirect-following rewrites a redirected POST to a bodyless GET
// on 301/302/303 (the ordinary case a gateway would use, not just 307/308),
// which would silently break real JSON-RPC traffic; this is why doRPC
// follows redirects itself instead of relying on that.
func TestDoRPC_FollowsSameHostRedirect_PreservingPOSTBody(t *testing.T) {
	// A single self-redirecting server, not two separate httptest servers
	// on two different ports: this is a same-origin redirect (a
	// canonicalizing 302 to a different path, same host/port/scheme — the
	// case validateSameHostRedirect's port policy actually allows; two
	// httptest servers would land on two different ports with no scheme
	// change between them, which fix #4's port-pivot protection now
	// correctly refuses, on purpose).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/v1/", http.StatusFound) // relative: same origin, inherits host+port+scheme
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("FAIL: target saw method %s, want POST — redirect lost the method", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("FAIL: target saw an empty body — redirect lost the POST body")
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x2a"}`)
	}))
	defer srv.Close()

	client, err := newSafeRPCClient(srv.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", srv.URL, err)
	}
	resp, err := doRPC(client, srv.URL, jsonRPCReq)
	if err != nil {
		t.Fatalf("doRPC through a same-host redirect should succeed: %v", err)
	}
	if resp.Result != "0x2a" {
		t.Fatalf("expected result 0x2a, got %v", resp.Result)
	}
}

// TestDoRPC_RefusesCrossHostRedirect: a redirect to a different host must
// still be refused — this is what actually prevents redirect-based SSRF (a
// same-address response 302ing to a disallowed target).
func TestDoRPC_RefusesCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("FAIL: the redirect target was reached — cross-host redirect was not refused")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "localhost" and "127.0.0.1" resolve to the same place but are
		// different *hostnames* — the redirect policy compares the literal
		// hostname string (never re-resolving it), so this must be refused
		// even though it isn't a rebinding attack in this particular case.
		loc := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
		http.Redirect(w, r, loc, http.StatusFound)
	}))
	defer redirector.Close()

	client, err := newSafeRPCClient(redirector.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", redirector.URL, err)
	}
	if _, err := doRPC(client, redirector.URL, jsonRPCReq); err == nil {
		t.Fatal("FAIL: cross-host redirect succeeded")
	}
}

// TestDoRPC_SameHostRedirect_CaseInsensitiveHostname: DNS hostnames are
// case-insensitive, so a same-host redirect that differs only in case must
// still be treated as same-host, not wrongly refused.
func TestDoRPC_SameHostRedirect_CaseInsensitiveHostname(t *testing.T) {
	// One server (same port throughout, isolating the case-sensitivity
	// variable) that redirects to an upper-cased absolute hostname on its
	// own port, then serves the real response.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			port := srv.Listener.Addr().(*net.TCPAddr).Port
			http.Redirect(w, r, fmt.Sprintf("http://LOCALHOST:%d/v1/", port), http.StatusFound)
			return
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x2a"}`)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	originalURL := fmt.Sprintf("http://localhost:%d/", port)

	client, err := newSafeRPCClient(originalURL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", originalURL, err)
	}
	resp, err := doRPC(client, originalURL, jsonRPCReq)
	if err != nil {
		t.Fatalf("case-different same-host redirect should be followed: %v", err)
	}
	if resp.Result != "0x2a" {
		t.Fatalf("expected result 0x2a, got %v", resp.Result)
	}
}

// TestDoRPC_RefusesDifferentPortRedirect: a same-host redirect to an
// explicit, different port must be refused — otherwise an allowed-but-
// later-hostile target could pivot the connection to an arbitrary
// co-located service (e.g. a cache or database) on the same host.
func TestDoRPC_RefusesDifferentPortRedirect(t *testing.T) {
	pivot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("FAIL: reached the pivot target — different-port redirect was not refused")
	}))
	defer pivot.Close()
	pivotPort := pivot.Listener.Addr().(*net.TCPAddr).Port

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("http://127.0.0.1:%d/", pivotPort), http.StatusFound)
	}))
	defer redirector.Close()

	client, err := newSafeRPCClient(redirector.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", redirector.URL, err)
	}
	if _, err := doRPC(client, redirector.URL, jsonRPCReq); err == nil {
		t.Fatal("FAIL: different-port same-host redirect succeeded")
	}
}

// TestValidateSameHostRedirect_RefusesHTTPSToHTTPDowngrade: a redirect must
// never silently downgrade an https rpcUrl to plaintext http.
func TestValidateSameHostRedirect_RefusesHTTPSToHTTPDowngrade(t *testing.T) {
	orig, _ := url.Parse("https://example.com/")
	target, _ := url.Parse("http://example.com/")
	if err := validateSameHostRedirect(orig, target); err == nil {
		t.Fatal("FAIL: https->http downgrade redirect was allowed")
	}
}

// TestValidateSameHostRedirect_AllowsHTTPToHTTPSUpgrade is the positive
// case for the same check — the scenario this policy exists to support.
func TestValidateSameHostRedirect_AllowsHTTPToHTTPSUpgrade(t *testing.T) {
	orig, _ := url.Parse("http://example.com/")
	target, _ := url.Parse("https://example.com/")
	if err := validateSameHostRedirect(orig, target); err != nil {
		t.Fatalf("http->https upgrade should be allowed: %v", err)
	}
}

// ── embedded-IPv4 encapsulation schemes ─────────────────────────────────────

func TestResolveToSafeIP_RejectsNAT64EncodedMetadataAddress(t *testing.T) {
	// 64:ff9b::a9fe:a9fe is the NAT64 Well-Known Prefix (RFC 6052)
	// encoding of 169.254.169.254 (a9fe:a9fe in hex).
	if _, err := resolveToSafeIP("64:ff9b::a9fe:a9fe"); err == nil {
		t.Fatal("FAIL: NAT64-encoded cloud metadata address was accepted")
	}
}

func TestResolveToSafeIP_Rejects6to4EncodedMetadataAddress(t *testing.T) {
	// 2002:a9fe:a9fe:: is the 6to4 (RFC 3056) encoding of 169.254.169.254.
	if _, err := resolveToSafeIP("2002:a9fe:a9fe::"); err == nil {
		t.Fatal("FAIL: 6to4-encoded cloud metadata address was accepted")
	}
}

func TestResolveToSafeIP_RejectsTeredoEncodedMetadataAddress(t *testing.T) {
	// Teredo (RFC 4380) embeds two IPv4 addresses: the relay/server's,
	// literal, at bytes 4-7, and the client's, XORed with 0xff per byte, at
	// bytes 12-15. Both must be checked — one test address per position.

	// server (bytes 4-7) = 169.254.169.254 (a9fe:a9fe), literal.
	serverEncoded := "2001:0000:a9fe:a9fe:0:0:0:1"
	if _, err := resolveToSafeIP(serverEncoded); err == nil {
		t.Fatalf("FAIL: Teredo address with disallowed server IPv4 was accepted: %s", serverEncoded)
	}

	// client (bytes 12-15) decodes to 169.254.169.254 once XORed with 0xff:
	// 0xa9^0xff=0x56, 0xfe^0xff=0x01 → obfuscated bytes 56:01:56:01.
	clientEncoded := "2001:0000:0808:0808:0:0:5601:5601"
	if _, err := resolveToSafeIP(clientEncoded); err == nil {
		t.Fatalf("FAIL: Teredo address with disallowed (XOR-obfuscated) client IPv4 was accepted: %s", clientEncoded)
	}
}

func TestResolveToSafeIP_AllowsEncapsulatedPublicAddress(t *testing.T) {
	// A NAT64-encoded *public* address must still work — and, per the fix
	// for the dial-target regression, resolveToSafeIP must return the
	// literal encapsulated address (still routable via the NAT64 gateway
	// on an IPv6-only host), not a bare substituted IPv4 literal that may
	// have no route at all in that environment.
	addr := "64:ff9b::0808:0808" // embeds 8.8.8.8
	ip, err := resolveToSafeIP(addr)
	if err != nil {
		t.Fatalf("NAT64-encoded public address should be allowed: %v", err)
	}
	if ip.String() != "64:ff9b::808:808" {
		t.Fatalf("expected the literal NAT64 address back (not substituted), got %s", ip)
	}
}

func TestResolveToSafeIP_RejectsThisNetworkRange(t *testing.T) {
	// 0.0.0.0/8 ("this network") — IsUnspecified() alone only catches the
	// exact address 0.0.0.0, not the rest of the /8.
	if _, err := resolveToSafeIP("0.1.2.3"); err == nil {
		t.Fatal("FAIL: 0.0.0.0/8 address 0.1.2.3 was accepted")
	}
}

// ── checkVault: the treeNumber overflow guard ───────────────────────────────

// TestCheckVault_RejectsOversizedTreeNumber proves an allowed-but-hostile
// RPC target can't launder a non-negative-but-huge treeNumber() result
// (passing ethCallUint256's negative check) into a small, wrong value via
// Uint64()'s silent truncation — this is what actually narrows to uint64,
// so the guard belongs here, not inside ethCallUint256 (shared with
// onChainRoot, a real field element that legitimately exceeds 64 bits).
func TestCheckVault_RejectsOversizedTreeNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := `"0x2a"`
		if req.Method == "eth_call" {
			// Every eth_call in this test returns the same oversized,
			// non-negative value — covers both currentRoot() and
			// treeNumber() without needing to distinguish selectors.
			result = `"0x10000000000000001"` // 2^64 + 1: fits in a field element, not in a uint64
		} else if req.Method == "eth_getLogs" {
			result = `[]`
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, result)
	}))
	defer srv.Close()

	client, err := newSafeRPCClient(srv.URL)
	if err != nil {
		t.Fatalf("newSafeRPCClient(%s): %v", srv.URL, err)
	}
	status := checkVault(client, srv.URL, "Erc20CoinVault", "0x1111111111111111111111111111111111111111", 8,
		"0xdeadbeef", "0xaaaaaaaa", "0xbbbbbbbb")
	if status.Error == "" {
		t.Fatalf("FAIL: oversized treeNumber was accepted silently — TreeNumber=%d", status.TreeNumber)
	}
	if status.TreeNumber != 0 {
		t.Fatalf("FAIL: TreeNumber should stay unset on error, got %d (the truncated low 64 bits, "+
			"exactly the silent-corruption this guard exists to prevent)", status.TreeNumber)
	}
}
