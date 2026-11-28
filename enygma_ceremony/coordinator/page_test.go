package coordinator

import (
	"bytes"
	"testing"
)

// TestPageLineEndings (audit I6): the page is hashed and served with LF line
// endings, as a browser reads it, so one built from a CRLF checkout serves a
// page whose script and style run.
func TestPageLineEndings(t *testing.T) {
	crlf := bytes.ReplaceAll(page, []byte("\n"), []byte("\r\n"))
	cr := bytes.ReplaceAll(page, []byte("\n"), []byte("\r"))
	for name, file := range map[string][]byte{"CRLF": crlf, "CR": cr} {
		got := lf(file)
		if !bytes.Equal(got, page) {
			t.Errorf("the page from a %s file is not the page", name)
		}
		for _, tag := range [][2]string{{"<script>", "</script>"}, {"<style>", "</style>"}} {
			if inlineHash(got, tag[0], tag[1]) != inlineHash(page, tag[0], tag[1]) {
				t.Errorf("the page from a %s file hashes its %s otherwise", name, tag[0])
			}
		}
	}
	if bytes.ContainsRune(page, '\r') {
		t.Error("the page served holds a CR")
	}
}
