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
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

const columnGap = "  "

var activeEditMarkerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

type tableColumn struct {
	title string
	width int
	value func(domain.Container) string
}

func renderContainers(containers []domain.Container, selectedID string, selected map[string]struct{}, editing bool, now time.Time, layout sharedui.Layout, memoryMode config.MemoryMode) string {
	columns := containerColumns(layout.ContentWidth, memoryMode, editing)
	rows := visibleContainers(containers, selectedID, visibleRowCount(layout))

	var builder strings.Builder
	builder.WriteString(renderTableRow(columns, domain.Container{}, " ", true, now, memoryMode))
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
				marker = activeEditMarkerStyle.Render(">" + marker)
			} else {
				marker = " " + marker
			}
		} else if container.ID == selectedID {
			marker = ">"
		}
		builder.WriteString("\n")
		builder.WriteString(renderTableRow(columns, container, marker, false, now, memoryMode))
	}

	return builder.String()
}

func containerColumns(width int, memoryMode config.MemoryMode, editing bool) []tableColumn {
	if width < 20 {
		width = 20
	}

	markerWidth := 1
	if editing {
		markerWidth = 4
	}
	columns := []tableColumn{
		{title: "", width: markerWidth},
		{title: "NAME", width: 10, value: func(container domain.Container) string { return container.Name }},
		{title: "STATE", width: 9, value: func(container domain.Container) string { return container.State }},
	}

	if width >= 50 {
		memoryWidth := 12
		if width >= 80 && memoryMode == config.MemoryBoth {
			memoryWidth = 18
		}
		columns = append(columns,
			tableColumn{title: "CPU", width: 10},
			tableColumn{title: "MEM", width: memoryWidth},
		)
	}
	if width >= 72 {
		columns = append(columns, tableColumn{title: "HEALTH", width: 9, value: func(container domain.Container) string { return container.Health }})
	}
	if width >= 80 {
		columns = append(columns, tableColumn{title: "UPTIME", width: 6})
	}
	if width >= 110 {
		columns = append(columns, tableColumn{title: "IMAGE", width: 12, value: func(container domain.Container) string { return container.Image }})
	}
	if width >= 150 {
		columns = append(columns, tableColumn{title: "ID", width: 12, value: func(container domain.Container) string { return container.ShortID }})
	}

	// NAME and IMAGE consume the remaining width after fixed columns.
	fixed := (len(columns) - 1) * len(columnGap)
	for index, column := range columns {
		if index == 1 || column.title == "IMAGE" {
			continue
		}
		fixed += column.width
	}
	remaining := width - fixed
	imageIndex := columnIndex(columns, "IMAGE")
	if imageIndex >= 0 {
		nameWidth := remaining / 2
		if nameWidth < 16 {
			nameWidth = 16
		}
		if nameWidth > 30 {
			nameWidth = 30
		}
		columns[1].width = nameWidth
		columns[imageIndex].width = remaining - nameWidth
	} else {
		columns[1].width = remaining
	}

	return columns
}

func renderTableRow(columns []tableColumn, container domain.Container, marker string, header bool, now time.Time, memoryMode config.MemoryMode) string {
	values := make([]string, len(columns))
	for index, column := range columns {
		value := column.title
		if !header {
			if index == 0 {
				value = marker
			} else if column.title == "CPU" {
				values[index] = cpuCell(container, column.width)
				continue
			} else if column.title == "MEM" {
				values[index] = memoryCell(container, column.width, memoryMode)
				continue
			} else if column.title == "UPTIME" {
				value = formatUptime(container.StartedAt, container.State, now)
			} else if column.value != nil {
				value = column.value(container)
			}
		}
		values[index] = fitCell(value, column.width)
	}

	return strings.Join(values, columnGap)
}

func cpuCell(container domain.Container, width int) string {
	if !container.CPUAvailable {
		return fitCell("-", width)
	}

	text := fmt.Sprintf("%.1f%%", container.CPUPercent)
	return metricBar(text, container.CPUPercent/100, width)
}

func memoryCell(container domain.Container, width int, mode config.MemoryMode) string {
	if !container.MemoryAvailable {
		return fitCell("-", width)
	}

	text := memoryText(container, mode, width)
	return metricBar(text, container.MemoryPercent/100, width)
}

func memoryText(container domain.Container, mode config.MemoryMode, width int) string {
	usage := formatBytes(container.MemoryUsage)
	limit := formatBytes(container.MemoryLimit)
	percent := fmt.Sprintf("%.1f%%", container.MemoryPercent)

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
		if column.title == title {
			return index
		}
	}

	return -1
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
