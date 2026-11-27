// Package termsafe keeps text that others chose from acting on a terminal.
//
// A terminal takes control characters as commands: an escape sequence can
// clear the screen and draw anything, such as a receipt for a sha256 the
// participant never made (audit M1 to M3). Format characters reorder or hide
// the text around them, and a line break lets a value start a line of its
// own that looks like the program's. Two rules keep that out:
//
//   - Line makes a value from elsewhere (a server's error, a name, a status)
//     one line of plain text, for printing inside a line of the program's own.
//   - NewWriter wraps the program's terminal output as a whole, so a sink that
//     missed Line still cannot carry an escape sequence through.
package termsafe

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// inert reports whether a terminal shows r as text and acts on nothing:
// not a control character (C0, DEL, C1, which start escape sequences), not
// a format character (bidirectional controls, zero-width characters), and
// not a line or paragraph separator.
func inert(r rune) bool {
	return !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) && !unicode.In(r, unicode.Zl, unicode.Zp)
}

// blank reports whether r draws as empty space: white space, line breaks
// included, and the letters and symbols that draw nothing though they are
// not white space (the Hangul fillers and the other default-ignorable code
// points, variation selectors, U+2800 BRAILLE PATTERN BLANK).
func blank(r rune) bool {
	return unicode.IsSpace(r) || r == '\u2800' ||
		unicode.In(r, unicode.Zl, unicode.Zp, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector)
}

// Line is s as one line of plain text: each run of blank runes (blank), line
// breaks included, becomes one space, every rune a terminal could act on is
// dropped, and bytes that are not UTF-8 become U+FFFD. A value from
// elsewhere printed through Line cannot start a line of its own, or pad
// itself to the terminal's width with empty cells to seem to.
func Line(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s { // invalid UTF-8 comes out as U+FFFD
		switch {
		case blank(r):
			space = true
		case inert(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Short is Line(s) cut to at most n runes, with "…" when cut: a value from
// elsewhere that may be long, such as a coordinator's notice.
func Short(s string, n int) string {
	s = Line(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// Status is an HTTP status code as the program shows it: the code, and Go's
// own text for it when it has one, never the reason phrase a server sent
// beside it, which it chose (audit M2).
func Status(code int) string {
	if text := http.StatusText(code); text != "" {
		return fmt.Sprintf("%d %s", code, text)
	}
	return strconv.Itoa(code)
}

// NewWriter returns a writer that passes to w what is written to it, with
// every rune a terminal could act on dropped except line breaks and tabs,
// and bytes that are not UTF-8 as U+FFFD. A rune split across two writes is
// held until the second. It is safe for concurrent use, as a terminal shared
// by a logger and a command is.
func NewWriter(w io.Writer) io.Writer { return &writer{w: w} }

type writer struct {
	mu      sync.Mutex
	w       io.Writer
	pending []byte // the start of a rune the last write ended in
}

func (t *writer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	data := append(t.pending, p...)
	t.pending = nil
	out := make([]byte, 0, len(data))
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 && !utf8.FullRune(data) {
			t.pending = append([]byte(nil), data...) // the rest of this rune comes in the next write
			break
		}
		data = data[size:]
		if r == '\n' || r == '\t' || inert(r) {
			out = utf8.AppendRune(out, r) // U+FFFD for a byte that is not UTF-8
		}
	}
	if _, err := t.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}
