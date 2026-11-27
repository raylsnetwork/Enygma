package drand

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

// Quicknet's round 1,000,000, as its relays serve it.
const (
	round      = 1000000
	signature  = "83ad29e4c409f9470fc2ef02f90214df49e02b441a1a241a82d622d9f608ef98fd8b11a029f1bee9d9e83b45088abe72"
	randomness = "b22aad4794f7451896f7a371aa46106fd84d919f3f569acd5b2fddf1d1440af3"
)

// TestVerify: a round's signature checks against quicknet's key and gives
// the round's published randomness; it is no signature of any other round,
// and a changed signature is no signature at all.
func TestVerify(t *testing.T) {
	sig, _ := hex.DecodeString(signature)

	t.Run("accepts quicknet's signature of the round", func(t *testing.T) {
		r, err := Verify(round, sig)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(r[:]); got != randomness {
			t.Fatalf("randomness %s, want %s", got, randomness)
		}
	})
	t.Run("refuses it as another round's", func(t *testing.T) {
		if _, err := Verify(round+1, sig); !errors.Is(err, ErrSignature) {
			t.Fatalf("Verify: %v, want %v", err, ErrSignature)
		}
	})
	t.Run("refuses the signature uncompressed, whose sha256 is not the round's randomness", func(t *testing.T) {
		var p bls.G1Affine
		if _, err := p.SetBytes(sig); err != nil {
			t.Fatal(err)
		}
		raw := p.RawBytes()
		if _, err := Verify(round, raw[:]); !errors.Is(err, ErrSignature) {
			t.Fatalf("Verify: %v, want %v", err, ErrSignature)
		}
	})
	t.Run("refuses a signature with a bit changed", func(t *testing.T) {
		bad := append([]byte(nil), sig...)
		bad[len(bad)-1] ^= 1
		if _, err := Verify(round, bad); !errors.Is(err, ErrSignature) {
			t.Fatalf("Verify: %v, want %v", err, ErrSignature)
		}
	})
}

// TestRoundAt: the round announced for a time is the first one out at or
// after it, never one already out before it.
func TestRoundAt(t *testing.T) {
	at := Time(round)
	if got := RoundAt(at); got != round {
		t.Errorf("RoundAt(round %d's time) = %d", round, got)
	}
	if got := RoundAt(at.Add(time.Nanosecond)); got != round+1 {
		t.Errorf("RoundAt(just after round %d) = %d, want %d", round, got, round+1)
	}
	if got := RoundAt(at.Add(-time.Nanosecond)); got != round {
		t.Errorf("RoundAt(just before round %d) = %d, want %d", round, got, round)
	}
	if got := RoundAt(Time(1).Add(-time.Hour)); got != 1 {
		t.Errorf("RoundAt(before quicknet began) = %d, want 1", got)
	}
}

// TestFetch: Fetch takes the first relay's answer that verifies, passing over
// a relay that serves a wrong signature, and asks no relay for a round whose
// time has not come.
func TestFetch(t *testing.T) {
	var hits atomic.Int32
	relay := func(sig string) string {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path != fmt.Sprintf("/%s/public/%d", ChainHash, round) {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `{"round":%d,"signature":%q}`, round, sig)
		}))
		t.Cleanup(s.Close)
		return s.URL
	}
	other := "a" + signature[1:] // another valid-looking signature
	good, bad := relay(signature), relay(other)

	t.Run("passes over a relay with a wrong signature", func(t *testing.T) {
		r, err := Fetch(context.Background(), http.DefaultClient, []string{bad, good}, round)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(r[:]); got != randomness {
			t.Fatalf("randomness %s, want %s", got, randomness)
		}
	})
	t.Run("fails when no relay's answer verifies", func(t *testing.T) {
		if _, err := Fetch(context.Background(), http.DefaultClient, []string{bad}, round); !errors.Is(err, ErrSignature) {
			t.Fatalf("Fetch: %v, want %v", err, ErrSignature)
		}
	})
	t.Run("asks no relay for a round not out yet", func(t *testing.T) {
		hits.Store(0)
		future := RoundAt(time.Now().Add(time.Hour))
		if _, err := Fetch(context.Background(), http.DefaultClient, []string{good}, future); !errors.Is(err, ErrNotYet) {
			t.Fatalf("Fetch: %v, want %v", err, ErrNotYet)
		}
		if hits.Load() != 0 {
			t.Fatalf("asked a relay %d times", hits.Load())
		}
	})
}
