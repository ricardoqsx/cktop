package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Section is a neutral content block rendered by the shared shell.
type Section struct {
	Title string
	Body  string
}

// View is a neutral tab or screen mounted by a product application.
type View struct {
	Title      string
	Status     Status
	Summary    string
	Sections   []Section
	HideStatus bool
}

// ShellOptions configures the neutral TUI shell without domain-specific data.
type ShellOptions struct {
	Title      string
	Subtitle   string
	Views      []View
	ActiveView int
	Footer     string
}

type shellModel struct {
	title    string
	subtitle string
	views    []View
	active   int
	showHelp bool
	bindings keyMap
	width    int
	height   int
	theme    theme
	footer   string
}

type keyMap struct {
	quit key.Binding
	next key.Binding
	prev key.Binding
	help key.Binding
	back key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		next: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		prev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// NewShell returns a Bubble Tea model with shared layout, navigation and states.
func NewShell(options ShellOptions) tea.Model {
	return newShell(options)
}

func newShell(options ShellOptions) shellModel {
	views := options.Views
	if len(views) == 0 {
		views = []View{{
			Title:   "Overview",
			Status:  StatusEmpty,
			Summary: "No content available yet.",
		}}
	}

	active := options.ActiveView
	if active < 0 || active >= len(views) {
		active = 0
	}

	return shellModel{
		title:    fallback(options.Title, "cktop"),
		subtitle: options.Subtitle,
		views:    views,
		active:   active,
		bindings: defaultKeys(),
		theme:    defaultTheme(),
		footer:   options.Footer,
	}
}

func (m shellModel) Init() tea.Cmd {
	return nil
}

func (m shellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.bindings.quit):
			return m, tea.Quit
		case key.Matches(msg, m.bindings.help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.bindings.back):
			m.showHelp = false
			return m, nil
		case key.Matches(msg, m.bindings.next):
			m.active = nextIndex(m.active, len(m.views))
			return m, nil
		case key.Matches(msg, m.bindings.prev):
			m.active = prevIndex(m.active, len(m.views))
			return m, nil
		}
	}

	return m, nil
}

func (m shellModel) View() string {
	layout := ResolveLayout(m.width, m.height)
	if !layout.Framed {
		return m.renderDense(layout)
	}

	body := strings.Join([]string{
		m.renderHeader(layout.ContentWidth),
		m.renderTabs(layout),
		m.renderContent(),
		m.renderFooter(layout),
	}, "\n\n")
	body = fitBlock(body, layout.ContentWidth)

	return m.theme.panel.Width(layout.ContentWidth + 4).Render(body)
}

func (m shellModel) renderDense(layout Layout) string {
	parts := []string{
		m.renderHeader(layout.ContentWidth),
		m.renderTabs(layout),
		m.renderContentDense(layout),
		m.renderFooter(layout),
	}

	return fitBlock(strings.Join(parts, "\n"), layout.ContentWidth)
}

func (m shellModel) renderHeader(width int) string {
	header := m.theme.appTitle.Render(m.title)
	if m.subtitle != "" {
		header += m.theme.subtitle.Render(" | " + m.subtitle)
	}

	return ansi.Truncate(header, width, "...")
}

func (m shellModel) renderTabs(layout Layout) string {
	if layout.Mode == LayoutMinimal {
		view := m.activeView()
		return m.theme.activeTab.Render(view.Title) + m.theme.inactiveTab.Render(tabPosition(m.active, len(m.views)))
	}

	parts := make([]string, 0, len(m.views))
	for index, view := range m.views {
		label := view.Title
		if view.Status != StatusReady {
			label = view.Status.label() + " " + label
		}
		if index == m.active {
			parts = append(parts, m.theme.activeTab.Render(label))
			continue
		}
		parts = append(parts, m.theme.inactiveTab.Render(label))
	}

	return ansi.Truncate(lipgloss.JoinHorizontal(lipgloss.Top, parts...), layout.ContentWidth, "...")
}

func (m shellModel) renderContent() string {
	if m.showHelp {
		return strings.Join([]string{
			m.theme.sectionTitle.Render("Help"),
			"Tab        next view",
			"Shift+Tab  previous view",
			"Esc        close help",
			"q          quit",
		}, "\n")
	}

	view := m.activeView()
	parts := make([]string, 0, len(view.Sections)+1)
	var summary strings.Builder
	if !view.HideStatus {
		summary.WriteString(m.theme.status(view.Status).Render(strings.ToUpper(view.Status.label())))
	}
	if view.Summary != "" {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		summary.WriteString(m.theme.content.Render(view.Summary))
	}
	if summary.Len() > 0 {
		parts = append(parts, summary.String())
	}
	for _, section := range view.Sections {
		var block strings.Builder
		if section.Title != "" {
			block.WriteString(m.theme.sectionTitle.Render(section.Title))
			block.WriteString("\n")
		}
		block.WriteString(m.theme.content.Render(section.Body))
		parts = append(parts, block.String())
	}

	return strings.Join(parts, "\n\n")
}

func (m shellModel) renderContentDense(layout Layout) string {
	if m.showHelp {
		return fitBlock("Help: tab next | shift+tab prev | esc close | q quit", layout.ContentWidth)
	}

	view := m.activeView()
	status := ""
	if !view.HideStatus {
		status = m.theme.status(view.Status).Render(strings.ToUpper(view.Status.label()))
	}
	if view.Summary != "" {
		if status != "" {
			status += " "
		}
		status += m.theme.content.Render(view.Summary)
	}

	parts := make([]string, 0, len(view.Sections)+1)
	if status != "" {
		parts = append(parts, status)
	}
	for _, section := range view.Sections {
		if layout.Height >= 10 && section.Title != "" {
			parts = append(parts, m.theme.sectionTitle.Render(section.Title))
		}
		parts = append(parts, section.Body)
	}

	return strings.Join(parts, "\n")
}

func (m shellModel) renderFooter(layout Layout) string {
	footer := m.footer
	if footer == "" {
		footer = "tab next  shift+tab prev  ? help  q quit"
	}
	if layout.Mode == LayoutMinimal && m.footer == "" {
		footer = "tab next  q quit"
	}

	return ansi.Truncate(m.theme.help.Render(footer), layout.ContentWidth, "...")
}

func (m shellModel) activeView() View {
	if len(m.views) == 0 {
		return View{Title: "Overview", Status: StatusEmpty, Summary: "No content available yet."}
	}

	return m.views[m.active]
}

func nextIndex(index, total int) int {
	if total <= 0 {
		return 0
	}

	return (index + 1) % total
}

func prevIndex(index, total int) int {
	if total <= 0 {
		return 0
	}

	if index <= 0 {
		return total - 1
	}

	return index - 1
}

func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func fitBlock(value string, width int) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = ansi.Truncate(line, width, "...")
	}

	return strings.Join(lines, "\n")
}

func tabPosition(active, total int) string {
	if total <= 0 {
		return ""
	}

	return " " + strconv.Itoa(active+1) + "/" + strconv.Itoa(total)
}
