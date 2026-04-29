package terminal

import (
	"fmt"
	"strings"
)

// Fallback returns a human-readable message containing the path so the user
// can cd manually when no supported terminal is available. The path is
// single-quoted for safety when the user copy-pastes the command.
func Fallback(dir string) string {
	return fmt.Sprintf("cd %s", shellQuote(dir))
}

// shellQuote wraps a string in single quotes, escaping any embedded single
// quotes using the POSIX '\” idiom.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, `'`, `'\''`) + "'"
}
