package store

import (
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// ── TryBeginPending ───────────────────────────────────────────────────────────

func TestTryBeginPending_FreeKeyIsClaimed(t *testing.T) {
	st := openTestStore(t)
	blocked, cached, err := st.TryBeginPending("key1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Error("expected a never-seen key to be free (blocked=false)")
	}
	if cached != nil {
		t.Error("expected no cached record for a never-seen key")
	}
}

func TestTryBeginPending_FreshPendingBlocksAsInFlight(t *testing.T) {
	st := openTestStore(t)
	if _, _, err := st.TryBeginPending("key1", time.Minute); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	blocked, cached, err := st.TryBeginPending("key1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked || cached != nil {
		t.Errorf("expected blocked=true, cached=nil for a fresh in-flight claim; got blocked=%v cached=%v", blocked, cached)
	}
}

func TestTryBeginPending_StalePendingIsReclaimed(t *testing.T) {
	st := openTestStore(t)
	if _, _, err := st.TryBeginPending("key1", time.Nanosecond); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	time.Sleep(time.Millisecond)
	blocked, cached, err := st.TryBeginPending("key1", time.Nanosecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked || cached != nil {
		t.Errorf("expected a stale pending claim to be reclaimed as free; got blocked=%v cached=%v", blocked, cached)
	}
}

func TestTryBeginPending_MinedReplaysCachedResult(t *testing.T) {
	st := openTestStore(t)
	if _, _, err := st.TryBeginPending("key1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := st.MarkMined("key1", "0xabc", 42, 21000); err != nil {
		t.Fatalf("MarkMined: %v", err)
	}
	blocked, cached, err := st.TryBeginPending("key1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked || cached == nil {
		t.Fatalf("expected blocked=true with a cached record; got blocked=%v cached=%v", blocked, cached)
	}
	if cached.TxHash != "0xabc" || cached.BlockNumber != 42 || cached.GasUsed != 21000 {
		t.Errorf("cached record mismatch: %+v", cached)
	}
}

func TestTryBeginPending_FailedClearsImmediately(t *testing.T) {
	st := openTestStore(t)
	if _, _, err := st.TryBeginPending("key1", time.Hour); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := st.MarkFailed("key1"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	// A long staleness window would still block a fresh Pending claim if
	// MarkFailed didn't clear the record outright — it must be free
	// immediately, not just eventually.
	blocked, cached, err := st.TryBeginPending("key1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked || cached != nil {
		t.Errorf("expected a failed key to be immediately free; got blocked=%v cached=%v", blocked, cached)
	}
}

// TestTryBeginPending_CorruptRecordFailsClosed guards against a corrupt or
// partially-written value (e.g. a crash mid-write) being silently treated as
// a free key and overwritten. A key holding an unparsable value must
// surface an error instead of quietly discarding whatever terminal state
// (possibly a MarkMined result) it represented.
func TestTryBeginPending_CorruptRecordFailsClosed(t *testing.T) {
	st := openTestStore(t)
	err := st.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte("key1"), []byte("not valid json"))
	})
	if err != nil {
		t.Fatalf("seed corrupt record: %v", err)
	}

	blocked, cached, err := st.TryBeginPending("key1", time.Minute)
	if err == nil {
		t.Fatal("expected an error for a corrupt dedup record, got nil (key was silently reclaimed)")
	}
	if blocked || cached != nil {
		t.Errorf("expected blocked=false, cached=nil alongside the error; got blocked=%v cached=%v", blocked, cached)
	}

	// The corrupt value must be left untouched, not overwritten by the
	// failed claim attempt — a human should be able to inspect it.
	var raw []byte
	_ = st.db.View(func(tx *bbolt.Tx) error {
		raw = tx.Bucket(bucketName).Get([]byte("key1"))
		return nil
	})
	if string(raw) != "not valid json" {
		t.Errorf("corrupt record was modified by the failed claim attempt: %q", raw)
	}
}

// ── MarkMined / MarkFailed ────────────────────────────────────────────────────

func TestMarkFailed_ClearsUnknownKeyWithoutError(t *testing.T) {
	st := openTestStore(t)
	if err := st.MarkFailed("never-claimed"); err != nil {
		t.Errorf("unexpected error clearing a never-claimed key: %v", err)
	}
}
