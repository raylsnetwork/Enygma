package agreement

import (
	"crypto/sha256"
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
//   - The "leader" (first-to-transact) calls GetOrEstablish, which encapsulates
//     to the peer's public key and persists the ciphertext to disk.
//   - The "follower" calls GetOrAccept, which reads the leader's ciphertext and
//     decapsulates using its own private key.
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

// GetOrEstablish is the leader path: encapsulate to peerEK, persist the ciphertext,
// and return the shared secret as a Baby JubJub field element.
// If a secret for this pair already exists (memory or disk), it is returned directly.
//
// Call this from the bank initiating a transaction.
func (m *Manager) GetOrEstablish(peerID int, peerEK []byte) (*big.Int, error) {
	key := pairKey(m.bankID, peerID)
	if ss := m.loadCached(key); ss != nil {
		return ss, nil
	}

	ct, rawSS, err := mlkem768.Encapsulate(peerEK)
	if err != nil {
		return nil, fmt.Errorf("encapsulate to peer %d: %w", peerID, err)
	}
	ctPath := filepath.Join(m.storeDir, fmt.Sprintf("ct_%d_to_%d.bin", m.bankID, peerID))
	if err := os.WriteFile(ctPath, ct, 0644); err != nil {
		return nil, fmt.Errorf("persist ciphertext: %w", err)
	}
	ss := toFieldElement(rawSS)
	m.persistSecret(key, ss)
	return ss, nil
}

// GetOrAccept is the follower path: read the leader's ciphertext from disk and
// decapsulate it using this bank's private key.
// If a secret for this pair already exists (memory or disk), it is returned directly.
//
// Call this from each non-sender bank to verify the shared secret.
// leaderID must have already called GetOrEstablish to produce the ciphertext file.
func (m *Manager) GetOrAccept(leaderID int) (*big.Int, error) {
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
