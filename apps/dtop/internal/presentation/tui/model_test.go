package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

func TestModelRendersLoadedContainers(t *testing.T) {
	service := application.NewContainerService(fakeRuntime{snapshot: domain.Snapshot{
		Engine: domain.EngineInfo{Name: "local", Endpoint: "unix:///tmp/docker.sock", Transport: "unix"},
		Containers: []domain.Container{{
			ID:        "1234567890abcdef",
			ShortID:   "1234567890ab",
			Name:      "web",
			Image:     "nginx:latest",
			State:     "running",
			Health:    "healthy",
			Created:   time.Unix(100, 0),
			StartedAt: time.Unix(100, 0),
		}},
	}})
	model := NewModel(service, config.MemoryBoth)
	model.now = func() time.Time { return time.Unix(100, 0).Add(2 * time.Hour) }

	updated, _ := model.Update(loadedMsg{snapshot: fakeRuntimeSnapshot(service), generation: 1})
	loaded := updated.(Model)
	resized, _ := loaded.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	loaded = resized.(Model)
	view := loaded.View()

	for _, expected := range []string{"LOCAL local", "SORT: State", "1234567890ab", "web", "nginx:latest", "healthy", "2h"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected view to contain %q, got %q", expected, view)
		}
	}
}

func TestHeaderShowsScopeAndAggregatedMetrics(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.loading = false
	model.snapshot = domain.Snapshot{
		Engine:               domain.EngineInfo{Name: "production", Remote: true, ServerVersion: "29.6.2", MemoryTotal: 8 * 1024 * 1024 * 1024},
		Containers:           []domain.Container{{State: "running"}, {State: "stopped"}},
		ContainerCPUPercent:  25,
		CPUAvailable:         true,
		ContainerMemoryUsage: 2 * 1024 * 1024 * 1024,
		MemoryAvailable:      true,
	}

	header := model.headerSummary()
	for _, expected := range []string{"REMOTE production", "CPU 25.0%", "RAM 2G/8G", "1/2 running", "Docker 29.6.2"} {
		if !strings.Contains(header, expected) {
			t.Fatalf("expected header to contain %q, got %q", expected, header)
		}
	}
}

func TestMemoryTextModes(t *testing.T) {
	container := domain.Container{MemoryUsage: 512 * 1024 * 1024, MemoryLimit: 2 * 1024 * 1024 * 1024, MemoryPercent: 25, MemoryAvailable: true}
	tests := []struct {
		mode  config.MemoryMode
		width int
		want  string
	}{
		{mode: config.MemoryUsage, width: 18, want: "512M/2G"},
		{mode: config.MemoryPercent, width: 18, want: "25.0%"},
		{mode: config.MemoryBoth, width: 18, want: "512M/2G 25.0%"},
		{mode: config.MemoryBoth, width: 12, want: "25.0%"},
	}
	for _, test := range tests {
		if got := memoryText(container, test.mode, test.width); got != test.want {
			t.Fatalf("mode %s width %d: expected %q, got %q", test.mode, test.width, test.want, got)
		}
	}
}

func TestMetricBarPreservesCellWidth(t *testing.T) {
	for _, ratio := range []float64{0, 0.5, 1, 2} {
		bar := metricBar("50.0%", ratio, 10)
		if got := ansi.StringWidth(bar); got != 10 {
			t.Fatalf("ratio %.1f expected width 10, got %d", ratio, got)
		}
	}
}

func TestModelDiscardsStaleLoad(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.generation = 2
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{Engine: domain.EngineInfo{Name: "stale"}}, generation: 1})
	if updated.(Model).snapshot.Engine.Name == "stale" {
		t.Fatal("expected stale generation to be discarded")
	}
}

func TestRefreshStartsNextGeneration(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{}, generation: 1})
	loaded := updated.(Model)

	updated, command := loaded.Update(refreshMsg(time.Now()))
	refreshing := updated.(Model)
	if command == nil {
		t.Fatal("expected refresh command")
	}
	if !refreshing.refreshing || refreshing.generation != 2 {
		t.Fatalf("expected generation 2 refreshing, got generation=%d refreshing=%v", refreshing.generation, refreshing.refreshing)
	}
}

func TestModelRendersEmptyState(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{Engine: domain.EngineInfo{Name: "local"}}, generation: 1})

	view := updated.(Model).View()
	if !strings.Contains(view, "no containers were found") {
		t.Fatalf("expected empty state, got %q", view)
	}
}

func TestModelRendersErrorState(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{err: errors.New("docker unavailable"), generation: 1})

	view := updated.(Model).View()
	if !strings.Contains(view, "docker unavailable") {
		t.Fatalf("expected error state, got %q", view)
	}
}

