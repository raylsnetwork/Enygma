package transcript

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestReadTranscript (audit L10): a transcript is read an entry at a time,
// and refused past MaxEntries, so a host's JSON costs its reader little
// whatever it holds; it holds only a start and a chain of contributions.
func TestReadTranscript(t *testing.T) {
	start := strings.Repeat("ab", 32)
	t.Run("refuses more than MaxEntries", func(t *testing.T) {
		var body bytes.Buffer
		fmt.Fprintf(&body, `{"start":%q,"contributions":[`, start)
		body.WriteString(strings.Repeat("{},", MaxEntries))
		body.WriteString("{}]}")
		if _, err := ReadTranscript(&body); err == nil || !strings.Contains(err.Error(), "more than") {
			t.Fatalf("%d entries: %v, want them refused", MaxEntries+1, err)
		}
	})
	t.Run("reads none, given as null", func(t *testing.T) {
		tr, err := ReadTranscript(strings.NewReader(fmt.Sprintf(`{"start":%q,"contributions":null}`, start)))
		if err != nil || tr.Start != start || len(tr.Contributions) != 0 {
			t.Fatalf("got %+v, %v", tr, err)
		}
	})
	t.Run("refuses what a transcript never holds", func(t *testing.T) {
		if _, err := ReadTranscript(strings.NewReader(fmt.Sprintf(`{"start":%q,"extra":[1,2,3]}`, start))); err == nil {
			t.Fatal("a transcript with another field was read")
		}
	})
	t.Run("refuses JSON of another shape", func(t *testing.T) {
		for _, body := range []string{`[]`, fmt.Sprintf(`{"start":%q,"contributions":{}}`, start)} {
			if _, err := ReadTranscript(strings.NewReader(body)); err == nil {
				t.Errorf("%s was read as a transcript", body)
			}
		}
	})
	t.Run("refuses entries that are no chain", func(t *testing.T) {
		if _, err := ReadTranscript(strings.NewReader(fmt.Sprintf(`{"start":%q,"contributions":[{}]}`, start))); err == nil {
			t.Fatal("an empty entry was read")
		}
	})
}

// TestFetchComparesFirst (audit L13, I4): a host that now serves another
// history than a copy holds, an entry rewritten or a contribution of
// another sha256, is refused before Fetch writes anything, and said to have
// changed its story; with Sync, what it contradicts is moved aside, not
// deleted.
func TestFetchComparesFirst(t *testing.T) {
	ctx := context.Background()
	t.Run("an entry rewritten", func(t *testing.T) {
		src := phase(t, 2, nil)
		url, _ := serve(t, src, nil)
		dir := t.TempDir()
		if _, err := Fetch(ctx, http.DefaultClient, url, dir, io.Discard, FetchOptions{}); err != nil {
			t.Fatal(err)
		}
		saved := readFile(t, Path(dir, 1)+".json")
		tr, err := Load(src)
		if err != nil {
			t.Fatal(err)
		}
		e := tr.Contributions[0]
		e.Participant = "someone else"
		writeEntry(t, src, e)
		rewritten, _ := serve(t, src, nil)
		if _, err := Fetch(ctx, http.DefaultClient, rewritten, dir, io.Discard, FetchOptions{}); err == nil || !strings.Contains(err.Error(), "rewritten its record") {
			t.Fatalf("Fetch of a rewritten entry: %v, want it refused", err)
		}
		if !bytes.Equal(readFile(t, Path(dir, 1)+".json"), saved) {
			t.Fatal("a refused fetch rewrote the saved entry")
		}
	})
	t.Run("an entry that is no entry, or no file", func(t *testing.T) {
		src := phase(t, 1, nil)
		url, _ := serve(t, src, nil)
		for name, plant := range map[string]func(path string) error{
			"rewritten its record": func(path string) error { return os.WriteFile(path, []byte("not JSON"), 0o644) },
			"not a regular file":   func(path string) error { return os.Mkdir(path, 0o755) },
		} {
			dir := t.TempDir()
			if err := plant(Path(dir, 1) + ".json"); err != nil {
				t.Fatal(err)
			}
			if _, err := Fetch(ctx, http.DefaultClient, url, dir, io.Discard, FetchOptions{}); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("Fetch into a directory whose entry 1 is %s: %v, want it refused", name, err)
			}
		}
	})
	t.Run("another phase", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(Path(dir, 0), []byte("another start"), 0o644); err != nil {
			t.Fatal(err)
		}
		url, _ := serve(t, phase(t, 1, nil), nil)
		if _, err := Fetch(ctx, http.DefaultClient, url, dir, io.Discard, FetchOptions{}); err == nil || !strings.Contains(err.Error(), "holds another phase") {
			t.Fatalf("Fetch into a directory of another phase: %v, want it refused", err)
		}
	})
	t.Run("a contribution of another sha256", func(t *testing.T) {
		src := phase(t, 2, nil)
		url, _ := serve(t, src, nil)
		dir := t.TempDir()
		if _, err := Fetch(ctx, http.DefaultClient, url, dir, io.Discard, FetchOptions{}); err != nil {
			t.Fatal(err)
		}
		old := readFile(t, Path(dir, 2))
		for _, path := range []string{Path(src, 2), Path(src, 2) + ".json"} {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		addContribution(t, src, 2, Path(src, 1))
		fork, _ := serve(t, src, nil)
		if _, err := Fetch(ctx, http.DefaultClient, fork, dir, io.Discard, FetchOptions{}); err == nil || !strings.Contains(err.Error(), "changed its story") {
			t.Fatalf("Fetch of a fork: %v, want it refused", err)
		}
		if !bytes.Equal(readFile(t, Path(dir, 2)), old) {
			t.Fatal("a refused fetch replaced state 2")
		}
		if _, err := Fetch(ctx, http.DefaultClient, fork, dir, io.Discard, FetchOptions{Sync: true}); err != nil {
			t.Fatal(err)
		}
		kept, err := filepath.Glob(filepath.Join(dir, "superseded-*", "state-2"))
		if err != nil || len(kept) != 1 || !bytes.Equal(readFile(t, kept[0]), old) {
			t.Fatalf("the contradicted state 2 was not kept aside: %v, %v", kept, err)
		}
	})
}

