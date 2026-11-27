package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"enygma_ceremony/coordinator"
	"enygma_ceremony/internal/testcircuits"
)

// TestCommandsRunACeremony runs a whole ceremony through the commands, as a
// coordinator and participants would, and checks that the keys it seals prove.
func TestCommandsRunACeremony(t *testing.T) {
	dir := t.TempDir()
	p := func(name string) string { return filepath.Join(dir, name) }
	if err := os.Mkdir(p("r1cs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testcircuits.WriteR1CS(p("r1cs")); err != nil {
		t.Fatal(err)
	}
	beacon := strings.Repeat("a5", 32)
	head := func(name string) string { return fmt.Sprintf("%x", sha256.Sum256(readFile(t, p(name)))) }
	circuits := "Cubic,Committed" // the test circuits' directory has no keys.json

	steps := []func() []string{
		func() []string { return []string{"phase1-new", "-size", "64", "-out", p("p1-0")} },
		func() []string { return []string{"contribute", "-in", p("p1-0"), "-out", p("p1-1")} },
		func() []string { return []string{"check", "-prev", p("p1-0"), "-next", p("p1-1")} },
		func() []string {
			return []string{"phase1-seal", "-unverified-beacon", beacon, "-head", head("p1-1"), "-out", p("srs"), p("p1-1")}
		},
		func() []string {
			return []string{"phase2-new", "-ceremony", "test", "-srs", p("srs"), "-r1cs", p("r1cs"), "-circuits", circuits, "-out", p("p2-0")}
		},
		func() []string { return []string{"contribute", "-in", p("p2-0"), "-out", p("p2-1")} },
		func() []string { return []string{"contribute", "-in", p("p2-1"), "-out", p("p2-2")} },
		func() []string { return []string{"check", "-prev", p("p2-1"), "-next", p("p2-2")} },
		func() []string { return []string{"info", p("p2-2")} },
		func() []string {
			return []string{"phase2-seal", "-ceremony", "test", "-srs", p("srs"), "-r1cs", p("r1cs"), "-circuits", circuits,
				"-unverified-beacon", beacon, "-head", head("p2-2"), "-keys", p("keys"), p("p2-1"), p("p2-2")}
		},
	}
	outputs := map[string]string{}
	for _, step := range steps {
		args := step()
		code, stdout, stderr := runCommand(args...)
		if code != 0 {
			t.Fatalf("ceremony %s: exit %d\n%s%s", strings.Join(args, " "), code, stdout, stderr)
		}
		outputs[strings.Join(args, " ")] = stdout
	}
	if err := testcircuits.CheckKeys(p("keys"), p("r1cs")); err != nil {
		t.Fatal(err)
	}

	t.Run("contribute prints the sha256 of the contribution it wrote", func(t *testing.T) {
		data, err := os.ReadFile(p("p2-2"))
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("your contribution sha256 %x", sha256.Sum256(data))
		printed := outputs[strings.Join([]string{"contribute", "-in", p("p2-1"), "-out", p("p2-2")}, " ")]
		if !strings.Contains(printed, want) {
			t.Fatalf("contribute printed\n%s\nwant a line with %q", printed, want)
		}
	})

	t.Run("check exits 1 on a contribution that does not follow", func(t *testing.T) {
		if code, stdout, stderr := runCommand("check", "-prev", p("p2-0"), "-next", p("p2-2")); code != 1 {
			t.Fatalf("exit %d, want 1\n%s%s", code, stdout, stderr)
		}
	})
}

// TestJoin: join takes a participant through a phase with nothing but its
// address, printing the sha256 the transcript records for their
// contribution; given no coordinator's address, it says so at once.
func TestJoin(t *testing.T) {
	dir, url := openPhase(t)
	for i, name := range []string{"alice", "bob"} {
		code, stdout, stderr := runCommand("join", "-url", url, "-name", name)
		if code != 0 {
			t.Fatalf("%s's join: %d\n%s%s", name, code, stdout, stderr)
		}
		state := filepath.Join(dir, fmt.Sprintf("state-%d", i+1))
		if want := fmt.Sprintf("your contribution sha256 %x", sha256.Sum256(readFile(t, state))); !strings.Contains(stdout, want) {
			t.Fatalf("%s's join printed\n%s\nwant a line with %q", name, stdout, want)
		}
	}
	if code, stdout, stderr := runCommand("join", "-url", "ftp://not.a.coordinator"); code != 1 {
		t.Fatalf("join to no coordinator: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
}

// openPhase starts an open Phase 1 at domain 8 in a new directory and serves it.
func openPhase(t *testing.T) (dir, url string) {
	t.Helper()
	dir = t.TempDir()
	if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
		t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
	}
	c, err := coordinator.New(coordinator.Config{
		Dir: dir, Auth: coordinator.AuthOpen, Turn: time.Minute, Presence: 5 * time.Second, Poll: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(c.Handler())
	t.Cleanup(srv.Close)
	return dir, srv.URL
}

// TestRunWithNoCommandJoinsItsCeremony: a program built for a ceremony needs
// nothing from its participant: run with no command, it joins its
// coordinator, contributes, and prints the sha256 to publish.
func TestRunWithNoCommandJoinsItsCeremony(t *testing.T) {
	dir, url := openPhase(t)
	defaultURL = url
	t.Cleanup(func() { defaultURL = "" })
	code, stdout, stderr := runCommand()
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if want := fmt.Sprintf("your contribution sha256 %x", sha256.Sum256(readFile(t, filepath.Join(dir, "state-1")))); !strings.Contains(stdout, want) {
		t.Fatalf("printed\n%s\nwant a line with %q", stdout, want)
	}

	t.Run("without a ceremony built in, it says how to use it", func(t *testing.T) {
		defaultURL = ""
		if code, _, stderr := runCommand(); code != 2 || !strings.Contains(stderr, "usage:") {
			t.Fatalf("exit %d, %q; want 2 and the usage", code, stderr)
		}
	})
}

// TestVerify: verify downloads a phase's transcript, checks every step, and
// finds a participant's contribution by the sha256 they published.
func TestVerify(t *testing.T) {
	dir, url := openPhase(t)
	for range 2 {
		if code, stdout, stderr := runCommand("join", "-url", url); code != 0 {
			t.Fatalf("join: %d\n%s%s", code, stdout, stderr)
		}
	}
	mine := fmt.Sprintf("%x", sha256.Sum256(readFile(t, filepath.Join(dir, "state-2"))))
	code, stdout, stderr := runCommand("verify", "-url", url, "-dir", t.TempDir(), "-mine", mine)
	if code != 0 || !strings.Contains(stdout, "OK: the start, and 2 contributions") || !strings.Contains(stdout, "Yours is contribution 2") {
		t.Fatalf("verify: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("verify", "-url", url, "-dir", t.TempDir(), "-mine", strings.Repeat("0", 64)); code != 1 {
		t.Fatalf("verify of a contribution nobody made: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
}

// TestBeacon: beacon -close-at names the first drand round out at least -turn
// after the close, once the last turn is over (audit R-031), and a seal takes
// its beacon one way, never both.
func TestBeacon(t *testing.T) {
	code, stdout, stderr := runCommand("beacon", "-close-at", "2026-12-01T14:00:00Z", "-turn", "60m")
	if want := "drand quicknet round 34444612, out at 2026-12-01T15:00:00Z"; code != 0 || !strings.Contains(stdout, want) {
		t.Fatalf("beacon -close-at: exit %d\n%s%s\nwant %q", code, stdout, stderr, want)
	}
	code, stdout, stderr = runCommand("phase1-seal", "-unverified-beacon", strings.Repeat("a5", 32), "-drand", "1000",
		"-head", strings.Repeat("0", 64), "-out", "srs", "p1-1")
	if code != 2 {
		t.Fatalf("phase1-seal with -unverified-beacon and -drand: exit %d, want 2\n%s%s", code, stdout, stderr)
	}
}

// TestCircuitsMustBeNamed (audit R-036): without a keys.json, a Phase 2's
// circuits must be named, and every .r1cs file there must be one of them: a
// stray file never becomes a circuit of the keys.
func TestCircuitsMustBeNamed(t *testing.T) {
	r1cs := filepath.Join(t.TempDir(), "r1cs")
	if err := os.Mkdir(r1cs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testcircuits.WriteR1CS(r1cs); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "p2-0")
	code, stdout, stderr := runCommand("phase2-new", "-ceremony", "test", "-srs", "srs", "-r1cs", r1cs, "-out", out)
	if code != 2 || !strings.Contains(stderr, "-circuits") {
		t.Fatalf("phase2-new with no keys.json and no -circuits: exit %d, want 2\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr = runCommand("phase2-new", "-ceremony", "test", "-srs", "srs", "-r1cs", r1cs, "-circuits", "Cubic", "-out", out)
	if code != 1 || !strings.Contains(stderr, "must be the same circuits") {
		t.Fatalf("phase2-new with a .r1cs that -circuits does not name: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
}

// TestSealBindings: a seal seals the chain up to the head announced at the
// close and no other (audit R-003), with a drand round out after the close
// and the last contribution (R-005), and records what it sealed.
func TestSealBindings(t *testing.T) {
	dir, url := openPhase(t)
	for range 2 {
		if code, stdout, stderr := runCommand("join", "-url", url); code != 0 {
			t.Fatalf("join: %d\n%s%s", code, stdout, stderr)
		}
	}
	head := fmt.Sprintf("%x", sha256.Sum256(readFile(t, filepath.Join(dir, "state-2"))))
	beacon := strings.Repeat("a5", 32)
	srs := filepath.Join(t.TempDir(), "srs")

	t.Run("refuses a chain that does not end at the announced head", func(t *testing.T) {
		first := fmt.Sprintf("%x", sha256.Sum256(readFile(t, filepath.Join(dir, "state-1"))))
		code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", first, "-unverified-beacon", beacon, "-out", srs)
		if code != 1 || !strings.Contains(stderr, "would leave contributions out") {
			t.Fatalf("seal to a head short of the transcript's end: exit %d\n%s%s", code, stdout, stderr)
		}
		code, stdout, stderr = runCommand("phase1-seal", "-head", head, "-unverified-beacon", beacon, "-out", srs, filepath.Join(dir, "state-1"))
		if code != 1 || !strings.Contains(stderr, "would leave contributions out") {
			t.Fatalf("seal of files short of the head: exit %d\n%s%s", code, stdout, stderr)
		}
	})
	t.Run("refuses a drand round out before the close", func(t *testing.T) {
		closes := "2026-12-01T15:00:00Z"
		code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", head, "-drand", "34444611", "-close-at", closes, "-out", srs)
		if code != 1 || !strings.Contains(stderr, "not after the close") {
			t.Fatalf("seal with a round out before the close: exit %d\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", head, "-drand", "34444612", "-out", srs); code != 2 {
			t.Fatalf("-drand without -close-at: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
	})
	t.Run("refuses a drand round out before the last contribution", func(t *testing.T) {
		// The contributions above were accepted just now; this round came out in 2020.
		code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", head, "-drand", "2", "-close-at", "2020-01-01T00:00:00Z", "-out", srs)
		if code != 1 || !strings.Contains(stderr, "not after the last contribution") {
			t.Fatalf("seal with a round out before the last contribution: exit %d\n%s%s", code, stdout, stderr)
		}
	})
	t.Run("seals the transcript to its head, and records it", func(t *testing.T) {
		code, stdout, stderr := runCommand("phase1-seal", "-transcript", dir, "-head", head, "-unverified-beacon", beacon, "-out", srs)
		if code != 0 || !strings.Contains(stdout, "WARNING: -unverified-beacon") {
			t.Fatalf("seal: exit %d\n%s%s", code, stdout, stderr)
		}
		var record struct {
			Kind          string
			Head          string
			Contributions int
			Beacon        struct{ Unverified bool }
		}
		if err := json.Unmarshal(readFile(t, srs+".json"), &record); err != nil {
			t.Fatal(err)
		}
		if record.Kind != "srs" || record.Head != head || record.Contributions != 2 || !record.Beacon.Unverified {
			t.Fatalf("record %+v, want the SRS of 2 contributions to head %s, its beacon unverified", record, head)
		}
	})
}

// readFile reads path or fails the test.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// runCommand runs the command line args and returns its exit code and output.
func runCommand(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}
