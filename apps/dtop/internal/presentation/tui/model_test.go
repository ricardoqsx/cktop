package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
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

func TestSpanishModelLocalizesShellHeaderTableAndTechnicalValues(t *testing.T) {
	model := NewModelWithUpdatesAndLocalizer(application.NewContainerService(fakeRuntime{}), config.Display{MemoryMode: config.MemoryBoth}, nil, nil, i18n.New("es"))
	model.loading, model.imagesLoading, model.networksLoading, model.volumesLoading, model.stacksLoading = false, false, false, false, false
	model.imagesLoaded, model.networksLoaded, model.volumesLoaded, model.stacksLoaded = true, true, true, true
	model.width, model.height = 160, 30
	model.snapshot = domain.Snapshot{
		Engine:       domain.EngineInfo{Name: "desktop-linux", ServerVersion: "29.6.2", MemoryTotal: 2 * 1024 * 1024 * 1024},
		Containers:   []domain.Container{{ID: "sha256:abcdef1234567890", ShortID: "abcdef123456", Name: "web", Image: "nginx:1.27", State: "running", Health: "healthy", CPUAvailable: true, CPUPercent: 12.5, MemoryAvailable: true, MemoryUsage: 512 * 1024 * 1024, MemoryLimit: 2 * 1024 * 1024 * 1024, MemoryPercent: 25.5}},
		CPUAvailable: true, ContainerCPUPercent: 12.5, MemoryAvailable: true, ContainerMemoryUsage: 512 * 1024 * 1024,
	}
	model.syncSelection()

	view := model.View()
	for _, expected := range []string{"Contenedores", "Imagenes", "Redes", "Volumenes", "Orden: Estado", "NOMBRE", "ESTADO", "SALUD", "12,5%", "25,5%", "en ejecucion", "saludable", "desktop-linux", "29.6.2", "nginx:1.27", "abcdef123456", "[x] avanzado"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected Spanish model view to contain %q, got %q", expected, view)
		}
	}
	if !strings.Contains(ansi.Strip(view), "en ejecucion  12,5%") {
		t.Fatalf("expected full Spanish state separated from CPU: %q", ansi.Strip(view))
	}
}

func TestSpanishModelLocalizesActionHelpAndPreservesRawErrors(t *testing.T) {
	model := NewModelWithUpdatesAndLocalizer(application.NewContainerService(fakeRuntime{}), config.Display{MemoryMode: config.MemoryBoth}, nil, nil, i18n.New("es"))
	model.loading = false
	model.width, model.height = 180, 30
	model.snapshot = domain.Snapshot{Engine: domain.EngineInfo{Name: "local"}, Containers: []domain.Container{{ID: "one", Name: "api", State: "running"}}}
	model.syncSelection()
	model.showHelp = true
	help := model.View()
	for _, expected := range []string{"Ayuda", "Conexion", "[Enter] acciones", "[x] avanza"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected Spanish help to contain %q, got %q", expected, help)
		}
	}

	model.showHelp = false
	model.action = actionState{stage: actionMenu, resource: actionContainers, targets: []actionTarget{{ID: "one", Name: "api"}}}
	menu := model.View()
	for _, expected := range []string{"Seleccion: 1 contenedor", "Detener", "Reiniciar", "Forzar eliminacion", "Cancelar", "Accion", "Controles"} {
		if !strings.Contains(menu, expected) {
			t.Fatalf("expected Spanish action menu to contain %q, got %q", expected, menu)
		}
	}

	model.action = actionState{}
	model.err = errors.New("raw daemon failure at unix:///tmp/docker.sock")
	errorView := model.View()
	if !strings.Contains(errorView, "raw daemon failure at unix:///tmp/docker.sock") || !strings.Contains(errorView, "dtop no pudo conectarse") {
		t.Fatalf("expected translated wrapper and unchanged raw error, got %q", errorView)
	}
}

func TestEnglishDefaultAndLocalizedUsagePlurals(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	if got := model.localizer.Text(i18n.MessageTabContainers); got != "Containers" {
		t.Fatalf("default constructor locale = %q, want English", got)
	}

	for _, test := range []struct {
		count int
		want  string
	}{{0, "0 contenedores"}, {1, "1 contenedor"}, {3, "3 contenedores"}} {
		if got := resourceUsageLocalized(test.count, true, i18n.New("es")); got != test.want {
			t.Errorf("Spanish usage for %d = %q, want %q", test.count, got, test.want)
		}
	}
}

