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
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/termsafe"
)

// Entry is one accepted contribution.
type Entry struct {
	Index       uint32 `json:"index"`
	Participant string `json:"participant"` // the name shown for them
	// Identity is what the coordinator admitted them as, and counts one
	// contribution each for: github:<user id> (signed in with GitHub), in
	// an open phase open:<random>, a value of the registration's own, or
	// hand:<the contribution's sha256, cut short> for one the organizers
	// added by hand (ceremony append). It is published, so it is never the
	// registration's session token, which stays the participant's secret
	// (audit R-074).
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

// MaxJSON is the most of a transcript's JSON a client reads from a
// coordinator.
const MaxJSON = 16 << 20

// MaxEntries is the most contributions a transcript read from elsewhere may
// hold: far more than a phase takes (a coordinator stops at
// -max-contributions, 1000 by default), and few enough that reading them
// costs little whatever the JSON (audit L10). Decoded whole, MaxJSON bytes
// of empty entries, which gzip sends in kilobytes, took 2 GB.
const MaxEntries = 100_000

// entryCap is MaxEntries, as reads count to it; a test lowers it.
var entryCap = MaxEntries

// ReadTranscript reads a transcript's JSON from r: at most MaxJSON bytes,
// an entry at a time and at most MaxEntries of them, holding nothing but
// "start" and "contributions". It returns it checked to be a chain, with
// names fit to show (check).
func ReadTranscript(r io.Reader) (Transcript, error) {
	dec := json.NewDecoder(io.LimitReader(r, MaxJSON))
	t := Transcript{Contributions: []Entry{}}
	if err := delim(dec, '{'); err != nil {
		return t, err
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return t, err
		}
		switch key {
		case "start":
			if err := dec.Decode(&t.Start); err != nil {
				return t, err
			}
		case "contributions":
			tok, err := dec.Token()
			if err != nil {
				return t, err
			}
			if tok == nil { // null: none yet
				continue
			}
			// tok opened the list: anything else fails at its end, below.
			for dec.More() {
				if len(t.Contributions) == entryCap {
					return t, fmt.Errorf("more than %d contributions", entryCap)
				}
				var e Entry
				if err := dec.Decode(&e); err != nil {
					return t, err
				}
				t.Contributions = append(t.Contributions, e)
			}
			if err := delim(dec, ']'); err != nil {
				return t, err
			}
		default:
			return t, fmt.Errorf("a transcript holds %q, which it never does", key)
		}
	}
	if err := delim(dec, '}'); err != nil {
		return t, err
	}
	return t, t.check()
}

// delim reads the next token from dec, which must be d.
func delim(dec *json.Decoder, d json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != d {
		return fmt.Errorf("%v where JSON has %q", tok, d)
	}
	return nil
}

// MaxName is the longest participant name an entry may hold, in characters.
const MaxName = 256

// Path is state i's file in dir.
func Path(dir string, i uint32) string { return filepath.Join(dir, fmt.Sprintf("state-%d", i)) }

// Load reads the transcript in dir: state-0's sha256 and each contribution's
// entry, in order, checking that each entry is numbered in turn and builds
// on the one before it, and that its name is fit to show. It hashes state-0
// only, not the contributions: Check does that.
func Load(dir string) (Transcript, error) {
	start, _, err := HashFile(Path(dir, 0))
	if err != nil {
		return Transcript{}, err
	}
	t := Transcript{Start: hex.EncodeToString(start[:]), Contributions: []Entry{}}
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
		t.Contributions = append(t.Contributions, e)
		if err := t.check(); err != nil {
			return Transcript{}, fmt.Errorf("%s.json: %w", Path(dir, i), err)
		}
	}
}

// check checks that t is a chain: each entry numbered in turn, built on the
// one before it from the start, with a sha256 and a name fit to show.
func (t Transcript) check() error {
	if !isSHA256(t.Start) {
		return fmt.Errorf("the start's sha256 is %q", t.Start)
	}
	prev := t.Start
	for i, e := range t.Contributions {
		if e.Index != uint32(i+1) || e.Prev != prev {
			return fmt.Errorf("entry %d is not contribution %d on top of %s", i+1, i+1, prev)
		}
		if !isSHA256(e.SHA256) {
			return fmt.Errorf("contribution %d's sha256 is %q", e.Index, e.SHA256)
		}
		if err := checkName(e.Participant); err != nil {
			return fmt.Errorf("contribution %d: %w", e.Index, err)
		}
		prev = e.SHA256
	}
	return nil
}

