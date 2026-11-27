package participant

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/coordinator"
)

// TestDoneMustBeThisRunsContribution (audit R-002): a coordinator's "you
// have contributed" is taken only for a contribution this run made, and only
// once its transcript holds it. Otherwise a coordinator could refuse or cut
// short an honest participant's upload, then hand them a contribution of its
// own as their receipt: the participant would publish it, and their own
// contribution would be nowhere in the keys.
func TestDoneMustBeThisRunsContribution(t *testing.T) {
	forged := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name    string
		upload  string // what the coordinator does with the upload: conflict, drop, or accept-drop (accept, lose the answer)
		forge   bool   // then answers the next ask with a forged "done"
		wantErr string
	}{
		{"a 409, then someone else's contribution", "conflict", true, "this program made no such contribution"},
		{"a lost upload, then someone else's contribution", "drop", true, "this program made no such contribution"},
		{"an accepted upload whose answer is lost", "accept-drop", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
			target, err := url.Parse(p.url)
			if err != nil {
				t.Fatal(err)
			}
			upstream := httputil.NewSingleHostReverseProxy(target)
			var uploaded atomic.Bool
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPut && r.URL.Path == "/v1/contribution" && !uploaded.Load():
					uploaded.Store(true)
					switch tc.upload {
					case "conflict":
						io.Copy(io.Discard, r.Body)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusConflict)
						fmt.Fprint(w, `{"error":"your turn is over"}`)
					case "drop":
						io.Copy(io.Discard, r.Body)
						closeConnection(t, w)
					case "accept-drop":
						upstream.ServeHTTP(httptest.NewRecorder(), r)
						closeConnection(t, w)
					}
				case r.URL.Path == "/v1/turn" && uploaded.Load() && tc.forge:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"status":"done","index":1,"sha256":%q}`, forged)
				default:
					upstream.ServeHTTP(w, r)
				}
			}))
			defer proxy.Close()

			c := p.client(t, "alice")
			c.URL, c.Patience = proxy.URL, 30*time.Second
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			r, err := c.Run(ctx)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %+v, %v; want an error with %q", r, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(p.dir, "state-1"))
			if err != nil {
				t.Fatal(err)
			}
			if r.Already || r.Index != 1 || r.SHA256 != sha256.Sum256(data) {
				t.Fatalf("got %+v, want contribution 1 made by this run, sha256 %x", r, sha256.Sum256(data))
			}
		})
	}
}

// TestPrintable (audit R-027): what the coordinator says reaches the
// participant's terminal with no escape sequences or reordering characters.
func TestPrintable(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                      "plain",
		"\x1b[2J\x1b[1;1HOK: done":   "[2J[1;1HOK: done",
		"left‮right":            "leftright",
		"bell\a and\r\nnewline":      "bell andnewline",
		"\u009b31mC1 control":        "31mC1 control",
	} {
		if got := printable(in); got != want {
			t.Errorf("printable(%q) = %q, want %q", in, got, want)
		}
	}
	if got := []rune(printable(strings.Repeat("x", 1000))); len(got) != 301 {
		t.Errorf("a 1000-character message printed as %d characters, want 300 and an ellipsis", len(got))
	}
}

// closeConnection drops the connection under w without answering.
func closeConnection(t *testing.T, w http.ResponseWriter) {
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		t.Error(err)
		return
	}
	conn.Close()
}