// TestFetchStatic (audit H2): a phase's directory published as it stands is
// fetched and checked as a coordinator's is, without the coordinator: a
// copy it cannot answer each reader of differently.
func TestFetchStatic(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{0, 3} {
		src := phase(t, n, nil)
		// Object storage answers 403 for a file it does not have; state-0 is
		// downloaded once, though a phase of no contributions names it
		// nowhere else.
		var starts atomic.Int32
		files := http.FileServer(http.Dir(src))
		host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := os.Stat(filepath.Join(src, filepath.Base(r.URL.Path))); err != nil {
				http.Error(w, "AccessDenied", http.StatusForbidden)
				return
			}
			if r.URL.Path == "/state-0" {
				starts.Add(1)
			}
			files.ServeHTTP(w, r)
		}))
		t.Cleanup(host.Close)
		dir := t.TempDir()
		tr, err := Fetch(ctx, http.DefaultClient, host.URL, dir, io.Discard, FetchOptions{Static: true, Check: true})
		if err != nil {
			t.Fatalf("%d contributions: %v", n, err)
		}
		want, err := Load(src)
		if err != nil {
			t.Fatal(err)
		}
		if tr.Start != want.Start || tr.Latest() != want.Latest() || len(tr.Contributions) != n {
			t.Fatalf("%d contributions: fetched %d ending at %s, want %s", n, len(tr.Contributions), tr.Latest(), want.Latest())
		}
		if _, err := Check(dir, io.Discard); err != nil {
			t.Fatalf("%d contributions: Check of the copy: %v", n, err)
		}
		if got := starts.Load(); got != 1 {
			t.Fatalf("%d contributions: state-0 downloaded %d times", n, got)
		}
	}
	t.Run("at most entryCap entries", func(t *testing.T) {
		defer func(n int) { entryCap = n }(entryCap)
		entryCap = 2
		host := httptest.NewServer(http.FileServer(http.Dir(phase(t, 3, nil))))
		t.Cleanup(host.Close)
		if _, err := Fetch(ctx, http.DefaultClient, host.URL, t.TempDir(), io.Discard, FetchOptions{Static: true}); err == nil || !strings.Contains(err.Error(), "more than 2 entries") {
			t.Fatalf("Fetch of 3 entries, at most 2: %v, want it refused", err)
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
