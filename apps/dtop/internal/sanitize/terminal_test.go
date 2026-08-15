package sanitize

import (
	"strings"
	"testing"
)

func TestTerminalLineRemovesTerminalControls(t *testing.T) {
	input := "safe\x1b]52;c;secret\a\x1b[31m red\u009b2J\nnext"
	if got, want := TerminalLine(input), "safe]52;c;secret[31m red2Jnext"; got != want {
		t.Fatalf("TerminalLine() = %q, want %q", got, want)
	}
}

func TestTerminalTextPreservesLayoutOnly(t *testing.T) {
	input := "first\nsecond\tvalue\r\x1b[2J"
	if got, want := TerminalText(input, MaxErrorRunes), "first\nsecond\tvalue[2J"; got != want {
		t.Fatalf("TerminalText() = %q, want %q", got, want)
	}
}

func TestTerminalLineBoundsRunes(t *testing.T) {
	got := TerminalLine(strings.Repeat("a", MaxLogLineRunes+1))
	if len([]rune(got)) != MaxLogLineRunes+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("TerminalLine() did not truncate safely: length=%d", len([]rune(got)))
	}
}
