package utils

// Native Go fuzzing (go test -fuzz) for the parsing-heavy functions in
// merkle_status.go that don't need a network round-trip to exercise —
// exactly the code shape (byte/string parsing with several encoding forms)
// where the 6to4/Teredo SSRF bypass and the negative-hex truncation bug
// both lived. Unit tests cover the specific cases a human thought to write;
// these explore the input space instead.

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzEmbeddedIPv4Candidates fuzzes raw 16-byte buffers as IPv6 literals.
// Invariants: the function must never panic, and every IPv4 address it
// claims is "embedded" in the input must actually be representable as one
// (i.e. To4() succeeds on it) — a candidate that isn't a real IPv4 address
// would be a bug in the extraction logic itself.
func FuzzEmbeddedIPv4Candidates(f *testing.F) {
	seeds := []string{
		"64:ff9b::a9fe:a9fe",          // NAT64-encoded 169.254.169.254
		"2002:a9fe:a9fe::",            // 6to4-encoded 169.254.169.254
		"2001:0000:a9fe:a9fe:0:0:0:1", // Teredo, server = 169.254.169.254
		"2001:0000:0808:0808:0:0:0:0", // Teredo, client = 255.255.255.255 (XOR of all-zero)
		"::1",                         // loopback — must NOT be treated as an embedding
		"::",                          // unspecified — must NOT be treated as an embedding
		"::ffff:169.254.169.254",      // modern IPv4-mapped — handled by To4(), not this function
		"64:ff9b::0808:0808",          // NAT64-encoded public 8.8.8.8
		"2001:db8::1",                 // ordinary IPv6, no embedding
	}
	for _, s := range seeds {
		if ip := net.ParseIP(s); ip != nil {
			f.Add([]byte(ip.To16()))
		}
	}
	f.Add(make([]byte, 16))                                                                                       // all-zero
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) // all-0xff

	f.Fuzz(func(t *testing.T, raw []byte) {
		buf := make([]byte, 16)
		copy(buf, raw) // pad/truncate to a valid 16-byte IPv6 literal
		ip := net.IP(buf)

		candidates := embeddedIPv4Candidates(ip) // must not panic

		for _, c := range candidates {
			if c.To4() == nil {
				t.Fatalf("embeddedIPv4Candidates(%v) returned a non-IPv4 candidate: %v", ip, c)
			}
		}
	})
}

// FuzzParseUnsignedHexUint256 fuzzes the hex-parsing logic that had the
// negative-value truncation bug. Invariant: the function must never panic,
// and whenever it succeeds, the result must be non-negative — a negative
// big.Int slipping through here is exactly the bug that let "0x-1" get
// silently truncated to a plausible-looking positive value downstream.
func FuzzParseUnsignedHexUint256(f *testing.F) {
	seeds := []string{
		"0x2a", "2a", "0x-1", "-1", "0x", "", "0xg", "-0", "0x00",
		"0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"0x10000000000000001", // > MaxUint64, non-negative
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, hexStr string) {
		v, err := parseUnsignedHexUint256(hexStr) // must not panic
		if err == nil && v.Sign() < 0 {
			t.Fatalf("parseUnsignedHexUint256(%q) returned negative value %v with no error", hexStr, v)
		}
	})
}

// FuzzValidateReceiptsPath fuzzes the path-traversal guard. Invariants: the
// function must never panic, and whenever it allows a path (err == nil),
// that path must not be absolute and must not traverse more than one
// directory level above the base — mirroring the function's own documented
// contract, checked independently rather than by re-reading its source.
func FuzzValidateReceiptsPath(f *testing.F) {
	seeds := []string{
		"../build/receipts.json",
		"receipts.json",
		"../../etc/passwd",
		"/etc/passwd",
		"",
		"....//....//etc/passwd",
		"a/../../b",
		"./../x",
		"~/receipts.json",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, p string) {
		err := validateReceiptsPath(p) // must not panic
		if err != nil {
			return
		}
		clean := filepath.ToSlash(filepath.Clean(p))
		if filepath.IsAbs(clean) {
			t.Fatalf("validateReceiptsPath(%q) allowed an absolute path", p)
		}
		depth := 0
		for _, seg := range strings.Split(clean, "/") {
			if seg == ".." {
				depth++
			}
		}
		if depth > 1 {
			t.Fatalf("validateReceiptsPath(%q) allowed traversing %d levels above the base", p, depth)
		}
	})
}
