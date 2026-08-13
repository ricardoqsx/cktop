package tui

import (
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
	Title        string
	Subtitle     string
	Views        []View
	ActiveView   int
	Localizer    Localizer
	Footer       string
	FooterNotice string
	AccentColor  string
	Banner       string
	BannerColor  string
}

type shellModel struct {
	title        string
	subtitle     string
	views        []View
	active       int
	showHelp     bool
	bindings     keyMap
	width        int
	height       int
	theme        theme
	localizer    Localizer
	footer       string
	footerNotice string
	banner       string
	bannerColor  string
}

type keyMap struct {
	quit key.Binding
	next key.Binding
	prev key.Binding
	help key.Binding
	back key.Binding
}

func defaultKeys(localizer Localizer) keyMap {
	return keyMap{
		quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", localizer.Text(MessageShellKeyQuit))),
		next: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", localizer.Text(MessageShellKeyNext))),
		prev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", localizer.Text(MessageShellKeyPrevious))),
		help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", localizer.Text(MessageShellKeyHelp))),
		back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", localizer.Text(MessageShellKeyBack))),
	}
}

// NewShell returns a Bubble Tea model with shared layout, navigation and states.
func NewShell(options ShellOptions) tea.Model {
	return newShell(options)
}

func newShell(options ShellOptions) shellModel {
	localizer := options.Localizer
	if localizer == nil {
		localizer = DefaultLocalizer()
	}

	views := options.Views
	if len(views) == 0 {
		views = []View{{
			Title:   localizer.Text(MessageShellFallbackViewTitle),
			Status:  StatusEmpty,
			Summary: localizer.Text(MessageShellFallbackViewSummary),
		}}
	}

	active := options.ActiveView
	if active < 0 || active >= len(views) {
		active = 0
	}

	theme := defaultTheme()
	if options.AccentColor != "" {
		theme.appTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(options.AccentColor))
		theme.activeTab = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color(options.AccentColor)).Padding(0, 1)
	}

	return shellModel{
		title:        fallback(options.Title, localizer.Text(MessageShellFallbackTitle)),
		subtitle:     options.Subtitle,
		views:        views,
		active:       active,
		bindings:     defaultKeys(localizer),
		theme:        theme,
		localizer:    localizer,
		footer:       options.Footer,
		footerNotice: options.FooterNotice,
		banner:       options.Banner,
		bannerColor:  fallback(options.BannerColor, "33"),
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

	parts := []string{m.renderHeader(layout.ContentWidth), m.renderTabs(layout), m.renderContent(), m.renderFooter(layout)}
	body := strings.Join(parts, "\n\n")
	body = fitBlock(body, layout.ContentWidth)

	return m.theme.panel.Width(layout.ContentWidth + 4).Render(body)
}

func (m shellModel) renderDense(layout Layout) string {
	parts := []string{m.renderHeader(layout.ContentWidth), m.renderTabs(layout), m.renderContentDense(layout), m.renderFooter(layout)}

	return fitBlock(strings.Join(parts, "\n"), layout.ContentWidth)
}

func (m shellModel) renderBanner(layout Layout) string {
	if m.banner == "" {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color(m.bannerColor)).Width(layout.ContentWidth).Render(fitBlock(m.banner, layout.ContentWidth))
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
		return m.theme.activeTab.Render(view.Title) + m.theme.inactiveTab.Render(m.tabPosition())
	}

	parts := make([]string, 0, len(m.views))
	for index, view := range m.views {
		label := view.Title
		if view.Status != StatusReady {
			label = m.localizer.Text(view.Status.messageID()) + " " + label
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
			m.theme.sectionTitle.Render(m.localizer.Text(MessageShellHelpTitle)),
			m.localizer.Text(MessageShellHelpNextView),
			m.localizer.Text(MessageShellHelpPreviousView),
			m.localizer.Text(MessageShellHelpClose),
			m.localizer.Text(MessageShellHelpQuit),
		}, "\n")
	}

	view := m.activeView()
	parts := make([]string, 0, len(view.Sections)+1)
	var summary strings.Builder
	if !view.HideStatus {
		summary.WriteString(m.theme.status(view.Status).Render(strings.ToUpper(m.localizer.Text(view.Status.messageID()))))
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
		return fitBlock(m.localizer.Text(MessageShellHelpCompact), layout.ContentWidth)
	}

	view := m.activeView()
	status := ""
	if !view.HideStatus {
		status = m.theme.status(view.Status).Render(strings.ToUpper(m.localizer.Text(view.Status.messageID())))
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
		footer = m.localizer.Text(MessageShellFooterDefault)
	}
	if layout.Mode == LayoutMinimal && m.footer == "" {
		footer = m.localizer.Text(MessageShellFooterMinimal)
	}

	parts := make([]string, 0, 3)
	if banner := m.renderBanner(layout); banner != "" {
		parts = append(parts, banner)
	}
	if m.footerNotice != "" {
		parts = append(parts, ansi.Truncate(m.theme.statusWarning.Bold(true).Render(m.footerNotice), layout.ContentWidth, "..."))
	}
	parts = append(parts, ansi.Truncate(m.theme.help.Render(footer), layout.ContentWidth, "..."))
	return strings.Join(parts, "\n")
}

func (m shellModel) activeView() View {
	if len(m.views) == 0 {
		return View{
			Title:   m.localizer.Text(MessageShellFallbackViewTitle),
			Status:  StatusEmpty,
			Summary: m.localizer.Text(MessageShellFallbackViewSummary),
		}
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

func (m shellModel) tabPosition() string {
	if len(m.views) <= 0 {
		return ""
	}

	return m.localizer.Text(MessageShellTabPosition, m.active+1, len(m.views))
}
