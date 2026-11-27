package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/testcircuits"
	"enygma_ceremony/transcript"
)

// forgedOK is what a transcript's host could write to draw a passing check
// over a failed one: clear the screen, and show verify's verdict.
const forgedOK = "\x1b[2J\x1b[HOK: the start, and 2 contributions, each a valid step on top of the one before it; latest sha256 abab"

// TestVerifyShowsNoServerText (audit M2): a host answering verify with an
// error, for the transcript or for a state, is reported by its status code,
// not by the reason phrase it sent, which it chose: that could draw a
// passing verdict over the failure.
func TestVerifyShowsNoServerText(t *testing.T) {
	for name, transcriptOK := range map[string]bool{"the transcript": false, "a state": true} {
		t.Run(name, func(t *testing.T) {
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if transcriptOK && r.URL.Path == "/v1/transcript" {
					fmt.Fprintf(w, `{"start":%q,"contributions":[]}`, strings.Repeat("ab", 32))
					return
				}
				conn, buf, err := http.NewResponseController(w).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				fmt.Fprintf(buf, "HTTP/1.1 500 %s\r\nContent-Length: 0\r\n\r\n", forgedOK)
				_ = buf.Flush() // the client reads what got through
			}))
			defer host.Close()
			code, stdout, stderr := runCommand("verify", "-url", host.URL, "-dir", t.TempDir())
			if code != 1 || strings.Contains(stdout+stderr, "\x1b") || strings.Contains(stdout+stderr, "OK:") || !strings.Contains(stderr, "500") {
				t.Fatalf("exit %d:\n%s%s", code, stdout, stderr)
			}
		})
	}
}

// mainEnv makes the test binary run main, as the program does, on its
// arguments: TestMainFiltersItsOutput's program.
const mainEnv = "ENYGMA_CEREMONY_TEST_MAIN"

// TestMainFiltersItsOutput (audit M1 to M3): everything the program prints
// goes through termsafe's writer, so a sink that missed termsafe.Line still
// cannot act on the terminal. verify prints the address it is given as it
// is: given one holding an escape, the program prints its text without it.
func TestMainFiltersItsOutput(t *testing.T) {
	cmd := exec.Command(os.Args[0], "verify", "-url", "http://127.0.0.1:1/x\x1b[2Jy", "-dir", t.TempDir())
	cmd.Env = append(os.Environ(), mainEnv+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("verify passed with no coordinator")
	}
	if out := stdout.String() + stderr.String(); strings.Contains(out, "\x1b") || !strings.Contains(stdout.String(), "x[2Jy") {
		t.Fatalf("printed %q", out)
	}
}

// TestErrorsAreOneLine (audit M1 to M3): a command's error is printed as one
// line of plain text: what it carries from elsewhere can neither act on the
// terminal nor start a line that looks like the program's.
func TestErrorsAreOneLine(t *testing.T) {
	dir := t.TempDir()
	if _, err := ceremony.NewPhase1(transcript.Path(dir, 0), 8); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand("verify", "-dir", dir, "-mine", "\r\n"+forgedOK)
	if code != 1 || strings.Contains(stderr, "\x1b") || strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, "ceremony verify: ") {
		t.Fatalf("exit %d, error printed as %q", code, stderr)
	}
}

