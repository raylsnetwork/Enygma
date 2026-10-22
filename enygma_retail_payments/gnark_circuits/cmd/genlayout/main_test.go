package main

import (
	"os"
	"testing"
)

// The committed generated files must be exactly what the current layouts
// produce. On failure, regenerate: cd gnark_circuits && go generate ./cmd/genlayout
func TestGeneratedFilesAreUpToDate(t *testing.T) {
	for path, want := range generate() {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run: go generate ./cmd/genlayout)", err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is stale (run: go generate ./cmd/genlayout)", path)
		}
	}
}