func TestSpanishStackMetadataAndDiagnosticsHeadingPreservePathsAndBody(t *testing.T) {
	diagnostic := `config /tmp/dtop.conf:7: raw diagnostic body`
	model := NewModelWithUpdatesAndLocalizer(application.NewContainerService(fakeRuntime{}), config.Display{MemoryMode: config.MemoryBoth}, nil, nil, i18n.New("es"), diagnostic)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.width, model.height = 140, 30
	model.stacks = []domain.Stack{{Name: "payments", State: "down", WorkingDir: "/srv/payments", Files: []string{"/srv/payments/compose.prod.yaml"}}}
	model.syncStackSelection()

	view := model.View()
	for _, expected := range []string{"Stack seleccionado", "Directorio de trabajo: /srv/payments", "Archivos Compose: /srv/payments/compose.prod.yaml", "Down: disponible", "Diagnosticos de registro", diagnostic} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected Spanish stack view to contain %q, got %q", expected, view)
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
		Images:   []domain.Image{{ID: "image", Name: "image:latest"}},
		Networks: []domain.Network{{ID: "network", Name: "network"}},
		Volumes:  []domain.Volume{{Name: "volume"}},
	}})
	loaded := updated.(Model)
	if !loaded.imagesLoaded || !loaded.networksLoaded || !loaded.volumesLoaded {
		t.Fatalf("expected all resource tabs loaded, got %#v", loaded)
	}
	if loaded.selectedImageID != "image" || loaded.selectedNetworkID != "network" || loaded.selectedVolumeName != "volume" {
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

func TestResourceReconciliationRemovesDeletedResourcesAndKeepsValidSelections(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.selectedImageID, model.selectedNetworkID, model.selectedVolumeName = "image-gone", "network-live", "volume-gone"
	model.selectedImages["image-gone"] = struct{}{}
	model.selectedNetworks["network-live"] = struct{}{}
	model.selectedVolumes["volume-gone"] = struct{}{}
	model.imageDetailOpen = true

	updated, _ := model.Update(resourcesLoadedMsg{resources: ports.ResourceLoad{
		Images:   []domain.Image{{ID: "image-live", Containers: 1, UsageKnown: true}, {ID: "image-gone", Containers: 1, UsageKnown: true}},
		Networks: []domain.Network{{ID: "network-live", Containers: 1, UsageKnown: true}, {ID: "network-gone", Containers: 1, UsageKnown: true}},
		Volumes:  []domain.Volume{{Name: "volume-live", Containers: 1, UsageKnown: true}, {Name: "volume-gone", Containers: 1, UsageKnown: true}},
	}})
	loaded := updated.(Model)
	updated, _ = loaded.Update(resourcesLoadedMsg{resources: ports.ResourceLoad{
		Images:   []domain.Image{{ID: "image-live", Containers: 0, UsageKnown: true}},
		Networks: []domain.Network{{ID: "network-live", Containers: 0, UsageKnown: true}},
		Volumes:  []domain.Volume{{Name: "volume-live", Containers: 0, UsageKnown: true}},
	}})
	reconciled := updated.(Model)
	if len(reconciled.images) != 1 || len(reconciled.networks) != 1 || len(reconciled.volumes) != 1 || reconciled.images[0].Containers != 0 || reconciled.networks[0].Containers != 0 || reconciled.volumes[0].Containers != 0 {
		t.Fatalf("expected deleted resources removed and usage recalculated, got images=%#v networks=%#v volumes=%#v", reconciled.images, reconciled.networks, reconciled.volumes)
	}
	if reconciled.selectedImageID != "image-live" || reconciled.selectedNetworkID != "network-live" || reconciled.selectedVolumeName != "volume-live" || reconciled.imageDetailOpen {
		t.Fatalf("expected only live selections and no stale detail panel, got %#v", reconciled)
	}
	if len(reconciled.selectedImages) != 0 || len(reconciled.selectedNetworks) != 1 || len(reconciled.selectedVolumes) != 0 {
		t.Fatalf("expected removed multi-selections cleared, got images=%v networks=%v volumes=%v", reconciled.selectedImages, reconciled.selectedNetworks, reconciled.selectedVolumes)
	}
}

func TestResourceRefreshDoesNotOverlap(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(resourcesLoadedMsg{generation: model.resourcesGen})
	ready := updated.(Model)
	updated, command := ready.Update(resourceRefreshMsg(time.Now()))
	loading := updated.(Model)
	if command == nil || !loading.resourcesLoading || loading.resourcesGen != ready.resourcesGen+1 {
		t.Fatalf("expected one aggregate reconciliation command, got %#v", loading)
	}
	updated, second := loading.Update(resourceRefreshMsg(time.Now()))
	if second != nil || updated.(Model).resourcesGen != loading.resourcesGen {
		t.Fatal("resource reconciliation must not overlap")
	}
}

func TestManualResourceLoadWinsOverStaleAggregateResult(t *testing.T) {
	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	model.active = 2
	updated, _ := model.Update(resourcesLoadedMsg{generation: model.resourcesGen, imagesGen: model.imagesGen, networksGen: model.networksGen, volumesGen: model.volumesGen, resources: ports.ResourceLoad{Images: []domain.Image{{ID: "current"}}}})
	loaded := updated.(Model)
	updated, _ = loaded.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	manual := updated.(Model)
	if manual.imagesGen == loaded.imagesGen {
		t.Fatal("expected manual image refresh generation")
	}
	updated, _ = manual.Update(resourcesLoadedMsg{generation: manual.resourcesGen, imagesGen: loaded.imagesGen, networksGen: loaded.networksGen, volumesGen: loaded.volumesGen, resources: ports.ResourceLoad{Images: []domain.Image{{ID: "stale"}}}})
	if got := updated.(Model).images[0].ID; got != "current" {
		t.Fatalf("stale aggregate replaced manual image state with %q", got)
	}
}

func TestPartialResourceFailureKeepsLastKnownRows(t *testing.T) {
	model := loadedImageSelectionModel(t)
	updated, _ := model.Update(resourcesLoadedMsg{imagesGen: model.imagesGen, resources: ports.ResourceLoad{ImagesErr: errors.New("images unavailable")}})
	failed := updated.(Model)
	if len(failed.images) != 2 || failed.imagesErr == nil || !strings.Contains(failed.imagesView(sharedui.ResolveLayout(100, 20)).Summary, "Showing last known") {
		t.Fatalf("expected visible partial image state, got %#v", failed)
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

func TestImageUpdateColumnShowsAllReadOnlyStatuses(t *testing.T) {
	layout := sharedui.ResolveLayout(100, 20)
	images := []domain.Image{{ID: "available", Name: "available:1", Update: domain.UpdateAvailable}, {ID: "current", Name: "current:1", Update: domain.UpdateCurrent}, {ID: "pinned", Name: "pinned:1", Update: domain.UpdatePinned}, {ID: "checking", Name: "checking:1", Update: domain.UpdateChecking}, {ID: "unknown", Name: "unknown:1", Update: domain.UpdateUnknown}, {ID: "recreate", Name: "recreate:1", Update: domain.UpdatePulledPendingRecreate}}
	view := renderImages(images, "available", nil, false, layout, time.Now())
	if !strings.Contains(view, "UPDATE") {
		t.Fatalf("missing update header: %q", view)
	}
	for _, indicator := range []string{"U", "=", "P", "...", "?", "R"} {
		if !strings.Contains(view, indicator) {
			t.Fatalf("missing update indicator %q in %q", indicator, view)
		}
	}
}

func TestCancelImageUpdateScanAllowsRescanAfterCompletedScan(t *testing.T) {
	model := Model{updatesStarted: true}

	model.cancelImageUpdateScan()

	if model.updatesStarted {
		t.Fatal("completed scan must be invalidated when running image references change")
	}
	if model.updatesRunning || model.updatesCancel != nil {
		t.Fatalf("scan state was not cleared: %#v", model)
	}
	if model.updatesGeneration != 1 {
		t.Fatalf("generation = %d, want 1", model.updatesGeneration)
	}
}

func TestReconcileImagesPreservesUpdateStatusWhenDigestsAreUnchanged(t *testing.T) {
	model := Model{images: []domain.Image{{ID: "sha256:image", RepoDigests: []string{"example.com/app@sha256:local"}, Update: domain.UpdateAvailable}}}

	model.reconcileImages([]domain.Image{{ID: "sha256:image", RepoDigests: []string{"example.com/app@sha256:local"}}}, nil)

	if got := model.images[0].Update; got != domain.UpdateAvailable {
		t.Fatalf("update status = %q, want %q", got, domain.UpdateAvailable)
	}
}

func TestCompletedScanNeverLeavesCheckedImageInChecking(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.updates = testImageUpdateService()
	model.loading = false
	model.resourcesLoading = false
	model.images[0].Update = domain.UpdateChecking
	model.updatesGeneration = 2
	model.updatesRunning = true
	model.updatesChecking = map[string]domain.UpdateStatus{"one": domain.UpdateAvailable}

	updated, _ := model.Update(imageUpdatesLoadedMsg{generation: 2})
	if got := updated.(Model).images[0].Update; got != domain.UpdateUnknown {
		t.Fatalf("empty terminal scan left status %q, want unknown", got)
	}
}

func TestCancelScanRestoresStatusInsteadOfLeavingChecking(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.images[0].Update = domain.UpdateChecking
	model.updatesChecking = map[string]domain.UpdateStatus{"one": domain.UpdateAvailable}
	model.updatesRunning = true

	model.cancelImageUpdateScan()
	if got := model.images[0].Update; got != domain.UpdateAvailable {
		t.Fatalf("cancel restored %q, want available", got)
	}
}

func TestPendingRecreateSurvivesReconcileAndLateScan(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.images[0].RepoDigests = []string{"one@sha256:old"}
	model.pendingRecreates["container"] = pendingImageRecreate{ContainerID: "container", ImageID: "one", Reference: "one:latest"}
	model.applyPendingImageStatuses()

	model.reconcileImages([]domain.Image{{ID: "one", Name: "one:old", RepoDigests: []string{"one@sha256:old"}}, {ID: "new", Name: "one:latest", RepoDigests: []string{"one@sha256:new"}}}, nil)
	model.updatesGeneration = 3
	updated, _ := model.Update(imageUpdatesLoadedMsg{generation: 3, updates: []domain.ImageUpdate{{ImageID: "one", Status: domain.UpdateAvailable}}})
	if got := updated.(Model).images[0].Update; got != domain.UpdatePulledPendingRecreate {
		t.Fatalf("late scan replaced pending recreate with %q", got)
	}
}

func TestScanResultReconstructsPendingRecreateAfterRestart(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.updates = testImageUpdateService()
	model.snapshot.Containers = []domain.Container{{ID: "container", Image: "one:latest", ImageID: "sha256:one", State: "running"}}
	model.updatesGeneration = 4

	updated, _ := model.Update(imageUpdatesLoadedMsg{generation: 4, updates: []domain.ImageUpdate{{ContainerID: "container", ImageID: "one", Reference: "one:latest", Status: domain.UpdatePulledPendingRecreate}}})
	got := updated.(Model)
	if got.images[0].Update != domain.UpdatePulledPendingRecreate {
		t.Fatalf("scan status = %q, want pending recreate", got.images[0].Update)
	}
	if pending, found := got.pendingRecreates["container"]; !found || pending.Reference != "one:latest" {
		t.Fatalf("scan did not reconstruct pending state: %#v", got.pendingRecreates)
	}
}

func TestSuccessfulPullTransitionsAvailableImageToPendingRecreate(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.updates = testImageUpdateService()
	model.loading = false
	model.resourcesLoading = false
	model.images[0].Update = domain.UpdateAvailable
	model.snapshot.Containers = []domain.Container{{ID: "container", Image: "one:latest", ImageID: "sha256:one", State: "running"}}
	targets := model.selectedImageTargetsForIDs([]string{"one"})

	model.markPulledImages(targets, []application.ActionResult{{ID: "one:latest", Action: application.ActionPull}})
	if got := model.images[0].Update; got != domain.UpdatePulledPendingRecreate {
		t.Fatalf("successful pull status = %q, want pending recreate", got)
	}
	if pending, found := model.pendingRecreates["container"]; !found || pending.Reference != "one:latest" || pending.ImageID != "sha256:one" {
		t.Fatalf("missing durable pending recreate: %#v", model.pendingRecreates)
	}

	model.snapshot.Containers[0].Image = "sha256:one"
	model.updatesStarted = false
	command := model.startImageUpdateScan()
	if command == nil {
		t.Fatal("expected terminal scan command")
	}
	if got := model.images[0].Update; got != domain.UpdatePulledPendingRecreate {
		t.Fatalf("scan replaced pending status with %q", got)
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

	if !strings.Contains(view, "very-long-container-...") || strings.Contains(view, "very-long-container-name-that-should-truncate") {
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

func TestDeleteConfirmationUsesY(t *testing.T) {
	model := loadedSelectionModel(t)
	model.action = actionState{
		stage:   actionConfirm,
		index:   2,
		targets: []actionTarget{{ID: "one", Name: "one", State: "running"}},
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("enter must not confirm deletion")
	}

	confirmed, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil || !confirmed.(Model).action.running {
		t.Fatal("y must confirm deletion")
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
	if result.action.stage != actionNone || result.editing || len(result.selected) != 0 || result.notice == "" {
		t.Fatalf("expected cleared selection and timed result notice, got %#v", result)
	}
}

func TestFocusedRowsAndActionMenuUseFullWidthANSIHighlights(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	layout := sharedui.ResolveLayout(80, 20)
	row := renderContainersWithColors([]domain.Container{{ID: "one", Name: "one", State: "running"}}, "one", nil, false, time.Now(), layout, config.MemoryBoth, "63", "15")
	lines := strings.Split(row, "\n")
	if !strings.Contains(lines[2], "\x1b[") || ansi.StringWidth(lines[2]) != tableWidth(containerColumns(layout.ContentWidth, config.MemoryBoth, false)) {
		t.Fatalf("expected full-width focused row, got %q", lines[2])
	}
	model := loadedSelectionModel(t)
	model.width, model.height = 80, 20
	model.action = actionState{stage: actionMenu, targets: []actionTarget{{ID: "one", Name: "one"}}}
	menu := model.actionMenuView().Sections[1].Body
	if !strings.Contains(menu, "\x1b[") || ansi.StringWidth(strings.Split(menu, "\n")[0]) != sharedui.ResolveLayout(80, 20).ContentWidth {
		t.Fatalf("expected full-width active menu row, got %q", menu)
	}
}

func TestConfirmationPersistsAndYNAreRequired(t *testing.T) {
	model := loadedSelectionModel(t)
	model.action = actionState{stage: actionMenu, targets: []actionTarget{{ID: "one", Name: "one"}}}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmation := updated.(Model)
	if command != nil || confirmation.action.stage != actionConfirm || confirmation.confirmationBanner() == "" {
		t.Fatalf("expected persistent confirmation, got %#v", confirmation)
	}
	updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("enter must not confirm a normal action")
	}
	updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil || !updated.(Model).action.running {
		t.Fatal("y must confirm a normal action")
	}
	updated, _ = confirmation.Update(actionNoticeExpiredMsg{generation: confirmation.noticeGeneration})
	if updated.(Model).action.stage != actionConfirm {
		t.Fatal("confirmation must not expire")
	}
}

func TestConfirmationBannerHighlightsEveryActionAndKeepsControlsVisible(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	for _, resource := range []actionResource{actionContainers, actionImages, actionNetworks, actionVolumes, actionStacks, actionStackContainers} {
		model := loadedSelectionModel(t)
		model.width, model.height = 48, 20
		model.action = actionState{stage: actionConfirm, resource: resource, targets: []actionTarget{{ID: "one", Name: "very-long-target-name-for-confirmation"}}}
		if resource == actionImages {
			model.action.choices = []application.Action{application.ActionPull}
		}
		banner := model.confirmationBanner()
		if !strings.Contains(banner, "CONFIRM:") || !strings.Contains(banner, "[Esc] cancel") {
			t.Fatalf("resource %d banner missing confirmation controls: %q", resource, banner)
		}
		view := model.View()
		if !strings.Contains(view, "CONFIRM:") || !strings.Contains(view, "\x1b[") {
			t.Fatalf("resource %d confirmation was not visibly highlighted: %q", resource, view)
		}
	}
}

func TestConfirmationBannerIsHiddenOutsideConfirmation(t *testing.T) {
	model := loadedSelectionModel(t)
	model.action = actionState{stage: actionMenu, targets: []actionTarget{{ID: "one", Name: "one"}}}
	if banner := model.confirmationBanner(); banner != "" {
		t.Fatalf("unexpected banner outside confirmation: %q", banner)
	}
}

func TestAdvancedMenuRequiresTypedPruneAndRefreshesResources(t *testing.T) {
	var pruneArgs []string
	model := NewModel(application.NewContainerService(fakeRuntime{
		pruneArgs:   &pruneArgs,
		pruneOutput: "Deleted Images: 2",
	}), config.MemoryBoth)
	model.loading = false
	model.width, model.height = 100, 30
	if !strings.Contains(model.View(), "[x] advanced") {
		t.Fatalf("normal navigation must expose Advanced: %q", model.View())
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	advanced := updated.(Model)
	if command != nil || advanced.advanced.stage != advancedMenu {
		t.Fatalf("expected Advanced menu, got stage=%d command=%v", advanced.advanced.stage, command)
	}
	for _, expected := range []string{"Advanced", "Delete stopped containers", "Delete unused images", "Delete unused networks", "Delete unused volumes", "Delete unused Docker data", "Cancel", "Command: [docker container prune --force]"} {
		if !strings.Contains(advanced.View(), expected) {
			t.Fatalf("Advanced menu missing %q: %q", expected, advanced.View())
		}
	}

	updated, _ = advanced.Update(tea.KeyMsg{Type: tea.KeyDown})
	advanced = updated.(Model)
	if !strings.Contains(advanced.View(), "Command: [docker image prune --all --force]") {
		t.Fatalf("expected focused image command: %q", advanced.View())
	}
	updated, _ = advanced.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmation := updated.(Model)
	if confirmation.advanced.stage != advancedConfirm || !strings.Contains(confirmation.View(), "Type [prune]") {
		t.Fatalf("expected typed confirmation: %q", confirmation.View())
	}
	updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).advanced.stage != advancedConfirm {
		t.Fatal("empty confirmation must not run prune")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("PRUNE")})
	updated, command = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).advanced.stage != advancedConfirm {
		t.Fatal("confirmation must require exact lowercase prune")
	}
	confirmation = updated.(Model)
	confirmation.advanced.input = ""
	updated, _ = confirmation.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("prune-extra")})
	updated, command = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).advanced.stage != advancedConfirm {
		t.Fatal("confirmation must reject additional input")
	}
	confirmation = updated.(Model)
	confirmation.advanced.input = ""
	updated = confirmation
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("prune")})
	updated, command = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	running := updated.(Model)
	if command == nil || running.advanced.stage != advancedRunning {
		t.Fatal("exact prune confirmation must start cleanup")
	}

	message := command()
	finished, ok := message.(advancedFinishedMsg)
	if !ok {
		t.Fatalf("prune command returned %T", message)
	}
	updated, refresh := running.Update(finished)
	result := updated.(Model)
	if !reflect.DeepEqual(pruneArgs, []string{"image", "prune", "--all", "--force"}) {
		t.Fatalf("prune args = %v", pruneArgs)
	}
	if result.advanced.stage != advancedResult || refresh == nil || !result.refreshing || !result.resourcesLoading {
		t.Fatalf("expected persistent result and resource refresh, got %#v", result.advanced)
	}
	if view := result.View(); !strings.Contains(view, "Deleted Images: 2") || !strings.Contains(view, "[Enter/Esc] close") {
		t.Fatalf("expected prune result output: %q", view)
	}
}