// sealedSRS makes an SRS from a Phase 1 of one contribution, sealed with a
// test beacon, and the test circuits' constraint systems beside it; it
// returns their paths.
func sealedSRS(t *testing.T) (srs, r1cs string) {
	t.Helper()
	dir := t.TempDir()
	p := func(name string) string { return filepath.Join(dir, name) }
	if err := os.Mkdir(p("r1cs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testcircuits.WriteR1CS(p("r1cs")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"phase1-new", "-size", "64", "-out", p("p1-0")},
		{"contribute", "-in", p("p1-0"), "-out", p("p1-1")},
	} {
		if code, stdout, stderr := runCommand(args...); code != 0 {
			t.Fatalf("%v: exit %d\n%s%s", args, code, stdout, stderr)
		}
	}
	head := fmt.Sprintf("%x", sha256.Sum256(readFile(t, p("p1-1"))))
	if code, stdout, stderr := runCommand("phase1-seal", "-unverified-beacon", strings.Repeat("a5", 32), "-head", head, "-out", p("srs"), p("p1-1")); code != 0 {
		t.Fatalf("phase1-seal: exit %d\n%s%s", code, stdout, stderr)
	}
	return p("srs"), p("r1cs")
}

// TestPhase2NamesArePlain (audit M3): a Phase 2's ceremony and circuit names
// are plain names, which no terminal acts on: phase2-new refuses others
// before it writes or prints anything with them, as decoding a state does.
func TestPhase2NamesArePlain(t *testing.T) {
	srs, r1cs := sealedSRS(t)
	t.Run("a ceremony name", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "p2-0")
		code, stdout, stderr := runCommand("phase2-new", "-ceremony", "retail"+forgedOK, "-srs", srs, "-r1cs", r1cs, "-circuits", "Cubic,Committed", "-out", out)
		if code != 1 || strings.Contains(stdout+stderr, "\x1b") || !strings.Contains(stderr, "must be letters, digits") {
			t.Fatalf("exit %d:\n%s%s", code, stdout, stderr)
		}
		if _, err := os.Stat(out); err == nil {
			t.Fatal("phase2-new wrote a state with that name")
		}
	})
	t.Run("a circuit's file name", func(t *testing.T) {
		dir := t.TempDir()
		hostile := "Cubic" + forgedOK
		if err := os.WriteFile(filepath.Join(dir, hostile+".r1cs"), readFile(t, filepath.Join(r1cs, "Cubic.r1cs")), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runCommand("phase2-new", "-ceremony", "retail", "-srs", srs, "-r1cs", dir, "-circuits", hostile, "-out", filepath.Join(t.TempDir(), "p2-0"))
		if code != 1 || strings.Contains(stdout+stderr, "\x1b") || !strings.Contains(stderr, "must be letters, digits") {
			t.Fatalf("exit %d:\n%s%s", code, stdout, stderr)
		}
	})
}

// TestChildReceiptHasNoNames (audit I15): the child's receipt carries hashes
// and numbers only, not the names in the state it was handed, which the
// coordinator chose: the parent reads the receipt's line whole.
func TestChildReceiptHasNoNames(t *testing.T) {
	srs, r1cs := sealedSRS(t)
	start := filepath.Join(t.TempDir(), "p2-0")
	if code, stdout, stderr := runCommand("phase2-new", "-ceremony", "retail", "-srs", srs, "-r1cs", r1cs, "-circuits", "Cubic,Committed", "-out", start); code != 0 {
		t.Fatalf("phase2-new: exit %d\n%s%s", code, stdout, stderr)
	}
	state := readFile(t, start)
	var out bytes.Buffer
	r, err := contributeInChild(context.Background())(bytes.NewReader(state), int64(len(state)), &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Ceremony != "" || r.Circuits != nil || r.Hash != sha256.Sum256(out.Bytes()) {
		t.Fatalf("receipt %+v", r)
	}
}

// floodEnv makes the test binary, started as a contribution's child, write
// a line past what its parent reads as one, and more after it.
const floodEnv = "ENYGMA_CEREMONY_TEST_FLOOD"

// TestChildOutputIsDrained (audit I15): a child writing a line longer than
// its parent reads as one is read to its end, rather than left blocked on a
// pipe nobody reads while the parent waits for it to exit.
func TestChildOutputIsDrained(t *testing.T) {
	t.Setenv(floodEnv, "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		state := []byte("a state")
		_, err := contributeInChild(ctx)(bytes.NewReader(state), int64(len(state)), &bytes.Buffer{}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "line past 1 MiB") {
			t.Fatalf("got %v, want the child's failure, saying what it wrote", err)
		}
	case <-time.After(time.Minute):
		cancel()
		t.Fatal("the parent and its child are waiting on each other")
	}
}

// flood is the child TestChildOutputIsDrained starts: 2 MiB on one line of
// stderr, then more, then failure.
func flood() int {
	line := append(bytes.Repeat([]byte("x"), 2<<20), '\n')
	_, _ = os.Stderr.Write(line)
	_, _ = os.Stderr.Write(line)
	return 1
}
