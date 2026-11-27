package drand

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

// FuzzVerify: no bytes make Verify panic, and it accepts no signature but
// quicknet's own: of every round and signature it is given, only round
// 1,000,000's published signature passes, giving the published randomness.
// The seal's beacon is what Verify accepts, and whoever serves it is untrusted.
func FuzzVerify(f *testing.F) {
	sig, err := hex.DecodeString(signature)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint64(round), sig)
	f.Add(uint64(round+1), sig)
	f.Add(uint64(0), make([]byte, len(sig)))
	infinity := make([]byte, len(sig))
	infinity[0] = 0xc0 // the point at infinity, compressed
	f.Add(uint64(round), infinity)
	f.Fuzz(func(t *testing.T, r uint64, s []byte) {
		got, err := Verify(r, s)
		if err != nil {
			if !errors.Is(err, ErrSignature) {
				t.Fatalf("refused for no reason the package gives: %v", err)
			}
			return
		}
		if r != round || !bytes.Equal(s, sig) || hex.EncodeToString(got[:]) != randomness {
			t.Fatalf("accepted round %d's signature %x, giving %x", r, s, got)
		}
	})
}