func TestModelNavigatesViewsWithArrowsAndWraps(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	if updated.(Model).active != 1 {
		t.Fatalf("expected active view 1, got %d", updated.(Model).active)
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if updated.(Model).active != 0 {
		t.Fatalf("expected previous view 0, got %d", updated.(Model).active)
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if updated.(Model).active != 4 {
		t.Fatalf("expected wrapped view 4, got %d", updated.(Model).active)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(Model).active != 0 {
		t.Fatalf("tab must not navigate views, got %d", updated.(Model).active)
	}
}

func TestInitSchedulesAggregateResourcesAndLoadsUnopenedTabs(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	message := model.Init()()
	batch, ok := message.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected snapshot and aggregate resource commands, got %#v", message)
	}

	updated, _ := model.Update(resourcesLoadedMsg{resources: ports.ResourceLoad{
		Stacks:   []domain.Stack{{Name: "stack"}},
		Images:   []domain.Image{{ID: "image", Name: "image:latest"}},
		Networks: []domain.Network{{ID: "network", Name: "network"}},
		Volumes:  []domain.Volume{{Name: "volume"}},
	}})
	loaded := updated.(Model)
	if !loaded.stacksLoaded || !loaded.imagesLoaded || !loaded.networksLoaded || !loaded.volumesLoaded {
		t.Fatalf("expected all resource tabs loaded, got %#v", loaded)
	}
	if loaded.selectedStackName != "stack" || loaded.selectedImageID != "image" || loaded.selectedNetworkID != "network" || loaded.selectedVolumeName != "volume" {
		t.Fatalf("expected selections for unopened tabs, got %#v", loaded)
	}
}

func TestAggregateResourcesPreservesResourceSelectionAndExpansion(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.selectedStackName, model.expandedStackName = "stack-b", "stack-b"
	model.selectedImageID, model.selectedNetworkID, model.selectedVolumeName = "image-b", "network-b", "volume-b"

	updated, _ := model.Update(resourcesLoadedMsg{resources: ports.ResourceLoad{
		Stacks:   []domain.Stack{{Name: "stack-a"}, {Name: "stack-b"}},
		Images:   []domain.Image{{ID: "image-a"}, {ID: "image-b"}},
		Networks: []domain.Network{{ID: "network-a"}, {ID: "network-b"}},
		Volumes:  []domain.Volume{{Name: "volume-a"}, {Name: "volume-b"}},
	}})
	loaded := updated.(Model)
	if loaded.selectedStackName != "stack-b" || loaded.expandedStackName != "stack-b" || loaded.selectedImageID != "image-b" || loaded.selectedNetworkID != "network-b" || loaded.selectedVolumeName != "volume-b" {
		t.Fatalf("aggregate load changed stable resource state: %#v", loaded)
	}
}

func TestAggregateResourcesKeepsPartialErrorsIndependent(t *testing.T) {
	imageErr := errors.New("images unavailable")
	updated, _ := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth).Update(resourcesLoadedMsg{resources: ports.ResourceLoad{
		ImagesErr: imageErr,
		Networks:  []domain.Network{{ID: "network"}},
		Volumes:   []domain.Volume{{Name: "volume", UsageKnown: false}},
	}})
	loaded := updated.(Model)
	if !errors.Is(loaded.imagesErr, imageErr) || loaded.networksErr != nil || loaded.volumesErr != nil || !loaded.networksLoaded || !loaded.volumesLoaded {
		t.Fatalf("expected independent resource results, got %#v", loaded)
	}
}

func TestResourceUsageShowsUnknown(t *testing.T) {
	if got := resourceUsage(0, false); got != "unknown" {
		t.Fatalf("expected unknown usage, got %q", got)
	}
}

func TestRenderImagesShowsUsageAndFitsLayout(t *testing.T) {
	layout := sharedui.ResolveLayout(100, 20)
	view := renderImages([]domain.Image{{ID: "one", Name: "nginx:latest", Size: 128 * 1024 * 1024, Containers: 2, UsageKnown: true, Created: time.Unix(100, 0)}}, "one", nil, false, layout, time.Unix(100, 0).Add(2*time.Hour))
	for _, expected := range []string{"NAME", "nginx:latest", "128M", "2 containers", "2h"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in image view, got %q", expected, view)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > layout.ContentWidth {
			t.Fatalf("rendered image line width %d, got %q", got, line)
		}
	}
}

func TestRenderContainersTruncatesLongValues(t *testing.T) {
	layout := sharedui.ResolveLayout(100, 30)
	view := renderContainers([]domain.Container{{
		ID:      "abc",
		ShortID: "abc",
		Name:    "very-long-container-name-that-should-truncate",
		Image:   "example.com/really/long/image/name:latest",
		State:   "running",
		Health:  "-",
		Created: time.Unix(0, 0),
	}}, "abc", nil, false, time.Unix(3600, 0), layout, config.MemoryBoth)

	if !strings.Contains(view, "very-long-container-name-t...") || strings.Contains(view, "very-long-container-name-that-should-truncate") {
		t.Fatalf("expected truncated name, got %q", view)
	}
}

func TestContainerColumnsFollowWidthPriority(t *testing.T) {
	tests := []struct {
		width   int
		present []string
		absent  []string
	}{
		{width: 48, present: []string{"NAME", "STATE"}, absent: []string{"CPU", "MEM", "HEALTH", "AGE", "IMAGE", "ID"}},
		{width: 72, present: []string{"NAME", "STATE", "CPU", "MEM", "HEALTH"}, absent: []string{"UPTIME", "IMAGE", "ID"}},
		{width: 100, present: []string{"NAME", "STATE", "CPU", "MEM", "HEALTH", "UPTIME"}, absent: []string{"IMAGE", "ID"}},
		{width: 140, present: []string{"NAME", "STATE", "CPU", "MEM", "HEALTH", "UPTIME", "IMAGE"}, absent: []string{"ID"}},
		{width: 160, present: []string{"NAME", "STATE", "CPU", "MEM", "HEALTH", "UPTIME", "IMAGE", "ID"}},
	}
	for _, test := range tests {
		columns := containerColumns(test.width, config.MemoryBoth, false)
		for _, title := range test.present {
			if columnIndex(columns, title) < 0 {
				t.Fatalf("width %d expected column %s", test.width, title)
			}
		}
		for _, title := range test.absent {
			if columnIndex(columns, title) >= 0 {
				t.Fatalf("width %d did not expect column %s", test.width, title)
			}
		}
		if got := tableWidth(columns); got != test.width {
			t.Fatalf("width %d produced table width %d", test.width, got)
		}
	}
}

