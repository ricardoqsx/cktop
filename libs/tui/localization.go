package tui

import (
	"fmt"
	"strconv"
)

// Localizer translates neutral UI text and formats locale-sensitive values.
type Localizer interface {
	Text(id string, args ...any) string
	Plural(id string, count int, args ...any) string
	Decimal(value float64, precision int) string
}

// Shared shell message IDs. Applications can provide these messages without
// coupling the shared TUI package to a particular translation implementation.
const (
	MessageShellFallbackTitle       = "shell.fallback.title"
	MessageShellFallbackViewTitle   = "shell.fallback.view.title"
	MessageShellFallbackViewSummary = "shell.fallback.view.summary"
	MessageShellStatusReady         = "shell.status.ready"
	MessageShellStatusLoading       = "shell.status.loading"
	MessageShellStatusEmpty         = "shell.status.empty"
	MessageShellStatusWarning       = "shell.status.warning"
	MessageShellStatusError         = "shell.status.error"
	MessageShellStatusUnavailable   = "shell.status.unavailable"
	MessageShellKeyQuit             = "shell.key.quit"
	MessageShellKeyNext             = "shell.key.next"
	MessageShellKeyPrevious         = "shell.key.previous"
	MessageShellKeyHelp             = "shell.key.help"
	MessageShellKeyBack             = "shell.key.back"
	MessageShellHelpTitle           = "shell.help.title"
	MessageShellHelpNextView        = "shell.help.next_view"
	MessageShellHelpPreviousView    = "shell.help.previous_view"
	MessageShellHelpClose           = "shell.help.close"
	MessageShellHelpQuit            = "shell.help.quit"
	MessageShellHelpCompact         = "shell.help.compact"
	MessageShellFooterDefault       = "shell.footer.default"
	MessageShellFooterMinimal       = "shell.footer.minimal"
	MessageShellTabPosition         = "shell.tab.position"
	MessageShellViewCount           = "shell.views"
)

var englishMessages = map[string]string{
	MessageShellFallbackTitle:        "cktop",
	MessageShellFallbackViewTitle:    "Overview",
	MessageShellFallbackViewSummary:  "No content available yet.",
	MessageShellStatusReady:          "ready",
	MessageShellStatusLoading:        "loading",
	MessageShellStatusEmpty:          "empty",
	MessageShellStatusWarning:        "warning",
	MessageShellStatusError:          "error",
	MessageShellStatusUnavailable:    "unavailable",
	MessageShellKeyQuit:              "quit",
	MessageShellKeyNext:              "next",
	MessageShellKeyPrevious:          "prev",
	MessageShellKeyHelp:              "help",
	MessageShellKeyBack:              "back",
	MessageShellHelpTitle:            "Help",
	MessageShellHelpNextView:         "[Tab]        next view",
	MessageShellHelpPreviousView:     "[Shift+Tab]  previous view",
	MessageShellHelpClose:            "[Esc]        close help",
	MessageShellHelpQuit:             "[q]          quit",
	MessageShellHelpCompact:          "Help: [Tab] next | [Shift+Tab] prev | [Esc] close | [q] quit",
	MessageShellFooterDefault:        "[Tab] next  [Shift+Tab] prev  [?] help  [q] quit",
	MessageShellFooterMinimal:        "[Tab] next  [q] quit",
	MessageShellTabPosition:          " %d/%d",
	MessageShellViewCount + ".one":   "%d view",
	MessageShellViewCount + ".other": "%d views",
}

type englishLocalizer struct{}

// DefaultLocalizer returns the English localizer used when none is supplied.
func DefaultLocalizer() Localizer {
	return englishLocalizer{}
}

func (englishLocalizer) Text(id string, args ...any) string {
	message, ok := englishMessages[id]
	if !ok {
		return id
	}
	return formatMessage(message, args...)
}

func (localizer englishLocalizer) Plural(id string, count int, args ...any) string {
	form := ".other"
	if count == 1 {
		form = ".one"
	}
	message, ok := englishMessages[id+form]
	if !ok {
		return localizer.Text(id, args...)
	}
	return formatMessage(message, append([]any{count}, args...)...)
}

func (englishLocalizer) Decimal(value float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func formatMessage(message string, args ...any) string {
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}