// isSHA256 reports whether s is a sha256 in lower-case hex.
func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// checkName refuses a participant name a terminal could take as commands
// (control characters, which start escape sequences) or that reorders the
// text around it (the bidirectional controls): a verifier prints the names a
// coordinator serves.
func checkName(name string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > MaxName {
		return fmt.Errorf("a participant name that is not text of at most %d characters", MaxName)
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return fmt.Errorf("a participant name holding the control character %U", r)
		}
	}
	return nil
}

// Latest is the sha256 of the transcript's latest state.
func (t Transcript) Latest() string {
	if n := len(t.Contributions); n > 0 {
		return t.Contributions[n-1].SHA256
	}
	return t.Start
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

// Phase2Start is what a Phase 2 starts from: the SRS phase1-seal wrote, and
// the circuits (ceremony.CircuitsIn). CheckStart recomputes the start from
// them, as phase2-seal does.
type Phase2Start struct {
	SRS      ceremony.SRS // the SRS, pinned to the sha256 announced for it
	Circuits []ceremony.Circuit
}

// ErrStartNotChecked is a Phase 2 checked without what it starts from: every
// step may be valid, but on top of a start that nothing checked.
var ErrStartNotChecked = errors.New("the start of this Phase 2 was not checked: it is made from the SRS and the circuits, so give them (-srs and -r1cs, as phase2-seal takes them)")

// Check checks the phase in dir from end to end:
//   - state-0 is the real start of its phase (CheckStart): for a Phase 2,
//     which Check cannot recompute, it checks every step and then returns
//     ErrStartNotChecked (CheckWith recomputes it);
//   - every state file has the sha256 its entry says;
//   - every contribution is a valid step on top of the state before it
//     (ceremony.Check).
//
// It reports each step to out.
func Check(dir string, out io.Writer) (Transcript, error) { return CheckWith(dir, nil, out) }

// CheckWith is Check, recomputing a Phase 2's start from p2.
func CheckWith(dir string, p2 *Phase2Start, out io.Writer) (Transcript, error) {
	t, err := Load(dir)
	if err != nil {
		return t, err
	}
	startErr := CheckStart(dir, p2, out)
	if startErr != nil && !errors.Is(startErr, ErrStartNotChecked) {
		return t, startErr
	}
	for _, e := range t.Contributions {
		if err := checkStep(dir, e, out); err != nil {
			return t, err
		}
	}
	return t, startErr
}

// CheckStart checks that state-0 in dir is the real start of its phase: for
// a Phase 1, the start of every Phase 1 of its size; for a Phase 2, the start
// recomputed from p2, or ErrStartNotChecked without it.
func CheckStart(dir string, p2 *Phase2Start, out io.Writer) (err error) {
	info, err := ceremony.Inspect(Path(dir, 0))
	if err != nil {
		return err
	}
	if info.Kind == ceremony.KindPhase2 && p2 == nil {
		fmt.Fprintf(out, "state-0: the start of %s's Phase 2: NOT CHECKED, without the SRS and the circuits\n", info.Ceremony)
		return ErrStartNotChecked
	}
	if info.Kind != ceremony.KindPhase1 && info.Kind != ceremony.KindPhase2 {
		return fmt.Errorf("state-0 holds a %s, not the start of a phase", info.Kind)
	}
	tmp, err := os.MkdirTemp("", "ceremony-check-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(tmp)) }()
	fresh := filepath.Join(tmp, "start")
	if info.Kind == ceremony.KindPhase1 {
		r, err := ceremony.NewPhase1(fresh, info.N)
		if err != nil {
			return err
		}
		if r.Hash != info.Hash {
			return fmt.Errorf("state-0 is not the start of a Phase 1 of domain %d: sha256 %x, want %x", info.N, info.Hash, r.Hash)
		}
		fmt.Fprintf(out, "state-0: the start of Phase 1 for domain %d\n", info.N)
		return nil
	}
	r, err := ceremony.NewPhase2(fresh, info.Ceremony, p2.SRS, p2.Circuits)
	if err != nil {
		return err
	}
	if r.Hash != info.Hash {
		return fmt.Errorf("state-0 is not the start of %s's Phase 2 from that SRS and those circuits: sha256 %x, want %x", info.Ceremony, info.Hash, r.Hash)
	}
	fmt.Fprintf(out, "state-0: the start of %s's Phase 2, recomputed from the SRS and the circuits\n", info.Ceremony)
	return nil
}

// checkStep checks contribution e in dir: its file has the sha256 its entry
// gives it, and it is a valid step on top of the state before it.
func checkStep(dir string, e Entry, out io.Writer) error {
	h, _, err := HashFile(Path(dir, e.Index))
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h[:]); got != e.SHA256 {
		return fmt.Errorf("state-%d has sha256 %s, its entry says %s", e.Index, got, e.SHA256)
	}
	if err := ceremony.Check(Path(dir, e.Index-1), Path(dir, e.Index)); err != nil {
		return fmt.Errorf("contribution %d (%q): %w", e.Index, e.Participant, err)
	}
	fmt.Fprintf(out, "state-%d: a valid contribution by %q, sha256 %s\n", e.Index, e.Participant, e.SHA256)
	return nil
}

// DefaultMaxBytes bounds what one Fetch downloads in all: room for hundreds
// of Phase 1 contributions at 2^18, of 50 MB each.
const DefaultMaxBytes = 64 << 30

// FetchOptions says what Fetch may do.
type FetchOptions struct {
	// Static reads a phase's directory published as it stands (state-<i>
	// and state-<i>.json), at a URL the coordinator does not serve, rather
	// than a coordinator: a copy it cannot answer each reader of differently
	// (audit H2). The entries end at the first the host answers 404 or 403
	// for (object storage answers 403 for a file it does not have), so it
	// is the head a verifier gives that says the copy is whole.
	Static bool
	// Sync lets Fetch make dir match what is served: move the states and
	// entries it contradicts, and those past its end, into a new
	// superseded-<time> directory inside dir. Without it, Fetch only adds to
	// a directory that is empty or holds what the served transcript agrees
	// with, and refuses anything else before it writes a file (audit L13):
	// a copy is evidence of what a host served, and a host serving a
	// different history later has changed its story (I4).
	Sync bool
	// Check checks each contribution as it is in place (as Check does),
	// stopping at the first that is not a valid step, before downloading
	// more. The start is not checked: CheckStart does that.
	Check bool
	// MaxBytes bounds what Fetch downloads in all; DefaultMaxBytes when 0.
	MaxBytes int64
	// MaxState bounds what Fetch downloads as one state;
	// ceremony.DefaultMaxState when 0.
	MaxState int64
}

// Fetch copies the transcript served at url into dir, in the layout Load
// reads, checking each state file against the sha256 the transcript gives
// it: a coordinator's, or with opt.Static a published copy of its
// directory. A state already in dir with the right sha256 is not downloaded
// again, so an interrupted fetch carries on. It writes only state files and
// their entries, never through a link.
func Fetch(ctx context.Context, client *http.Client, url, dir string, out io.Writer, opt FetchOptions) (Transcript, error) {
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = DefaultMaxBytes
	}
	if opt.MaxState <= 0 {
		opt.MaxState = ceremony.DefaultMaxState
	}
	url = strings.TrimSuffix(url, "/")
	stateURL := func(i int) string { return fmt.Sprintf("%s/v1/states/%d", url, i) }
	var (
		t   Transcript
		err error
	)
	if opt.Static {
		stateURL = func(i int) string { return fmt.Sprintf("%s/state-%d", url, i) }
		t, err = fetchStatic(ctx, client, url)
	} else {
		t, err = fetchTranscript(ctx, client, url+"/v1/transcript")
	}
	if err != nil {
		return t, fmt.Errorf("the transcript served: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return t, err
	}
	// Nothing is written before what dir holds is compared with what is
	// served: a fetch it then refuses has changed nothing.
	budget := opt.MaxBytes
	downloaded := "" // state-0, when it was downloaded to learn its sha256
	if t.Start == "" {
		// A static copy of a phase with no contributions names its start
		// nowhere but in state-0: downloaded as any state is, beside it,
		// it is what state-0 has.
		downloaded = filepath.Join(dir, ".start")
		start, n, err := download(ctx, client, stateURL(0), dir, downloaded, "", min(opt.MaxState, budget))
		if err != nil {
			return t, fmt.Errorf("state 0: %w", err)
		}
		defer func() { _ = os.Remove(downloaded) }() // gone already, once in place
		budget -= n
		t.Start = start
	}
	from, why, err := contradicted(dir, t)
	if err != nil {
		return t, err
	}
	if why != "" {
		if !opt.Sync {
			return t, errors.New(why + ": keep this copy, which is evidence of what was served before; " +
				"check into an empty directory (or -sync to replace it, keeping what it replaces)")
		}
		if err := supersede(dir, from, out); err != nil {
			return t, err
		}
	}
	for i := range len(t.Contributions) + 1 {
		hash, path := t.Start, Path(dir, uint32(i))
		var e Entry
		if i > 0 {
			e = t.Contributions[i-1]
			hash = e.SHA256
			// The entry first, as a coordinator writes it: a state is in the
			// transcript only with its entry beside it. An entry already
			// there is the one served: contradicted said so.
			if _, err := os.Lstat(Path(dir, e.Index) + ".json"); errors.Is(err, os.ErrNotExist) {
				if err := saveEntry(dir, e); err != nil {
					return t, err
				}
			} else if err != nil {
				return t, err
			}
		}
		have, err := existing(path)
		if err != nil {
			return t, err
		}
		if i == 0 && have != hash && downloaded != "" {
			if err := os.Rename(downloaded, path); err != nil {
				return t, err
			}
			have = hash
		}
		if have != hash {
			fmt.Fprintf(out, "downloading state %d of %d\n", i, len(t.Contributions))
			_, n, err := download(ctx, client, stateURL(i), dir, path, hash, min(opt.MaxState, budget))
			if err != nil {
				return t, fmt.Errorf("state %d: %w", i, err)
			}
			budget -= n
		}
		if i > 0 && opt.Check {
			if err := checkStep(dir, e, out); err != nil {
				return t, err
			}
		}
	}
	return t, nil
}

// contradicted compares dir with t, writing nothing. It returns the first
// index from which dir holds something t contradicts (a state of another
// sha256, an entry that says otherwise, or states past t's end), and why;
// why is "" when dir holds nothing t contradicts.
func contradicted(dir string, t Transcript) (uint32, string, error) {
	for i := range len(t.Contributions) + 1 {
		hash := t.Start
		if i > 0 {
			hash = t.Contributions[i-1].SHA256
		}
		have, err := existing(Path(dir, uint32(i)))
		if err != nil {
			return 0, "", err
		}
		switch {
		case have == "" || have == hash:
		case i == 0:
			return 0, fmt.Sprintf("%s holds another phase: its state-0 has sha256 %s, the one served %s", dir, have, hash), nil
		default:
			return uint32(i), fmt.Sprintf("the host now serves a different contribution %d (sha256 %s) on top of the same chain as your copy, "+
				"which holds sha256 %s there: it has changed its story", i, hash, have), nil
		}
		if i == 0 {
			continue
		}
		saved, err := savedEntry(dir, uint32(i))
		if err != nil {
			return 0, "", err
		}
		if saved != nil && !sameEntry(*saved, t.Contributions[i-1]) {
			return uint32(i), fmt.Sprintf("the host now serves contribution %d's entry otherwise than your copy holds it "+
				"(by %q at %s, where your copy says by %q at %s): it has rewritten its record", i,
				t.Contributions[i-1].Participant, t.Contributions[i-1].At.Format(time.RFC3339), saved.Participant, saved.At.Format(time.RFC3339)), nil
		}
	}
	end := uint32(len(t.Contributions))
	for _, path := range []string{Path(dir, end+1), Path(dir, end+1) + ".json"} {
		switch _, err := os.Lstat(path); {
		case err == nil:
			return end + 1, fmt.Sprintf("your copy holds contribution %d, past the end of what the host now serves (%d contributions): "+
				"it has dropped contributions it served before", end+1, end), nil
		case !errors.Is(err, os.ErrNotExist):
			return 0, "", err
		}
	}
	return 0, "", nil
}

// savedEntry reads state i's entry in dir: nil when there is none, and one
// that does not parse is taken as contradicted, as an entry of nothing.
func savedEntry(dir string, i uint32) (*Entry, error) {
	path := Path(dir, i) + ".json"
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e Entry
	if json.Unmarshal(data, &e) != nil {
		return &Entry{}, nil
	}
	return &e, nil
}

// sameEntry reports whether a and b say the same of a contribution.
func sameEntry(a, b Entry) bool {
	return a.Index == b.Index && a.Participant == b.Participant && a.Identity == b.Identity &&
		a.SHA256 == b.SHA256 && a.Prev == b.Prev && a.At.Equal(b.At)
}

// supersede moves every state and entry in dir from index from on into a
// new directory, superseded-<time>, inside dir, rather than delete them
// (audit I4): what a host served before is evidence when it serves
// otherwise now.
func supersede(dir string, from uint32, out io.Writer) error {
	to := filepath.Join(dir, "superseded-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err := os.Mkdir(to, 0o755); err != nil {
		return err
	}
	moved := 0
	for i := from; ; i++ {
		found := false
		for _, name := range []string{fmt.Sprintf("state-%d", i), fmt.Sprintf("state-%d.json", i)} {
			switch err := os.Rename(filepath.Join(dir, name), filepath.Join(to, name)); {
			case err == nil:
				found = true
				moved++
			case !errors.Is(err, os.ErrNotExist):
				return err
			}
		}
		if !found {
			fmt.Fprintf(out, "Moved %d files that the host now contradicts into %s.\n", moved, to)
			return nil
		}
	}
}

// fetchTranscript reads the transcript a coordinator serves at url.
func fetchTranscript(ctx context.Context, client *http.Client, url string) (Transcript, error) {
	body, err := get(ctx, client, url)
	if err != nil {
		return Transcript{}, err
	}
	defer body.Close()
	return ReadTranscript(body)
}

// fetchStatic reads the transcript of a phase directory published at url:
// its entries, state-<i>.json from 1 until one is not there (404 or 403), at
// most entryCap, and the start's sha256 from the first entry; "" when there
// is none, for Fetch to take from state-0 itself.
func fetchStatic(ctx context.Context, client *http.Client, url string) (Transcript, error) {
	t := Transcript{Contributions: []Entry{}}
	for i := 1; ; i++ {
		body, err := get(ctx, client, fmt.Sprintf("%s/state-%d.json", url, i))
		if errors.Is(err, errNotThere) {
			break
		}
		if err != nil {
			return t, err
		}
		if i > entryCap {
			body.Close()
			return t, fmt.Errorf("more than %d entries", entryCap)
		}
		var e Entry
		err = json.NewDecoder(io.LimitReader(body, 1<<16)).Decode(&e)
		body.Close()
		if err != nil {
			return t, fmt.Errorf("state-%d.json: %w", i, err)
		}
		t.Contributions = append(t.Contributions, e)
	}
	if len(t.Contributions) == 0 {
		return t, nil
	}
	t.Start = t.Contributions[0].Prev
	return t, t.check()
}

// errNotThere is a host answering 404, or 403 as object storage does: it
// has no such file.
var errNotThere = errors.New("not there (404 or 403)")

// get returns url's body, if it answers 200.
func get(ctx context.Context, client *http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusNotFound, http.StatusForbidden:
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %w", url, errNotThere)
	}
	resp.Body.Close()
	return nil, fmt.Errorf("%s: %w", url, statusError(resp.StatusCode))
}

