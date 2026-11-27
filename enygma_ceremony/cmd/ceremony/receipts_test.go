package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"enygma_ceremony/ceremony"
)

// TestReceiptCountsUnderTheHead (audit H2): a coordinator can serve one
// participant a chain of their own, valid from the real start, holding their
// contribution, while the chain it announces and seals is another. verify
// -head finds the receipt only in the chain ending at the announced head,
// fetched from a copy published apart from the coordinator (-static); and
// a seal with -include refuses a chain without the receipts of the
// contributors lined up.
func TestReceiptCountsUnderTheHead(t *testing.T) {
	// Two chains from the same start: the one alice is shown, and the one
	// announced, which holds someone else's contribution only.
	fork, forkURL := openPhase(t)
	announced, announcedURL := openPhase(t)
	if code, stdout, stderr := runCommand("join", "-url", forkURL, "-name", "alice"); code != 0 {
		t.Fatalf("alice's join: %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("join", "-url", announcedURL, "-name", "bob"); code != 0 {
		t.Fatalf("bob's join: %d\n%s%s", code, stdout, stderr)
	}
	alice, other := sha256Hex(t, filepath.Join(fork, "state-1")), sha256Hex(t, filepath.Join(announced, "state-1"))
	head := other
	published := httptest.NewServer(http.FileServer(http.Dir(announced)))
	t.Cleanup(published.Close)

	for _, c := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"alice's receipt, in the chain announced", []string{"-static", published.URL, "-head", head, "-mine", alice}, 1, "not in the chain checked: " + alice},
		{"the chain alice is shown, which does not end at the head", []string{"-url", forkURL, "-head", head, "-mine", alice}, 1, "not at the announced head"},
		{"the chain alice is shown, without -head", []string{"-url", forkURL, "-mine", alice}, 0, "Without -head"},
		{"a receipt in the chain announced", []string{"-static", published.URL, "-head", head, "-mine", other}, 0, "latest sha256 " + head + ", the announced head"},
		{"two receipts, one missing", []string{"-static", published.URL, "-head", head, "-mine", other + "," + alice}, 1, "not in the chain checked: " + alice},
		{"both -url and -static", []string{"-url", forkURL, "-static", published.URL}, 2, "at most one of -url and -static"},
		{"two heads", []string{"-static", published.URL, "-head", head + "," + alice}, 2, "-head is the sha256"},
		{"a -mine that is no sha256", []string{"-static", published.URL, "-mine", "mine"}, 2, "-mine"},
	} {
		code, stdout, stderr := runCommand(append([]string{"verify", "-dir", t.TempDir()}, c.args...)...)
		if code != c.code || !strings.Contains(stdout+stderr, c.want) {
			t.Errorf("%s: exit %d, want %d and %q\n%s%s", c.name, code, c.code, c.want, stdout, stderr)
		}
	}

	t.Run("a seal refuses a chain without the receipts lined up", func(t *testing.T) {
		seal := func(include string) (int, string, string) {
			return runCommand("phase1-seal", "-transcript", announced, "-head", head, "-include", include,
				"-unverified-beacon", strings.Repeat("a5", 32), "-out", filepath.Join(t.TempDir(), "srs"))
		}
		if code, stdout, stderr := seal(alice); code != 1 || !strings.Contains(stderr, "must be in the chain") {
			t.Fatalf("-include alice's receipt: exit %d, want 1\n%s%s", code, stdout, stderr)
		}
		// Said before the beacon is fetched, or the chain replayed: here
		// the beacon would be refused, as a round out before the close.
		code, stdout, stderr := runCommand("phase1-seal", "-transcript", announced, "-head", head, "-include", alice,
			"-drand", "1", "-close-at", "2026-01-01T00:00:00Z", "-out", filepath.Join(t.TempDir(), "srs"))
		if code != 1 || !strings.Contains(stderr, "must be in the chain") {
			t.Fatalf("-include alice's receipt, with a beacon refused later: exit %d, want 1 at once\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := seal("not-a-sha256"); code != 2 {
			t.Fatalf("-include of no sha256: exit %d, want 2\n%s%s", code, stdout, stderr)
		}
		if code, stdout, stderr := seal(""); code != 0 || !strings.Contains(stdout, "No -include") {
			t.Fatalf("no -include: exit %d, want 0 and a warning\n%s%s", code, stdout, stderr)
		}
		out := filepath.Join(t.TempDir(), "srs")
		code, stdout, stderr = runCommand("phase1-seal", "-transcript", announced, "-head", head, "-include", other,
			"-unverified-beacon", strings.Repeat("a5", 32), "-out", out)
		if code != 0 {
			t.Fatalf("-include a receipt in the chain: exit %d\n%s%s", code, stdout, stderr)
		}
		var record struct {
			Included []string `json:"included"`
		}
		if err := json.Unmarshal(readFile(t, out+".json"), &record); err != nil || len(record.Included) != 1 || record.Included[0] != other {
			t.Fatalf("the record says included %v (%v), want [%s]", record.Included, err, other)
		}
	})
}

