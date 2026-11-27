//go:build darwin || linux

package coordinator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockDir takes dir's lock (LockFile), or fails if another coordinator holds
// it. The lock is the open file's: closing it, or the process ending, lets it go.
func lockDir(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, LockFile), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%s: another coordinator is serving this phase (it holds %s)", dir, LockFile)
		}
		return nil, fmt.Errorf("locking %s: %w", dir, err)
	}
	return f, nil
}

// syncDir makes the names in dir, as renames left them, survive a crash.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return syscall.Fsync(int(f.Fd()))
}

// freeSpace is the disk space under dir that this process may use; ok is
// false when it cannot be told.
func freeSpace(dir string) (free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}
