package tui

import "github.com/charmbracelet/lipgloss"

type theme struct {
	appTitle      lipgloss.Style
	subtitle      lipgloss.Style
	activeTab     lipgloss.Style
	inactiveTab   lipgloss.Style
	sectionTitle  lipgloss.Style
	content       lipgloss.Style
	help          lipgloss.Style
	panel         lipgloss.Style
	statusReady   lipgloss.Style
	statusLoading lipgloss.Style
	statusWarning lipgloss.Style
	statusError   lipgloss.Style
	statusMuted   lipgloss.Style
}

func defaultTheme() theme {
	return theme{
		appTitle:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		subtitle:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		activeTab:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Padding(0, 1),
		inactiveTab:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1),
		sectionTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		content:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		help:          lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		panel:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2),
		statusReady:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		statusLoading: lipgloss.NewStyle().Foreground(lipgloss.Color("75")),
		statusWarning: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		statusError:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		statusMuted:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	}
}

func (t theme) status(status Status) lipgloss.Style {
	switch status {
	case StatusLoading:
		return t.statusLoading
	case StatusWarning:
		return t.statusWarning
	case StatusError:
		return t.statusError
	case StatusEmpty, StatusUnavailable:
		return t.statusMuted
	default:
		return t.statusReady
	}
}
