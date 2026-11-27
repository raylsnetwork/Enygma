package coordinator

import (
	_ "embed"
	"net/http"
)

// page is the phase's page: the phase's progress, which its script reads
// from /v1/status and /v1/transcript, and where to get the participant's
// program (Config.ReleaseURL). It runs no participant code: the program
// that draws a participant's randomness never comes from the coordinator.
//
//go:embed page.html
var page []byte

// handlePage serves the phase's page. Its script puts every name a
// participant chose into the page as text, never as markup.
func (s *Server) handlePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(page); err != nil {
		s.cfg.Log.Printf("sending the page: %v", err)
	}
}
