package coordinator

import (
	"embed"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// webScripts are the browser client's worker scripts.
//
//go:embed web/worker.js web/heartbeat.js
var webScripts embed.FS

// webBuilt are the browser client's files make ceremony-dist builds into the
// download directory, by name, with their content types: the participant's
// program built for the browser, and Go's loader for it.
var webBuilt = map[string]string{
	"ceremony.wasm": "application/wasm",
	"wasm_exec.js":  "text/javascript; charset=utf-8",
}

// workerPolicy is the Content Security Policy of the worker scripts: they
// load the program from this coordinator, compile it, and talk to nothing else.
const workerPolicy = "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'"

// browser reports whether the page can contribute from the browser: the
// download directory holds the program built for it.
func (s *Server) browser() bool {
	if s.cfg.Downloads == "" {
		return false
	}
	for name := range webBuilt {
		if fi, err := os.Stat(filepath.Join(s.cfg.Downloads, name)); err != nil || !fi.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// handleWeb serves the browser client's files: the worker scripts, and the
// program built for the browser, gzipped when make ceremony-dist left a
// .gz beside it and the browser takes gzip.
func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", workerPolicy)
	w.Header().Set("Cache-Control", "no-cache") // a rebuilt program replaces the old at once
	if data, err := webScripts.ReadFile("web/" + name); err == nil {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		if _, err := w.Write(data); err != nil {
			s.cfg.Log.Printf("sending %s: %v", name, err)
		}
		return
	}
	ctype, ok := webBuilt[name]
	if !ok || !s.browser() {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.cfg.Downloads, name)
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Vary", "Accept-Encoding")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		if f, err := os.Open(path + ".gz"); err == nil {
			defer f.Close()
			if fi, err := f.Stat(); err == nil && fi.Mode().IsRegular() {
				w.Header().Set("Content-Encoding", "gzip")
				http.ServeContent(w, r, "", fi.ModTime(), f)
				return
			}
		}
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

// handleGitHubRelay relays a browser's GitHub device-flow request
// (/github/login/device/code or /github/login/oauth/access_token) to GitHub,
// which does not answer browsers' requests itself. It always asks for this
// coordinator's OAuth app, and passes on only the device flow's fields.
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
