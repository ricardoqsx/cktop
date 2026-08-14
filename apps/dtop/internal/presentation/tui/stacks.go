package tui

import (
	"fmt"
	"strings"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

func (m Model) stacksView(layout sharedui.Layout) sharedui.View {
	if (m.action.resource == actionStacks || m.action.resource == actionStackContainers) && m.action.stage == actionMenu {
		return m.actionView()
	}
	if m.stacksLoading || !m.stacksLoaded {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageTabStacks), Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageStacksLoading)}
	}
	if m.stacksErr != nil {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageTabStacks), Status: sharedui.StatusError, Summary: m.stacksErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageCommonRetry)}}}
	}
	if m.shellActive {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageContainerShellTitle), Status: sharedui.StatusLoading, Summary: m.localizer.Text(i18n.MessageContainerShellStarting)}
	}
	if m.shellErr != nil {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageTabStacks), Status: sharedui.StatusError, Summary: m.localizer.Text(i18n.MessageContainerShellFailed) + m.shellErr.Error(), Sections: []sharedui.Section{{Title: m.localizer.Text(i18n.MessageSectionNext), Body: m.localizer.Text(i18n.MessageContainerShellFailureNext)}}}
	}
	if len(m.stacks) == 0 && len(m.stackDiagnostics) == 0 {
		return sharedui.View{Title: m.localizer.Text(i18n.MessageTabStacks), Status: sharedui.StatusEmpty, Summary: m.localizer.Text(i18n.MessageStacksEmpty)}
	}
	sections := []sharedui.Section{{Body: renderStacksLocalized(m.stacks, m.selectedStackName, m.selectedStackContainerID, m.selectedStacks, m.selectedStackContainers, m.stackEditing, m.stackContainerEditing, m.expandedStackName, layout, m.accentColor, m.focusColor, m.localizer)}}
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
		down := m.localizer.Text(i18n.MessageCommonAvailable)
		if reason := stack.DownUnavailableReason(); reason != "" {
			down = m.localizer.Text(i18n.MessageCommonUnavailableReason, reason)
		}
		lines := []string{m.localizer.Text(i18n.MessageStackWorkingDirectory, workingDir), m.localizer.Text(i18n.MessageStackComposeFiles, files), m.localizer.Text(i18n.MessageStackDown, down)}
		if stack.UpdatePending {
			if stack.UpdateUnknown {
				reason := stack.UpdateReason
				if reason == "" {
					reason = m.localizer.Text(i18n.MessageCommonUnknown)
				}
				lines = append(lines, m.localizer.Text(i18n.MessageStackUpdateUnknown, reason))
			} else {
				lines = append(lines, m.localizer.Text(i18n.MessageStackUpdatePending))
			}
		}
		sections = append(sections, sharedui.Section{Title: m.localizer.Text(i18n.MessageSectionSelectedStack), Body: strings.Join(lines, "\n")})
	}
	if len(m.stackDiagnostics) > 0 {
		sections = append(sections, sharedui.Section{Title: m.localizer.Text(i18n.MessageSectionRegistrationDiagnostics), Body: strings.Join(m.stackDiagnostics, "\n")})
	}
	return sharedui.View{Title: m.localizer.Text(i18n.MessageTabStacks), Status: sharedui.StatusReady, HideStatus: true, Sections: sections}
}

func renderStacks(stacks []domain.Stack, selected, selectedContainer string, selections, selectedContainers map[string]struct{}, editing, childEditing bool, expanded string, layout sharedui.Layout) string {
	return renderStacksLocalized(stacks, selected, selectedContainer, selections, selectedContainers, editing, childEditing, expanded, layout, "63", "15", i18n.New("en"))
}

func renderStacksWithColors(stacks []domain.Stack, selected, selectedContainer string, selections, selectedContainers map[string]struct{}, editing, childEditing bool, expanded string, layout sharedui.Layout, accentColor, focusColor string) string {
	return renderStacksLocalized(stacks, selected, selectedContainer, selections, selectedContainers, editing, childEditing, expanded, layout, accentColor, focusColor, i18n.New("en"))
}

