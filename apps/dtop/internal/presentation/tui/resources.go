package tui

import (
	"strings"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

type resourceColumn[T any] struct {
	title string
	width int
	value func(T, time.Time) string
}

func renderNetworks(networks []domain.Network, selectedID string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time) string {
	return renderNetworksWithColors(networks, selectedID, selected, editing, layout, now, "63", "15")
}

func renderNetworksWithColors(networks []domain.Network, selectedID string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time, accentColor, focusColor string) string {
	columns := []resourceColumn[domain.Network]{{title: "", width: resourceMarkerWidth(editing)}, {title: "NAME", width: 10, value: func(n domain.Network, _ time.Time) string { return n.Name }}}
	if layout.ContentWidth >= 48 {
		columns = append(columns, resourceColumn[domain.Network]{title: "DRIVER", width: 10, value: func(n domain.Network, _ time.Time) string { return n.Driver }})
	}
	if layout.ContentWidth >= 68 {
		columns = append(columns, resourceColumn[domain.Network]{title: "USED", width: 12, value: func(n domain.Network, _ time.Time) string { return resourceUsage(n.Containers, n.UsageKnown) }})
	}
	if layout.ContentWidth >= 84 {
		columns = append(columns, resourceColumn[domain.Network]{title: "SCOPE", width: 8, value: func(n domain.Network, _ time.Time) string { return n.Scope }})
	}
	if layout.ContentWidth >= 120 {
		columns = append(columns, resourceColumn[domain.Network]{title: "ID", width: 12, value: func(n domain.Network, _ time.Time) string { return n.ShortID }})
	}
	return renderResourceTable(networks, selectedID, selected, editing, columns, layout, now, func(n domain.Network) string { return n.ID }, accentColor, focusColor)
}

func renderVolumes(volumes []domain.Volume, selectedName string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time) string {
	return renderVolumesWithColors(volumes, selectedName, selected, editing, layout, now, "63", "15")
}

func renderVolumesWithColors(volumes []domain.Volume, selectedName string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time, accentColor, focusColor string) string {
	columns := []resourceColumn[domain.Volume]{{title: "", width: resourceMarkerWidth(editing)}, {title: "NAME", width: 10, value: func(v domain.Volume, _ time.Time) string { return v.Name }}}
	if layout.ContentWidth >= 48 {
		columns = append(columns, resourceColumn[domain.Volume]{title: "DRIVER", width: 10, value: func(v domain.Volume, _ time.Time) string { return v.Driver }})
	}
	if layout.ContentWidth >= 68 {
		columns = append(columns, resourceColumn[domain.Volume]{title: "USED", width: 12, value: func(v domain.Volume, _ time.Time) string { return resourceUsage(v.Containers, v.UsageKnown) }})
	}
	if layout.ContentWidth >= 84 {
		columns = append(columns, resourceColumn[domain.Volume]{title: "SCOPE", width: 8, value: func(v domain.Volume, _ time.Time) string { return v.Scope }})
	}
	return renderResourceTable(volumes, selectedName, selected, editing, columns, layout, now, func(v domain.Volume) string { return v.Name }, accentColor, focusColor)
}

func resourceMarkerWidth(editing bool) int {
	if editing {
		return 4
	}
	return 1
}

func renderResourceTable[T any](items []T, selectedID string, selected map[string]struct{}, editing bool, columns []resourceColumn[T], layout sharedui.Layout, now time.Time, id func(T) string, accentColor, focusColor string) string {
	fixed := (len(columns) - 1) * len(columnGap)
	for index, column := range columns {
		if index != 1 {
			fixed += column.width
		}
	}
	columns[1].width = layout.ContentWidth - fixed
	if columns[1].width < 1 {
		columns[1].width = 1
	}
	var builder strings.Builder
	render := func(item T, header bool, marker string) string {
		values := make([]string, len(columns))
		for index, column := range columns {
			value := column.title
			if !header {
				if index == 0 {
					value = marker
				} else {
					value = column.value(item, now)
				}
			}
			values[index] = fitCell(value, column.width)
		}
		return strings.Join(values, columnGap)
	}
	builder.WriteString(render(*new(T), true, " "))
	builder.WriteString("\n")
	builder.WriteString(strings.Repeat("-", imageTableWidthResource(columns)))
	for _, item := range visibleResourceItems(items, selectedID, visibleRowCount(layout), id) {
		marker := " "
		if editing {
			marker = "[ ]"
			if _, isSelected := selected[id(item)]; isSelected {
				marker = "[x]"
			}
			if id(item) == selectedID {
				marker = activeEditMarkerStyle(accentColor).Render(">" + marker)
			} else {
				marker = " " + marker
			}
		} else if id(item) == selectedID {
			marker = ">"
		}
		builder.WriteString("\n")
		row := render(item, false, marker)
		if id(item) == selectedID {
			row = focusedTableRow(row, imageTableWidthResource(columns), focusColor, accentColor)
		}
		builder.WriteString(row)
	}
	return builder.String()
}

func imageTableWidthResource[T any](columns []resourceColumn[T]) int {
	width := (len(columns) - 1) * len(columnGap)
	for _, column := range columns {
		width += column.width
	}
	return width
}

func visibleResourceItems[T any](items []T, selectedID string, limit int, id func(T) string) []T {
	if len(items) <= limit {
		return items
	}
	selected := 0
	for index, item := range items {
		if id(item) == selectedID {
			selected = index
			break
		}
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(items) {
		start = len(items) - limit
	}
	return items[start : start+limit]
}

func resourceUsage(count int, known bool) string {
	if !known {
		return "unknown"
	}
	if count == 1 {
		return "1 container"
	}
	return formatCount(int64(count)) + " containers"
}
