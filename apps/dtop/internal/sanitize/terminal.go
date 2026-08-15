package sanitize

import (
	"strings"
	"unicode"
)

const (
	MaxLogLineRunes = 4096
	MaxErrorRunes   = 8192
)

// TerminalText removes control characters that could be interpreted by a terminal.
func TerminalText(value string, maxRunes int) string {
	return terminalText(value, maxRunes, true)
}

// TerminalLine sanitizes a single line and bounds its rendered size.
func TerminalLine(value string) string {
	return terminalText(value, MaxLogLineRunes, false)
}

func terminalText(value string, maxRunes int, multiline bool) string {
	var result strings.Builder
	result.Grow(min(len(value), maxRunes))
	written := 0
	truncated := false
	for _, r := range value {
		if unicode.IsControl(r) {
			if multiline && (r == '\n' || r == '\t') {
				// Newlines and tabs are layout, not terminal commands.
			} else {
				continue
			}
		}
		if maxRunes > 0 && written == maxRunes {
			truncated = true
			break
		}
		result.WriteRune(r)
		written++
	}
	if truncated {
		result.WriteString("...")
	}
	return result.String()
}
