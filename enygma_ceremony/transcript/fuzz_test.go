package transcript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// hexSHA is the hex sha256 of data.
func hexSHA(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// entryJSON is the JSON of an entry, as a coordinator writes it.
func entryJSON(tb testing.TB, e Entry) []byte {
	tb.Helper()
	data, err := json.Marshal(e)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

// FuzzLoad: whatever a transcript's entries hold, Load never panics, and what
// it returns is a chain: entry i is contribution i, built on the state before
// it, starting from state-0, with every contribution in the directory in it.
func FuzzLoad(f *testing.F) {
	states := [][]byte{[]byte("state 0"), []byte("state 1"), []byte("state 2")}
	at := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	f.Add(entryJSON(f, Entry{Index: 1, Participant: "alice", Identity: "open:a", SHA256: hexSHA(states[1]), Prev: hexSHA(states[0]), At: at}),
		entryJSON(f, Entry{Index: 2, Participant: "bob", Identity: "open:b", SHA256: hexSHA(states[2]), Prev: hexSHA(states[1]), At: at}))
	f.Add([]byte(`{"index":1}`), []byte(`not an entry`))
	// A second entry on top of the wrong state, and one numbered out of turn.
	f.Add(entryJSON(f, Entry{Index: 1, SHA256: hexSHA(states[1]), Prev: hexSHA(states[0])}),
		entryJSON(f, Entry{Index: 2, SHA256: hexSHA(states[2]), Prev: hexSHA(states[0])}))
	f.Add(entryJSON(f, Entry{Index: 1, SHA256: hexSHA(states[1]), Prev: hexSHA(states[0])}),
		entryJSON(f, Entry{Index: 3, SHA256: hexSHA(states[2]), Prev: hexSHA(states[1])}))
	f.Fuzz(func(t *testing.T, first, second []byte) {
		dir := t.TempDir()
		for i, data := range states {
			if err := os.WriteFile(Path(dir, uint32(i)), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for i, data := range [][]byte{first, second} {
			if err := os.WriteFile(Path(dir, uint32(i+1))+".json", data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		tr, err := Load(dir)
		if err != nil {
			return
		}
		if tr.Start != hexSHA(states[0]) || len(tr.Contributions) != 2 {
			t.Fatalf("loaded start %s and %d contributions, want %s and 2", tr.Start, len(tr.Contributions), hexSHA(states[0]))
		}
		prev := tr.Start
		for i, e := range tr.Contributions {
			if e.Index != uint32(i+1) || e.Prev != prev {
				t.Fatalf("contribution %d is %+v, not number %d on top of %s", i, e, i+1, prev)
			}
			prev = e.SHA256
		}
	})
}

// stateFile is the name of a file Fetch may write: a state, or its entry.
var stateFile = regexp.MustCompile(`^state-[0-9]+(\.json)?$`)

// FuzzFetch: whatever a coordinator serves, Fetch never panics, and writes
// only state files and their entries, inside its directory: a verifier
// fetches from a coordinator they need not trust. When it succeeds, every
// state it wrote has the sha256 the transcript gives it.
func FuzzFetch(f *testing.F) {
	s0, s1 := []byte("state 0"), []byte("state 1")
	good, err := json.Marshal(Transcript{Start: hexSHA(s0), Contributions: []Entry{
		{Index: 1, Participant: "alice", Identity: "open:a", SHA256: hexSHA(s1), Prev: hexSHA(s0)},
	}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(good, s0, s1)
	f.Add(good, s0, []byte("not the state the transcript names"))
	f.Add([]byte(`{"start":"`+hexSHA(s0)+`","contributions":[{"index":4294967295,"sha256":"`+hexSHA(s1)+`"}]}`), s0, s1)
	f.Add([]byte(`{"start":"../../escape","contributions":[]}`), s0, s1)
	type served struct{ transcript, s0, s1 []byte }
	var now atomic.Pointer[served]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := now.Load()
		switch {
		case r.URL.Path == "/v1/transcript":
			w.Write(s.transcript)
		case r.URL.Path == "/v1/states/0":
			w.Write(s.s0)
		case strings.HasPrefix(r.URL.Path, "/v1/states/"):
			w.Write(s.s1)
		default:
			http.NotFound(w, r)
		}
	}))
	f.Cleanup(srv.Close)
	f.Fuzz(func(t *testing.T, transcript, s0, s1 []byte) {
		now.Store(&served{transcript, s0, s1})
		parent := t.TempDir()
		dir := filepath.Join(parent, "transcript")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tr, err := Fetch(ctx, srv.Client(), srv.URL, dir, io.Discard, FetchOptions{})
		walkErr := filepath.WalkDir(parent, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == parent || path == dir {
				return err
			}
			if filepath.Dir(path) != dir || d.IsDir() || !stateFile.MatchString(d.Name()) {
				t.Errorf("Fetch wrote %s", path)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
		if err != nil {
			return
		}
		want := []string{tr.Start}
		for _, e := range tr.Contributions {
			want = append(want, e.SHA256)
		}
		for i, hash := range want {
			data, err := os.ReadFile(Path(dir, uint32(i)))
			if err != nil || hexSHA(data) != hash {
				t.Fatalf("state %d: %v, sha256 %s, the transcript says %s", i, err, hexSHA(data), hash)
			}
		}
	})
}
