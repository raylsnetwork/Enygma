package coordinator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/fuzzedit"
	"enygma_ceremony/transcript"
)

// FuzzCleanName: a name the coordinator takes is shown tidied: valid UTF-8,
// 1 to 64 characters, no control, invisible or unprintable characters, at
// least one letter or digit, and no space but single spaces between words;
// tidying it again changes nothing. What it refuses, it refuses for one of
// the reasons it gives.
func FuzzCleanName(f *testing.F) {
	for _, s := range []string{"Jane Smith", " Jane  Smith ", "Zoë\u00a0Ünal", "李雷", "a\u202eb", "x\ny", "  Zoë \t  Ünal ", strings.Repeat("a", 65), "\xff", "\u00a0"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		name, err := cleanName(s)
		if err != nil {
			if !errors.Is(err, errNameLength) && !errors.Is(err, errNameCharacters) && !errors.Is(err, errNameVisible) {
				t.Fatalf("refused %q for no reason it gives: %v", s, err)
			}
			return
		}
		if n := utf8.RuneCountInString(name); !utf8.ValidString(name) || n < 1 || n > maxName {
			t.Fatalf("took %q as %q", s, name)
		}
		if strings.HasPrefix(name, " ") || strings.HasSuffix(name, " ") || strings.Contains(name, "  ") {
			t.Fatalf("took %q as %q, spaces untidied", s, name)
		}
		visible := false
		for _, r := range name {
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector) ||
				(unicode.IsSpace(r) && r != ' ') || (!unicode.IsPrint(r) && r != ' ') {
				t.Fatalf("took %q as %q, holding %U", s, name, r)
			}
			visible = visible || unicode.IsLetter(r) || unicode.IsDigit(r)
		}
		if !visible {
			t.Fatalf("took %q as %q, with nothing to read (audit R-048)", s, name)
		}
		if again, err := cleanName(name); err != nil || again != name {
			t.Fatalf("tidying %q again gives %q, %v", name, again, err)
		}
	})
}

// FuzzRequests: no request, from anyone, makes the coordinator panic or fail
// on its own side (a 5xx), and none breaks its transcript: after each, the
// transcript on disk still loads as a chain, holding at most the one
// contribution its one token could make. FuzzUpload fuzzes uploads.
func FuzzRequests(f *testing.F) {
	dir := f.TempDir()
	if _, err := ceremony.NewPhase1(filepath.Join(dir, "state-0"), 8); err != nil {
		f.Fatal(err)
	}
	srv, err := New(Config{
		Dir: dir, Auth: AuthOpen, Turn: time.Minute, TurnPresence: time.Minute,
		Presence: time.Minute, Away: time.Hour, Poll: time.Second,
		Log: log.New(io.Discard, "", 0),
	})
	if err != nil {
		f.Fatal(err)
	}
	handler := srv.Handler()
	serve := func(method, target string, body []byte, auth string, from byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader(body))
		req.Method = method
		u, err := url.ParseRequestURI(target)
		if err != nil {
			return nil
		}
		req.URL, req.RequestURI = u, target
		req.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", from)
		if u.Path == "/v1/register" {
			req.Header.Set("Content-Type", "application/json") // as a participant's program sends it
		}
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	rec := serve(http.MethodPost, "/v1/register", []byte(`{"name":"fuzzer"}`), "", 0)
	var session Session
	if err := json.Unmarshal(rec.Body.Bytes(), &session); rec.Code != http.StatusOK || err != nil {
		f.Fatalf("registering: %d %s", rec.Code, rec.Body)
	}
	start, err := os.ReadFile(filepath.Join(dir, "state-0"))
	if err != nil {
		f.Fatal(err)
	}
	var next bytes.Buffer
	if _, err := ceremony.ContributeTo(bytes.NewReader(start), int64(len(start)), &next); err != nil {
		f.Fatal(err)
	}
	// auth: 0 sends no token, 1 the registered one, anything else the body's start.
	for _, seed := range []struct {
		method, target string
		body           []byte
		auth           byte
	}{
		{"POST", "/v1/register", []byte(`{"name":"someone"}`), 0},
		{"POST", "/v1/turn", nil, 1},
		{"POST", "/v1/heartbeat", nil, 1},
		{"GET", "/v1/state", nil, 1},
		{"PUT", "/v1/contribution", start, 1},
		{"PUT", "/v1/contribution", next.Bytes(), 1},
		{"PUT", "/v1/contribution", []byte("not a state"), 2},
		{"GET", "/v1/states/0", nil, 0},
		{"GET", "/v1/states/18446744073709551616", nil, 0},
		{"GET", "/v1/status", nil, 0},
		{"GET", "/v1/transcript", nil, 0},
		{"GET", "/v1/config", nil, 0},
		{"GET", "/web/..%2f..%2fstate-0", nil, 0},
		{"GET", "/download/..%2fstate-0", nil, 0},
		{"POST", "/github/login/device/code", []byte("client_id=x"), 0},
		{"GET", "/", nil, 0},
	} {
		f.Add(seed.method, seed.target, seed.body, seed.auth)
	}
	f.Fuzz(func(t *testing.T, method, target string, body []byte, auth byte) {
		token := ""
		switch auth {
		case 0:
		case 1:
			token = session.Token
		default:
			token = string(body[:min(len(body), 48)])
		}
		rec := serve(method, target, body, token, auth)
		if rec == nil {
			return
		}
		if rec.Code >= 500 {
			t.Fatalf("%s %s: %d %s", method, target, rec.Code, rec.Body)
		}
		tr, err := transcript.Load(dir)
		if err != nil {
			t.Fatalf("after %s %s, the transcript does not load: %v", method, target, err)
		}
		if len(tr.Contributions) > 1 {
			t.Fatalf("after %s %s, %d contributions, from one registration", method, target, len(tr.Contributions))
		}
	})
}

