package agreement

// TestM11_* reproduce and verify the M-11 fixes
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/LATENT):
//
//   defect 1 — no leader rule: both sides of a pair could independently
//     call GetOrEstablish, each persisting a different secret under the
//     same cache key with nothing to detect the divergence.
//   defect 2 — no id=Hash(s) confirmation: FIPS 203 implicit rejection
//     means any correctly-sized ciphertext decapsulates successfully
//     with a pseudorandom key, and nothing invalidated a bad result
//     before caching it.
//   defect 3 — peer keys never authenticated, silently manufactured:
//     see LoadPeerEncapsulationKey's doc comment in manager.go.
//
// Leader and follower use SEPARATE storeDirs in every test below, with
// only the ciphertext/confirmation-id files copied across via transmit()
// — anything less is not a faithful simulation of two hosts that do not
// share a filesystem (which is the whole point of key agreement, and
// exactly the scenario M-11's defects manifest in: sharing one storeDir,
// as the old go_client/transaction/main.go did, is itself defect 3).
//
// Run:
//
//	go test ./agreement/... -v

import (
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// transmit copies filename from srcDir to dstDir, simulating the
// out-of-band channel a real cross-host deployment would use to move a
// ciphertext (and its confirmation id) from the leader to the follower.
func transmit(t *testing.T, srcDir, dstDir, filename string) {
	t.Helper()
	src, err := os.Open(filepath.Join(srcDir, filename))
	if err != nil {
		t.Fatalf("transmit %s: open source: %v", filename, err)
	}
	defer src.Close()
	dst, err := os.Create(filepath.Join(dstDir, filename))
	if err != nil {
		t.Fatalf("transmit %s: create dest: %v", filename, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatalf("transmit %s: copy: %v", filename, err)
	}
}

func TestM11_LeaderMustHaveLowerID(t *testing.T) {
	dirHigh, dirLow := t.TempDir(), t.TempDir()
	high, err := New(5, dirHigh)
	if err != nil {
		t.Fatalf("New(5): %v", err)
	}
	low, err := New(1, dirLow)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}

	// bank 5 trying to lead against bank 1 (higher id encapsulating) must
	// be rejected — pre-fix, this silently produced a divergent secret
	// from whatever bank 1 itself established.
	if _, err := high.GetOrEstablish(1, low.EncapsulationKey()); err == nil {
		t.Fatal("FAIL (M-11 regressed): higher-id bank 5 was allowed to lead against lower-id bank 1")
	} else if !strings.Contains(err.Error(), "lower id always encapsulates") {
		t.Fatalf("GetOrEstablish(wrong direction) rejected, but not for the leader-rule reason: %v", err)
	}
}

func TestM11_FollowerMustHaveHigherID(t *testing.T) {
	dirLow := t.TempDir()
	low, err := New(1, dirLow)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}

	// bank 1 trying to accept bank 5 as leader (lower id following) must
	// be rejected — the mirror image of the above. No ciphertext for
	// this pair even needs to exist; the id-ordering check runs first.
	if _, err := low.GetOrAccept(5); err == nil {
		t.Fatal("FAIL (M-11 regressed): lower-id bank 1 was allowed to follow higher-id bank 5")
	} else if !strings.Contains(err.Error(), "lower id always encapsulates") {
		t.Fatalf("GetOrAccept(wrong direction) rejected, but not for the leader-rule reason: %v", err)
	}
}

func TestM11_HonestRoundTripAgrees(t *testing.T) {
	dirLeader, dirFollower := t.TempDir(), t.TempDir()
	leader, err := New(1, dirLeader)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}
	follower, err := New(2, dirFollower)
	if err != nil {
		t.Fatalf("New(2): %v", err)
	}

	ssLeader, err := leader.GetOrEstablish(2, follower.EncapsulationKey())
	if err != nil {
		t.Fatalf("GetOrEstablish: %v", err)
	}
	transmit(t, dirLeader, dirFollower, "ct_1_to_2.bin")
	transmit(t, dirLeader, dirFollower, "id_1_to_2.txt")

	ssFollower, err := follower.GetOrAccept(1)
	if err != nil {
		t.Fatalf("GetOrAccept: %v", err)
	}
	if ssLeader.Cmp(ssFollower) != 0 {
		t.Fatalf("leader and follower derived different secrets: %s vs %s", ssLeader, ssFollower)
	}
}

