package agreement

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/mlkem768"
)

// curveP is the Baby JubJub subgroup order.
var curveP, _ = new(big.Int).SetString(
	"2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

// Manager manages an ML-KEM-768 view key and pairwise shared secrets for one bank.
//
// Usage pattern:
//   - The "leader" (Fix M-11: always the lower bankID in the pair — see
//     pairKey and the checks in GetOrEstablish/GetOrAccept below) calls
//     GetOrEstablish, which encapsulates to the peer's public key and
//     persists the ciphertext, plus a confirmation hash, to disk.
//   - The "follower" (always the higher bankID) calls GetOrAccept, which
//     reads the leader's ciphertext, decapsulates using its own private
//     key, and verifies the confirmation hash before ever caching the
//     result (Fix M-11 defect 2).
//   - Both end up with the same shared secret field element, cached in memory and
//     persisted to disk so subsequent calls are instant.
type Manager struct {
	bankID   int
	storeDir string
	mu       sync.RWMutex
	dk       *mlkem768.DecapsulationKey
	cache    map[string]*big.Int
}

// New loads or creates the view keypair for bankID, storing files under storeDir.
//
// Fix M-11 defect 3: New is for loading THIS bank's own identity only — it
// is the only place in this package that may call mlkem768.GenerateKey().
// Never call New to obtain a peer's manager; use LoadPeerEncapsulationKey
// instead, which refuses to fabricate a keypair for anyone but the local
// caller.
func New(bankID int, storeDir string) (*Manager, error) {
	m := &Manager{
		bankID:   bankID,
		storeDir: storeDir,
		cache:    make(map[string]*big.Int),
	}
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return nil, err
	}
	if err := m.loadOrCreate(); err != nil {
		return nil, err
	}
	return m, nil
}

// EncapsulationKey returns the 1184-byte public key for this bank.
// Share this with peers so they can encapsulate to you.
func (m *Manager) EncapsulationKey() []byte {
	return m.dk.EncapsulationKey()
}

// LoadPeerEncapsulationKey returns peerID's published encapsulation key
// from storeDir, WITHOUT ever generating one.
//
// Fix M-11 defect 3 ("the most serious"): the vulnerable pattern this
// replaces was calling New(peerID, storeDir) to obtain "a peer's
// manager" — New's loadOrCreate silently generates a fresh keypair (and
// hence a fresh, locally-known "private" key) for any bankID whose seed
// file is missing. A process that does this for every peer generates
// every counterparty's private seed itself and then encapsulates to
// public keys it invented moments earlier — every pairwise secret is
// unilaterally decided by one party, and no counterparty can derive its
// own blinding factor or tag, completely silently. This function can
// only ever read; a missing key is a hard error, never a fallback to
// generation.
//
// The audit's remediation asks for encapsulation keys to be sourced from
// Enygma.viewKeys (the on-chain registry) rather than a local file at
// all. That is deliberately not done here: the one caller of this
// pattern in this tree (go_client/transaction/main.go's
// initializeSecrets) has its own separately-tracked accountId
// indexing bug (L-08/L-10 — the CLI is consistently 0-based against a
// 1-based on-chain convention) that must be fixed in lockstep with
// wherever accountIds are looked up, or ordinary transfers get silently
// routed to account 0. Conflating that fix into this one risks
// papering over one bug with another. What ships here still closes the
// actual security defect — no more locally-fabricated peer keys — using
// the same storeDir convention EncapsulationKey/New already establish;
// switching the source to on-chain viewKeys is separate follow-up work
// once L-08/L-10 land.
func LoadPeerEncapsulationKey(storeDir string, peerID int) ([]byte, error) {
	ekPath := filepath.Join(storeDir, fmt.Sprintf("bank_%d_ek.bin", peerID))
	ek, err := os.ReadFile(ekPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("peer %d has no published encapsulation key at %s — refusing to fabricate one (Fix M-11): "+
				"the peer must run its own agreement.New first and publish bank_%d_ek.bin", peerID, ekPath, peerID)
		}
		return nil, fmt.Errorf("read peer %d encapsulation key: %w", peerID, err)
	}
	return ek, nil
}

