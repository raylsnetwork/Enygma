//go:build !(darwin || linux)

package coordinator

import (
	"os"
	"path/filepath"
)

// lockDir opens dir's lock file (LockFile) without locking it: a coordinator
// is run on Linux or macOS, where it is locked; here, one coordinator to a
// phase is the operator's to keep.
func lockDir(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, LockFile), os.O_RDWR|os.O_CREATE, 0o644)
}

// syncDir does nothing here: a directory is not synced on its own, and a
// coordinator is run on Linux or macOS.
func syncDir(string) error { return nil }

// freeSpace cannot be told here: turns are not held back for disk space.
func freeSpace(string) (uint64, bool) { return 0, false }
