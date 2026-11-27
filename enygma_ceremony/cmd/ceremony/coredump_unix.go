//go:build darwin || linux

package main

import "syscall"

// disableCoreDumps keeps this process's memory, and so a contribution's
// randomness, out of any core dump it could leave.
func disableCoreDumps() {
	_ = syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0}) // best effort: without it, the system's own setting stands
}
