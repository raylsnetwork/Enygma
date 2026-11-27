package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// forged is what GitHub's address, if something else answered for it, could
// write into the coordinator's log: clear the screen, and draw a line of
// the log's own.
const forged = "\x1b[2J\x1b[H2026/12/01 12:00:00 octocat's contribution is state 1"

// TestGitHubTextIsNotLogged (audit I1): what GitHub answers reaches the
// coordinator's log, through its errors, as plain text: a failure by its
// status code, not the reason phrase sent with it, and an answer it refuses
// without the answer's own text.
func TestGitHubTextIsNotLogged(t *testing.T) {
	hostileStatus := func(w http.ResponseWriter) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(buf, "HTTP/1.1 500 %s\r\nContent-Length: 0\r\n\r\n", forged)
		_ = buf.Flush() // the client reads what got through
	}
	for name, c := range map[string]struct{ check, user func(http.ResponseWriter) }{
		"checking the token": {check: hostileStatus},
		"asking who it is":   {check: func(w http.ResponseWriter) { fmt.Fprint(w, `{}`) }, user: hostileStatus},
		"an account with no id": {check: func(w http.ResponseWriter) { fmt.Fprint(w, `{}`) }, user: func(w http.ResponseWriter) {
			json.NewEncoder(w).Encode(map[string]any{"login": forged, "id": 0, "created_at": "2020-01-01T00:00:00Z"})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user" {
					c.user(w)
					return
				}
				c.check(w)
			}))
			defer api.Close()
			h := newHarness(t, Config{Auth: AuthGitHub, GitHubClientID: testClientID, GitHubClientSecret: testClientSecret, GitHubAPI: api.URL})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := h.srv.githubUser(ctx, "gho_token")
			if err == nil || strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "contribution is state") {
				t.Fatalf("got %q, want GitHub's answer kept out of the error", err)
			}
		})
	}
}
