package coordinator

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// lockedHere reports whether this system locks a phase's directory and
// tells its free space (sys_unix.go).
func lockedHere() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "linux" }

// TestOneCoordinatorPerPhase (audit R-043): a second coordinator on a phase's
// directory is refused while the first serves it, before it clears the
// first's uploads under way; once the first lets go, it starts.
func TestOneCoordinatorPerPhase(t *testing.T) {
	if !lockedHere() {
		t.Skip("the directory is locked on Linux and macOS")
	}
	h := open(t, "alice")
	upload := filepath.Join(h.dir, "incoming", "upload-under-way")
	if err := os.WriteFile(upload, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(h.cfg); err == nil || !strings.Contains(err.Error(), "another coordinator") {
		t.Fatalf("a second coordinator on the directory: %v, want it refused", err)
	}
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("the refused coordinator cleared the first's upload under way: %v", err)
	}
	h.srv.Release()
	srv, err := New(h.cfg)
	if err != nil {
		t.Fatalf("a coordinator after the first let go: %v", err)
	}
	srv.Release()
}

// blockedWriter is a response whose body waits to be written until released.
type blockedWriter struct {
	header           http.Header
	started, release chan struct{}
	once             sync.Once
}

func (b *blockedWriter) Header() http.Header { return b.header }
func (b *blockedWriter) WriteHeader(int)     {}
func (b *blockedWriter) Write(p []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return len(p), nil
}

// TestPublicDownloadsPerAddress (audit R-010): one network address has one
// public download of a state at a time, so a slow reader holds its own slot,
// not everyone's; other addresses download meanwhile.
func TestPublicDownloadsPerAddress(t *testing.T) {
	h := open(t)
	handler := h.srv.Handler()
	get := func(from string, w http.ResponseWriter) {
		req := httptest.NewRequest(http.MethodGet, "/v1/states/0", nil)
		req.RemoteAddr = from + ":1234"
		handler.ServeHTTP(w, req)
	}
	slow := &blockedWriter{header: http.Header{}, started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		get("192.0.2.1", slow)
	}()
	<-slow.started
	rec := httptest.NewRecorder()
	if get("192.0.2.1", rec); rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("a second download from the address: %d %s, want 503 with Retry-After", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	if get("192.0.2.2", rec); rec.Code != http.StatusOK {
		t.Fatalf("a download from another address meanwhile: %d %s", rec.Code, rec.Body)
	}
	close(slow.release)
	<-done
	rec = httptest.NewRecorder()
	if get("192.0.2.1", rec); rec.Code != http.StatusOK {
		t.Fatalf("a download from the address once its first is done: %d %s", rec.Code, rec.Body)
	}
}

// TestContributionLimits (audit R-012): a phase takes so many contributions
// from one network address, and in all, and gives no turn while the disk
// under it is short of space.
func TestContributionLimits(t *testing.T) {
	t.Run("from one address", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxContributionsPerAddress: 1})
		for name, addr := range map[string]string{"alice": "192.0.2.1", "bob": "192.0.2.1", "carol": "192.0.2.2"} {
			h.addrs[name] = addr
			h.register(name, Registration{Name: name})
		}
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.mustContribute("alice")
		if code, body := h.do("bob", http.MethodPost, "/v1/turn", nil); code != http.StatusForbidden {
			t.Fatalf("bob, from alice's address: %d %s, want 403", code, body)
		}
		wantStatus(t, "carol", h.ask("carol"), Go, 0)
	})
	t.Run("in all", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxContributions: 1})
		for i, name := range []string{"alice", "bob"} {
			h.addrs[name] = fmt.Sprintf("192.0.2.%d", i+1)
			h.register(name, Registration{Name: name})
		}
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.mustContribute("alice")
		if r := h.ask("bob"); r.Status != Closed || r.Notice == "" {
			t.Fatalf("bob, after the phase's one contribution: %+v, want closed, saying why", r)
		}
		if !h.status().Closed {
			t.Fatal("the status of a phase that takes no more says it is open")
		}
	})
	t.Run("short of disk space", func(t *testing.T) {
		if !lockedHere() {
			t.Skip("free space is told on Linux and macOS")
		}
		h := newHarness(t, Config{Auth: AuthOpen, MinFree: 1 << 62})
		h.addrs["alice"] = "192.0.2.1"
		h.register("alice", Registration{Name: "alice"})
		wantStatus(t, "alice", h.ask("alice"), Queued, 1)
	})
}

// TestRegistrationsAreBounded (audit R-013): the coordinator keeps so many
// registrations at once, and asks for more to wait.
func TestRegistrationsAreBounded(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, MaxSessions: 2})
	for i, name := range []string{"a", "b", "c"} {
		h.addrs[name] = fmt.Sprintf("192.0.2.%d", i+1)
	}
	h.register("a", Registration{Name: "a"})
	h.register("b", Registration{Name: "b"})
	if code, body := h.tryRegister("c", Registration{Name: "c"}); code != http.StatusServiceUnavailable {
		t.Fatalf("a third registration, at most 2: %d %s, want 503", code, body)
	}
}

// TestRefusalsKeepPathsOut (audit R-040): an uploader is told why their
// contribution was refused, without the coordinator's file names.
func TestRefusalsKeepPathsOut(t *testing.T) {
	h := open(t, "alice")
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	code, body := h.do("alice", http.MethodPut, "/v1/contribution", []byte("not a state"))
	if code != http.StatusUnprocessableEntity || bytes.Contains(body, []byte(h.dir)) || bytes.Contains(body, []byte("incoming")) {
		t.Fatalf("a refused upload: %d %s, want 422 naming no file of the coordinator's", code, body)
	}
}

// failingWriter is a response whose body cannot be written.
type failingWriter struct{ header http.Header }

func (f *failingWriter) Header() http.Header       { return f.header }
func (f *failingWriter) WriteHeader(int)           {}
func (f *failingWriter) Write([]byte) (int, error) { return 0, errors.New("connection gone") }

// TestRepliesLogThroughConfig (audit R-044): an answer that cannot be written
// is told to the coordinator's own log, as everything else is.
func TestRepliesLogThroughConfig(t *testing.T) {
	var logged bytes.Buffer
	h := newHarness(t, Config{Auth: AuthOpen, Log: log.New(&logged, "", 0)})
	h.srv.reply(&failingWriter{header: http.Header{}}, Reply{Status: Queued})
	h.srv.fail(&failingWriter{header: http.Header{}}, http.StatusBadRequest, "no")
	if got := logged.String(); !strings.Contains(got, "writing a reply") || !strings.Contains(got, "writing an error") {
		t.Fatalf("the coordinator's log: %q, want both failed answers", got)
	}
}
