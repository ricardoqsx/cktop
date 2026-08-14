package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestShellViewIncludesProductContent(t *testing.T) {
	model := newShell(ShellOptions{
		Title:    "dtop",
		Subtitle: "Docker TUI",
		Views: []View{{
			Title:    "Containers",
			Status:   StatusReady,
			Summary:  "ready",
			Sections: []Section{{Title: "Status", Body: "running"}},
		}},
	})

	view := model.View()
	for _, expected := range []string{"dtop", "Docker TUI", "Containers", "Status", "ready", "running", "[q] quit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected view to contain %q, got %q", expected, view)
		}
	}
}

func TestShellRendersFooterNoticeBeforeHelpLine(t *testing.T) {
	model := newShell(ShellOptions{Footer: "q quit", FooterNotice: "Docker Hub: run docker login", Views: []View{{Title: "Containers"}}})
	view := ansi.Strip(model.View())
	notice := strings.Index(view, "Docker Hub: run docker login")
	help := strings.Index(view, "[q] quit")
	if notice < 0 || help < 0 || notice > help {
		t.Fatalf("footer notice was not rendered before help: %q", view)
	}
}

func TestShellUsesFallbackTitle(t *testing.T) {
	model := newShell(ShellOptions{})

	if model.title != "cktop" {
		t.Fatalf("expected fallback title cktop, got %q", model.title)
	}
}

func TestShellUsesFallbackView(t *testing.T) {
	model := newShell(ShellOptions{})

	if len(model.views) != 1 {
		t.Fatalf("expected one fallback view, got %d", len(model.views))
	}
	if model.views[0].Status != StatusEmpty {
		t.Fatalf("expected empty fallback view, got %v", model.views[0].Status)
	}
}

func TestDefaultLocalizerFormatsEnglishDecimalsAndPlurals(t *testing.T) {
	localizer := DefaultLocalizer()
	if got := localizer.Decimal(12.5, 2); got != "12.50" {
		t.Fatalf("expected English decimal, got %q", got)
	}
	if got := localizer.Plural(MessageShellViewCount, 1); got != "1 view" {
		t.Fatalf("expected singular view, got %q", got)
	}
	if got := localizer.Plural(MessageShellViewCount, 2); got != "2 views" {
		t.Fatalf("expected plural views, got %q", got)
	}
}

func TestShellBracketsEnglishAndSpanishKeyboardHelp(t *testing.T) {
	tests := []struct {
		name      string
		localizer Localizer
		footer    []string
		help      []string
		minimal   []string
		compact   []string
	}{
		{
			name:   "English fallback",
			footer: []string{"[Tab] next", "[Shift+Tab] prev", "[?] help", "[q] quit"},
			help:   []string{"Help", "[Tab]", "[Shift+Tab]", "[Esc]", "[q]"},
			minimal: []string{
				"[Tab] next", "[q] quit",
			},
			compact: []string{"Help", "[Tab] next", "[Shift+Tab] prev", "[Esc] close", "[q] quit"},
		},
		{
			name: "Spanish",
			localizer: testLocalizer{messages: map[string]string{
				MessageShellHelpTitle:        "Ayuda",
				MessageShellHelpNextView:     "Tab        vista siguiente",
				MessageShellHelpPreviousView: "Shift+Tab  vista anterior",
				MessageShellHelpClose:        "Esc        cerrar ayuda",
				MessageShellHelpQuit:         "q          salir",
				MessageShellHelpCompact:      "Ayuda: tab siguiente | shift+tab anterior | esc cerrar | q salir",
				MessageShellFooterDefault:    "tab siguiente  shift+tab anterior  ? ayuda  q salir",
				MessageShellFooterMinimal:    "tab siguiente  q salir",
			}},
			footer: []string{"[Tab] siguiente", "[Shift+Tab] anterior", "[?] ayuda", "[q] salir"},
			help:   []string{"Ayuda", "[Tab]", "vista siguiente", "[Shift+Tab]", "vista anterior", "[Esc]", "cerrar ayuda", "[q]", "salir"},
			minimal: []string{
				"[Tab] siguiente", "[q] salir",
			},
			compact: []string{"Ayuda", "[Tab] siguiente", "[Shift+Tab] anterior", "[Esc] cerrar", "[q] salir"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := newShell(ShellOptions{Localizer: test.localizer})
			footer := ansi.Strip(model.View())
			for _, expected := range test.footer {
				if !strings.Contains(footer, expected) {
					t.Fatalf("expected footer to contain %q, got %q", expected, footer)
				}
			}

			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
			help := ansi.Strip(updated.View())
			for _, expected := range test.help {
				if !strings.Contains(help, expected) {
					t.Fatalf("expected help to contain %q, got %q", expected, help)
				}
			}

			minimalLayout := Layout{Mode: LayoutMinimal, ContentWidth: 200}
			minimalFooter := ansi.Strip(model.renderFooter(minimalLayout))
			for _, expected := range test.minimal {
				if !strings.Contains(minimalFooter, expected) {
					t.Fatalf("expected minimal footer to contain %q, got %q", expected, minimalFooter)
				}
			}

			model.showHelp = true
			compactHelp := ansi.Strip(model.renderContentDense(minimalLayout))
			for _, expected := range test.compact {
				if !strings.Contains(compactHelp, expected) {
					t.Fatalf("expected compact help to contain %q, got %q", expected, compactHelp)
				}
			}
		})
	}
}

