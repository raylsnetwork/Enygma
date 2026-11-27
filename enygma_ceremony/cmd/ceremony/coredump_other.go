//go:build !(darwin || linux)

package main

// disableCoreDumps does nothing on systems without RLIMIT_CORE.
func disableCoreDumps() {}