func renderStacksLocalized(stacks []domain.Stack, selected, selectedContainer string, selections, selectedContainers map[string]struct{}, editing, childEditing bool, expanded string, layout sharedui.Layout, accentColor, focusColor string, localizer sharedui.Localizer) string {
	// Stack rows use the resource table sizing rules but retain expandable detail rows.
	width := layout.ContentWidth
	if width < 20 {
		width = 20
	}
	markerWidth := resourceMarkerWidth(editing || childEditing)
	type column struct {
		id    string
		title string
		width int
	}
	stateWidth := 12
	if width >= 80 {
		stateWidth = 20
	}
	cols := []column{{"marker", "", markerWidth}, {"update", "", 1}, {"name", localizer.Text(i18n.MessageColumnName), 10}, {"state", localizer.Text(i18n.MessageColumnState), stateWidth}}
	if width >= 52 {
		cols = append(cols, column{"cpu", localizer.Text(i18n.MessageColumnCPU), 8}, column{"memory", localizer.Text(i18n.MessageColumnMemory), 12})
	}
	if width >= 72 {
		cols = append(cols, column{"health", localizer.Text(i18n.MessageColumnHealth), 9})
	}
	if width >= 100 {
		cols = append(cols, column{"services", localizer.Text(i18n.MessageColumnServices), 8})
	}
	if width >= 120 {
		cols = append(cols, column{"containers", localizer.Text(i18n.MessageColumnContainers), 10})
	}
	fixed := (len(cols) - 1) * len(columnGap)
	for _, col := range cols {
		if col.id != "name" {
			fixed += col.width
		}
	}
	nameIndex := 0
	for index, col := range cols {
		if col.id == "name" {
			nameIndex = index
			break
		}
	}
	cols[nameIndex].width = width - fixed
	if cols[nameIndex].width < 1 {
		cols[nameIndex].width = 1
	}
	render := func(stack domain.Stack, container *domain.Container, header bool, marker string) string {
		values := make([]string, len(cols))
		for i, col := range cols {
			value := col.title
			if !header {
				if i == 0 {
					value = marker
				} else if container == nil {
					value = stackCellLocalized(stack, col.id, localizer)
				} else {
					value = stackContainerCellLocalized(*container, col.id, localizer)
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
	return stackCellLocalized(stack, strings.ToLower(title), i18n.New("en"))
}

func stackCellLocalized(stack domain.Stack, id string, localizer sharedui.Localizer) string {
	switch id {
	case "name":
		return stack.Name
	case "update":
		return containerUpdateIndicator(stackUpdateStatus(stack))
	case "state":
		return strings.ToUpper(localizeState(localizer, stack.State))
	case "cpu":
		if stack.CPUAvailable {
			return localizer.Decimal(stack.CPUPercent, 1) + "%"
		}
	case "memory":
		if stack.MemoryAvailable {
			if stack.MemoryLimit == 0 {
				return formatBytes(stack.MemoryUsage)
			}
			return formatBytes(stack.MemoryUsage) + "/" + formatBytes(stack.MemoryLimit)
		}
	case "health":
		return "-"
	case "services":
		return fmt.Sprintf("%d", len(stack.Services))
	case "containers":
		return fmt.Sprintf("%d", stack.Containers)
	}
	return "-"
}

func stackContainerCell(container domain.Container, title string) string {
	return stackContainerCellLocalized(container, strings.ToLower(title), i18n.New("en"))
}

func stackContainerCellLocalized(container domain.Container, id string, localizer sharedui.Localizer) string {
	switch id {
	case "name":
		service := container.ComposeService
		if service == "" {
			service = localizer.Text(i18n.MessageStackContainerFallback)
		}
		return "+- " + service + "/" + container.Name
	case "update":
		return containerUpdateIndicator(container.Update)
	case "state":
		return strings.ToUpper(localizeState(localizer, container.State))
	case "cpu":
		if container.CPUAvailable {
			return localizer.Decimal(container.CPUPercent, 1) + "%"
		}
	case "memory":
		if container.MemoryAvailable {
			if container.MemoryLimit == 0 {
				return formatBytes(container.MemoryUsage)
			}
			return formatBytes(container.MemoryUsage) + "/" + formatBytes(container.MemoryLimit)
		}
	case "health":
		return localizeHealth(localizer, container.Health)
	}
	return ""
}
