package transcript

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/ceremony"
)

// phase writes a Phase 1 transcript into a new directory: start as state-0
// (the start of a Phase 1 at domain 8 when nil), then n contributions, each
// with its entry, as a coordinator writes them.
func phase(t *testing.T, n int, start func(path string)) string {
	t.Helper()
	dir := t.TempDir()
	if start == nil {
		start = func(path string) {
			if _, err := ceremony.NewPhase1(path, 8); err != nil {
				t.Fatal(err)
			}
		}
	}
	start(Path(dir, 0))
	for i := uint32(1); i <= uint32(n); i++ {
		addContribution(t, dir, i, Path(dir, i-1))
	}
	return dir
}

// addContribution writes state i of dir as a contribution on top of the
// state at on, with its entry.
func addContribution(t *testing.T, dir string, i uint32, on string) {
	t.Helper()
	r, err := ceremony.Contribute(on, Path(dir, i))
	if err != nil {
		t.Fatal(err)
	}
	prev, _, err := HashFile(Path(dir, i-1))
	if err != nil {
		t.Fatal(err)
	}
	writeEntry(t, dir, Entry{
		Index: i, Participant: fmt.Sprintf("p%d", i), Identity: fmt.Sprintf("open:%d", i),
		SHA256: hex.EncodeToString(r.Hash[:]), Prev: hex.EncodeToString(prev[:]), At: time.Unix(int64(i), 0).UTC(),
	})
}

