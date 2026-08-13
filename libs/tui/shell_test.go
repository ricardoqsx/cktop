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
	for _, expected := range []string{"dtop", "Docker TUI", "Containers", "Status", "ready", "running", "q quit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected view to contain %q, got %q", expected, view)
		}
	}
}

func TestShellRendersFooterNoticeBeforeHelpLine(t *testing.T) {
	model := newShell(ShellOptions{Footer: "q quit", FooterNotice: "Docker Hub: run docker login", Views: []View{{Title: "Containers"}}})
	view := ansi.Strip(model.View())
	notice := strings.Index(view, "Docker Hub: run docker login")
	help := strings.Index(view, "q quit")
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