func TestAdvancedMenuIsGlobalUsesSafeSystemCommandAndCancelHasNoCommand(t *testing.T) {
	for active := 0; active < 5; active++ {
		model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
		model.active = active
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		if updated.(Model).advanced.stage != advancedMenu {
			t.Fatalf("tab %d did not open Advanced", active)
		}
	}

	model := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	advanced := updated.(Model)
	for range 4 {
		updated, _ = advanced.Update(tea.KeyMsg{Type: tea.KeyDown})
		advanced = updated.(Model)
	}
	view := advanced.View()
	if !strings.Contains(view, "Command: [docker system prune --all --force]") || strings.Contains(view, "--volumes") {
		t.Fatalf("unsafe system command: %q", view)
	}
	updated, _ = advanced.Update(tea.KeyMsg{Type: tea.KeyDown})
	cancel := updated.(Model)
	if !strings.Contains(cancel.View(), "Command: [-]") {
		t.Fatalf("cancel must not show an executable command: %q", cancel.View())
	}
	updated, command := cancel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).advanced.stage != advancedClosed {
		t.Fatal("Cancel must close Advanced without a command")
	}
}

func TestAdvancedDoesNotOpenOverContextualViews(t *testing.T) {
	model := loadedSelectionModel(t)
	model.panel = panelDetails
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if updated.(Model).advanced.stage != advancedClosed {
		t.Fatal("Advanced opened over container details")
	}

	model.panel = panelContainers
	model.action.stage = actionMenu
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if updated.(Model).advanced.stage != advancedClosed {
		t.Fatal("Advanced opened over an action menu")
	}

	model.action = actionState{}
	model.showHelp = true
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if updated.(Model).advanced.stage != advancedClosed {
		t.Fatal("Advanced opened over Help")
	}
}