// GetOrEstablish is the leader path: encapsulate to peerEK, persist the ciphertext
// and a confirmation hash, and return the shared secret as a Baby JubJub field element.
// If a secret for this pair already exists (memory or disk), it is returned directly.
//
// Fix M-11 defect 1: only the lower-id party in a pair may lead (encapsulate).
// Before this, both sides of a pair could independently call GetOrEstablish;
// each would persist a DIFFERENT shared secret under the same cache key with
// nothing to detect the divergence, since GetOrAccept (the "follower" half)
// had no non-test caller at all and neither entry point ever compared its
// result against the other side's.
func (m *Manager) GetOrEstablish(peerID int, peerEK []byte) (*big.Int, error) {
	if !(m.bankID < peerID) {
		return nil, fmt.Errorf("bank %d cannot lead key agreement with bank %d: the lower id always encapsulates (Fix M-11) — call GetOrAccept(%d) instead",
			m.bankID, peerID, peerID)
	}

	key := pairKey(m.bankID, peerID)
	if ss := m.loadCached(key); ss != nil {
		return ss, nil
	}

	ct, rawSS, err := mlkem768.Encapsulate(peerEK)
	if err != nil {
		return nil, fmt.Errorf("encapsulate to peer %d: %w", peerID, err)
	}
	ctPath := filepath.Join(m.storeDir, fmt.Sprintf("ct_%d_to_%d.bin", m.bankID, peerID))
	// Fix M-11: was 0644 — a ciphertext file world-readable by any local
	// user, not just the operator who owns this storeDir.
	if err := os.WriteFile(ctPath, ct, 0600); err != nil {
		return nil, fmt.Errorf("persist ciphertext: %w", err)
	}
	ss := toFieldElement(rawSS)

	// Fix M-11 defect 2: publish id = Hash(rawSS) alongside the
	// ciphertext, per protocol_description.md's own specified check
	// ("the recipient must recompute id' = Hash(s') and check it") —
	// never actually implemented in code before this fix. The follower
	// compares against this before ever caching its own decapsulated
	// result.
	idPath := confirmationIDPath(m.storeDir, m.bankID, peerID)
	if err := os.WriteFile(idPath, []byte(hex.EncodeToString(confirmationID(rawSS))), 0600); err != nil {
		return nil, fmt.Errorf("persist confirmation id: %w", err)
	}

	m.persistSecret(key, ss)
	return ss, nil
}

// GetOrAccept is the follower path: read the leader's ciphertext from disk and
// decapsulate it using this bank's private key.
// If a secret for this pair already exists (memory or disk), it is returned directly.
//
// Fix M-11 defect 1: only the higher-id party in a pair may follow
// (decapsulate) — the mirror image of GetOrEstablish's check, see its
// doc comment.
//
// Fix M-11 defect 2: FIPS 203's implicit rejection means ANY correctly-
// sized ciphertext decapsulates successfully with err == nil and a
// pseudorandom key — a corrupted, truncated, or outright malicious
// ciphertext produces no error signal from Decapsulate itself. Before
// this fix, that pseudorandom result was cached and returned exactly
// like a genuine shared secret, with no invalidation path anywhere in
// this package. This is silent protocol corruption (not key compromise
// — the attacker doesn't learn the resulting key either), but it means
// the two parties end up holding different secrets with no way to
// detect it. Comparing against the leader's published confirmation id
// closes this: a mismatch is now a hard failure, and the bad result is
// never written to cache or disk.
func (m *Manager) GetOrAccept(leaderID int) (*big.Int, error) {
	if !(leaderID < m.bankID) {
		return nil, fmt.Errorf("bank %d cannot accept bank %d as leader: the lower id always encapsulates (Fix M-11) — call GetOrEstablish(%d, ...) instead",
			m.bankID, leaderID, leaderID)
	}

	key := pairKey(leaderID, m.bankID)
	if ss := m.loadCached(key); ss != nil {
		return ss, nil
	}

	ctPath := filepath.Join(m.storeDir, fmt.Sprintf("ct_%d_to_%d.bin", leaderID, m.bankID))
	ct, err := os.ReadFile(ctPath)
	if err != nil {
		return nil, fmt.Errorf("read ciphertext from leader %d: %w (call GetOrEstablish first)", leaderID, err)
	}
	rawSS, err := mlkem768.Decapsulate(m.dk, ct)
	if err != nil {
		return nil, fmt.Errorf("decapsulate ciphertext from leader %d: %w", leaderID, err)
	}

	// Fix M-11 defect 2: verify the leader's published confirmation
	// BEFORE deriving/caching anything a caller could observe. A missing
	// id file (leader hasn't published one — an old ciphertext from
	// before this fix, or a leader that skipped GetOrEstablish) is
	// treated the same as a mismatch: fail closed, never assume success.
	idPath := confirmationIDPath(m.storeDir, leaderID, m.bankID)
	wantHex, err := os.ReadFile(idPath)
	if err != nil {
		return nil, fmt.Errorf("read confirmation id from leader %d: %w (Fix M-11: refusing to trust an unconfirmed decapsulation)", leaderID, err)
	}
	want, err := hex.DecodeString(strings.TrimSpace(string(wantHex)))
	if err != nil {
		return nil, fmt.Errorf("parse confirmation id from leader %d: %w", leaderID, err)
	}
	got := confirmationID(rawSS)
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return nil, fmt.Errorf("confirmation id mismatch with leader %d (Fix M-11): decapsulation did not reproduce the leader's shared secret — "+
			"this is FIPS 203 implicit rejection (a corrupted or malicious ciphertext), not caching it", leaderID)
	}

	ss := toFieldElement(rawSS)
	m.persistSecret(key, ss)
	return ss, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (m *Manager) loadOrCreate() error {
	seedPath := filepath.Join(m.storeDir, fmt.Sprintf("bank_%d_dk.seed", m.bankID))
	ekPath := filepath.Join(m.storeDir, fmt.Sprintf("bank_%d_ek.bin", m.bankID))

	seed, err := os.ReadFile(seedPath)
	if err == nil {
		dk, err := mlkem768.NewKeyFromSeed(seed)
		if err != nil {
			return fmt.Errorf("load view key for bank %d: %w", m.bankID, err)
		}
		m.dk = dk
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("read seed for bank %d: %w", m.bankID, err)
	}

	dk, err := mlkem768.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate view key for bank %d: %w", m.bankID, err)
	}
	if err := os.WriteFile(seedPath, dk.Bytes(), 0600); err != nil {
		return fmt.Errorf("save seed for bank %d: %w", m.bankID, err)
	}
	if err := os.WriteFile(ekPath, dk.EncapsulationKey(), 0644); err != nil {
		return fmt.Errorf("save encapsulation key for bank %d: %w", m.bankID, err)
	}
	m.dk = dk
	return nil
}

