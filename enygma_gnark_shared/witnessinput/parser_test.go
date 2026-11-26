package witnessinput_test

import (
	"testing"

	"enygma_gnark_shared/witnessinput"
)

func TestParserChecksFieldRange(t *testing.T) {
	const fieldOrder = "21888242871839275222246405745257275088548364400416034343698204186575808495617"
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"0", true},
		{"21888242871839275222246405745257275088548364400416034343698204186575808495616", true},
		{fieldOrder, false},
		{"-1", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			var parser witnessinput.Parser
			n := parser.Parse(tc.value)
			if valid := parser.Err() == nil; valid != tc.valid {
				t.Fatalf("valid = %v, want %v", valid, tc.valid)
			}
			if tc.valid && n.String() != tc.value {
				t.Fatalf("got %s, want %s", n, tc.value)
			}
		})
	}
}

func TestParserAcceptsDecimalAndHex(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"123", "123"},
		{"0xabcd", "43981"},
		{"0Xc1f7", "49655"},
		{"010", "10"},
		{"09", "9"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var parser witnessinput.Parser
			got := parser.Parse(tc.input)
			if err := parser.Err(); err != nil {
				t.Fatalf("parse %q: %v", tc.input, err)
			}
			if got.String() != tc.want {
				t.Fatalf("parse %q = %s, want %s", tc.input, got, tc.want)
			}
		})
	}
}