func TestHelpShowsDockerHubLoginGuidanceUntilConfigured(t *testing.T) {
	model := loadedSelectionModel(t)
	model.showHelp = true
	model.dockerHubLoginChecked = true
	help := model.containersView(sharedui.ResolveLayout(100, 30))
	if !strings.Contains(help.Sections[len(help.Sections)-1].Body, "log in to Docker Hub") {
		t.Fatalf("missing Docker Hub login guidance: %#v", help.Sections)
	}

	model.dockerHubLoginConfigured = true
	help = model.containersView(sharedui.ResolveLayout(100, 30))
	for _, section := range help.Sections {
		if strings.Contains(section.Body, "log in to Docker Hub") {
			t.Fatalf("login guidance remained after configured session: %#v", help.Sections)
		}
	}
}

func TestDockerHubUpdateErrorRestoresLoginGuidance(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.updates = testImageUpdateService()
	model.dockerHubLoginChecked = true
	model.dockerHubLoginConfigured = true
	model.updatesGeneration = 2

	updated, _ := model.Update(imageUpdatesLoadedMsg{generation: 2, updates: []domain.ImageUpdate{{ImageID: "one", Status: domain.UpdateUnknown, Reason: application.DockerHubLoginRequiredReason}}})
	got := updated.(Model)
	if got.dockerHubLoginConfigured {
		t.Fatal("Docker Hub access error did not restore login guidance")
	}
}

func TestDockerHubFooterNoticeTracksLoginState(t *testing.T) {
	model := loadedSelectionModel(t)
	model.dockerHubLoginChecked = true
	if !strings.Contains(model.dockerHubFooterNotice(), "docker login") {
		t.Fatal("missing Docker Hub notice on main screen")
	}
	model.dockerHubLoginConfigured = true
	if notice := model.dockerHubFooterNotice(); notice != "" {
		t.Fatalf("unexpected notice with configured login: %q", notice)
	}
}

func TestPullActionRunsWithoutConfirmation(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.action = actionState{stage: actionMenu, resource: actionImages, targets: []actionTarget{{ID: "one", Name: "one:latest", PullRefs: []string{"one:latest"}}}, choices: []application.Action{application.ActionPull, application.ActionDelete, "cancel"}}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !updated.(Model).action.running || updated.(Model).action.stage != actionMenu {
		t.Fatalf("pull did not run directly: %#v", updated.(Model).action)
	}
}

