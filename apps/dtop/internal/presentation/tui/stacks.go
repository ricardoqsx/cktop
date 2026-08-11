package tui

import (
	"fmt"
	"strings"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

func (m Model) stacksView(layout sharedui.Layout) sharedui.View {
	if (m.action.resource == actionStacks || m.action.resource == actionStackContainers) && m.action.stage == actionMenu {
		return m.actionView()
	}
	if m.stacksLoading || !m.stacksLoaded {
		return sharedui.View{Title: "Stacks", Status: sharedui.StatusLoading, Summary: "Loading Docker Compose stacks..."}
	}
	if m.stacksErr != nil {
		return sharedui.View{Title: "Stacks", Status: sharedui.StatusError, Summary: m.stacksErr.Error(), Sections: []sharedui.Section{{Title: "Next", Body: "Press r to retry."}}}
	}
	if m.shellActive {
		return sharedui.View{Title: "Container shell", Status: sharedui.StatusLoading, Summary: "Starting an interactive shell..."}
	}
	if m.shellErr != nil {
		return sharedui.View{Title: "Stacks", Status: sharedui.StatusError, Summary: "Container shell failed: " + m.shellErr.Error(), Sections: []sharedui.Section{{Title: "Next", Body: "Check that the container is running, /bin/sh exists, and Docker permissions allow exec. Press s to try again."}}}
	}
	if len(m.stacks) == 0 && len(m.stackDiagnostics) == 0 {
		return sharedui.View{Title: "Stacks", Status: sharedui.StatusEmpty, Summary: "No stacks found. Only stacks with Docker Compose labels can be discovered."}
	}
	sections := []sharedui.Section{{Body: renderStacksWithColors(m.stacks, m.selectedStackName, m.selectedStackContainerID, m.selectedStacks, m.selectedStackContainers, m.stackEditing, m.stackContainerEditing, m.expandedStackName, layout, m.accentColor, m.focusColor)}}
	if stack := m.selectedStack(); stack != nil {
		width := layout.ContentWidth
		if width < 20 {
			width = 20
		}
		workingDir := stack.WorkingDir
		if workingDir == "" {
			workingDir = "-"
		}
		files := "-"
		if len(stack.Files) > 0 {
			files = strings.Join(stack.Files, ", ")
		}
		down := "available"
		if reason := stack.DownUnavailableReason(); reason != "" {
			down = "unavailable: " + reason
		}
		sections = append(sections, sharedui.Section{Title: "Selected stack", Body: "Working directory: " + fitCell(workingDir, width-19) + "\nCompose files: " + fitCell(files, width-15) + "\nDown: " + fitCell(down, width-6)})
	}
	if len(m.stackDiagnostics) > 0 {
		sections = append(sections, sharedui.Section{Title: "Registration diagnostics", Body: strings.Join(m.stackDiagnostics, "\n")})
	}
	return sharedui.View{Title: "Stacks", Status: sharedui.StatusReady, HideStatus: true, Sections: sections}
}

func renderStacks(stacks []domain.Stack, selected, selectedContainer string, selections, selectedContainers map[string]struct{}, editing, childEditing bool, expanded string, layout sharedui.Layout) string {
	return renderStacksWithColors(stacks, selected, selectedContainer, selections, selectedContainers, editing, childEditing, expanded, layout, "63", "15")
}

func renderStacksWithColors(stacks []domain.Stack, selected, selectedContainer string, selections, selectedContainers map[string]struct{}, editing, childEditing bool, expanded string, layout sharedui.Layout, accentColor, focusColor string) string {
	// Stack rows use the resource table sizing rules but retain expandable detail rows.
	width := layout.ContentWidth
	if width < 20 {
		width = 20
	}
	markerWidth := resourceMarkerWidth(editing || childEditing)
	type column struct {
		title string
		width int
	}
	stateWidth := 9
	if width >= 80 {
		stateWidth = 20
	}
	cols := []column{{"", markerWidth}, {"NAME", 10}, {"STATE", stateWidth}}
	if width >= 52 {
		cols = append(cols, column{"CPU", 8}, column{"MEM", 12})
	}
	if width >= 72 {
		cols = append(cols, column{"HEALTH", 9})
	}
	if width >= 100 {
		cols = append(cols, column{"SERVICES", 8})
	}
	if width >= 120 {
		cols = append(cols, column{"CONTAINERS", 10})
	}
	fixed := (len(cols) - 1) * len(columnGap)
	for i, col := range cols {
		if i != 1 {
			fixed += col.width
		}
	}
	cols[1].width = width - fixed
	if cols[1].width < 1 {
		cols[1].width = 1
	}
	render := func(stack domain.Stack, container *domain.Container, header bool, marker string) string {
		values := make([]string, len(cols))
		for i, col := range cols {
			value := col.title
			if !header {
				if i == 0 {
					value = marker
				} else if container == nil {
					value = stackCell(stack, col.title)
				} else {
					value = stackContainerCell(*container, col.title)
				}
			}
			values[i] = fitCell(value, col.width)
		}
		return strings.Join(values, columnGap)
	}
	var builder strings.Builder
	builder.WriteString(render(domain.Stack{}, nil, true, " "))
	builder.WriteString("\n")
	builder.WriteString(strings.Repeat("-", width))
	visibleStacks := visibleResourceItems(stacks, selected, visibleRowCount(layout), func(s domain.Stack) string { return s.Name })
	for _, stack := range visibleStacks {
		marker := " "
		if editing {
			marker = "[ ]"
			if _, ok := selections[stack.Name]; ok {
				marker = "[x]"
			}
			if stack.Name == selected {
				marker = activeEditMarkerStyle(accentColor).Render(">") + marker
			} else {
				marker = " " + marker
			}
		} else if stack.Name == selected {
			marker = ">"
		}
		builder.WriteString("\n")
		row := render(stack, nil, false, marker)
		if stack.Name == selected {
			row = focusedTableRow(row, width, focusColor, accentColor)
		}
		builder.WriteString(row)
		if stack.Name != expanded {
			continue
		}
		childLimit := visibleRowCount(layout) - len(visibleStacks)
		if childLimit < 1 {
			childLimit = 1
		}
		for _, container := range visibleResourceItems(stack.ContainerItems, selectedContainer, childLimit, func(c domain.Container) string { return c.ID }) {
			marker := " "
			if childEditing {
				marker = "[ ]"
				if _, ok := selectedContainers[container.ID]; ok {
					marker = "[x]"
				}
				if container.ID == selectedContainer {
					marker = activeEditMarkerStyle(accentColor).Render(">") + marker
				} else {
					marker = " " + marker
				}
			} else if container.ID == selectedContainer {
				marker = ">"
			}
			row := render(domain.Stack{}, &container, false, marker)
			if container.ID == selectedContainer {
				row = focusedTableRow(row, width, focusColor, accentColor)
			}
			builder.WriteString("\n" + row)
		}
	}
	return builder.String()
}

func stackCell(stack domain.Stack, title string) string {
	switch title {
	case "NAME":
		return stack.Name
	case "STATE":
		return strings.ToUpper(stack.State)
	case "CPU":
		if stack.CPUAvailable {
			return fmt.Sprintf("%.1f%%", stack.CPUPercent)
		}
	case "MEM":
		if stack.MemoryAvailable {
			if stack.MemoryLimit == 0 {
				return formatBytes(stack.MemoryUsage)
			}
			return formatBytes(stack.MemoryUsage) + "/" + formatBytes(stack.MemoryLimit)
		}
	case "HEALTH":
		return "-"
	case "SERVICES":
		return fmt.Sprintf("%d", len(stack.Services))
	case "CONTAINERS":
		return fmt.Sprintf("%d", stack.Containers)
	}
	return "-"
}

func stackContainerCell(container domain.Container, title string) string {
	switch title {
	case "NAME":
		service := container.ComposeService
		if service == "" {
			service = "container"
		}
		return "+- " + service + "/" + container.Name
	case "STATE":
		return strings.ToUpper(container.State)
	case "CPU":
		if container.CPUAvailable {
			return fmt.Sprintf("%.1f%%", container.CPUPercent)
		}
	case "MEM":
		if container.MemoryAvailable {
			if container.MemoryLimit == 0 {
				return formatBytes(container.MemoryUsage)
			}
			return formatBytes(container.MemoryUsage) + "/" + formatBytes(container.MemoryLimit)
		}
	case "HEALTH":
		return container.Health
	}
	return ""
}
