package coordinator

import (
	_ "embed"
	"net/http"
)

// page is the phase's page: a button that contributes from the browser
// (through the workers in web/), the program for those who prefer it, and
// the phase's progress, which its script reads from /v1/status and
// /v1/transcript.
//
//go:embed page.html
var page []byte

// handlePage serves the phase's page. Its script puts every name a
// participant chose into the page as text, never as markup.
func (s *Server) handlePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; worker-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(page); err != nil {
		s.cfg.Log.Printf("sending the page: %v", err)
	}
}
