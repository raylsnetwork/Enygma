// Package transcript reads, fetches and checks a ceremony phase's
// transcript: the directory a coordinator keeps it in, and anyone can copy.
//
//	state-0          the phase's start, as phase1-new or phase2-new wrote it
//	state-<i>        the i-th accepted contribution
//	state-<i>.json   its Entry: who made it, when, its sha256 and the one it built on
//
// A coordinator writes state-<i>.json before state-<i>, so a state file is
// in the transcript only with its entry beside it.
package transcript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"enygma_ceremony/ceremony"
)

// Entry is one accepted contribution.
type Entry struct {
	Index       uint32 `json:"index"`
	Participant string `json:"participant"` // the name shown for them
	// Identity is what the coordinator admitted them as, and counts one
	// contribution each for: open:<session> (self-named) or github:<user id>
	// (signed in with GitHub).
	Identity string    `json:"identity"`
	SHA256   string    `json:"sha256"`
	Prev     string    `json:"prev"` // sha256 of the state it built on
	At       time.Time `json:"at"`
}

// Transcript is a phase so far: its start and every accepted contribution.
type Transcript struct {
	Start         string  `json:"start"` // sha256 of state-0
	Contributions []Entry `json:"contributions"`
}

// Path is state i's file in dir.
func Path(dir string, i uint32) string { return filepath.Join(dir, fmt.Sprintf("state-%d", i)) }

// Load reads the transcript in dir: state-0's sha256 and each contribution's
// entry, in order, checking that each entry is numbered in turn and builds
// on the one before it. It hashes state-0 only, not the contributions:
// Check does that.
func Load(dir string) (Transcript, error) {
	start, _, err := HashFile(Path(dir, 0))
	if err != nil {
		return Transcript{}, err
	}
	t := Transcript{Start: hex.EncodeToString(start[:]), Contributions: []Entry{}}
	prev := t.Start
	for i := uint32(1); ; i++ {
		if _, err := os.Stat(Path(dir, i)); errors.Is(err, os.ErrNotExist) {
			return t, nil
		}
		var e Entry
		data, err := os.ReadFile(Path(dir, i) + ".json")
		if err == nil {
			err = json.Unmarshal(data, &e)
		}
		if err != nil {
			return Transcript{}, fmt.Errorf("state %d's entry: %w", i, err)
		}
		if e.Index != i || e.Prev != prev {
			return Transcript{}, fmt.Errorf("%s.json is not contribution %d on top of %s", Path(dir, i), i, prev)
		}
		t.Contributions = append(t.Contributions, e)
		prev = e.SHA256
	}
}

// Latest is the sha256 of the transcript's latest state.
func (t Transcript) Latest() string {
	if n := len(t.Contributions); n > 0 {
		return t.Contributions[n-1].SHA256
	}
	return t.Start
}

// Check checks the phase in dir from end to end:
//   - every state file has the sha256 its entry says;
//   - every contribution is a valid step on top of the state before it
//     (ceremony.Check);
//   - a Phase 1 starts where every Phase 1 of its size starts. A Phase 2's
//     start depends on the SRS and the circuits; phase2-seal recomputes it.
//
// It reports each step to out.
func Check(dir string, out io.Writer) (Transcript, error) {
	t, err := Load(dir)
	if err != nil {
		return t, err
	}
	info, err := ceremony.Inspect(Path(dir, 0))
	if err != nil {
		return t, err
	}
	if info.Kind == ceremony.KindPhase1 {
		tmp, err := os.MkdirTemp("", "ceremony-check-")
		if err != nil {
			return t, err
		}
		r, err := ceremony.NewPhase1(filepath.Join(tmp, "start"), info.N)
		if err := errors.Join(err, os.RemoveAll(tmp)); err != nil {
			return t, err
		}
		if r.Hash != info.Hash {
			return t, fmt.Errorf("state-0 is not the start of a Phase 1 of domain %d: sha256 %x, want %x", info.N, info.Hash, r.Hash)
		}
		fmt.Fprintf(out, "state-0: the start of Phase 1 for domain %d\n", info.N)
	} else {
		fmt.Fprintf(out, "state-0: the start of %s; phase2-seal recomputes it from the SRS and the circuits\n", info.Kind)
	}
	for _, e := range t.Contributions {
		h, _, err := HashFile(Path(dir, e.Index))
		if err != nil {
			return t, err
		}
		if got := hex.EncodeToString(h[:]); got != e.SHA256 {
			return t, fmt.Errorf("state-%d has sha256 %s, its entry says %s", e.Index, got, e.SHA256)
		}
		if err := ceremony.Check(Path(dir, e.Index-1), Path(dir, e.Index)); err != nil {
			return t, fmt.Errorf("contribution %d (%s): %w", e.Index, e.Participant, err)
		}
		fmt.Fprintf(out, "state-%d: a valid contribution by %s, sha256 %s\n", e.Index, e.Participant, e.SHA256)
	}
	return t, nil
}