// existing returns the sha256, in hex, of the state at path, or "" when
// there is none. Anything there but a regular file is refused: through a
// link, Fetch would read or write outside its directory.
func existing(path string) (string, error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	h, _, err := HashFile(path)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h[:]), nil
}

// saveEntry writes e beside its state in dir, through a new file renamed in
// place, so never through a link.
func saveEntry(dir string, e Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".entry-")
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), Path(dir, e.Index)+".json")
	}
	if err != nil {
		return errors.Join(err, os.Remove(f.Name()))
	}
	return nil
}

// busyTries is how many times getState asks a coordinator too busy to send a
// state (503), waiting as it says between (Retry-After, at most busyWait).
const (
	busyTries = 20
	busyWait  = time.Minute
)

// getState asks for the state at url, waiting out a coordinator that has as
// many downloads under way as it serves at once (audit R-010): a verifier
// gave up at the first 503, so a few slow downloads elsewhere stopped it.
func getState(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	for try := 1; ; try++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusServiceUnavailable || try == busyTries {
			return resp, err
		}
		resp.Body.Close()
		wait := 10 * time.Second
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			wait = min(max(time.Duration(secs)*time.Second, 0), busyWait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// download saves url's body at path, in dir, which must have sha256 hash
// (any, when hash is "") and be at most limit bytes, and returns its sha256
// and how many bytes it read. It writes a new file, renamed in place once
// it is right.
func download(ctx context.Context, client *http.Client, url, dir, path, hash string, limit int64) (string, int64, error) {
	resp, err := getState(ctx, client, url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("%s: %w", url, statusError(resp.StatusCode))
	}
	f, err := os.CreateTemp(dir, ".download-")
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > limit {
		err = fmt.Errorf("served more than %d bytes: more than a state may be (-max-state), or than is left to download (-max-bytes)", limit)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if err == nil && hash != "" && got != hash {
		err = fmt.Errorf("served with sha256 %s, the transcript says %s", got, hash)
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return "", n, errors.Join(err, os.Remove(f.Name()))
	}
	return got, n, nil
}

// statusError is an answer other than 200, by its code: never the reason
// phrase a server sent beside it, which it chose (audit M2).
func statusError(code int) error {
	return fmt.Errorf("answered %s", termsafe.Status(code))
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
