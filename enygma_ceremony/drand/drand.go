// Package drand fetches a round of drand's quicknet beacon
// (https://drand.love) and checks its signature against the network's public
// key, pinned here, so the beacon that seals a ceremony phase rests on no
// one's word, a relay's included. A round's randomness is the sha256 of its
// signature; a round is out every Period, and its number can be announced
// before it is out (RoundAt), so nobody knows the beacon when the last
// contribution is made.
package drand

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"

	"enygma_ceremony/internal/termsafe"
)

// ChainHash names quicknet, the drand chain whose rounds this package takes.
const ChainHash = "52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971"

const (
	// publicKey is quicknet's group public key, in G2 (chain info, /info).
	publicKey = "83cf0f2896adee7eb8b5f01fcad3912212c437e0073e911fb90022d3e760183c8c4b450b6a0a6c3ac6a5776a2d1064510d1fec758c921cc22b0e17e63aaf4bcb5ed66304de9cf809bd274ca73bab4af5a6e9c76a4bc09e76eae8991ef5ece45a"
	// genesis is when quicknet's round 1 was out, in Unix seconds.
	genesis = 1692803367
	// dst is the hash-to-curve domain of quicknet's scheme, bls-unchained-g1-rfc9380.
	dst = "BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_"
)

// Period is the time between quicknet's rounds.
const Period = 3 * time.Second

// Relays are public HTTP relays of quicknet. Any one will do: each answer's
// signature is checked against the pinned key.
var Relays = []string{
	"https://api.drand.sh",
	"https://drand.cloudflare.com",
	"https://api2.drand.sh",
	"https://api3.drand.sh",
}

var (
	// ErrSignature is a signature that is not quicknet's of the round.
	ErrSignature = errors.New("not quicknet's signature of the round")
	// ErrNotYet is a round whose time has not come.
	ErrNotYet = errors.New("round not out yet")
)

// Time returns when round is out.
func Time(round uint64) time.Time {
	return time.Unix(genesis, 0).Add(time.Duration(round-1) * Period)
}

// RoundAt returns the first round out at or after t: the round to announce
// for a beacon nobody can know before t.
func RoundAt(t time.Time) uint64 {
	if !t.After(time.Unix(genesis, 0)) {
		return 1
	}
	r := uint64(t.Sub(time.Unix(genesis, 0))/Period) + 1 // the last round out by t
	if Time(r).Before(t) {
		r++
	}
	return r
}

// groupKey is publicKey as a point.
var groupKey = sync.OnceValues(func() (bls.G2Affine, error) {
	var pk bls.G2Affine
	b, err := hex.DecodeString(publicKey)
	if err != nil {
		return pk, err
	}
	_, err = pk.SetBytes(b)
	return pk, err
})

// Verify checks that sig is quicknet's signature of round, compressed as
// drand serves it, and returns the round's randomness, the sha256 of sig.
func Verify(round uint64, sig []byte) ([32]byte, error) {
	if len(sig) != bls.SizeOfG1AffineCompressed {
		return [32]byte{}, fmt.Errorf("%w: %d bytes, want %d", ErrSignature, len(sig), bls.SizeOfG1AffineCompressed)
	}
	var s bls.G1Affine
	if _, err := s.SetBytes(sig); err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrSignature, err)
	}
	var r [8]byte
	binary.BigEndian.PutUint64(r[:], round)
	msg := sha256.Sum256(r[:])
	h, err := bls.HashToG1(msg[:], []byte(dst))
	if err != nil {
		return [32]byte{}, err
	}
	pk, err := groupKey()
	if err != nil {
		return [32]byte{}, err
	}
	// e(sig, g₂) = e(H(round), pk), as e(sig, -g₂)·e(H(round), pk) = 1.
	_, _, _, g2 := bls.Generators()
	var negG2 bls.G2Affine
	negG2.Neg(&g2)
	ok, err := bls.PairingCheck([]bls.G1Affine{s, h}, []bls.G2Affine{negG2, pk})
	if err != nil {
		return [32]byte{}, err
	}
	if !ok {
		return [32]byte{}, fmt.Errorf("%w: round %d", ErrSignature, round)
	}
	return sha256.Sum256(sig), nil
}

// Fetch gets round from relays, in order, and returns its randomness from
// the first answer whose signature Verify accepts.
func Fetch(ctx context.Context, client *http.Client, relays []string, round uint64) ([32]byte, error) {
	if due := Time(round); time.Now().Before(due) {
		return [32]byte{}, fmt.Errorf("%w: round %d is out at %s", ErrNotYet, round, due.UTC().Format(time.RFC3339))
	}
	var errs []error
	for _, relay := range relays {
		r, err := fetchFrom(ctx, client, relay, round)
		if err == nil {
			return r, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", relay, err))
	}
	return [32]byte{}, errors.Join(errs...)
}

// fetchFrom gets round from one relay and verifies it.
func fetchFrom(ctx context.Context, client *http.Client, relay string, round uint64) ([32]byte, error) {
	url := fmt.Sprintf("%s/%s/public/%d", strings.TrimSuffix(relay, "/"), ChainHash, round)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return [32]byte{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return [32]byte{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return [32]byte{}, errors.New(termsafe.Status(resp.StatusCode))
	}
	// Only the signature counts: Verify checks it is of the round asked for.
	var b struct {
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&b); err != nil {
		return [32]byte{}, err
	}
	sig, err := hex.DecodeString(b.Signature)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrSignature, err)
	}
	return Verify(round, sig)
}