func TestFormatUptimeUsesRunningStartTime(t *testing.T) {
	now := time.Unix(10_000, 0)
	if got := formatUptime(now.Add(-3*time.Hour), "running", now); got != "3h" {
		t.Fatalf("expected 3h uptime, got %q", got)
	}
	if got := formatUptime(now.Add(-3*time.Hour), "exited", now); got != "-" {
		t.Fatalf("expected stopped uptime to be unavailable, got %q", got)
	}
	if got := formatUptime(time.Time{}, "running", now); got != "-" {
		t.Fatalf("expected missing start time to be unavailable, got %q", got)
	}
}

func TestModelCyclesSortWithoutLosingSelection(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{Containers: []domain.Container{
		{ID: "one", Name: "one", State: "running", CPUAvailable: true, CPUPercent: 10},
		{ID: "two", Name: "two", State: "running", CPUAvailable: true, CPUPercent: 90},
	}}, generation: 1})
	loaded := updated.(Model)
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected := updated.(Model)
	if selected.selectedID != "two" {
		t.Fatalf("expected second container selected, got %q", selected.selectedID)
	}

	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	sorted := updated.(Model)
	if sorted.sortMode != application.SortCPU {
		t.Fatalf("expected CPU sort, got %q", sorted.sortMode)
	}
	if sorted.selectedID != "two" {
		t.Fatalf("expected selection to remain on two, got %q", sorted.selectedID)
	}
	if sorted.snapshot.Containers[0].ID != "two" {
		t.Fatalf("expected high CPU container first, got %q", sorted.snapshot.Containers[0].ID)
	}
}

func TestEditingSelectsMultipleContainersAndOpensMenu(t *testing.T) {
	model := loadedSelectionModel(t)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	editing := updated.(Model)
	if !editing.editing {
		t.Fatal("expected edit mode")
	}

	updated, _ = editing.Update(tea.KeyMsg{Type: tea.KeySpace})
	selected := updated.(Model)
	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	selected = updated.(Model)
	if len(selected.selected) != 2 {
		t.Fatalf("expected two selected containers, got %d", len(selected.selected))
	}

	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if menu.action.stage != actionMenu || len(menu.action.targets) != 2 {
		t.Fatalf("expected action menu for two targets, got %#v", menu.action)
	}
}

func TestDeleteConfirmationRequiresExactText(t *testing.T) {
	model := loadedSelectionModel(t)
	model.action = actionState{
		stage:   actionConfirm,
		index:   2,
		targets: []actionTarget{{ID: "one", Name: "one", State: "running"}},
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("delete should not run without confirmation text")
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("delete")})
	confirmed, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !confirmed.(Model).action.running {
		t.Fatal("expected force delete command after typing delete")
	}
}

func TestActionResultClearsSelection(t *testing.T) {
	model := loadedSelectionModel(t)
	model.editing = true
	model.selected["one"] = struct{}{}
	model.action = actionState{stage: actionConfirm, targets: []actionTarget{{ID: "one", Name: "one"}}}

	updated, command := model.Update(actionFinishedMsg{results: []application.ActionResult{{ID: "one", Action: application.ActionStop}}})
	result := updated.(Model)
	if command == nil {
		t.Fatal("expected refresh after action")
	}
	if result.action.stage != actionResult || result.editing || len(result.selected) != 0 {
		t.Fatalf("expected cleared selection and result stage, got %#v", result)
	}
}

func TestEditTableShowsCheckboxes(t *testing.T) {
	layout := sharedui.ResolveLayout(80, 20)
	view := renderContainers([]domain.Container{{ID: "one", Name: "one", State: "running"}}, "one", map[string]struct{}{"one": {}}, true, time.Now(), layout, config.MemoryBoth)
	if !strings.Contains(view, "[x]") {
		t.Fatalf("expected selected checkbox, got %q", view)
	}
	if !strings.Contains(view, ">[x]") {
		t.Fatalf("expected active edit cursor, got %q", view)
	}
}

func TestImageEditingSelectsMultipleImagesAndOpensDeleteOnlyMenu(t *testing.T) {
	model := loadedImageSelectionModel(t)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	editing := updated.(Model)
	updated, _ = editing.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	selected := updated.(Model)
	if !selected.imageEditing || len(selected.selectedImages) != 2 {
		t.Fatalf("expected two selected images, got editing=%v selected=%d", selected.imageEditing, len(selected.selectedImages))
	}

	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if menu.action.stage != actionMenu || menu.action.resource != actionImages || len(menu.action.targets) != 2 {
		t.Fatalf("expected image action menu, got %#v", menu.action)
	}
	if got := actionChoices(actionImages); len(got) != 2 || got[0] != application.ActionDelete || got[1] != "cancel" {
		t.Fatalf("expected only delete and cancel, got %v", got)
	}
}

func TestImageDeleteConfirmationRequiresExactTextAndExplainsSafety(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.action = actionState{stage: actionConfirm, resource: actionImages, targets: []actionTarget{{ID: "one", Name: "one:latest"}}}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("image delete should not run without confirmation text")
	}
	view := updated.(Model).actionConfirmationView()
	if !strings.Contains(view.Sections[1].Body, "without force") || !strings.Contains(view.Sections[1].Body, "used by containers") {
		t.Fatalf("expected non-force usage warning, got %q", view.Sections[1].Body)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Delete")})
	updated, command = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("image delete should require lowercase exact confirmation text")
	}

	model = loadedImageSelectionModel(t)
	model.action = actionState{stage: actionConfirm, resource: actionImages, targets: []actionTarget{{ID: "one", Name: "one:latest"}}}
	updated = model
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("delete")})
	confirmed, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !confirmed.(Model).action.running {
		t.Fatal("expected image delete command after typing exact delete")
	}
}