func TestConfirmationTimerDoesNotClearRunningAction(t *testing.T) {
	model := loadedSelectionModel(t)
	model.action = actionState{stage: actionConfirm, resource: actionContainers, running: true}
	model.notice = "running"
	model.noticeGeneration = 4

	updated, _ := model.Update(actionNoticeExpiredMsg{generation: 4})
	got := updated.(Model)
	if !got.action.running || got.action.stage != actionConfirm || got.notice == "" {
		t.Fatalf("timer cleared running action: %#v", got.action)
	}
}

func TestResultNoticeExpiresWithoutResultScreen(t *testing.T) {
	model := loadedSelectionModel(t)
	model.showActionResult([]application.ActionResult{{ID: "one"}})
	if model.action.stage != actionNone || model.notice == "" {
		t.Fatalf("expected table state with result notice, got %#v", model)
	}
	updated, _ := model.Update(actionNoticeExpiredMsg{generation: model.noticeGeneration})
	if updated.(Model).notice != "" {
		t.Fatal("expired result notice must clear")
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

func TestContainerRowsShowOnlyActionableUpdateIndicators(t *testing.T) {
	layout := sharedui.ResolveLayout(80, 20)
	view := renderContainers([]domain.Container{
		{ID: "available", Name: "web", State: "running", Update: domain.UpdateAvailable},
		{ID: "pending", Name: "worker", State: "running", Update: domain.UpdatePulledPendingRecreate},
		{ID: "current", Name: "db", State: "running", Update: domain.UpdateCurrent},
	}, "available", nil, false, time.Now(), layout, config.MemoryBoth)
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "U  web") || !strings.Contains(plain, "R  worker") {
		t.Fatalf("missing container update indicators: %q", plain)
	}
	if strings.Contains(plain, "=  db") {
		t.Fatalf("current container must keep the update marker empty: %q", plain)
	}
}

func TestContainerUpdateActionsSupportMixedMultipleSelection(t *testing.T) {
	model := loadedSelectionModel(t)
	model.images = []domain.Image{{ID: "one-image", Update: domain.UpdateAvailable}, {ID: "two-image", Update: domain.UpdateCurrent}}
	model.snapshot.Containers[0].Image, model.snapshot.Containers[0].ImageID = "one:latest", "one-image"
	model.snapshot.Containers[1].Image, model.snapshot.Containers[1].ImageID = "two:latest", "two-image"
	model.containerUpdates["one"], model.containerUpdates["two"] = domain.UpdateAvailable, domain.UpdateCurrent
	model.applyContainerUpdateStatuses()
	model.selected = map[string]struct{}{"one": {}, "two": {}}
	model.editing = true

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if got, want := fmt.Sprint(menu.actionChoices()), "[pull update stop restart delete cancel]"; got != want {
		t.Fatalf("mixed container actions = %s, want %s", got, want)
	}
	view := menu.actionMenuView()
	found := false
	for _, section := range view.Sections {
		found = found || strings.Contains(section.Body, "1 eligible, 1 skipped")
	}
	if !found {
		t.Fatalf("mixed menu did not explain eligibility: %#v", view.Sections)
	}
}

func TestFailedContainerUpdateAfterPullRemainsPendingApply(t *testing.T) {
	model := loadedSelectionModel(t)
	model.updates = testImageUpdateService()
	model.snapshot.Containers[0].Image, model.snapshot.Containers[0].ImageID, model.snapshot.Containers[0].Update = "one:latest", "one-image", domain.UpdateAvailable
	targets := model.containerTargetsForIDs([]string{"one"})

	model.reconcileContainerUpdateResults(application.ActionUpdate, targets, []application.ActionResult{{ID: "one", Action: application.ActionUpdate, Err: errors.New("recreate failed"), Pulled: true}})
	if pending, found := model.pendingRecreates["one"]; !found || pending.Reference != "one:latest" {
		t.Fatalf("failed recreate did not preserve pending update: %#v", model.pendingRecreates)
	}
	menu := actionState{stage: actionMenu, resource: actionContainers, targets: model.containerTargetsForIDs([]string{"one"})}
	model.action = menu
	if got, want := fmt.Sprint(model.actionChoices()), "[apply_update stop restart delete cancel]"; got != want {
		t.Fatalf("pending container actions = %s, want %s", got, want)
	}
}

func TestComposeChildUpdateDelegatesOnceToStack(t *testing.T) {
	pullCalls, upCalls := 0, 0
	runtime := fakeRuntime{
		pullStackCalls: &pullCalls,
		upCalls:        &upCalls,
		composeConfig:  []ports.ComposeServiceImage{{Service: "web", Reference: "web:latest"}, {Service: "worker", Reference: "worker:latest"}},
		images: []domain.Image{
			{ID: "sha256:web-new", Tags: []string{"web:latest"}, RepoDigests: []string{"docker.io/library/web@sha256:web"}},
			{ID: "sha256:worker-new", Tags: []string{"worker:latest"}, RepoDigests: []string{"docker.io/library/worker@sha256:worker"}},
		},
		snapshot: domain.Snapshot{Containers: []domain.Container{
			{ComposeProject: "app", ComposeService: "web", ImageID: "sha256:web-new"},
			{ComposeProject: "app", ComposeService: "worker", ImageID: "sha256:worker-new"},
		}},
	}
	model := NewModel(application.NewContainerService(runtime), config.MemoryBoth)
	stack := domain.Stack{Name: "app", Registered: true, WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}, ContainerItems: []domain.Container{
		{ID: "web", Name: "web", ComposeProject: "app", ImageID: "image", Update: domain.UpdateAvailable},
		{ID: "worker", Name: "worker", ComposeProject: "app", ImageID: "image", Update: domain.UpdateAvailable},
	}}
	stackTarget := stack
	targets := []actionTarget{{ID: "web", Name: "web", Update: domain.UpdateAvailable, PullRefs: []string{"web:latest"}, Stack: &stackTarget, Service: "web"}, {ID: "worker", Name: "worker", Update: domain.UpdateAvailable, PullRefs: []string{"worker:latest"}, Stack: &stackTarget, Service: "worker"}}

	results := model.runComposeContainerUpdate(context.Background(), application.ActionUpdate, targets)
	if len(results) != 1 || results[0].ID != "app" || pullCalls != 1 || upCalls != 1 {
		t.Fatalf("Compose update was not deduplicated by stack: results=%#v pull=%d up=%d", results, pullCalls, upCalls)
	}
}

