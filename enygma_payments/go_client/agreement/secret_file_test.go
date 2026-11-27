//go:build unix

package agreement

import (
	"errors"
	"io"
	"math/big"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func requireSecretWriteFailure(t *testing.T, role string, agree func() (*big.Int, error)) {
	t.Helper()
	for attempt := 1; attempt <= 2; attempt++ {
		if ss, err := agree(); err == nil || ss != nil {
			t.Fatalf("%s accepted failed secret write on attempt %d", role, attempt)
		}
	}
}

func TestSecretWriteFailure(t *testing.T) {
	// Limit only the child. Its write can store one digit before it runs out of space.
	if dir := os.Getenv("ENYGMA_TEST_FILE_SIZE_LIMIT_DIR"); dir != "" {
		signal.Ignore(syscall.SIGXFSZ)
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 1, Max: 1}); err != nil {
			t.Fatal(err)
		}
		follower, err := New(2, dir)
		if err != nil {
			t.Fatal(err)
		}
		if ss, err := follower.GetOrAccept(1); ss != nil || !errors.Is(err, syscall.EFBIG) {
			t.Fatalf("want file-size limit error from secret write, got %v", err)
		}
		return
	}

	leaderDir, followerDir := t.TempDir(), t.TempDir()
	leader, err := New(1, leaderDir)
	if err != nil {
		t.Fatal(err)
	}
	follower, err := New(2, followerDir)
	if err != nil {
		t.Fatal(err)
	}

	// A directory at the destination blocks the leader's secret file only.
	leaderPath := filepath.Join(leaderDir, "ss_1_2.txt")
	if err := os.Mkdir(leaderPath, 0700); err != nil {
		t.Fatal(err)
	}
	requireSecretWriteFailure(t, "leader", func() (*big.Int, error) {
		return leader.GetOrEstablish(2, follower.EncapsulationKey())
	})
	if _, err := os.Stat(filepath.Join(leaderDir, "id_1_to_2.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("confirmation available after failed leader save: %v", err)
	}
	if err := os.Remove(leaderPath); err != nil {
		t.Fatal(err)
	}
	if _, err := leader.GetOrEstablish(2, follower.EncapsulationKey()); err != nil {
		t.Fatal(err)
	}
	transmit(t, leaderDir, followerDir, "ct_1_to_2.bin")
	transmit(t, leaderDir, followerDir, "id_1_to_2.txt")

	// Limit only the child so the rest of the test process can write normally.
	cmd := exec.Command(os.Args[0], "-test.run=^TestSecretWriteFailure$")
	cmd.Env = append(os.Environ(), "ENYGMA_TEST_FILE_SIZE_LIMIT_DIR="+followerDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("file-size limit test: %v\n%s", err, output)
	}

	// Root can write to a read-only directory. Block the destination itself
	// as well in that case so both callers exercise a real filesystem error.
	if os.Geteuid() == 0 {
		if err := os.Mkdir(filepath.Join(followerDir, "ss_1_2.txt"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(followerDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(followerDir, 0700); err != nil {
			t.Error(err)
		}
	})
	requireSecretWriteFailure(t, "follower", func() (*big.Int, error) { return follower.GetOrAccept(1) })
}

func TestSecretFileReplacement(t *testing.T) {
	dir := t.TempDir()
	leader, err := New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	follower, err := New(2, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// An empty legacy file is a cache miss. A reader may already have it open.
	path := filepath.Join(dir, "ss_1_2.txt")
	legacy, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if err := legacy.Chmod(0644); err != nil {
		t.Fatal(err)
	}

	// Force a newly created file to start at 0400, exercising the chmod.
	mask := syscall.Umask(0200)
	ss, err := leader.GetOrEstablish(2, follower.EncapsulationKey())
	syscall.Umask(mask)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("shared secret mode = %04o, want 0600", got)
	}
	if data, err := io.ReadAll(legacy); err != nil || len(data) != 0 {
		t.Errorf("previous reader saw replacement data or read error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != ss.String() {
		t.Errorf("secret file did not contain the established secret: %v", err)
	}
}

func TestLeaderRetriesConfirmationAfterRestart(t *testing.T) {
	dir := t.TempDir()
	leader, err := New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	followerDir := t.TempDir()
	follower, err := New(2, followerDir)
	if err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(dir, "id_1_to_2.txt")
	if err := os.Mkdir(idPath, 0700); err != nil {
		t.Fatal(err)
	}
	requireSecretWriteFailure(t, "leader confirmation", func() (*big.Int, error) {
		return leader.GetOrEstablish(2, follower.EncapsulationKey())
	})
	if err := os.Remove(idPath); err != nil {
		t.Fatal(err)
	}
	leader, err = New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	// The saved secret and pending confirmation suffice, even without the peer key.
	ss, err := leader.GetOrEstablish(2, nil)
	if err != nil {
		t.Fatalf("resume confirmation from disk: %v", err)
	}
	transmit(t, dir, followerDir, "ct_1_to_2.bin")
	transmit(t, dir, followerDir, "id_1_to_2.txt")
	got, err := follower.GetOrAccept(1)
	if err != nil || got.Cmp(ss) != 0 {
		t.Fatalf("follower could not accept the saved leader secret: %v", err)
	}
	cached, err := leader.GetOrEstablish(2, nil)
	if err != nil || cached.Cmp(ss) != 0 {
		t.Fatalf("completed agreement unavailable from cache: %v", err)
	}
}

func TestConcurrentLeadersShareAgreement(t *testing.T) {
	dir := t.TempDir()
	first, err := New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	follower, err := New(2, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Hold the first leader inside its disk-cache read until the second calls
	// GetOrEstablish. Unlinking the FIFO keeps later reads on the normal path.
	path := filepath.Join(dir, "ss_1_2.txt")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	type result struct {
		ss  *big.Int
		err error
	}
	firstDone := make(chan result, 1)
	go func() {
		ss, err := first.GetOrEstablish(2, follower.EncapsulationKey())
		firstDone <- result{ss, err}
	}()
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// The open FIFO pins the first manager in its cache read. An independent
	// descriptor must encounter its lock before we let that read finish.
	probe := flock.New(filepath.Join(dir, "ss_1_2.lock"))
	available, err := probe.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if available {
		if err := probe.Unlock(); err != nil {
			t.Fatal(err)
		}
		t.Error("pair lock available during first leader's cache read")
	}
	secondDone := make(chan result, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		ss, err := second.GetOrEstablish(2, nil)
		secondDone <- result{ss, err}
	}()
	<-started
	var other result
	select {
	case other = <-secondDone:
		t.Errorf("second leader returned while the first was still establishing: %v", other.err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	one := <-firstDone
	if t.Failed() {
		return
	}
	other = <-secondDone
	if one.err != nil || other.err != nil || one.ss.Cmp(other.ss) != 0 {
		t.Fatalf("concurrent leaders did not return the same secret: %v, %v", one.err, other.err)
	}

	// A separate pair's lock cannot be opened. Never establish it unlocked.
	if err := os.Symlink(filepath.Join(dir, "missing", "lock"), filepath.Join(dir, "ss_1_3.lock")); err != nil {
		t.Fatal(err)
	}
	if ss, err := first.GetOrEstablish(3, follower.EncapsulationKey()); ss != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want lock open error, got %v", err)
	}
}

func TestReadOnlyCompletedAgreement(t *testing.T) {
	dir := t.TempDir()
	leader, err := New(1, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ss_1_2.txt"), []byte("42"), 0600); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		// Root bypasses mode bits. Make lock creation fail for it as well.
		if err := os.Symlink(filepath.Join(dir, "missing", "lock"), filepath.Join(dir, "ss_1_2.lock")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	})
	ss, err := leader.GetOrEstablish(2, nil)
	if err != nil || ss.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("could not read completed agreement from read-only store: %v", err)
	}
}
