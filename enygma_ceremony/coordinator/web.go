package coordinator

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// handleGitHubRelay relays a GitHub device-flow request
// (/github/login/device/code or /github/login/oauth/access_token) to GitHub.
// It always asks for this coordinator's OAuth app, and passes on only the
// device flow's fields.
//
// The coordinator serves no participant code: participants run the program
// from the ceremony's GitHub release, or built from source.
func (s *Server) handleGitHubRelay(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Auth != AuthGitHub {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<12)
	if err := r.ParseForm(); err != nil {
		fail(w, http.StatusBadRequest, "want a form")
		return
	}
	form := url.Values{"client_id": {s.cfg.GitHubClientID}}
	for _, field := range []string{"device_code", "grant_type"} {
		if v := r.PostForm.Get(field); v != "" {
			form.Set(field, v)
		}
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		strings.TrimSuffix(s.cfg.GitHub, "/")+strings.TrimPrefix(r.URL.Path, "/github"), strings.NewReader(form.Encode()))
	if err != nil {
		fail(w, http.StatusInternalServerError, "cannot ask GitHub")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := githubClient.Do(req)
	if err != nil {
		s.cfg.Log.Printf("relaying a GitHub sign-in: %v", err)
		fail(w, http.StatusBadGateway, "GitHub could not be reached; try again")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, io.LimitReader(resp.Body, 1<<16)); err != nil {
		s.cfg.Log.Printf("relaying a GitHub sign-in: %v", err)
	}
}