func TestComposeContainerUpdateConfirmationNamesStackOnce(t *testing.T) {
	model := loadedSelectionModel(t)
	stack := domain.Stack{Name: "app", Registered: true, WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	model.action = actionState{stage: actionConfirm, resource: actionContainers, index: 1, targets: []actionTarget{
		{ID: "web", Name: "web-1", Update: domain.UpdateAvailable, Stack: &stack, Service: "web"},
		{ID: "worker", Name: "worker-1", Update: domain.UpdateAvailable, Stack: &stack, Service: "worker"},
	}}
	banner := model.confirmationBanner()
	if !strings.Contains(banner, "Target: app/web, app/worker on") || strings.Contains(banner, "web-1") || strings.Contains(banner, "worker-1") {
		t.Fatalf("Compose confirmation did not identify stack scope: %q", banner)
	}
}

func TestPersistedComposePullReplacesDownStackUpWithConfirmedApply(t *testing.T) {
	store := &tuiComposeUpdateStore{projects: make(map[string]domain.ComposeUpdateProject)}
	runtime := fakeRuntime{
		composeConfig: []ports.ComposeServiceImage{{Service: "web", Reference: "app:latest"}},
		images:        []domain.Image{{ID: "sha256:new-image", Tags: []string{"app:latest"}, RepoDigests: []string{"docker.io/library/app@sha256:new"}}},
	}
	project := application.ComposeProject{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	stack := domain.Stack{Name: "app", Registered: true, State: "running", WorkingDir: project.WorkingDir, Files: project.Files}
	service := application.NewContainerServiceWithComposeUpdates(runtime, store, project)
	if result := service.ActStacks(context.Background(), application.ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatalf("pull: %v", result.Err)
	}

	restarted := application.NewContainerServiceWithComposeUpdates(runtime, store, project)
	resources, err := restarted.LoadResources(context.Background())
	if err != nil || len(resources.Stacks) != 1 {
		t.Fatalf("load resources: %#v, %v", resources, err)
	}
	down := resources.Stacks[0]
	if down.State != "down" || !down.UpdatePending || down.UpdateUnknown || down.Update != domain.UpdatePulledPendingRecreate {
		t.Fatalf("persisted down stack = %#v", down)
	}
	targets := []actionTarget{{ID: down.Name, Name: down.Name, State: down.State, Update: stackUpdateStatus(down), UpdatePending: down.UpdatePending, Stack: &down}}
	if got, want := fmt.Sprint(stackActionChoices(targets)), "[apply_update cancel]"; got != want {
		t.Fatalf("pending Down choices = %s, want %s", got, want)
	}

	model := NewModel(restarted, config.MemoryBoth)
	model.active, model.stacksLoaded, model.stacksLoading = 1, true, false
	model.stacks = resources.Stacks
	model.syncStackSelection()
	model.action = actionState{stage: actionMenu, resource: actionStacks, targets: targets, choices: stackActionChoices(targets)}
	if view := model.View(); !strings.Contains(view, "Apply downloaded update") || strings.Contains(view, "Up stack") {
		t.Fatalf("pending Down menu = %q", view)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmation := updated.(Model)
	if confirmation.action.stage != actionConfirm || !strings.Contains(confirmation.confirmationBanner(), "CONFIRM: Apply downloaded update") {
		t.Fatalf("Apply did not require confirmation: %q", confirmation.confirmationBanner())
	}
	before, _ := store.Get("app")
	updated, command := confirmation.Update(tea.KeyMsg{Type: tea.KeyEsc})
	after, _ := store.Get("app")
	if command != nil || updated.(Model).action.stage != actionNone || !reflect.DeepEqual(before, after) {
		t.Fatal("cancelling Apply mutated persistent state")
	}
}

func TestActionResultShowsNonFatalComposeStateWarning(t *testing.T) {
	model := loadedSelectionModel(t)
	model.showActionResult([]application.ActionResult{{ID: "app", Action: application.ActionPull, Warning: errors.New("state disk is full"), Pulled: true}})
	if !strings.Contains(model.notice, "1 action completed") || !strings.Contains(model.notice, "warning: state disk is full") {
		t.Fatalf("warning notice = %q", model.notice)
	}
}

func TestUnpersistedComposePulledStatusOffersPullButNotApply(t *testing.T) {
	stack := domain.Stack{Name: "app", Registered: true, WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	model := loadedSelectionModel(t)
	model.action = actionState{stage: actionMenu, resource: actionContainers, targets: []actionTarget{{ID: "web", Update: domain.UpdatePulledPendingRecreate, Stack: &stack, Service: "web", PullRefs: []string{"app:latest"}}}}
	if got, want := fmt.Sprint(model.actionChoices()), "[pull update stop restart delete cancel]"; got != want {
		t.Fatalf("unpersisted Compose status actions = %s, want %s", got, want)
	}
}

func TestUnpersistedStackPulledStatusOffersPullButNotApply(t *testing.T) {
	stack := domain.Stack{Name: "app", Registered: true, State: "running", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	targets := []actionTarget{{ID: "app", State: "running", Update: domain.UpdatePulledPendingRecreate, Stack: &stack}}
	if got, want := fmt.Sprint(stackActionChoices(targets)), "[pull update stop restart down cancel]"; got != want {
		t.Fatalf("unpersisted Stack status actions = %s, want %s", got, want)
	}
}

func TestComposeOneOffDoesNotOfferManagedServiceUpdate(t *testing.T) {
	stack := domain.Stack{Name: "app", Registered: true, WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	model := loadedSelectionModel(t)
	model.snapshot.Containers = []domain.Container{{ID: "run", Name: "run", Image: "app:latest", ComposeProject: "app", ComposeService: "web", ComposeOneOff: true, Update: domain.UpdateAvailable}}
	model.stacks = []domain.Stack{stack}
	target := model.containerActionTarget(model.snapshot.Containers[0])
	model.action = actionState{stage: actionMenu, resource: actionContainers, targets: []actionTarget{target}}
	if got, want := fmt.Sprint(model.actionChoices()), "[stop restart delete cancel]"; got != want {
		t.Fatalf("one-off actions = %s, want %s", got, want)
	}
}

func TestSuccessfulComposeServiceUpdateKeepsOtherServicePending(t *testing.T) {
	model := loadedSelectionModel(t)
	stack := domain.Stack{Name: "app", ContainerItems: []domain.Container{
		{ID: "web", ComposeService: "web", Image: "web:latest", ImageID: "web-image", Update: domain.UpdateAvailable},
		{ID: "worker", ComposeService: "worker", Image: "worker:latest", ImageID: "worker-image", Update: domain.UpdatePulledPendingRecreate},
	}}
	model.pendingRecreates["worker"] = pendingImageRecreate{ContainerID: "worker", ImageID: "worker-image", Reference: "worker:latest", Compose: true}
	target := actionTarget{ID: "web", Name: "web", Update: domain.UpdateAvailable, Stack: &stack, Service: "web"}

	model.reconcileContainerUpdateResults(application.ActionUpdate, []actionTarget{target}, []application.ActionResult{{ID: "app", Action: application.ActionUpdate, Pulled: true, Applied: true}})
	if _, found := model.pendingRecreates["worker"]; !found {
		t.Fatalf("service-scoped update cleared another service pending state: %#v", model.pendingRecreates)
	}
}

func TestMixedStackUpdateExecutesOnlyEligibleStacks(t *testing.T) {
	pullCalls, upCalls := 0, 0
	model := NewModel(application.NewContainerService(fakeRuntime{
		pullStackCalls: &pullCalls,
		upCalls:        &upCalls,
		composeConfig:  []ports.ComposeServiceImage{{Service: "web", Reference: "app:latest"}},
		images:         []domain.Image{{ID: "sha256:new", Tags: []string{"app:latest"}, RepoDigests: []string{"docker.io/library/app@sha256:new"}}},
		snapshot:       domain.Snapshot{Containers: []domain.Container{{ComposeProject: "eligible", ComposeService: "web", ImageID: "sha256:new"}}},
	}), config.MemoryBoth)
	model.stacks = []domain.Stack{
		{Name: "eligible", Registered: true, WorkingDir: "/srv/eligible", Files: []string{"/srv/eligible/compose.yaml"}},
		{Name: "current", Registered: true, WorkingDir: "/srv/current", Files: []string{"/srv/current/compose.yaml"}},
	}
	eligible, current := model.stacks[0], model.stacks[1]
	model.action = actionState{resource: actionStacks, targets: []actionTarget{
		{ID: eligible.Name, Name: eligible.Name, Update: domain.UpdateAvailable, Stack: &eligible},
		{ID: current.Name, Name: current.Name, Update: domain.UpdateCurrent, Stack: &current},
	}}

	message := model.runAction(application.ActionUpdate)()
	results := message.(actionFinishedMsg).results
	if len(results) != 1 || results[0].ID != "eligible" || pullCalls != 1 || upCalls != 1 {
		t.Fatalf("mixed stack update included skipped target: results=%#v pull=%d up=%d", results, pullCalls, upCalls)
	}
}

func TestMixedStackMenuKeepsUpdateWhenAnotherStackLacksMetadata(t *testing.T) {
	eligible := domain.Stack{Name: "eligible", Registered: true, WorkingDir: "/srv/eligible", Files: []string{"/srv/eligible/compose.yaml"}}
	unavailable := domain.Stack{Name: "unavailable"}
	targets := []actionTarget{
		{ID: eligible.Name, State: "running", Update: domain.UpdateAvailable, Stack: &eligible},
		{ID: unavailable.Name, State: "stopped", Unavailable: unavailable.DownUnavailableReason(), Stack: &unavailable},
	}
	if got, want := fmt.Sprint(stackActionChoices(targets)), "[pull update cancel]"; got != want {
		t.Fatalf("mixed stack choices = %s, want %s", got, want)
	}
}

func TestSnapshotPrunesPendingRecreateForReplacedContainerID(t *testing.T) {
	model := loadedSelectionModel(t)
	model.pendingRecreates["old"] = pendingImageRecreate{ContainerID: "old", ImageID: "image", Reference: "app:latest"}
	model.snapshot.Containers = []domain.Container{{ID: "new", Name: "app", State: "running"}}
	model.prunePendingRecreates()
	if len(model.pendingRecreates) != 0 {
		t.Fatalf("obsolete pending ID survived snapshot reconciliation: %#v", model.pendingRecreates)
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

func TestUpdatedImageActionsOfferPullAndDeleteForSingleAndMultipleSelection(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.images[0].Update = domain.UpdateAvailable
	model.images[1].Update = domain.UpdateAvailable
	model.snapshot.Containers = []domain.Container{{ID: "one-container", Image: "one:latest", ImageID: "one", State: "running"}, {ID: "two-container", Image: "two:latest", ImageID: "two", State: "running"}}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if got, want := fmt.Sprint(menu.actionChoices()), "[pull delete cancel]"; got != want || len(menu.action.targets[0].PullRefs) != 1 {
		t.Fatalf("single-image actions = %v targets=%#v, want %s", menu.actionChoices(), menu.action.targets, want)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
	multiple := updated.(Model)
	updated, _ = multiple.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got, want := fmt.Sprint(updated.(Model).actionChoices()), "[pull delete cancel]"; got != want || len(updated.(Model).action.targets) != 2 {
		t.Fatalf("multi-image actions = %s targets=%#v, want %s", got, updated.(Model).action.targets, want)
	}
}

func TestPulledImageOffersRecreateForDirectContainers(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.images[0].Update = domain.UpdatePulledPendingRecreate
	model.snapshot.Containers = []domain.Container{{ID: "container", Image: "one:latest", ImageID: "one", State: "running"}}
	model.pendingRecreates["container"] = pendingImageRecreate{ContainerID: "container", ImageID: "one", Reference: "one:latest"}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if got, want := fmt.Sprint(menu.actionChoices()), "[recreate delete cancel]"; got != want || len(menu.action.targets[0].Recreate) != 1 {
		t.Fatalf("pulled image actions = %s targets=%#v, want %s", got, menu.action.targets, want)
	}

	model.snapshot.Containers[0].ComposeProject = "stack"
	model.pendingRecreates["container"] = pendingImageRecreate{ContainerID: "container", ImageID: "one", Reference: "one:latest", Compose: true}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got, want := fmt.Sprint(updated.(Model).actionChoices()), "[delete cancel]"; got != want {
		t.Fatalf("Compose image actions = %s, want %s", got, want)
	}
}

func TestImageDeleteConfirmationUsesY(t *testing.T) {
	model := loadedImageSelectionModel(t)
	model.action = actionState{stage: actionConfirm, resource: actionImages, targets: []actionTarget{{ID: "one", Name: "one:latest"}}}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).action.running {
		t.Fatal("enter must not confirm image deletion")
	}

	confirmed, command := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil || !confirmed.(Model).action.running {
		t.Fatal("y must confirm image deletion")
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

func TestNetworkAndVolumeEnterOpenIndividualDeleteAndDOpensDetails(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		resource actionResource
		target   string
	}{
		{name: "network", model: loadedNetworkSelectionModel(t), resource: actionNetworks, target: "network"},
		{name: "volume", model: loadedVolumeSelectionModel(t), resource: actionVolumes, target: "volume"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updated, command := test.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			menu := updated.(Model)
			if command != nil || menu.action.stage != actionMenu || menu.action.resource != test.resource || len(menu.action.targets) != 1 || menu.action.targets[0].ID != test.target {
				t.Fatalf("expected individual delete menu: %#v", menu.action)
			}
			if got := fmt.Sprint(menu.actionChoices()); got != "[delete cancel]" {
				t.Fatalf("individual choices = %s", got)
			}

			updated, command = test.model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
			details := updated.(Model)
			if command == nil || test.resource == actionNetworks && !details.networkDetailOpen || test.resource == actionVolumes && !details.volumeDetailOpen {
				t.Fatalf("d did not open %s details", test.name)
			}
		})
	}
}

func TestDOpensImageDetails(t *testing.T) {
	model := loadedImageSelectionModel(t)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if command == nil || !updated.(Model).imageDetailOpen || !updated.(Model).imageDetailLoading {
		t.Fatal("d did not open image details")
	}
}

func TestNetworkAndVolumeEditingOpenDeleteMenuAndConfirmWithY(t *testing.T) {
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
			updated, command := confirmation.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if command != nil || updated.(Model).action.running {
				t.Fatal("enter must not confirm deletion")
			}
			updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			if command == nil || !updated.(Model).action.running {
				t.Fatal("y must confirm deletion")
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

func TestEnterOpensActionsAndDOpensDetailsForCurrentContainer(t *testing.T) {
	model := loadedSelectionModel(t)

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	menu := updated.(Model)
	if command != nil || menu.action.stage != actionMenu || len(menu.action.targets) != 1 || menu.action.targets[0].ID != "one" {
		t.Fatalf("expected individual action menu, got %#v", menu.action)
	}

	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	opened := updated.(Model)
	if command == nil || opened.panel != panelDetails || !opened.detailLoading {
		t.Fatalf("expected d to open details, got panel=%d loading=%v", opened.panel, opened.detailLoading)
	}
}

func TestSelectAllOnlyWorksInEditModeAndTogglesCheckboxes(t *testing.T) {
	model := loadedSelectionModel(t)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if updated.(Model).editing || len(updated.(Model).selected) != 0 {
		t.Fatal("a must not enter edit mode")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	all := updated.(Model)
	if !all.editing || len(all.selected) != len(all.snapshot.Containers) {
		t.Fatalf("expected all containers selected in edit mode: %#v", all.selected)
	}
	updated, _ = all.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	none := updated.(Model)
	if !none.editing || len(none.selected) != 0 {
		t.Fatalf("expected empty selection while remaining in edit mode: %#v", none.selected)
	}
}

func TestSelectAllCoversResourceViewsAndExpandedStackChildren(t *testing.T) {
	images := loadedImageSelectionModel(t)
	images.imageEditing = true
	updated, _ := images.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(updated.(Model).selectedImages) != len(images.images) {
		t.Fatal("expected all images selected")
	}

	networks := loadedNetworkSelectionModel(t)
	networks.networkEditing = true
	updated, _ = networks.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(updated.(Model).selectedNetworks) != len(networks.networks) {
		t.Fatal("expected all networks selected")
	}

	volumes := loadedVolumeSelectionModel(t)
	volumes.volumeEditing = true
	updated, _ = volumes.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(updated.(Model).selectedVolumes) != len(volumes.volumes) {
		t.Fatal("expected all volumes selected")
	}

	stacks := NewModel(application.NewContainerService(fakeRuntime{}), config.MemoryBoth)
	stacks.active, stacks.stacksLoaded, stacks.stacksLoading = 1, true, false
	stacks.stacks = []domain.Stack{{Name: "app", ContainerItems: []domain.Container{{ID: "one"}, {ID: "two"}}}, {Name: "other"}}
	stacks.stackEditing = true
	updated, _ = stacks.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(updated.(Model).selectedStacks) != 2 {
		t.Fatalf("expected all stacks selected: %#v", updated.(Model).selectedStacks)
	}
	stacks.stackEditing = false
	stacks.selectedStackName, stacks.expandedStackName, stacks.selectedStackContainerID, stacks.stackContainerEditing = "app", "app", "one", true
	updated, _ = stacks.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(updated.(Model).selectedStackContainers) != 2 {
		t.Fatalf("expected all expanded stack children selected: %#v", updated.(Model).selectedStackContainers)
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
	snapshot       domain.Snapshot
	images         []domain.Image
	imageDetails   domain.ImageDetails
	stacks         []domain.Stack
	networks       []domain.Network
	volumes        []domain.Volume
	resources      ports.ResourceLoad
	composeConfig  []ports.ComposeServiceImage
	err            error
	pullStackCalls *int
	upCalls        *int
	pruneArgs      *[]string
	pruneOutput    string
}

func (f fakeRuntime) Stacks(context.Context) ([]domain.Stack, error) { return f.stacks, f.err }

func (f fakeRuntime) LoadResources(context.Context) (ports.ResourceLoad, error) {
	return f.resources, f.err
}

func (f fakeRuntime) ComposeConfig(context.Context, domain.Stack) ([]ports.ComposeServiceImage, error) {
	return append([]ports.ComposeServiceImage(nil), f.composeConfig...), f.err
}

func TestStacksLoadExpandAndKeepSelection(t *testing.T) {
	stacks := []domain.Stack{{Name: "alpha", State: "mixed", Containers: 2, ContainerItems: []domain.Container{{ID: "web-id", Name: "web-1", ComposeService: "web", State: "running", Health: "healthy"}}}, {Name: "beta", State: "stopped", Containers: 1}}
	model := NewModel(application.NewContainerService(fakeRuntime{stacks: stacks}), config.MemoryBoth)
	snapshot := domain.Snapshot{Containers: []domain.Container{{ID: "web-id", Name: "web-1", ComposeProject: "alpha", ComposeService: "web", State: "running", Health: "healthy"}, {ID: "beta-id", Name: "beta-1", ComposeProject: "beta", ComposeService: "web", State: "exited", Health: "-"}}}
	updated, _ := model.Update(loadedMsg{snapshot: snapshot, generation: model.generation})
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
	updated, _ = updated.(Model).Update(loadedMsg{snapshot: snapshot, generation: loaded.generation})
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
	if selectedAction(actionStackContainers, 0) != application.ActionRestart || !strings.Contains(confirmation.confirmationBanner(), "api-1") {
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
	if !strings.Contains(confirm.confirmationBanner(), "[y/N]") {
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

func (fakeRuntime) PullImage(context.Context, string) error { return nil }

func (fakeRuntime) RecreateContainer(context.Context, string, string) error { return nil }

func (fakeRuntime) RemoveNetwork(context.Context, string) error { return nil }

func (fakeRuntime) RemoveVolume(context.Context, string) error { return nil }

func (f fakeRuntime) Prune(_ context.Context, args ...string) (string, error) {
	if f.pruneArgs != nil {
		*f.pruneArgs = append([]string(nil), args...)
	}
	return f.pruneOutput, f.err
}

func (fakeRuntime) Down(context.Context, domain.Stack) error { return nil }
func (f fakeRuntime) Up(context.Context, domain.Stack) error {
	if f.upCalls != nil {
		*f.upCalls++
	}
	return nil
}
func (fakeRuntime) StopStack(context.Context, domain.Stack) error    { return nil }
func (fakeRuntime) RestartStack(context.Context, domain.Stack) error { return nil }
func (f fakeRuntime) PullStack(context.Context, domain.Stack) error {
	if f.pullStackCalls != nil {
		*f.pullStackCalls++
	}
	return nil
}
func (f fakeRuntime) PullStackServices(context.Context, domain.Stack, []string) error {
	if f.pullStackCalls != nil {
		*f.pullStackCalls++
	}
	return nil
}
func (f fakeRuntime) UpStackServices(context.Context, domain.Stack, []string) error {
	if f.upCalls != nil {
		*f.upCalls++
	}
	return nil
}
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

func testImageUpdateService() *application.ImageUpdateService {
	return application.NewImageUpdateService(application.CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"Descriptor":{"digest":"sha256:remote"}}`), nil
	}), application.UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
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

type tuiComposeUpdateStore struct {
	projects map[string]domain.ComposeUpdateProject
}

func (store *tuiComposeUpdateStore) Get(project string) (domain.ComposeUpdateProject, bool) {
	value, found := store.projects[project]
	copy := value
	copy.Services = make(map[string]domain.ComposeUpdateService, len(value.Services))
	for name, service := range value.Services {
		copy.Services[name] = service
	}
	return copy, found
}

func (store *tuiComposeUpdateStore) Put(project domain.ComposeUpdateProject) error {
	copy := project
	copy.Services = make(map[string]domain.ComposeUpdateService, len(project.Services))
	for name, service := range project.Services {
		copy.Services[name] = service
	}
	store.projects[project.Name] = copy
	return nil
}

func (store *tuiComposeUpdateStore) Health() error { return nil }

func (store *tuiComposeUpdateStore) BeginMutation() (func(), error) { return func() {}, nil }
