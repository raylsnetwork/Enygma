package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/testcircuits"
)

// phase1Transcript runs an open Phase 1 at domain 8 with two contributions
// and seals it for a rehearsal; it returns the transcript's directory, the
// SRS and its sha256.
func phase1Transcript(t *testing.T) (dir, srs, srsHash string) {
	t.Helper()
	dir, url := openPhase(t)
	for range 2 {
		if code, stdout, stderr := runCommand("join", "-url", url); code != 0 {
			t.Fatalf("join: %d\n%s%s", code, stdout, stderr)
		}
	}
	srs = filepath.Join(t.TempDir(), "phase1.srs")
	head := sha256Hex(t, filepath.Join(dir, "state-2"))
	if code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", head, "-unverified-beacon", strings.Repeat("a5", 32), "-out", srs); code != 0 {
		t.Fatalf("phase1-seal: %d\n%s%s", code, stdout, stderr)
	}
	return dir, srs, sha256Hex(t, srs)
}

// cubic writes the Cubic test circuit alone into a new directory, which
// fits Phase 1 at domain 8.
func cubic(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := testcircuits.WriteCircuit(dir, "Cubic", &testcircuits.Cubic{}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestPhase2NamesItsPhase1 (audit H1): a Phase 2 starts, is verified and is
// sealed only from the SRS announced for it, by its sha256, or from the
// seal of the Phase 1 transcript given (-phase1); an SRS another Phase 1
// sealed, even one claiming the announced head, is refused; and the SRS,
// the seal record and info say which Phase 1 it is.
func TestPhase2NamesItsPhase1(t *testing.T) {
	p1, srs, srsHash := phase1Transcript(t)
	r1cs := cubic(t)
	p2 := t.TempDir()
	start := filepath.Join(p2, "state-0")
	newPhase2 := func(args ...string) (int, string, string) {
		return runCommand(append([]string{"phase2-new", "-ceremony", "test", "-r1cs", r1cs, "-circuits", "Cubic", "-out", start}, args...)...)
	}

	// Another Phase 1, sealed, with the announced head written over its own:
	// an SRS whose provenance claims the announced Phase 1.
	other, _, _ := phase1Transcript(t)
	forged := filepath.Join(t.TempDir(), "forged.srs")
	if code, stdout, stderr := runCommand("phase1-seal", "-transcript", other, "-head", sha256Hex(t, filepath.Join(other, "state-2")),
		"-unverified-beacon", strings.Repeat("a5", 32), "-out", forged); code != 0 {
		t.Fatalf("phase1-seal: %d\n%s%s", code, stdout, stderr)
	}
	data := readFile(t, forged)
	headAt := len("ENYGMA-CEREMONY\x02") + 1 + 8 // after the header and the domain
	copy(data[headAt:headAt+32], readFileHash(t, filepath.Join(p1, "state-2")))
	if err := os.WriteFile(forged, data, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("phase2-new takes only the SRS announced", func(t *testing.T) {
		if code, stdout, stderr := newPhase2("-srs", srs, "-rehearsal"); code != 2 || !strings.Contains(stderr, "-srs-sha256") {
			t.Fatalf("no -srs-sha256: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := newPhase2("-srs", srs, "-srs-sha256", srsHash[:62], "-rehearsal"); code != 2 {
			t.Fatalf("a -srs-sha256 too short: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := newPhase2("-srs", filepath.Join(p1, "state-1"), "-srs-sha256", srsHash, "-rehearsal"); code != 1 || !strings.Contains(stderr, "not an SRS") {
			t.Fatalf("a Phase 1 state for the SRS: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		// The wrong file is refused before a seal loads its chain or fetches
		// its beacon: here there is no chain to load.
		code, stdout, stderr := runCommand("phase2-seal", "-ceremony", "test", "-srs", forged, "-srs-sha256", srsHash, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic",
			"-unverified-beacon", strings.Repeat("5a", 32), "-head", srsHash, "-transcript", filepath.Join(t.TempDir(), "none"), "-keys", t.TempDir())
		if code != 1 || !strings.Contains(stderr, "not the SRS announced") {
			t.Fatalf("phase2-seal from another SRS: exit %d, want 1 at once\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := newPhase2("-srs", forged, "-srs-sha256", srsHash, "-rehearsal"); code != 1 || !strings.Contains(stderr, "not the SRS announced") {
			t.Fatalf("another SRS: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := newPhase2("-srs", srs, "-srs-sha256", srsHash); code != 1 || !strings.Contains(stderr, "rehearsal") {
			t.Fatalf("a rehearsal's SRS, not told so: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		code, stdout, stderr = newPhase2("-srs", srs, "-srs-sha256", srsHash, "-rehearsal")
		if code != 0 || !strings.Contains(stdout, "the seal of Phase 1 up to head "+sha256Hex(t, filepath.Join(p1, "state-2"))+" (2 contributions)") {
			t.Fatalf("phase2-new: exit %d\n%s%s", code, stdout, stderr)
		}
	})
	t.Run("verify -phase1 reseals Phase 1, and takes its seal only", func(t *testing.T) {
		announced := sha256Hex(t, filepath.Join(p1, "state-2"))
		code, stdout, stderr := runCommand("verify", "-dir", p2, "-srs", srs, "-phase1", p1, "-phase1-head", announced, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic")
		if code != 0 || !strings.Contains(stdout, "Resealed Phase 1") || !strings.Contains(stdout, "OK: test's Phase 2, on the SRS of sha256 "+srsHash) {
			t.Fatalf("verify -phase1: exit %d\n%s%s", code, stdout, stderr)
		}
		code, stdout, stderr = runCommand("verify", "-dir", p2, "-srs", forged, "-phase1", p1, "-phase1-head", announced, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic")
		if code != 1 || !strings.Contains(stderr, "is not that Phase 1's seal") {
			t.Fatalf("verify -phase1 of a forged SRS: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		// The SRS of another Phase 1, given with that Phase 1's own
		// transcript, reseals to itself: only the announced head tells it
		// from the announced Phase 1's seal.
		otherSRS := filepath.Join(t.TempDir(), "other.srs")
		if code, stdout, stderr := runCommand("phase1-seal", "-transcript", other, "-head", sha256Hex(t, filepath.Join(other, "state-2")),
			"-unverified-beacon", strings.Repeat("a5", 32), "-out", otherSRS); code != 0 {
			t.Fatalf("phase1-seal: %d\n%s%s", code, stdout, stderr)
		}
		code, stdout, stderr = runCommand("verify", "-dir", p2, "-srs", otherSRS, "-phase1", other, "-phase1-head", announced, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic")
		if code != 1 || !strings.Contains(stderr, "not the announced "+announced) {
			t.Fatalf("verify -phase1 of another Phase 1 with its own transcript: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := runCommand("verify", "-dir", p2, "-srs", srs, "-phase1", p1, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic"); code != 2 {
			t.Fatalf("verify -phase1 without -phase1-head: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := runCommand("verify", "-dir", p2, "-srs", srs, "-r1cs", r1cs, "-circuits", "Cubic"); code != 2 {
			t.Fatalf("verify -srs alone: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := runCommand("verify", "-dir", p2, "-srs", srs, "-srs-sha256", srsHash, "-phase1", p1, "-phase1-head", announced, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic"); code != 2 {
			t.Fatalf("verify with both -srs-sha256 and -phase1: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := runCommand("verify", "-dir", p2, "-srs", srs, "-phase1", p1, "-phase1-head", announced, "-r1cs", r1cs, "-circuits", "Cubic"); code != 1 || !strings.Contains(stderr, "rehearsal") {
			t.Fatalf("verify -phase1 of a rehearsal's SRS, not told so: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
	})
	t.Run("info says what an SRS and a Phase 2 build on", func(t *testing.T) {
		_, stdout, _ := runCommand("info", srs)
		if !strings.Contains(stdout, "the seal of Phase 1 up to head") || !strings.Contains(stdout, "a rehearsal's") {
			t.Fatalf("info of the SRS:\n%s", stdout)
		}
		_, stdout, _ = runCommand("info", start)
		if !strings.Contains(stdout, "on the SRS of sha256 "+srsHash) || !strings.Contains(stdout, "circuit Cubic: constraint system sha256 "+sha256Hex(t, filepath.Join(r1cs, "Cubic.r1cs"))) {
			t.Fatalf("info of the Phase 2 start:\n%s", stdout)
		}
	})
	t.Run("the seal record names Phase 1", func(t *testing.T) {
		keys := t.TempDir()
		next := filepath.Join(p2, "p2-1")
		if code, stdout, stderr := runCommand("contribute", "-in", start, "-out", next); code != 0 || !strings.Contains(stdout, "on the SRS of sha256 "+srsHash) {
			t.Fatalf("contribute: exit %d\n%s%s", code, stdout, stderr)
		}
		code, stdout, stderr := runCommand("phase2-seal", "-ceremony", "test", "-srs", srs, "-srs-sha256", srsHash, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic",
			"-unverified-beacon", strings.Repeat("5a", 32), "-head", sha256Hex(t, next), "-keys", keys, next)
		if code != 0 {
			t.Fatalf("phase2-seal: exit %d\n%s%s", code, stdout, stderr)
		}
		var record struct {
			SRS    string `json:"srs_sha256"`
			Phase1 *struct {
				Head          string `json:"head"`
				Contributions uint32 `json:"contributions"`
				Unverified    bool   `json:"unverified"`
			} `json:"phase1"`
		}
		if err := json.Unmarshal(readFile(t, filepath.Join(keys, "ceremony-seal.json")), &record); err != nil {
			t.Fatal(err)
		}
		if record.SRS != srsHash || record.Phase1 == nil || record.Phase1.Head != sha256Hex(t, filepath.Join(p1, "state-2")) ||
			record.Phase1.Contributions != 2 || !record.Phase1.Unverified {
			t.Fatalf("the seal recorded SRS %s, Phase 1 %+v", record.SRS, record.Phase1)
		}
	})
}

// readFileHash is the sha256 of the file at path.
func readFileHash(t *testing.T, path string) []byte {
	h := sha256.Sum256(readFile(t, path))
	return h[:]
}

// TestVerifyChecksThePhaseAskedFor (audit I3): verify says which phase it
// checked, and refuses one other than it was asked for: a Phase 1 given the
// SRS and circuits of a Phase 2, another domain, or another protocol.
func TestVerifyChecksThePhaseAskedFor(t *testing.T) {
	p1, srs, srsHash := phase1Transcript(t)
	r1cs := cubic(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-srs", srs, "-srs-sha256", srsHash, "-rehearsal", "-r1cs", r1cs, "-circuits", "Cubic"}, "-srs and -r1cs are for a Phase 2"},
		{[]string{"-size", "16"}, "not Phase 1 for circuits of up to 16 constraints"},
		{[]string{"-ceremony", "enygma_retail_payments"}, "not enygma_retail_payments's Phase 2"},
	} {
		code, stdout, stderr := runCommand(append([]string{"verify", "-dir", p1}, c.args...)...)
		if code != 1 || !strings.Contains(stderr, c.want) || strings.Contains(stdout, "OK:") {
			t.Errorf("verify %v: exit %d\n%s%s", c.args, code, stdout, stderr)
		}
	}
	if code, stdout, stderr := runCommand("verify", "-dir", p1, "-size", "8"); code != 0 || !strings.Contains(stdout, "OK: Phase 1 for circuits of up to 8 constraints") {
		t.Errorf("verify -size 8: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("verify", "-dir", p1, "-size", "8", "-ceremony", "test"); code != 2 {
		t.Errorf("verify -size and -ceremony: exit %d, want 2\n%s%s", code, stdout, stderr)
	}
}

// TestSealCommand (audit H1): the seal command coordinate prints at the end
// takes the flags the seal needs: a Phase 2's names the SRS by the sha256
// its start records; a Phase 1's says where to write the SRS.
func TestSealCommand(t *testing.T) {
	p1, srs, srsHash := phase1Transcript(t)
	if got := sealCommand(mustInspect(t, filepath.Join(p1, "state-0"))); !strings.Contains(got, "phase1-seal -out ") {
		t.Errorf("Phase 1: %q", got)
	}
	start := filepath.Join(t.TempDir(), "state-0")
	if code, stdout, stderr := runCommand("phase2-new", "-ceremony", "test", "-srs", srs, "-srs-sha256", srsHash, "-rehearsal", "-r1cs", cubic(t), "-circuits", "Cubic", "-out", start); code != 0 {
		t.Fatalf("phase2-new: exit %d\n%s%s", code, stdout, stderr)
	}
	if got := sealCommand(mustInspect(t, start)); !strings.Contains(got, "phase2-seal -ceremony test ") || !strings.Contains(got, "-srs-sha256 "+srsHash) {
		t.Errorf("Phase 2: %q", got)
	}
	if got := phaseFlags(mustInspect(t, start)); !strings.Contains(got, "-ceremony test ") || !strings.Contains(got, "-srs-sha256 "+srsHash) || !strings.Contains(got, "-r1cs") {
		t.Errorf("Phase 2's verify flags: %q", got)
	}
	if got := phaseFlags(mustInspect(t, filepath.Join(p1, "state-0"))); got != "-size 8" {
		t.Errorf("Phase 1's verify flags: %q", got)
	}
}

// mustInspect describes the state file at path.
func mustInspect(t *testing.T, path string) ceremony.Info {
	t.Helper()
	i, err := ceremony.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	return i
}