func TestImageEditTableShowsCheckboxes(t *testing.T) {
	layout := sharedui.ResolveLayout(80, 20)
	view := renderImages([]domain.Image{{ID: "one", Name: "one:latest"}}, "one", map[string]struct{}{"one": {}}, true, layout, time.Now())
	if !strings.Contains(view, "[x]") || !strings.Contains(view, ">[x]") {
		t.Fatalf("expected active selected image checkbox, got %q", view)
	}
}

func TestNetworkAndVolumeEditTablesShowActiveCheckboxCursor(t *testing.T) {
	layout := sharedui.ResolveLayout(80, 20)
	networks := renderNetworks([]domain.Network{{ID: "network", Name: "network"}}, "network", map[string]struct{}{"network": {}}, true, layout, time.Now())
	volumes := renderVolumes([]domain.Volume{{Name: "volume"}}, "volume", map[string]struct{}{"volume": {}}, true, layout, time.Now())
	if !strings.Contains(networks, ">[x]") || !strings.Contains(volumes, ">[x]") {
		t.Fatalf("expected active selected checkboxes, networks=%q volumes=%q", networks, volumes)
	}
}

func TestNetworkAndVolumeEditingOpenDeleteMenuAndRequireExactConfirmation(t *testing.T) {
	for _, test := range []struct {
		name     string
		model    Model
		resource actionResource
		warning  string
	}{
		{name: "network", model: loadedNetworkSelectionModel(t), resource: actionNetworks, warning: "fails if a network is connected"},
		{name: "volume", model: loadedVolumeSelectionModel(t), resource: actionVolumes, warning: "can delete persistent data"},
	} {
		t.Run(test.name, func(t *testing.T) {
			updated, _ := test.model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
			updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
			updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			menu := updated.(Model)
			if menu.action.stage != actionMenu || menu.action.resource != test.resource || len(menu.action.targets) != 1 {
				t.Fatalf("expected delete menu, got %#v", menu.action)
			}
			updated, _ = menu.Update(tea.KeyMsg{Type: tea.KeyEnter})
			confirmation := updated.(Model)
			if !strings.Contains(confirmation.actionConfirmationView().Sections[1].Body, test.warning) {
				t.Fatalf("missing safety warning: %q", confirmation.actionConfirmationView().Sections[1].Body)
			}
			updated, command := confirmation.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if command != nil || updated.(Model).action.running {
				t.Fatal("delete should require exact confirmation")
			}
			updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Delete")})
			updated, command = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			if command != nil || updated.(Model).action.running {
				t.Fatal("delete should require lowercase exact confirmation")
			}
			confirmation.action.input = "delete"
			updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if command == nil || !updated.(Model).action.running {
				t.Fatal("expected delete command after exact confirmation")
			}
		})
	}
}

func TestNetworkAndVolumeSelectionsRemainStableAcrossRefreshOrder(t *testing.T) {
	model := loadedNetworkSelectionModel(t)
	model.selectedNetworkID = "network-two"
	model.selectedNetworks["network-two"] = struct{}{}
	updated, _ := model.Update(networksLoadedMsg{networks: []domain.Network{{ID: "network-one"}, {ID: "network-two"}}, generation: model.networksGen})
	networks := updated.(Model)
	if networks.selectedNetworkID != "network-two" {
		t.Fatalf("expected stable network cursor, got %q", networks.selectedNetworkID)
	}
	if _, selected := networks.selectedNetworks["network-two"]; !selected {
		t.Fatalf("expected stable network selection, got %v", networks.selectedNetworks)
	}

	model = loadedVolumeSelectionModel(t)
	model.selectedVolumeName = "volume-two"
	model.selectedVolumes["volume-two"] = struct{}{}
	updated, _ = model.Update(volumesLoadedMsg{volumes: []domain.Volume{{Name: "volume-one"}, {Name: "volume-two"}}, generation: model.volumesGen})
	volumes := updated.(Model)
	if volumes.selectedVolumeName != "volume-two" {
		t.Fatalf("expected stable volume cursor, got %q", volumes.selectedVolumeName)
	}
	if _, selected := volumes.selectedVolumes["volume-two"]; !selected {
		t.Fatalf("expected stable volume selection, got %v", volumes.selectedVolumes)
	}
}

func TestModelPreservesSelectionDuringResize(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	snapshot := domain.Snapshot{Containers: []domain.Container{
		{ID: "one", Name: "one"},
		{ID: "two", Name: "two"},
	}}
	updated, _ := model.Update(loadedMsg{snapshot: snapshot, generation: 1})
	loaded := updated.(Model)
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected := updated.(Model)
	if selected.selectedID != "two" {
		t.Fatalf("expected second container selected, got %q", selected.selectedID)
	}

	updated, _ = selected.Update(tea.WindowSizeMsg{Width: 32, Height: 8})
	if got := updated.(Model).selectedID; got != "two" {
		t.Fatalf("expected selection to survive resize, got %q", got)
	}
}

func TestEnterOpensDetailsForCurrentContainer(t *testing.T) {
	model := loadedSelectionModel(t)

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened := updated.(Model)
	if command == nil || opened.panel != panelDetails || !opened.detailLoading {
		t.Fatalf("expected details to open and load, got panel=%d loading=%v", opened.panel, opened.detailLoading)
	}
}

func TestEscCancelsLogsAndReturnsToContainers(t *testing.T) {
	model := loadedSelectionModel(t)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	logs := updated.(Model)
	if command == nil || logs.panel != panelLogs || logs.logCancel == nil {
		t.Fatalf("expected log stream to start, got panel=%d cancel=%v", logs.panel, logs.logCancel != nil)
	}

	updated, _ = logs.Update(tea.KeyMsg{Type: tea.KeyEscape})
	closed := updated.(Model)
	if closed.panel != panelContainers || closed.logCancel != nil {
		t.Fatalf("expected logs to close, got panel=%d cancel=%v", closed.panel, closed.logCancel != nil)
	}
}