// loadCached checks the in-memory cache then disk for a previously derived secret.
func (m *Manager) loadCached(key string) *big.Int {
	m.mu.RLock()
	ss := m.cache[key]
	m.mu.RUnlock()
	if ss != nil {
		return ss
	}
	ssPath := filepath.Join(m.storeDir, fmt.Sprintf("ss_%s.txt", key))
	data, err := os.ReadFile(ssPath)
	if err != nil {
		return nil
	}
	n, ok := new(big.Int).SetString(strings.TrimSpace(string(data)), 10)
	if !ok {
		return nil
	}
	m.mu.Lock()
	m.cache[key] = n
	m.mu.Unlock()
	return n
}

// persistSecret stores ss in the in-memory cache and writes it to disk.
func (m *Manager) persistSecret(key string, ss *big.Int) {
	m.mu.Lock()
	m.cache[key] = ss
	m.mu.Unlock()
	ssPath := filepath.Join(m.storeDir, fmt.Sprintf("ss_%s.txt", key))
	_ = os.WriteFile(ssPath, []byte(ss.String()), 0600)
}

// pairKey returns a canonical "min_max" string for a pair (a, b).
// The lower ID is always first so both participants reference the same key.
func pairKey(a, b int) string {
	if a < b {
		return fmt.Sprintf("%d_%d", a, b)
	}
	return fmt.Sprintf("%d_%d", b, a)
}

// toFieldElement maps a 32-byte ML-KEM shared key to a Baby JubJub field element.
// Applies SHA-256 with a domain tag then reduces modulo curveP.
func toFieldElement(rawSS []byte) *big.Int {
	h := sha256.Sum256(append([]byte("enygma-view-key-v1:"), rawSS...))
	n := new(big.Int).SetBytes(h[:])
	return n.Mod(n, curveP)
}

// confirmationID computes id = Hash(rawSS) — the mutual-confirmation
// value protocol_description.md specifies (Fix M-11 defect 2). Domain-
// separated from toFieldElement's tag so the two hashes can never
// collide with each other by construction.
func confirmationID(rawSS []byte) []byte {
	h := sha256.Sum256(append([]byte("enygma-view-key-confirm-v1:"), rawSS...))
	return h[:]
}

// confirmationIDPath is the on-disk location of a leader→follower pair's
// published confirmation id.
func confirmationIDPath(storeDir string, leaderID, followerID int) string {
	return filepath.Join(storeDir, fmt.Sprintf("id_%d_to_%d.txt", leaderID, followerID))
}