// FuzzUpload: whatever a turn holder uploads, the coordinator neither panics
// nor fails on its own side, and the transcript holds what it accepted, and
// only that. Raw bodies are anything at all; edited ones are a contribution
// made on the phase's start, edited (package fuzzedit, as FuzzCheckEdits),
// of which it accepts the unedited one only.
func FuzzUpload(f *testing.F) {
	start := filepath.Join(f.TempDir(), "state-0")
	if _, err := ceremony.NewPhase1(start, 8); err != nil {
		f.Fatal(err)
	}
	state, err := os.ReadFile(start)
	if err != nil {
		f.Fatal(err)
	}
	var next bytes.Buffer
	if _, err := ceremony.ContributeTo(bytes.NewReader(state), int64(len(state)), &next); err != nil {
		f.Fatal(err)
	}
	contribution := next.Bytes()
	f.Add([]byte(nil), []byte(nil))
	f.Add([]byte(nil), fuzzedit.Op(1, 500, 0))
	f.Add(state, []byte(nil))
	f.Add([]byte("not a state"), []byte(nil))
	f.Fuzz(func(t *testing.T, raw, script []byte) {
		body := raw
		if len(raw) == 0 {
			body = fuzzedit.Apply(contribution, script)
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "state-0"), state, 0o644); err != nil {
			t.Fatal(err)
		}
		srv, err := New(Config{Dir: dir, Auth: AuthOpen, Log: log.New(io.Discard, "", 0)})
		if err != nil {
			t.Fatal(err)
		}
		h := srv.Handler()
		do := func(method, path, token string, body []byte) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, path, bytes.NewReader(body))
			if method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			return rec
		}
		var session Session
		if rec := do(http.MethodPost, "/v1/register", "", []byte(`{"name":"uploader"}`)); json.Unmarshal(rec.Body.Bytes(), &session) != nil {
			t.Fatalf("registering: %d %s", rec.Code, rec.Body)
		}
		var reply Reply
		if rec := do(http.MethodPost, "/v1/turn", session.Token, nil); json.Unmarshal(rec.Body.Bytes(), &reply) != nil || reply.Status != Go {
			t.Fatalf("taking the turn: %d %s", rec.Code, rec.Body)
		}
		rec := do(http.MethodPut, "/v1/contribution", session.Token, body)
		if rec.Code >= 500 {
			t.Fatalf("upload of %d bytes: %d %s", len(body), rec.Code, rec.Body)
		}
		accepted := rec.Code == http.StatusOK
		if len(raw) == 0 && accepted != bytes.Equal(body, contribution) {
			t.Fatalf("upload of a contribution edited to %d bytes: %d %s", len(body), rec.Code, rec.Body)
		}
		tr, err := transcript.Load(dir)
		if err != nil {
			t.Fatalf("after the upload, the transcript does not load: %v", err)
		}
		switch {
		case !accepted && len(tr.Contributions) != 0:
			t.Fatalf("after an upload answered %d, %d contributions", rec.Code, len(tr.Contributions))
		case accepted && (len(tr.Contributions) != 1 || tr.Contributions[0].SHA256 != sha(body)):
			t.Fatalf("accepted an upload of sha256 %s; the transcript holds %+v", sha(body), tr.Contributions)
		}
	})
}
