package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

type imageColumn struct {
	title string
	width int
	value func(domain.Image, time.Time) string
}

func renderImages(images []domain.Image, selectedID string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time) string {
	return renderImagesWithColors(images, selectedID, selected, editing, layout, now, "63", "15")
}

func renderImagesWithColors(images []domain.Image, selectedID string, selected map[string]struct{}, editing bool, layout sharedui.Layout, now time.Time, accentColor, focusColor string) string {
	columns := imageColumns(layout.ContentWidth, editing)
	rows := visibleImages(images, selectedID, visibleRowCount(layout))
	var builder strings.Builder
	builder.WriteString(renderImageRow(columns, domain.Image{}, true, now, " "))
	builder.WriteString("\n")
	builder.WriteString(strings.Repeat("-", imageTableWidth(columns)))
	for _, image := range rows {
		marker := " "
		if editing {
			if _, isSelected := selected[image.ID]; isSelected {
				marker = "[x]"
			} else {
				marker = "[ ]"
			}
			if image.ID == selectedID {
				marker = activeEditMarkerStyle(accentColor).Render(">" + marker)
			} else {
				marker = " " + marker
			}
		} else if image.ID == selectedID {
			marker = ">"
		}
		builder.WriteString("\n")
		row := renderImageRow(columns, image, false, now, marker)
		if image.ID == selectedID {
			row = focusedTableRow(row, imageTableWidth(columns), focusColor, accentColor)
		}
		builder.WriteString(row)
	}
	return builder.String()
}

func imageColumns(width int, editing bool) []imageColumn {
	if width < 20 {
		width = 20
	}
	markerWidth := 1
	if editing {
		markerWidth = 4
	}
	columns := []imageColumn{
		{title: "", width: markerWidth},
		{title: "NAME", width: 10, value: func(image domain.Image, _ time.Time) string { return image.Name }},
	}
	if width >= 36 {
		columns = append(columns, imageColumn{title: "UPDATE", width: 6, value: func(image domain.Image, _ time.Time) string { return imageUpdateIndicator(image.Update) }})
	}
	if width >= 48 {
		columns = append(columns, imageColumn{title: "SIZE", width: 8, value: func(image domain.Image, _ time.Time) string { return formatBytes(image.Size) }})
	}
	if width >= 68 {
		columns = append(columns, imageColumn{title: "USED", width: 12, value: func(image domain.Image, _ time.Time) string { return imageUsage(image) }})
	}
	if width >= 84 {
		columns = append(columns, imageColumn{title: "AGE", width: 6, value: func(image domain.Image, now time.Time) string { return formatImageAge(image.Created, now) }})
	}
	if width >= 120 {
		columns = append(columns, imageColumn{title: "ID", width: 12, value: func(image domain.Image, _ time.Time) string { return image.ShortID }})
	}
	fixed := (len(columns) - 1) * len(columnGap)
	for index, column := range columns {
		if index != 1 {
			fixed += column.width
		}
	}
	columns[1].width = width - fixed
	return columns
}

func imageUpdateIndicator(status domain.UpdateStatus) string {
	switch status {
	case domain.UpdateAvailable:
		return "U"
	case domain.UpdateCurrent:
		return "="
	case domain.UpdatePinned:
		return "P"
	case domain.UpdateChecking:
		return "..."
	case domain.UpdatePulledPendingRecreate:
		return "R"
	default:
		return "?"
	}
}

func renderImageRow(columns []imageColumn, image domain.Image, header bool, now time.Time, marker string) string {
	values := make([]string, len(columns))
	for index, column := range columns {
		value := column.title
		if !header {
			if index == 0 {
				value = marker
			} else if column.value != nil {
				value = column.value(image, now)
			}
		}
		values[index] = fitCell(value, column.width)
	}
	return strings.Join(values, columnGap)
}

func imageUsage(image domain.Image) string {
	if !image.UsageKnown {
		return "-"
	}
	if image.Dangling {
		return "dangling"
	}
	if image.Containers == 1 {
		return "1 container"
	}
	return strings.TrimSpace(strings.Join([]string{formatCount(image.Containers), "containers"}, " "))
}

func formatCount(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatImageAge(created, now time.Time) string {
	if created.IsZero() || created.After(now) {
		return "-"
	}
	duration := now.Sub(created)
	switch {
	case duration >= 24*time.Hour:
		return strconv.Itoa(int(duration/(24*time.Hour))) + "d"
	case duration >= time.Hour:
		return strconv.Itoa(int(duration/time.Hour)) + "h"
	case duration >= time.Minute:
		return strconv.Itoa(int(duration/time.Minute)) + "m"
	default:
		return strconv.Itoa(int(duration/time.Second)) + "s"
	}
}

func imageTableWidth(columns []imageColumn) int {
	width := (len(columns) - 1) * len(columnGap)
	for _, column := range columns {
		width += column.width
	}
	return width
}

func visibleImages(images []domain.Image, selectedID string, limit int) []domain.Image {
	if len(images) <= limit {
		return images
	}
	selected := 0
	for index, image := range images {
		if image.ID == selectedID {
			selected = index
			break
		}
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(images) {
		start = len(images) - limit
	}
	return images[start : start+limit]
}