func TestVisibleLogsRespectsOffsetAndLimit(t *testing.T) {
	lines := []string{"one", "two", "three", "four"}
	if got := strings.Join(visibleLogs(lines, 1, 2), ","); got != "two,three" {
		t.Fatalf("expected offset log window, got %q", got)
	}
}

func TestModelLinesFitReferenceSizes(t *testing.T) {
	containers := make([]domain.Container, 20)
	for index := range containers {
		containers[index] = domain.Container{
			ID:      "container-id-" + string(rune('a'+index)),
			ShortID: "123456789012",
			Name:    "container-with-a-long-name",
			Image:   "example.com/namespace/image:latest",
			State:   "running",
			Health:  "healthy",
			Created: time.Unix(100, 0),
		}
	}
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{
		Engine:     domain.EngineInfo{Name: "desktop-linux", ServerVersion: "29.6.2"},
		Containers: containers,
	}, generation: 1})
	model = updated.(Model)

	for _, size := range []tea.WindowSizeMsg{{Width: 160, Height: 45}, {Width: 100, Height: 30}, {Width: 72, Height: 20}, {Width: 48, Height: 12}, {Width: 32, Height: 8}} {
		updated, _ := model.Update(size)
		view := updated.(Model).View()
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

func TestVisibleContainersKeepsSelectionInViewport(t *testing.T) {
	containers := make([]domain.Container, 10)
	for index := range containers {
		containers[index] = domain.Container{ID: fmt.Sprintf("container-%d", index)}
	}

	visible := visibleContainers(containers, "container-8", 3)
	found := false
	for _, container := range visible {
		if container.ID == "container-8" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected selected container in viewport, got %#v", visible)
	}
}

type fakeRuntime struct {
	snapshot     domain.Snapshot
	images       []domain.Image
	imageDetails domain.ImageDetails
	stacks       []domain.Stack
	networks     []domain.Network
	volumes      []domain.Volume
	resources    ports.ResourceLoad
	err          error
}

func (f fakeRuntime) Stacks(context.Context) ([]domain.Stack, error) { return f.stacks, f.err }

func (f fakeRuntime) LoadResources(context.Context) (ports.ResourceLoad, error) {
	return f.resources, f.err
}

func TestStacksLoadExpandAndKeepSelection(t *testing.T) {
	stacks := []domain.Stack{{Name: "alpha", State: "mixed", Containers: 2, ContainerItems: []domain.Container{{ID: "web-id", Name: "web-1", ComposeService: "web", State: "running", Health: "healthy"}}}, {Name: "beta", State: "stopped", Containers: 1}}
	model := NewModel(application.NewContainerService(fakeRuntime{stacks: stacks}), config.MemoryBoth)
	model.snapshot.Containers = []domain.Container{{ID: "web-id", Name: "web-1", ComposeProject: "alpha", ComposeService: "web", State: "running", Health: "healthy"}}
	updated, _ := model.Update(resourcesLoadedMsg{resources: ports.ResourceLoad{Stacks: stacks}})
	loaded := updated.(Model)
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyRight})
	loaded = updated.(Model)
	if loaded.active != 1 || loaded.stacksLoading {
		t.Fatalf("expected aggregate-loaded stacks without a lazy load, got active=%d loading=%v", loaded.active, loaded.stacksLoading)
	}
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(Model); got.selectedStackContainerID != "web-id" || !strings.Contains(got.View(), "web-1") {
		t.Fatalf("expected focused expanded container, got %#v", got)
	}
	updated, _ = updated.(Model).Update(resourcesLoadedMsg{resources: ports.ResourceLoad{Stacks: stacks}})
	if got := updated.(Model); got.selectedStackName != "alpha" || got.expandedStackName != "alpha" {
		t.Fatalf("expected stable stack state after refresh, got selected=%q expanded=%q", got.selectedStackName, got.expandedStackName)
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	loaded = updated.(Model)
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected := updated.(Model)
	if selected.selectedStackName != "beta" || selected.selectedStackContainerID != "" {
		t.Fatalf("expected beta selected, got %q", selected.selectedStackName)
	}
	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	expanded := updated.(Model)
	if expanded.expandedStackName != "beta" {
		t.Fatalf("expected beta expanded, got %q", expanded.expandedStackName)
	}
	updated, _ = expanded.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := updated.(Model); got.expandedStackName != "" || got.selectedStackName != "beta" {
		t.Fatalf("expected collapsed stable selection, got expanded=%q selected=%q", got.expandedStackName, got.selectedStackName)
	}
}

func TestStackMetadataIsAlwaysVisibleForSelectedParent(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.width, model.height = 100, 30
	model.stacks = []domain.Stack{{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}}
	model.syncStackSelection()
	view := model.View()
	for _, expected := range []string{"Selected stack", "Working directory: /srv/app", "Compose files: /srv/app/compose.yaml", "Down: available"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in persistent metadata panel: %q", expected, view)
		}
	}
}

