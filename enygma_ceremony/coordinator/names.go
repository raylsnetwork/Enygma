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
	errNameCharacters = errors.New("a name cannot hold control or invisible formatting characters")
)

// cleanName returns the name a participant asked to be shown by, as the
// transcript shows it: trimmed, its runs of spaces made one. Letters, digits,
// spaces and punctuation of any script will do: "Jane Smith", "Zoë", "李雷".
// It refuses only what could pass for something else: control characters (a
// new line forging a line of the coordinator's log) and invisible formatting
// (right-to-left overrides that make one name read as another).
func cleanName(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", errNameCharacters
	}
	name := strings.Join(strings.Fields(s), " ")
	if n := utf8.RuneCountInString(name); n == 0 || n > maxName {
		return "", errNameLength
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", errNameCharacters
		}
	}
	return name, nil
}
