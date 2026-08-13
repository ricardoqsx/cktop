package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

const columnGap = "  "

type tableColumn struct {
	id    string
	title string
	width int
	value func(domain.Container) string
}

func renderContainers(containers []domain.Container, selectedID string, selected map[string]struct{}, editing bool, now time.Time, layout sharedui.Layout, memoryMode config.MemoryMode) string {
	return renderContainersLocalized(containers, selectedID, selected, editing, now, layout, memoryMode, "63", "15", i18n.New("en"))
}

func renderContainersWithColors(containers []domain.Container, selectedID string, selected map[string]struct{}, editing bool, now time.Time, layout sharedui.Layout, memoryMode config.MemoryMode, accentColor, focusColor string) string {
	return renderContainersLocalized(containers, selectedID, selected, editing, now, layout, memoryMode, accentColor, focusColor, i18n.New("en"))
}

func renderContainersLocalized(containers []domain.Container, selectedID string, selected map[string]struct{}, editing bool, now time.Time, layout sharedui.Layout, memoryMode config.MemoryMode, accentColor, focusColor string, localizer sharedui.Localizer) string {
	columns := containerColumnsLocalized(layout.ContentWidth, memoryMode, editing, localizer)
	rows := visibleContainers(containers, selectedID, visibleRowCount(layout))

	var builder strings.Builder
	builder.WriteString(renderTableRowLocalized(columns, domain.Container{}, " ", true, now, memoryMode, localizer))
	builder.WriteString("\n")
	builder.WriteString(strings.Repeat("-", tableWidth(columns)))

	for _, container := range rows {
		marker := " "
		if editing {
			if _, isSelected := selected[container.ID]; isSelected {
				marker = "[x]"
			} else {
				marker = "[ ]"
			}
			if container.ID == selectedID {
				marker = activeEditMarkerStyle(accentColor).Render(">" + marker)
			} else {
				marker = " " + marker
			}
		} else if container.ID == selectedID {
			marker = ">"
		}
		builder.WriteString("\n")
		focused := container.ID == selectedID
		row := renderTableRowLocalized(columns, container, marker, false, now, memoryMode, localizer)
		if focused {
			row = renderFocusedTableRowLocalized(columns, container, marker, now, memoryMode, accentColor, localizer)
		}
		if focused {
			row = focusedTableRow(row, tableWidth(columns), focusColor, accentColor)
		}
		builder.WriteString(row)
	}

	return builder.String()
}

func activeEditMarkerStyle(accentColor string) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor))
}

func focusedTableRow(row string, width int, focusColor, accentColor string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(focusColor)).Background(lipgloss.Color(accentColor)).Width(width).Render(row)
}

func focusedMenuRow(row string, width int, focusColor, accentColor string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(focusColor)).Background(lipgloss.Color(accentColor)).Width(width).Render(row)
}

func containerColumns(width int, memoryMode config.MemoryMode, editing bool) []tableColumn {
	return containerColumnsLocalized(width, memoryMode, editing, i18n.New("en"))
}

func containerColumnsLocalized(width int, memoryMode config.MemoryMode, editing bool, localizer sharedui.Localizer) []tableColumn {
	if width < 20 {
		width = 20
	}

	markerWidth := 1
	if editing {
		markerWidth = 4
	}
	columns := []tableColumn{
		{id: "marker", title: "", width: markerWidth},
		{id: "update", title: "", width: 1},
		{id: "name", title: localizer.Text(i18n.MessageColumnName), width: 10, value: func(container domain.Container) string { return container.Name }},
		{id: "state", title: localizer.Text(i18n.MessageColumnState), width: 9, value: func(container domain.Container) string { return localizeState(localizer, container.State) }},
	}

	if width >= 50 {
		memoryWidth := 12
		if width >= 80 && memoryMode == config.MemoryBoth {
			memoryWidth = 18
		}
		columns = append(columns,
			tableColumn{id: "cpu", title: localizer.Text(i18n.MessageColumnCPU), width: 10},
			tableColumn{id: "memory", title: localizer.Text(i18n.MessageColumnMemory), width: memoryWidth},
		)
	}
	if width >= 72 {
		columns = append(columns, tableColumn{id: "health", title: localizer.Text(i18n.MessageColumnHealth), width: 9, value: func(container domain.Container) string { return localizeHealth(localizer, container.Health) }})
	}
	if width >= 80 {
		columns = append(columns, tableColumn{id: "uptime", title: localizer.Text(i18n.MessageColumnUptime), width: 6})
	}
	if width >= 110 {
		columns = append(columns, tableColumn{id: "image", title: localizer.Text(i18n.MessageColumnImage), width: 12, value: func(container domain.Container) string { return container.Image }})
	}
	if width >= 150 {
		columns = append(columns, tableColumn{id: "id", title: localizer.Text(i18n.MessageColumnID), width: 12, value: func(container domain.Container) string { return container.ShortID }})
	}

	// NAME and IMAGE consume the remaining width after fixed columns.
	fixed := (len(columns) - 1) * len(columnGap)
	for _, column := range columns {
		if column.id == "name" || column.id == "image" {
			continue
		}
		fixed += column.width
	}
	remaining := width - fixed
	imageIndex := columnIndex(columns, "image")
	if imageIndex >= 0 {
		nameWidth := remaining / 2
		if nameWidth < 16 {
			nameWidth = 16
		}
		if nameWidth > 30 {
			nameWidth = 30
		}
		columns[columnIndex(columns, "name")].width = nameWidth
		columns[imageIndex].width = remaining - nameWidth
	} else {
		columns[columnIndex(columns, "name")].width = remaining
	}

	return columns
}