func TestStackChildNavigationSelectionLogsAndActions(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.width, model.height = 100, 30
	model.stacks = []domain.Stack{{Name: "app", ContainerItems: []domain.Container{
		{ID: "one", Name: "api-1", ComposeService: "api", State: "running", Health: "healthy", CPUAvailable: true, CPUPercent: 2.5, MemoryAvailable: true, MemoryUsage: 1024},
		{ID: "two", Name: "worker-1", ComposeService: "worker", State: "exited", Health: "-"},
	}}}
	model.syncStackSelection()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	expanded := updated.(Model)
	if expanded.selectedStackContainerID != "one" || !strings.Contains(expanded.View(), "api-1") || !strings.Contains(expanded.View(), "2.5%") {
		t.Fatalf("expected focused child metrics, got %#v", expanded)
	}
	updated, _ = expanded.Update(tea.KeyMsg{Type: tea.KeyDown})
	child := updated.(Model)
	if child.selectedStackContainerID != "two" {
		t.Fatalf("expected second child, got %q", child.selectedStackContainerID)
	}
	updated, command := child.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if command == nil || updated.(Model).panel != panelLogs {
		t.Fatal("expected child logs to open")
	}
	child = expanded
	updated, _ = child.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	selected := updated.(Model)
	stackTable := renderStacks(selected.stacks, selected.selectedStackName, selected.selectedStackContainerID, selected.selectedStacks, selected.selectedStackContainers, selected.stackEditing, selected.stackContainerEditing, selected.expandedStackName, sharedui.ResolveLayout(100, 30))
	if !selected.stackContainerEditing || len(selected.selectedStackContainers) != 1 || !strings.Contains(ansi.Strip(stackTable), ">[x]") {
		t.Fatalf("expected child selection marker, got %q", ansi.Strip(stackTable))
	}
	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if menu.action.resource != actionStackContainers || menu.action.stage != actionMenu || len(menu.action.targets) != 1 || menu.action.targets[0].ID != "one" {
		t.Fatalf("expected child action menu, got %#v", menu.action)
	}
	updated, _ = menu.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmation := updated.(Model)
	if selectedAction(actionStackContainers, 0) != application.ActionRestart || !strings.Contains(confirmation.actionConfirmationView().Sections[0].Body, "api-1") {
		t.Fatalf("expected restart confirmation for child, got %#v", confirmation.action)
	}
	updated, _ = child.Update(tea.KeyMsg{Type: tea.KeyEsc})
	collapsed := updated.(Model)
	if collapsed.selectedStackContainerID != "" || collapsed.expandedStackName != "" || collapsed.selectedStackName != "app" {
		t.Fatalf("expected Esc to return to collapsed parent, got %#v", collapsed)
	}
}

func TestStackChildShellStartsWithDockerExec(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = []domain.Stack{{Name: "app", ContainerItems: []domain.Container{{ID: "child-id", Name: "api", State: "running"}}}}
	model.syncStackSelection()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	updated, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	shell := updated.(Model)
	if command == nil || !shell.shellActive || shell.selectedStackContainerID != "child-id" {
		t.Fatalf("expected child shell command and preserved focus, got %#v", shell)
	}
}

func TestContainerShellStartsForSelectedContainer(t *testing.T) {
	model := loadedSelectionModel(t)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	shell := updated.(Model)
	if command == nil || !shell.shellActive || shell.selectedID != "one" {
		t.Fatalf("expected shell command for selected container, got %#v", shell)
	}
}

func TestContainerShellDoesNotStartInEditMode(t *testing.T) {
	model := loadedSelectionModel(t)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if command != nil || updated.(Model).shellActive {
		t.Fatal("shell must not start while editing containers")
	}
}

func TestStackParentShellDoesNothing(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = []domain.Stack{{Name: "app"}}
	model.syncStackSelection()

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if command != nil || updated.(Model).shellActive {
		t.Fatal("stack parent must not start a shell")
	}
}

func TestShellCommandUsesDockerExecPTY(t *testing.T) {
	command := makeShellCommand("container-id")
	if command.Args[0] != "docker" {
		t.Fatalf("expected docker command, got %q", command.Args[0])
	}
	if got, want := strings.Join(command.Args[1:], " "), "exec -it container-id /bin/sh -l"; got != want {
		t.Fatalf("unexpected shell arguments: got %q want %q", got, want)
	}
}

func TestShellFinishedResumesStacksWithoutNormalExitError(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.shellActive, model.stacksLoaded, model.stacksLoading = 1, true, true, false
	model.selectedStackName, model.expandedStackName, model.selectedStackContainerID = "app", "app", "child-id"

	updated, _ := model.Update(shellFinishedMsg{})
	finished := updated.(Model)
	if finished.shellActive || finished.shellErr != nil || finished.active != 1 || finished.selectedStackContainerID != "child-id" {
		t.Fatalf("normal shell exit did not resume focused Stacks: %#v", finished)
	}

	updated, _ = finished.Update(shellFinishedMsg{err: errors.New("exit status 126")})
	failed := updated.(Model)
	if failed.shellActive || failed.shellErr == nil || !strings.Contains(failed.stacksView(sharedui.ResolveLayout(100, 30)).Summary, "exit status 126") {
		t.Fatalf("shell failure was not rendered as a controlled error: %#v", failed)
	}
}

func TestStackRegistrationDiagnosticsAndDetailsAreVisible(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth, "config test.conf:1: Compose registration \"bad\" requires a nonempty name, working_dir, and files")
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = []domain.Stack{{Name: "down", State: "down", WorkingDir: "/srv/down", Files: []string{"/srv/down/compose.yaml"}}}
	model.syncStackSelection()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := updated.(Model).View()
	for _, expected := range []string{"Working directory: /srv/down", "Compose files: /srv/down/compose.yaml", "Registration diagnostics", "requires a nonempty"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in stack view, got %q", expected, view)
		}
	}
}

func TestRenderStacksShowsMissingComposeFileState(t *testing.T) {
	view := renderStacks([]domain.Stack{{Name: "missing", State: "missing compose file"}}, "missing", "", nil, nil, false, false, "", sharedui.ResolveLayout(100, 20))
	if !strings.Contains(view, "MISSING COMPOSE FILE") {
		t.Fatalf("expected complete missing manifest state, got %q", view)
	}
}

