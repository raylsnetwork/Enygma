package fuzzedit

import (
	"bytes"
	"testing"
)

// TestApply: each op edits as Apply says, offsets wrap, and data is left as it was.
func TestApply(t *testing.T) {
	data := []byte("abcdef")
	for _, c := range []struct {
		name   string
		script []byte
		want   string
	}{
		{"no script", nil, "abcdef"},
		{"set", Op(0, 1, 'X'), "aXcdef"},
		{"flip", Op(1, 0, 0), "`bcdef"}, // 'a' ^ 1
		{"delete", Op(2, 2, 1), "abef"},
		{"delete past the end", Op(2, 4, 31), "abcd"},
		{"insert", Op(3, 0, 'Z'), "Zabcdef"},
		{"offset wraps", Op(0, 7, 'X'), "aXcdef"},
		{"two edits", append(Op(0, 0, 'X'), Op(2, 5, 0)...), "Xbcde"},
		{"a trailing part is ignored", append(Op(0, 0, 'X'), 1, 2), "Xbcdef"},
	} {
		if got := Apply(data, c.script); string(got) != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if !bytes.Equal(data, []byte("abcdef")) {
		t.Errorf("Apply changed its input: %q", data)
	}
	if got := Apply(nil, append(Op(0, 0, 'X'), Op(3, 9, 'Y')...)); string(got) != "Y" {
		t.Errorf("editing nothing: %q, want %q", got, "Y")
	}
}