func renderTableRow(columns []tableColumn, container domain.Container, marker string, header bool, now time.Time, memoryMode config.MemoryMode) string {
	return renderTableRowLocalized(columns, container, marker, header, now, memoryMode, i18n.New("en"))
}

func renderTableRowLocalized(columns []tableColumn, container domain.Container, marker string, header bool, now time.Time, memoryMode config.MemoryMode, localizer sharedui.Localizer) string {
	values := make([]string, len(columns))
	for index, column := range columns {
		value := column.title
		if !header {
			if column.id == "marker" {
				value = marker
			} else if column.id == "update" {
				value = containerUpdateIndicator(container.Update)
			} else if column.id == "cpu" {
				values[index] = cpuCellLocalized(container, column.width, localizer)
				continue
			} else if column.id == "memory" {
				values[index] = memoryCellLocalized(container, column.width, memoryMode, localizer)
				continue
			} else if column.id == "uptime" {
				value = formatUptime(container.StartedAt, container.State, now)
			} else if column.value != nil {
				value = column.value(container)
			}
		}
		values[index] = fitCell(value, column.width)
	}

	return strings.Join(values, columnGap)
}

func renderFocusedTableRow(columns []tableColumn, container domain.Container, marker string, now time.Time, memoryMode config.MemoryMode, accentColor string) string {
	return renderFocusedTableRowLocalized(columns, container, marker, now, memoryMode, accentColor, i18n.New("en"))
}

func renderFocusedTableRowLocalized(columns []tableColumn, container domain.Container, marker string, now time.Time, memoryMode config.MemoryMode, accentColor string, localizer sharedui.Localizer) string {
	values := make([]string, len(columns))
	for index, column := range columns {
		value := ""
		if column.id == "marker" {
			value = marker
		} else if column.id == "update" {
			value = containerUpdateIndicator(container.Update)
		} else if column.id == "cpu" {
			values[index] = focusedMetricCell(localizer.Decimal(container.CPUPercent, 1)+"%", container.CPUAvailable, column.width, accentColor)
			continue
		} else if column.id == "memory" {
			values[index] = focusedMetricCell(memoryTextLocalized(container, memoryMode, column.width, localizer), container.MemoryAvailable, column.width, accentColor)
			continue
		} else if column.id == "uptime" {
			value = formatUptime(container.StartedAt, container.State, now)
		} else if column.value != nil {
			value = column.value(container)
		}
		values[index] = fitCell(value, column.width)
	}
	return strings.Join(values, columnGap)
}

func containerUpdateIndicator(status domain.UpdateStatus) string {
	if status == domain.UpdateAvailable || status == domain.UpdatePulledPendingRecreate {
		return imageUpdateIndicator(status)
	}
	return ""
}