func TestShellDoesNotPrefixEmptyTabTitle(t *testing.T) {
	model := newShell(ShellOptions{Views: []View{
		{Title: "Volumes", Status: StatusEmpty},
		{Title: "Images", Status: StatusLoading},
	}})

	tabs := ansi.Strip(model.renderTabs(ResolveLayout(120, 30)))
	if strings.Contains(tabs, "empty Volumes") {
		t.Fatalf("empty tab title was prefixed: %q", tabs)
	}
	if !strings.Contains(tabs, "Volumes") {
		t.Fatalf("expected empty tab title, got %q", tabs)
	}
	if !strings.Contains(tabs, "loading Images") {
		t.Fatalf("expected loading tab prefix, got %q", tabs)
	}
}

func TestShellNavigatesViews(t *testing.T) {
	model := newShell(ShellOptions{
		Views: []View{{Title: "One"}, {Title: "Two"}},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(shellModel)
	if next.active != 1 {
		t.Fatalf("expected active view 1, got %d", next.active)
	}

	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	prev := updated.(shellModel)
	if prev.active != 0 {
		t.Fatalf("expected active view 0, got %d", prev.active)
	}
}

func TestShellTogglesHelp(t *testing.T) {
	model := newShell(ShellOptions{})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	help := updated.(shellModel)
	if !help.showHelp {
		t.Fatal("expected help to be visible")
	}
	if !strings.Contains(help.View(), "Shift+Tab") {
		t.Fatalf("expected help content, got %q", help.View())
	}

	updated, _ = help.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := updated.(shellModel)
	if closed.showHelp {
		t.Fatal("expected help to close on esc")
	}
}

func TestShellRendersCompactView(t *testing.T) {
	model := newShell(ShellOptions{
		Title: "dtop",
		Views: []View{{Title: "Containers", Status: StatusLoading, Summary: "waiting"}},
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 32, Height: 10})
	compact := updated.(shellModel).View()

	for _, expected := range []string{"dtop", "Containers", "LOADING waiting"} {
		if !strings.Contains(compact, expected) {
			t.Fatalf("expected compact view to contain %q, got %q", expected, compact)
		}
	}
}

func TestShellNeverExceedsTerminalWidth(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 160, Height: 45},
		{Width: 100, Height: 30},
		{Width: 72, Height: 20},
		{Width: 48, Height: 12},
		{Width: 32, Height: 8},
	}
	for _, size := range sizes {
		model := newShell(ShellOptions{
			Title:    "dtop",
			Subtitle: "desktop-linux | Docker 29.6.2 | 3/4 running",
			Views: []View{{
				Title:      "Containers",
				Status:     StatusReady,
				HideStatus: true,
				Sections:   []Section{{Body: strings.Repeat("x", 200)}},
			}},
		})
		updated, _ := model.Update(size)
		view := updated.(shellModel).View()
		lines := strings.Split(view, "\n")
		if len(lines) > size.Height {
			t.Fatalf("size %dx%d rendered %d lines", size.Width, size.Height, len(lines))
		}
		for _, line := range lines {
			if got := ansi.StringWidth(line); got > size.Width {
				t.Fatalf("size %dx%d rendered line width %d: %q", size.Width, size.Height, got, line)
			}
		}
	}
}

func TestResolveLayoutUsesWidthAndHeight(t *testing.T) {
	if got := ResolveLayout(160, 45).Mode; got != LayoutWide {
		t.Fatalf("expected wide layout, got %v", got)
	}
	if got := ResolveLayout(100, 30).Mode; got != LayoutMedium {
		t.Fatalf("expected medium layout, got %v", got)
	}
	if got := ResolveLayout(72, 20).Mode; got != LayoutCompact {
		t.Fatalf("expected compact layout, got %v", got)
	}
	if got := ResolveLayout(100, 10).Mode; got != LayoutMinimal {
		t.Fatalf("expected low terminal to use minimal layout, got %v", got)
	}
}

type testLocalizer struct {
	messages map[string]string
}

func (localizer testLocalizer) Text(id string, args ...any) string {
	if message, ok := localizer.messages[id]; ok {
		return formatMessage(message, args...)
	}
	return DefaultLocalizer().Text(id, args...)
}

func (testLocalizer) Plural(id string, count int, args ...any) string {
	return DefaultLocalizer().Plural(id, count, args...)
}

func (testLocalizer) Decimal(value float64, precision int) string {
	return DefaultLocalizer().Decimal(value, precision)
}