func TestStackSelectionSurvivesRegisteredProjectMerge(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.selectedStackName, model.expandedStackName = "detected", "detected"
	model.stacks = mergeStacksForTest([]domain.Stack{{Name: "detected", State: "running"}})
	model.syncStackSelection()
	if model.selectedStackName != "detected" || model.expandedStackName != "detected" {
		t.Fatalf("registered project changed stable selection: %#v", model)
	}
}

func TestPeriodicSnapshotRefreshRebuildsStacksAndPreservesExpandedChildFocus(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	initial := domain.Snapshot{Containers: []domain.Container{{ID: "web", Name: "web-1", ComposeProject: "app", ComposeService: "web", State: "running", ComposeWorkingDir: "/srv/app", ComposeConfigFiles: "compose.yaml"}}}
	updated, _ := model.Update(loadedMsg{snapshot: initial, generation: 1})
	loaded := updated.(Model)
	loaded.active = 1
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyEnter})
	expanded := updated.(Model)
	expanded.selectedStacks["app"] = struct{}{}
	expanded.selectedStackContainers["web"] = struct{}{}

	expanded.generation = 2
	refreshedSnapshot := initial
	refreshedSnapshot.Containers = append(refreshedSnapshot.Containers, domain.Container{ID: "worker", Name: "worker-1", ComposeProject: "app", ComposeService: "worker", State: "running", ComposeWorkingDir: "/srv/app", ComposeConfigFiles: "compose.yaml"})
	updated, _ = expanded.Update(loadedMsg{snapshot: refreshedSnapshot, generation: 2})
	refreshed := updated.(Model)
	if len(refreshed.stacks) != 1 || len(refreshed.stacks[0].ContainerItems) != 2 || refreshed.expandedStackName != "app" || refreshed.selectedStackContainerID != "web" {
		t.Fatalf("expected expanded stack and child focus to survive refresh, got %#v", refreshed)
	}
	if _, selected := refreshed.selectedStacks["app"]; !selected {
		t.Fatalf("expected parent selection to survive refresh: %#v", refreshed.selectedStacks)
	}
	if _, selected := refreshed.selectedStackContainers["web"]; !selected {
		t.Fatalf("expected child selection to survive refresh: %#v", refreshed.selectedStackContainers)
	}
}

func TestPeriodicSnapshotRefreshAddsNewDetectedStack(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{}, generation: 1})
	loaded := updated.(Model)
	loaded.generation = 2
	updated, _ = loaded.Update(loadedMsg{snapshot: domain.Snapshot{Containers: []domain.Container{{ID: "web", Name: "web-1", ComposeProject: "new-app", ComposeService: "web", State: "running"}}}, generation: 2})
	if got := updated.(Model).stacks; len(got) != 1 || got[0].Name != "new-app" || got[0].Containers != 1 {
		t.Fatalf("periodic snapshot did not create detected stack: %#v", got)
	}
}

func TestFailedPeriodicSnapshotDoesNotReplaceStacks(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{Containers: []domain.Container{{ID: "web", ComposeProject: "app", State: "running"}}}, generation: 1})
	loaded := updated.(Model)
	loaded.generation = 2
	updated, _ = loaded.Update(loadedMsg{err: errors.New("Docker unavailable"), generation: 2})
	if got := updated.(Model); len(got.stacks) != 1 || got.stacks[0].Name != "app" {
		t.Fatalf("failed refresh replaced known stack data: %#v", got.stacks)
	}
}

func TestStackEditingShowsCheckboxesAndDownConfirmationUsesYOrN(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = []domain.Stack{{Name: "app", State: "down", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}, {Name: "missing"}}
	model.syncStackSelection()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	editing := updated.(Model)
	if !strings.Contains(editing.View(), ">[x]") {
		t.Fatalf("expected active stack checkbox: %q", editing.View())
	}
	updated, _ = editing.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if menu.action.stage != actionMenu || menu.action.resource != actionStacks {
		t.Fatalf("expected stack action menu: %#v", menu.action)
	}
	updated, _ = menu.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirm := updated.(Model)
	if !strings.Contains(confirm.actionConfirmationView().Sections[1].Body, "[y/N]") {
		t.Fatal("expected y/n confirmation")
	}
	updated, command := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if command != nil || updated.(Model).action.stage != actionNone {
		t.Fatal("n must cancel down")
	}
	confirm.action.stage = actionConfirm
	updated, command = confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil || !updated.(Model).action.running {
		t.Fatal("y must start down")
	}
}

func TestStackActionMenuShowsUnavailableSelectedStack(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.action = actionState{stage: actionMenu, resource: actionStacks, targets: []actionTarget{{ID: "valid", Name: "valid"}, {ID: "missing", Name: "missing", State: "Compose config files label is missing"}}}
	view := model.actionMenuView()
	if !strings.Contains(view.Sections[0].Body, "missing (unavailable: Compose config files label is missing)") {
		t.Fatalf("unavailable stack was silently hidden: %q", view.Sections[0].Body)
	}
}

func TestStackActionsFollowStateAndMetadata(t *testing.T) {
	tests := []struct {
		state, unavailable string
		want               []application.Action
	}{
		{"down", "", []application.Action{application.ActionUp, "cancel"}},
		{"missing compose file", "", []application.Action{application.ActionUp, "cancel"}},
		{"running", "", []application.Action{application.ActionStop, application.ActionRestart, application.ActionDown, "cancel"}},
		{"mixed", "", []application.Action{application.ActionStop, application.ActionRestart, application.ActionDown, "cancel"}},
		{"stopped", "", []application.Action{application.ActionUp, application.ActionRestart, application.ActionDown, "cancel"}},
		{"down", "Compose config files are unavailable", []application.Action{"cancel"}},
	}
	for _, test := range tests {
		got := stackActionChoices([]actionTarget{{State: test.state, Unavailable: test.unavailable}})
		if fmt.Sprint(got) != fmt.Sprint(test.want) {
			t.Fatalf("state=%q unavailable=%q got=%v want=%v", test.state, test.unavailable, got, test.want)
		}
	}
}