func focusedMetricCell(text string, available bool, width int, accentColor string) string {
	if !available {
		text = "-"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Background(lipgloss.Color(accentColor)).Render(fitCell(text, width))
}

func cpuCell(container domain.Container, width int) string {
	return cpuCellLocalized(container, width, i18n.New("en"))
}

func cpuCellLocalized(container domain.Container, width int, localizer sharedui.Localizer) string {
	if !container.CPUAvailable {
		return fitCell("-", width)
	}

	text := localizer.Decimal(container.CPUPercent, 1) + "%"
	return metricBar(text, container.CPUPercent/100, width)
}

func memoryCell(container domain.Container, width int, mode config.MemoryMode) string {
	return memoryCellLocalized(container, width, mode, i18n.New("en"))
}

func memoryCellLocalized(container domain.Container, width int, mode config.MemoryMode, localizer sharedui.Localizer) string {
	if !container.MemoryAvailable {
		return fitCell("-", width)
	}

	text := memoryTextLocalized(container, mode, width, localizer)
	return metricBar(text, container.MemoryPercent/100, width)
}

func memoryText(container domain.Container, mode config.MemoryMode, width int) string {
	return memoryTextLocalized(container, mode, width, i18n.New("en"))
}

func memoryTextLocalized(container domain.Container, mode config.MemoryMode, width int, localizer sharedui.Localizer) string {
	usage := formatBytes(container.MemoryUsage)
	limit := formatBytes(container.MemoryLimit)
	percent := localizer.Decimal(container.MemoryPercent, 1) + "%"

	switch mode {
	case config.MemoryUsage:
		if container.MemoryLimit == 0 {
			return usage
		}
		return usage + "/" + limit
	case config.MemoryPercent:
		return percent
	default:
		if width < 16 {
			return percent
		}
		if container.MemoryLimit == 0 {
			return usage + " " + percent
		}
		return usage + "/" + limit + " " + percent
	}
}

func metricBar(text string, ratio float64, width int) string {
	cell := fitCell(text, width)
	ratio = math.Max(0, math.Min(1, ratio))
	filled := int(math.Round(float64(width) * ratio))
	if filled <= 0 {
		return cell
	}

	left := ansi.Cut(cell, 0, filled)
	right := ansi.Cut(cell, filled, width)
	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("22"))

	return barStyle.Render(left) + right
}

func formatBytes(value uint64) string {
	const (
		kiB = 1024
		miB = 1024 * kiB
		giB = 1024 * miB
		tiB = 1024 * giB
	)

	switch {
	case value >= tiB:
		return compactNumber(float64(value)/float64(tiB)) + "T"
	case value >= giB:
		return compactNumber(float64(value)/float64(giB)) + "G"
	case value >= miB:
		return compactNumber(float64(value)/float64(miB)) + "M"
	case value >= kiB:
		return compactNumber(float64(value)/float64(kiB)) + "K"
	default:
		return fmt.Sprintf("%dB", value)
	}
}

func compactNumber(value float64) string {
	if value >= 10 || math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", value)
	}

	return fmt.Sprintf("%.1f", value)
}

func fitCell(value string, width int) string {
	if width <= 0 {
		return ""
	}

	value = ansi.Truncate(value, width, "...")
	padding := width - ansi.StringWidth(value)
	if padding > 0 {
		value += strings.Repeat(" ", padding)
	}

	return value
}

func tableWidth(columns []tableColumn) int {
	width := (len(columns) - 1) * len(columnGap)
	for _, column := range columns {
		width += column.width
	}

	return width
}

func columnIndex(columns []tableColumn, title string) int {
	for index, column := range columns {
		if column.id == strings.ToLower(title) || column.title == title {
			return index
		}
	}

	return -1
}

func localizeState(localizer sharedui.Localizer, state string) string {
	switch strings.ToLower(state) {
	case "running":
		return localizer.Text(i18n.MessageStateRunning)
	case "stopped":
		return localizer.Text(i18n.MessageStateStopped)
	case "exited":
		return localizer.Text(i18n.MessageStateExited)
	case "mixed":
		return localizer.Text(i18n.MessageStateMixed)
	case "down":
		return localizer.Text(i18n.MessageStateDown)
	case "missing compose file":
		return localizer.Text(i18n.MessageStateMissingComposeFile)
	default:
		return state
	}
}

func localizeHealth(localizer sharedui.Localizer, health string) string {
	switch strings.ToLower(health) {
	case "healthy":
		return localizer.Text(i18n.MessageHealthHealthy)
	case "unhealthy":
		return localizer.Text(i18n.MessageHealthUnhealthy)
	case "starting":
		return localizer.Text(i18n.MessageHealthStarting)
	default:
		return health
	}
}

func visibleRowCount(layout sharedui.Layout) int {
	reserved := 5
	if layout.Framed {
		reserved = 12
	}

	rows := layout.Height - reserved
	if rows < 1 {
		return 1
	}

	return rows
}

func visibleContainers(containers []domain.Container, selectedID string, limit int) []domain.Container {
	if len(containers) <= limit {
		return containers
	}

	selected := 0
	for index, container := range containers {
		if container.ID == selectedID {
			selected = index
			break
		}
	}

	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(containers) {
		start = len(containers) - limit
	}

	return containers[start : start+limit]
}

func formatUptime(startedAt time.Time, state string, now time.Time) string {
	if state != "running" || startedAt.IsZero() {
		return "-"
	}
	if startedAt.After(now) {
		return "0s"
	}

	duration := now.Sub(startedAt)
	switch {
	case duration >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(duration/(24*time.Hour)))
	case duration >= time.Hour:
		return fmt.Sprintf("%dh", int(duration/time.Hour))
	case duration >= time.Minute:
		return fmt.Sprintf("%dm", int(duration/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(duration/time.Second))
	}
}