// writeEntry writes e as its state's entry in dir.
func writeEntry(t *testing.T, dir string, e Entry) {
	t.Helper()
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir, e.Index)+".json", data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheck: a transcript checks when every state is the one its entry
// names and follows the one before it, from a true start; otherwise Check
// says which step fails.
func TestCheck(t *testing.T) {
	t.Run("accepts a transcript, and finds a participant's contribution", func(t *testing.T) {
		dir := phase(t, 2, nil)
		tr, err := Check(dir, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		mine := tr.Contributions[1].SHA256
		if e, ok := tr.Find(strings.ToUpper(mine)); !ok || e.Index != 2 {
			t.Fatalf("Find(%s) = %+v, %v", mine, e, ok)
		}
		if _, ok := tr.Find(strings.Repeat("0", 64)); ok {
			t.Fatal("found a contribution nobody made")
		}
	})
	t.Run("refuses a state that is not the one its entry names", func(t *testing.T) {
		dir := phase(t, 2, nil)
		if err := os.Remove(Path(dir, 2)); err != nil {
			t.Fatal(err)
		}
		if _, err := ceremony.Contribute(Path(dir, 1), Path(dir, 2)); err != nil { // another contribution, same place
			t.Fatal(err)
		}
		if _, err := Check(dir, io.Discard); err == nil || !strings.Contains(err.Error(), "its entry says") {
			t.Fatalf("Check: %v, want state-2's sha256 refused", err)
		}
	})
	t.Run("refuses a contribution that does not follow the state before it", func(t *testing.T) {
		dir := phase(t, 1, nil)
		addContribution(t, dir, 2, Path(dir, 0)) // on top of state 0, not state 1
		if _, err := Check(dir, io.Discard); err == nil || !strings.Contains(err.Error(), "contribution 2") {
			t.Fatalf("Check: %v, want contribution 2 refused", err)
		}
	})
	t.Run("refuses a Phase 1 that does not start where every Phase 1 starts", func(t *testing.T) {
		dir := phase(t, 1, func(path string) { // a contribution passed off as the start
			fresh := filepath.Join(t.TempDir(), "start")
			if _, err := ceremony.NewPhase1(fresh, 8); err != nil {
				t.Fatal(err)
			}
			if _, err := ceremony.Contribute(fresh, path); err != nil {
				t.Fatal(err)
			}
		})
		if _, err := Check(dir, io.Discard); err == nil || !strings.Contains(err.Error(), "not the start") {
			t.Fatalf("Check: %v, want state-0 refused as a start", err)
		}
	})
	t.Run("refuses entries that do not chain", func(t *testing.T) {
		dir := phase(t, 2, nil)
		data, err := os.ReadFile(Path(dir, 2) + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		e.Prev = strings.Repeat("0", 64)
		writeEntry(t, dir, e)
		if _, err := Load(dir); err == nil {
			t.Fatal("Load: no error on an entry that does not build on the one before it")
		}
	})
}

// serve serves dir as a coordinator serves its transcript, and counts the
// state downloads; tamper, when set, changes what state i is served as.
func serve(t *testing.T, dir string, tamper func(i int, data []byte) []byte) (string, *atomic.Int32) {
	t.Helper()
	tr, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var downloads atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/transcript", func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(tr); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("GET /v1/states/{i}", func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		var i int
		if _, err := fmt.Sscan(r.PathValue("i"), &i); err != nil {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(Path(dir, uint32(i)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if tamper != nil {
			data = tamper(i, data)
		}
		if _, err := w.Write(data); err != nil {
			t.Error(err)
		}
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s.URL, &downloads
}

// TestFetch: Fetch copies a coordinator's transcript into a directory Check
// accepts, refuses a state served other than the transcript names it, and
// carries on an interrupted copy without downloading again what it has.
func TestFetch(t *testing.T) {
	t.Run("copies a transcript that checks", func(t *testing.T) {
		url, _ := serve(t, phase(t, 2, nil), nil)
		dir := t.TempDir()
		if _, err := Fetch(context.Background(), http.DefaultClient, url, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		if _, err := Check(dir, io.Discard); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("refuses a state served other than the transcript names it", func(t *testing.T) {
		other := phase(t, 2, nil)
		url, _ := serve(t, phase(t, 2, nil), func(i int, data []byte) []byte {
			if i == 2 {
				alt, err := os.ReadFile(Path(other, 2))
				if err != nil {
					t.Fatal(err)
				}
				return alt
			}
			return data
		})
		_, err := Fetch(context.Background(), http.DefaultClient, url, t.TempDir(), io.Discard)
		if err == nil || !strings.Contains(err.Error(), "the transcript says") {
			t.Fatalf("Fetch: %v, want state 2 refused", err)
		}
	})
	t.Run("does not download again what it has", func(t *testing.T) {
		url, downloads := serve(t, phase(t, 2, nil), nil)
		dir := t.TempDir()
		if _, err := Fetch(context.Background(), http.DefaultClient, url, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		downloads.Store(0)
		if _, err := Fetch(context.Background(), http.DefaultClient, url, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		if n := downloads.Load(); n != 0 {
			t.Fatalf("downloaded %d states again", n)
		}
	})
	t.Run("clears states past the transcript's end, as a longer phase left", func(t *testing.T) {
		dir := t.TempDir()
		longer, _ := serve(t, phase(t, 3, nil), nil)
		if _, err := Fetch(context.Background(), http.DefaultClient, longer, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		shorter, _ := serve(t, phase(t, 1, nil), nil)
		if _, err := Fetch(context.Background(), http.DefaultClient, shorter, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		if tr, err := Check(dir, io.Discard); err != nil || len(tr.Contributions) != 1 {
			t.Fatalf("Check after fetching the shorter phase: %d contributions, %v; want 1", len(tr.Contributions), err)
		}
		for i := uint32(2); i <= 3; i++ {
			for _, path := range []string{Path(dir, i), Path(dir, i) + ".json"} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s left behind: %v", path, err)
				}
			}
		}
	})
	t.Run("refuses a state larger than any state", func(t *testing.T) {
		const limit = 100
		for _, size := range []int{limit, limit + 1} {
			body := bytes.Repeat([]byte{1}, size)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }))
			t.Cleanup(srv.Close)
			sum := sha256.Sum256(body)
			path := filepath.Join(t.TempDir(), "state-0")
			err := download(context.Background(), http.DefaultClient, srv.URL, path, hex.EncodeToString(sum[:]), limit)
			switch _, statErr := os.Stat(path); {
			case size <= limit && (err != nil || statErr != nil):
				t.Fatalf("%d bytes, at most %d: %v, %v", size, limit, err, statErr)
			case size > limit && (err == nil || !strings.Contains(err.Error(), "more than any state")):
				t.Fatalf("%d bytes, at most %d: %v, want it refused", size, limit, err)
			case size > limit && !errors.Is(statErr, os.ErrNotExist):
				t.Fatalf("%d bytes, at most %d: kept at %s", size, limit, path)
			}
		}
	})
}
