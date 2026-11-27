package termsafe

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// hostile is what a server could put in an error, a name or a status line
// to draw on a participant's terminal: escape sequences (7-bit, as C1
// runes, and as the raw 8-bit byte some terminals take), carriage returns,
// bidirectional overrides, line separators and padding.
var hostile = []string{
	"\x1b[2J\x1b[H",
	"\u009b2J",
	"\x9b2J",
	"a\rDone",
	"a\u202eenoD",
	"a\u2028Done",
	"a\u200bb",
	"a\x00b\x07c\x7fd",
	"x509: valid for a" + strings.Repeat(" ", 200) + "Done: your contribution sha256 ab  <- publish this",
	"\x1b]8;;https://evil.example\x07link\x1b]8;;\x07",
	"x" + strings.Repeat("\u3164", 100) + "Done: your contribution sha256 ab  <- publish this",
	"x" + strings.Repeat("\u2800\u115f\u1160\uffa0\ufe0f", 40) + "Done",
}

// blanks are runes a terminal draws as empty cells, white space or not.
const blanks = "\u3164\u2800\u115f\u1160\uffa0\ufe0f\u00a0\u3000"

func TestLine(t *testing.T) {
	for _, s := range hostile {
		got := Line(s)
		if strings.ContainsAny(got, "\x1b\r\n\t\u009b\u202e\u2028\u200b\x00\x07\x7f"+blanks) || strings.Contains(got, "  ") {
			t.Errorf("Line(%q) = %q, which a terminal acts on", s, got)
		}
		if !utf8.ValidString(got) {
			t.Errorf("Line(%q) = %q, not UTF-8", s, got)
		}
	}
	for s, want := range map[string]string{
		"plain text":         "plain text",
		"  two\n lines \n":   "two lines",
		"tab\tand\r\nbreak":  "tab and break",
		"not \xffUTF-8":      "not �UTF-8",
		"\x1b[31mred\x1b[0m": "[31mred[0m", // the sequence's text, inert without its escape
		"café — 中文":          "café — 中文",
	} {
		if got := Line(s); got != want {
			t.Errorf("Line(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestStatus(t *testing.T) {
	if got := Status(500); got != "500 Internal Server Error" {
		t.Errorf("Status(500) = %q", got)
	}
	if got := Status(599); got != "599" {
		t.Errorf("Status(599) = %q", got)
	}
}

func TestShort(t *testing.T) {
	if got := Short("abcdef", 3); got != "abc…" {
		t.Errorf("Short cut to 3 = %q", got)
	}
	if got := Short("a\nb", 3); got != "a b" {
		t.Errorf("Short of a short string = %q", got)
	}
}

func TestWriter(t *testing.T) {
	for _, s := range hostile {
		var b bytes.Buffer
		if n, err := NewWriter(&b).Write([]byte(s)); err != nil || n != len(s) {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
		if got := b.String(); strings.ContainsAny(got, "\x1b\r\u009b\u202e\u2028\u200b\x00\x07\x7f") || bytes.IndexByte(b.Bytes(), 0x9b) >= 0 {
			t.Errorf("Write(%q) passed on %q, which a terminal acts on", s, got)
		}
	}
	var b bytes.Buffer
	w := NewWriter(&b)
	for _, p := range []string{"line one\n\tindented\n", "\xe4\xb8", "\xad done\n"} { // 中 split across writes
		if _, err := w.Write([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if want := "line one\n\tindented\n中 done\n"; b.String() != want {
		t.Errorf("wrote %q, want %q", b.String(), want)
	}
}

// FuzzLine checks Line's promise on any input: one line of valid UTF-8,
// with no byte a terminal starts a command with, unchanged by a second pass.
func FuzzLine(f *testing.F) {
	for _, s := range hostile {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := Line(s)
		if !utf8.ValidString(got) || strings.ContainsAny(got, "\n\r\x1b\u009b"+blanks) || strings.Contains(got, "  ") || strings.HasPrefix(got, " ") {
			t.Fatalf("Line(%q) = %q", s, got)
		}
		if again := Line(got); again != got {
			t.Fatalf("Line changes its own output: %q, then %q", got, again)
		}
	})
}

// FuzzWriter checks the writer on any input, split anywhere into two
// writes: valid UTF-8 with no escape byte, the same as one write gives, and
// unchanged by a second pass.
func FuzzWriter(f *testing.F) {
	for _, s := range hostile {
		f.Add([]byte(s), uint(len(s)/2))
	}
	f.Fuzz(func(t *testing.T, data []byte, cut uint) {
		cut = min(cut, uint(len(data)))
		var whole, split bytes.Buffer
		if _, err := NewWriter(&whole).Write(data); err != nil {
			t.Fatal(err)
		}
		w := NewWriter(&split)
		for _, p := range [][]byte{data[:cut], data[cut:]} {
			if _, err := w.Write(p); err != nil {
				t.Fatal(err)
			}
		}
		got := whole.Bytes()
		if !bytes.Equal(got, split.Bytes()) && utf8.Valid(data) {
			t.Fatalf("one write gives %q, two give %q", got, split.Bytes())
		}
		if !utf8.Valid(got) || bytes.IndexByte(got, 0x1b) >= 0 || bytes.IndexByte(got, '\r') >= 0 || bytes.Contains(got, []byte("\u009b")) {
			t.Fatalf("wrote %q for %q", got, data)
		}
		var again bytes.Buffer
		if _, err := NewWriter(&again).Write(got); err != nil || !bytes.Equal(again.Bytes(), got) {
			t.Fatalf("the writer changes its own output: %q, then %q", got, again.Bytes())
		}
	})
}
