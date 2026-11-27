package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"enygma_ceremony/ceremony"
)

// TestMain lets the test binary be a contribution's child, as the program is:
// contributeInChild starts os.Executable() again.
func TestMain(m *testing.M) {
	if isChild() {
		os.Exit(runChild(os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// TestContributeInChild (audit R-025): the contribution runs in a child
// process of its own, so the randomness never lives in the participant's
// program, which keeps running. The child reports its progress, and a
// receipt for exactly the contribution it wrote; a state it cannot
// contribute to comes back as an error.
func TestContributeInChild(t *testing.T) {
	dir := t.TempDir()
	start := filepath.Join(dir, "state-0")
	if _, err := ceremony.NewPhase1(start, 8); err != nil {
		t.Fatal(err)
	}
	state := readFile(t, start)
	contribute := contributeInChild(context.Background())

	var out bytes.Buffer
	reports := 0
	r, err := contribute(bytes.NewReader(state), int64(len(state)), &out, func(ceremony.Progress) { reports++ })
	if err != nil {
		t.Fatal(err)
	}
	if r.Hash != sha256.Sum256(out.Bytes()) || r.In != sha256.Sum256(state) || r.Index != 1 {
		t.Fatalf("receipt %+v for a contribution of sha256 %x on top of %x", r, sha256.Sum256(out.Bytes()), sha256.Sum256(state))
	}
	if reports == 0 {
		t.Error("the child reported no progress")
	}
	next := filepath.Join(dir, "state-1")
	if err := os.WriteFile(next, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ceremony.Check(start, next); err != nil {
		t.Fatalf("the child's contribution: %v", err)
	}

	t.Run("a state it cannot contribute to", func(t *testing.T) {
		junk := []byte("not a ceremony state")
		var out bytes.Buffer
		if _, err := contribute(bytes.NewReader(junk), int64(len(junk)), &out, nil); err == nil {
			t.Fatal("the child contributed to junk")
		}
	})
}