// Fix M-11 defect 2: a follower whose ciphertext has been tampered with
// in transit (simulating either active corruption or the FIPS 203
// implicit-rejection case, where Decapsulate itself returns no error at
// all) must hard-fail and never cache the bogus result.
func TestM11_CorruptedCiphertextRejectedNotCached(t *testing.T) {
	dirLeader, dirFollower := t.TempDir(), t.TempDir()
	leader, err := New(1, dirLeader)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}
	follower, err := New(2, dirFollower)
	if err != nil {
		t.Fatalf("New(2): %v", err)
	}
	if _, err := leader.GetOrEstablish(2, follower.EncapsulationKey()); err != nil {
		t.Fatalf("GetOrEstablish: %v", err)
	}
	transmit(t, dirLeader, dirFollower, "ct_1_to_2.bin")
	transmit(t, dirLeader, dirFollower, "id_1_to_2.txt")

	// Flip bytes in the follower's copy of the ciphertext — same size,
	// different content, exactly what FIPS 203 implicit rejection is
	// designed to swallow silently inside Decapsulate itself.
	ctPath := filepath.Join(dirFollower, "ct_1_to_2.bin")
	ct, err := os.ReadFile(ctPath)
	if err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	for i := range ct {
		ct[i] ^= 0xFF
	}
	if err := os.WriteFile(ctPath, ct, 0600); err != nil {
		t.Fatalf("corrupt ciphertext: %v", err)
	}

	if _, err := follower.GetOrAccept(1); err == nil {
		t.Fatal("FAIL (M-11 regressed): GetOrAccept succeeded against a corrupted ciphertext")
	} else if !strings.Contains(err.Error(), "confirmation id mismatch") {
		t.Fatalf("GetOrAccept(corrupted ciphertext) failed, but not with a confirmation mismatch: %v", err)
	}

	// The bad result must not have been cached to disk either — a
	// subsequent, correctly-implemented retry must not silently return
	// the poisoned value from before.
	ssPath := filepath.Join(dirFollower, "ss_1_2.txt")
	if _, err := os.Stat(ssPath); err == nil {
		t.Fatal("FAIL (M-11 regressed): a mismatched decapsulation was persisted to disk")
	}
}

// Fix M-11 defect 2: a follower that never sees the leader's published
// confirmation id at all (e.g. an old ciphertext from before this fix)
// must fail closed, not assume success.
func TestM11_MissingConfirmationIDRejected(t *testing.T) {
	dirLeader, dirFollower := t.TempDir(), t.TempDir()
	leader, err := New(1, dirLeader)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}
	follower, err := New(2, dirFollower)
	if err != nil {
		t.Fatalf("New(2): %v", err)
	}
	if _, err := leader.GetOrEstablish(2, follower.EncapsulationKey()); err != nil {
		t.Fatalf("GetOrEstablish: %v", err)
	}
	// Transmit only the ciphertext, deliberately not the confirmation id.
	transmit(t, dirLeader, dirFollower, "ct_1_to_2.bin")

	if _, err := follower.GetOrAccept(1); err == nil {
		t.Fatal("FAIL (M-11 regressed): GetOrAccept succeeded with no confirmation id published")
	}
}

func TestM11_ConfirmationIDIsHexSHA256(t *testing.T) {
	dirLeader := t.TempDir()
	leader, err := New(1, dirLeader)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}
	follower, err := New(2, t.TempDir())
	if err != nil {
		t.Fatalf("New(2): %v", err)
	}
	if _, err := leader.GetOrEstablish(2, follower.EncapsulationKey()); err != nil {
		t.Fatalf("GetOrEstablish: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dirLeader, "id_1_to_2.txt"))
	if err != nil {
		t.Fatalf("read confirmation id: %v", err)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("confirmation id is not valid hex: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("confirmation id length = %d, want 32 (SHA-256)", len(decoded))
	}
}

// ── LoadPeerEncapsulationKey (defect 3) ──────────────────────────────────────

func TestM11_LoadPeerEncapsulationKey_RefusesMissingPeer(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadPeerEncapsulationKey(dir, 99); err == nil {
		t.Fatal("FAIL (M-11 regressed): LoadPeerEncapsulationKey succeeded for a peer that never published a key")
	}
	// And critically: it must not have fabricated one either.
	if _, err := os.Stat(filepath.Join(dir, "bank_99_dk.seed")); err == nil {
		t.Fatal("FAIL (M-11 regressed): LoadPeerEncapsulationKey generated a private seed for a peer")
	}
	if _, err := os.Stat(filepath.Join(dir, "bank_99_ek.bin")); err == nil {
		t.Fatal("FAIL (M-11 regressed): LoadPeerEncapsulationKey generated an encapsulation key for a peer")
	}
}

func TestM11_LoadPeerEncapsulationKey_ReadsPublishedKey(t *testing.T) {
	dir := t.TempDir()
	peer, err := New(7, dir) // peer legitimately generates its OWN key first
	if err != nil {
		t.Fatalf("New(7): %v", err)
	}
	got, err := LoadPeerEncapsulationKey(dir, 7)
	if err != nil {
		t.Fatalf("LoadPeerEncapsulationKey: %v", err)
	}
	want := peer.EncapsulationKey()
	if len(got) != len(want) {
		t.Fatalf("key length: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("LoadPeerEncapsulationKey returned a different key than peer.EncapsulationKey()")
		}
	}
}

// ── File permissions ──────────────────────────────────────────────────────────

// Fix M-11 remediation's last item: ciphertext files were 0644 (world-
// readable); now 0600, matching the seed/shared-secret files.
func TestM11_CiphertextFileMode0600(t *testing.T) {
	dirLeader := t.TempDir()
	leader, err := New(1, dirLeader)
	if err != nil {
		t.Fatalf("New(1): %v", err)
	}
	follower, err := New(2, t.TempDir())
	if err != nil {
		t.Fatalf("New(2): %v", err)
	}
	if _, err := leader.GetOrEstablish(2, follower.EncapsulationKey()); err != nil {
		t.Fatalf("GetOrEstablish: %v", err)
	}
	info, err := os.Stat(filepath.Join(dirLeader, "ct_1_to_2.bin"))
	if err != nil {
		t.Fatalf("stat ciphertext: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("FAIL (M-11 regressed): ciphertext file mode = %o, want 0600", got)
	}
}
