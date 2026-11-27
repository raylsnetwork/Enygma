package coordinator

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxName is the longest name, in characters, a participant may be shown by.
const maxName = 64

var (
	// errNameLength is a name that is empty, or longer than maxName characters.
	errNameLength = errors.New("a name is 1 to 64 characters")
	// errNameCharacters is a name holding what could pass for something else.
	errNameCharacters = errors.New("a name cannot hold control or invisible characters")
	// errNameVisible is a name with nothing in it to read.
	errNameVisible = errors.New("a name holds at least one letter or digit")
)

// cleanName returns the name a participant asked to be shown by, as the
// transcript shows it: trimmed, its runs of spaces made one. Letters, digits,
// spaces and punctuation of any script will do: "Jane Smith", "Zoë", "李雷".
// It refuses what could pass for something else: control characters (a new
// line forging a line of the coordinator's log), invisible formatting
// (right-to-left overrides that make one name read as another), and the
// other characters shown as nothing at all (default-ignorables, variation
// selectors), which could make a name look empty or like another (audit
// R-048). A name holds at least one letter or digit.
func cleanName(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", errNameCharacters
	}
	name := strings.Join(strings.Fields(s), " ")
	if n := utf8.RuneCountInString(name); n == 0 || n > maxName {
		return "", errNameLength
	}
	visible := false
	for _, r := range name {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector) ||
			!unicode.IsPrint(r) && r != ' ' {
			return "", errNameCharacters
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			visible = true
		}
	}
	if !visible {
		return "", errNameVisible
	}
	return name, nil
}
