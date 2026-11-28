package coordinator

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"net/http"
)

// page is the phase's page: the phase's progress, which its script reads
// from /v1/status and /v1/transcript, and where to get the participant's
// program (Config.ReleaseURL). It runs no participant code: the program
// that draws a participant's randomness never comes from the coordinator.
//
//go:embed page.html
var pageFile []byte

// page is pageFile as a browser reads it, its line endings LF (lf): a
// checkout with CRLF ones would otherwise serve a page whose script and
// style hash to other than pagePolicy says, and never run (audit I6).
var page = lf(pageFile)

// lf is html with each CRLF, and each CR alone, made LF, as a browser
// normalizes a page's text before hashing its inline script and style.
func lf(html []byte) []byte {
	return bytes.ReplaceAll(bytes.ReplaceAll(html, []byte("\r\n"), []byte("\n")), []byte("\r"), []byte("\n"))
}

// pagePolicy is the page's Content Security Policy (audit R-051, R-052): its
// own inline script and style, by their sha256, and nothing else. It may only
// fetch from this coordinator, has no base or form to redirect, and is never
// shown in a frame, where its text could be dressed up as something else.
var pagePolicy = "default-src 'none'" +
	"; script-src " + inlineHash(page, "<script>", "</script>") +
	"; style-src " + inlineHash(page, "<style>", "</style>") +
	"; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// inlineHash is the CSP source for the text between open and close, the
// first such element of html: its sha256, in base64.
func inlineHash(html []byte, open, close string) string {
	_, after, ok := bytes.Cut(html, []byte(open))
	if !ok {
		panic("coordinator: the page has no " + open)
	}
	text, _, ok := bytes.Cut(after, []byte(close))
	if !ok {
		panic("coordinator: the page has no " + close)
	}
	sum := sha256.Sum256(text)
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// handlePage serves the phase's page. Its script puts every name a
// participant chose into the page as text, never as markup.
func (s *Server) handlePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", pagePolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if _, err := w.Write(page); err != nil {
		s.cfg.Log.Printf("sending the page: %v", err)
	}
}