// Find returns the entry of the contribution with sha256 hash, the hash a
// participant published.
func (t Transcript) Find(hash string) (Entry, bool) {
	for _, e := range t.Contributions {
		if strings.EqualFold(e.SHA256, hash) {
			return e, true
		}
	}
	return Entry{}, false
}

// Fetch copies the transcript the coordinator at url serves into dir, in the
// layout Load reads, checking each state file against the sha256 the
// transcript gives it. A state already in dir with the right sha256 is not
// downloaded again, so an interrupted fetch carries on. States in dir past
// the transcript's end, as a longer phase fetched there before leaves, are
// removed: Load would read on into them.
func Fetch(ctx context.Context, client *http.Client, url, dir string, out io.Writer) (Transcript, error) {
	url = strings.TrimSuffix(url, "/")
	var t Transcript
	if err := getJSON(ctx, client, url+"/v1/transcript", &t); err != nil {
		return t, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return t, err
	}
	want := []string{t.Start}
	for _, e := range t.Contributions {
		want = append(want, e.SHA256)
	}
	for i, hash := range want {
		path := Path(dir, uint32(i))
		if h, _, err := HashFile(path); err == nil && hex.EncodeToString(h[:]) == hash {
			continue
		}
		fmt.Fprintf(out, "downloading state %d of %d\n", i, len(want)-1)
		if err := download(ctx, client, fmt.Sprintf("%s/v1/states/%d", url, i), path, hash, ceremony.MaxDownload); err != nil {
			return t, fmt.Errorf("state %d: %w", i, err)
		}
	}
	for _, e := range t.Contributions {
		data, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return t, err
		}
		if err := os.WriteFile(Path(dir, e.Index)+".json", append(data, '\n'), 0o644); err != nil {
			return t, err
		}
	}
	for i := uint32(len(want)); ; i++ {
		removed := false
		for _, path := range []string{Path(dir, i), Path(dir, i) + ".json"} {
			switch err := os.Remove(path); {
			case err == nil:
				removed = true
			case !errors.Is(err, os.ErrNotExist):
				return t, err
			}
		}
		if !removed {
			return t, nil
		}
	}
}

// download saves url's body at path, which must have sha256 hash and be at
// most limit bytes.
func download(ctx context.Context, client *http.Client, url, path, hash string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > limit {
		err = fmt.Errorf("served more than %d bytes, more than any state", limit)
	}
	if err == nil {
		if got := hex.EncodeToString(h.Sum(nil)); got != hash {
			err = fmt.Errorf("served with sha256 %s, the transcript says %s", got, hash)
		}
	}
	if err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return os.Rename(tmp, path)
}

// getJSON decodes url's JSON body into v.
func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(v)
}

// HashFile returns the sha256 and size of the file at path.
func HashFile(path string) ([32]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest, n, err
}