// TestContributePrev (audit M3): contribute -prev refuses a state handed
// over other than the one named, keeping no contribution to publish, and
// says, as join does, what the receipt needs to count.
func TestContributePrev(t *testing.T) {
	dir := t.TempDir()
	p := func(name string) string { return filepath.Join(dir, name) }
	if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", p("p1-0")); code != 0 {
		t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
	}
	wrong := strings.Repeat("00", 32)
	if code, stdout, stderr := runCommand("contribute", "-in", p("p1-0"), "-out", p("p1-1"), "-prev", wrong+","+wrong); code != 2 {
		t.Fatalf("contribute with two -prev: exit %d, want 2\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("contribute", "-in", p("p1-0"), "-out", p("p1-1"), "-prev", wrong); code != 1 || !strings.Contains(stderr, "not the "+wrong) ||
		strings.Contains(stdout, "Contributing.") { // refused before contributing
		t.Fatalf("contribute to another state than -prev: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
	if _, err := os.Stat(p("p1-1")); err == nil {
		t.Fatal("a refused contribution was kept")
	}
	code, stdout, stderr := runCommand("contribute", "-in", p("p1-0"), "-out", p("p1-1"), "-prev", sha256Hex(t, p("p1-0")))
	if code != 0 || !strings.Contains(stdout, "-head <the announced head>") {
		t.Fatalf("contribute -prev: exit %d\n%s%s", code, stdout, stderr)
	}
}

// TestFollowHTTPS: a static copy may be served through redirects, as
// release downloads and object storage serve files, over HTTPS only.
func TestFollowHTTPS(t *testing.T) {
	req := func(u string) *http.Request {
		r, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if err := followHTTPS(req("https://objects.example/state-1"), nil); err != nil {
		t.Errorf("a redirect over HTTPS: %v", err)
	}
	if err := followHTTPS(req("http://objects.example/state-1"), nil); err == nil {
		t.Error("a redirect to plain HTTP was followed")
	}
	if err := followHTTPS(req("https://objects.example/state-1"), make([]*http.Request, 10)); err == nil {
		t.Error("an eleventh redirect was followed")
	}
	if err := staticClient().CheckRedirect(req("https://objects.example/state-1"), nil); err != nil {
		t.Errorf("verify -static's client refuses a redirect over HTTPS: %v", err)
	}
}

// TestCheckAdvice (audit H2): the check printed for a receipt is the one
// that counts, with what a Phase 2 needs, or says so when the phase is not
// known.
func TestCheckAdvice(t *testing.T) {
	var h [32]byte
	for kind, want := range map[ceremony.Kind][2]bool{ // [-srs-sha256 in the command, a note for a Phase 2]
		ceremony.KindPhase1: {false, false},
		ceremony.KindPhase2: {true, false},
		0:                   {false, true},
	} {
		got := checkAdvice(h, kind)
		command := got[strings.Index(got, "ceremony verify"):]
		command = command[:strings.Index(command, "\n")]
		if strings.Contains(command, "-srs-sha256") != want[0] || strings.Contains(got, "(for a Phase 2") != want[1] || !strings.Contains(command, "-head <the announced head>") {
			t.Errorf("kind %d: %q", kind, got)
		}
	}
}