func TestStackParentLogsUseComposeStreamAndEscReturnsStacks(t *testing.T) {
	runtime := &stackLogRuntime{}
	model := NewModel(application.NewContainerService(runtime), config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = []domain.Stack{{Name: "app", State: "running", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}}
	model.syncStackSelection()
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	logs := updated.(Model)
	if command == nil || logs.panel != panelLogs || !logs.stackLogs || logs.logTitle != "app" {
		t.Fatalf("expected Compose parent logs, got %#v", logs)
	}
	_ = command()
	if runtime.composeLogCalls != 1 {
		t.Fatalf("expected parent to use Compose stream, calls=%d", runtime.composeLogCalls)
	}
	updated, _ = logs.Update(tea.KeyMsg{Type: tea.KeyEscape})
	closed := updated.(Model)
	if closed.active != 1 || closed.panel != panelContainers || closed.stackLogs {
		t.Fatalf("expected Esc back to Stacks, got %#v", closed)
	}
	select {
	case <-runtime.ctx.Done():
	default:
		t.Fatal("expected Esc to cancel the Compose stream")
	}
}

type stackLogRuntime struct {
	fakeRuntime
	composeLogCalls int
	ctx             context.Context
}

func (f *stackLogRuntime) ComposeLogs(ctx context.Context, _ domain.Stack, _ int) (ports.LogStream, error) {
	f.composeLogCalls++
	f.ctx = ctx
	return ports.LogStream{Lines: make(chan string), Errors: make(chan error)}, nil
}

func TestRenderStacksFitsResponsiveWidthAndShowsUnavailableReason(t *testing.T) {
	layout := sharedui.ResolveLayout(48, 20)
	view := renderStacks([]domain.Stack{{Name: "very-long-stack-name", MetadataReason: "Compose config files label is missing"}}, "very-long-stack-name", "", nil, nil, false, false, "very-long-stack-name", layout)
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > layout.ContentWidth {
			t.Fatalf("line exceeds layout: %q", line)
		}
	}
}

func mergeStacksForTest(detected []domain.Stack) []domain.Stack {
	return application.MergeStacks(detected, []application.ComposeProject{{Name: "registered", WorkingDir: "/srv/registered", Files: []string{"/srv/registered/compose.yaml"}}})
}

func (f fakeRuntime) Snapshot(context.Context) (domain.Snapshot, error) {
	return f.snapshot, f.err
}

func (f fakeRuntime) Images(context.Context) ([]domain.Image, error) {
	return f.images, f.err
}

func (f fakeRuntime) ImageDetails(context.Context, string) (domain.ImageDetails, error) {
	return f.imageDetails, f.err
}

func (f fakeRuntime) Networks(context.Context) ([]domain.Network, error) { return f.networks, f.err }
func (f fakeRuntime) NetworkDetails(context.Context, string) (domain.NetworkDetails, error) {
	return domain.NetworkDetails{}, f.err
}
func (f fakeRuntime) Volumes(context.Context) ([]domain.Volume, error) { return f.volumes, f.err }
func (f fakeRuntime) VolumeDetails(context.Context, string) (domain.VolumeDetails, error) {
	return domain.VolumeDetails{}, f.err
}

func (fakeRuntime) Details(context.Context, string) (domain.ContainerDetails, error) {
	return domain.ContainerDetails{}, nil
}

func (fakeRuntime) Logs(context.Context, string, int) (ports.LogStream, error) {
	return ports.LogStream{}, nil
}

func (fakeRuntime) Stop(context.Context, string) error {
	return nil
}

func (fakeRuntime) Restart(context.Context, string) error {
	return nil
}

func (fakeRuntime) Remove(context.Context, string, bool) error {
	return nil
}

func (fakeRuntime) RemoveImage(context.Context, string, bool) error {
	return nil
}

func (fakeRuntime) RemoveNetwork(context.Context, string) error { return nil }

func (fakeRuntime) RemoveVolume(context.Context, string) error { return nil }

func (fakeRuntime) Down(context.Context, domain.Stack) error         { return nil }
func (fakeRuntime) Up(context.Context, domain.Stack) error           { return nil }
func (fakeRuntime) StopStack(context.Context, domain.Stack) error    { return nil }
func (fakeRuntime) RestartStack(context.Context, domain.Stack) error { return nil }
func (fakeRuntime) ComposeLogs(context.Context, domain.Stack, int) (ports.LogStream, error) {
	return ports.LogStream{}, nil
}

func fakeRuntimeSnapshot(service application.ContainerService) domain.Snapshot {
	snapshot, _ := service.Load(context.Background())
	return snapshot
}

func loadedSelectionModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(loadedMsg{snapshot: domain.Snapshot{Containers: []domain.Container{
		{ID: "one", Name: "one", State: "running"},
		{ID: "two", Name: "two", State: "running"},
	}}, generation: 1})
	return updated.(Model)
}

func loadedImageSelectionModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active = 2
	model.imagesLoaded = true
	model.imagesLoading = false
	model.images = []domain.Image{{ID: "one", Name: "one:latest"}, {ID: "two", Name: "two:latest"}}
	model.syncImageSelection()
	return model
}

func loadedNetworkSelectionModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.networksLoaded, model.networksLoading = 3, true, false
	model.networks = []domain.Network{{ID: "network", Name: "network"}}
	model.syncNetworkSelection()
	return model
}

func loadedVolumeSelectionModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active, model.volumesLoaded, model.volumesLoading = 4, true, false
	model.volumes = []domain.Volume{{Name: "volume"}}
	model.syncVolumeSelection()
	return model
}
