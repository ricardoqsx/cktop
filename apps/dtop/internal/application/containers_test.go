package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

func TestContainerServiceLoadsRuntimeSnapshot(t *testing.T) {
	want := domain.Snapshot{Containers: []domain.Container{{ID: "abc", Name: "web"}}}
	service := NewContainerService(&fakeRuntime{snapshot: want})

	got, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Fatalf("unexpected snapshot: %#v", got)
	}
}

func TestContainerServiceLoadsAggregateResources(t *testing.T) {
	want := ports.ResourceLoad{Images: []domain.Image{{ID: "image"}}}
	service := NewContainerService(&fakeRuntime{resources: want})

	got, err := service.LoadResources(context.Background())
	if err != nil || len(got.Images) != 1 || got.Images[0].ID != "image" {
		t.Fatalf("unexpected aggregate resources: %#v err=%v", got, err)
	}
}

func TestContainerServiceSortsByStateAndName(t *testing.T) {
	service := NewContainerService(&fakeRuntime{})
	snapshot := service.Sort(domain.Snapshot{Containers: []domain.Container{
		{Name: "zeta", State: "exited"},
		{Name: "beta", State: "running"},
		{Name: "alpha", State: "running"},
		{Name: "gamma", State: "paused"},
	}}, SortState)

	want := []string{"alpha", "beta", "gamma", "zeta"}
	for index, name := range want {
		if snapshot.Containers[index].Name != name {
			t.Fatalf("position %d: expected %q, got %q", index, name, snapshot.Containers[index].Name)
		}
	}
}

func TestContainerServiceSortsMetricsWithUnavailableLast(t *testing.T) {
	service := NewContainerService(&fakeRuntime{})
	snapshot := domain.Snapshot{Containers: []domain.Container{
		{Name: "unavailable", CPUAvailable: false, MemoryAvailable: false},
		{Name: "low", CPUAvailable: true, CPUPercent: 10, MemoryAvailable: true, MemoryUsage: 10},
		{Name: "high", CPUAvailable: true, CPUPercent: 90, MemoryAvailable: true, MemoryUsage: 90},
	}}

	cpu := service.Sort(snapshot, SortCPU)
	if got := []string{cpu.Containers[0].Name, cpu.Containers[1].Name, cpu.Containers[2].Name}; got[0] != "high" || got[1] != "low" || got[2] != "unavailable" {
		t.Fatalf("unexpected CPU order: %v", got)
	}

	memory := service.Sort(snapshot, SortMemory)
	if got := []string{memory.Containers[0].Name, memory.Containers[1].Name, memory.Containers[2].Name}; got[0] != "high" || got[1] != "low" || got[2] != "unavailable" {
		t.Fatalf("unexpected memory order: %v", got)
	}
}

func TestEnrichStacksCarriesComposeContainersAndMetrics(t *testing.T) {
	stacks := EnrichStacks([]domain.Stack{{Name: "app"}}, domain.Snapshot{Containers: []domain.Container{
		{ID: "worker", Name: "worker-1", ComposeProject: "app", ComposeService: "worker", State: "exited", Health: "-"},
		{ID: "web", Name: "web-1", ComposeProject: "app", ComposeService: "web", State: "running", Health: "healthy", CPUAvailable: true, CPUPercent: 12.5, MemoryAvailable: true, MemoryUsage: 1024, MemoryLimit: 2048},
	}})
	if len(stacks[0].ContainerItems) != 2 || stacks[0].ContainerItems[0].ID != "web" || stacks[0].ContainerItems[0].ComposeService != "web" {
		t.Fatalf("expected sorted compose child identities, got %#v", stacks[0].ContainerItems)
	}
	if !stacks[0].CPUAvailable || stacks[0].CPUPercent != 12.5 || !stacks[0].MemoryAvailable || stacks[0].MemoryUsage != 1024 {
		t.Fatalf("expected aggregate metrics from running child, got %#v", stacks[0])
	}
}

func TestNextSortModeCycles(t *testing.T) {
	mode := SortState
	for _, want := range []SortMode{SortCPU, SortMemory, SortName, SortState} {
		mode = NextSortMode(mode)
		if mode != want {
			t.Fatalf("expected %q, got %q", want, mode)
		}
	}
}

func TestContainerServiceReturnsResultForEachAction(t *testing.T) {
	runtime := &fakeRuntime{errors: map[string]error{"two": errors.New("cannot restart")}}
	service := NewContainerService(runtime)

	results := service.Act(context.Background(), ActionRestart, []string{"one", "two"})
	if len(results) != 2 {
		t.Fatalf("expected two results, got %d", len(results))
	}
	if results[0].Err != nil || results[1].Err == nil {
		t.Fatalf("expected success then failure, got %#v", results)
	}
	if got := runtime.actions; len(got) != 2 || got[0] != "restart:one" || got[1] != "restart:two" {
		t.Fatalf("unexpected actions: %v", got)
	}
}

func TestContainerServiceForceDeletes(t *testing.T) {
	runtime := &fakeRuntime{}
	service := NewContainerService(runtime)

	results := service.Act(context.Background(), ActionDelete, []string{"one"})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("expected successful delete, got %#v", results)
	}
	if got := runtime.actions; len(got) != 1 || got[0] != "remove:one" {
		t.Fatalf("unexpected actions: %v", got)
	}
}

func TestContainerServiceRemovesImagesWithoutForceSequentially(t *testing.T) {
	runtime := &fakeRuntime{errors: map[string]error{"two": errors.New("image is used by a container")}}
	service := NewContainerService(runtime)

	results := service.RemoveImages(context.Background(), []string{"one", "two"})
	if len(results) != 2 || results[0].Err != nil || results[1].Err == nil {
		t.Fatalf("expected success then failure, got %#v", results)
	}
	if got := runtime.actions; len(got) != 2 || got[0] != "remove-image:one:false" || got[1] != "remove-image:two:false" {
		t.Fatalf("expected sequential non-force image removals, got %v", got)
	}
}

func TestContainerServiceRemovesNetworksAndVolumesWithoutForceSequentially(t *testing.T) {
	runtime := &fakeRuntime{errors: map[string]error{"network-two": errors.New("network has active endpoints"), "volume-two": errors.New("volume is in use")}}
	service := NewContainerService(runtime)

	networks := service.RemoveNetworks(context.Background(), []string{"network-one", "network-two"})
	volumes := service.RemoveVolumes(context.Background(), []string{"volume-one", "volume-two"})
	if len(networks) != 2 || networks[0].Err != nil || networks[1].Err == nil || len(volumes) != 2 || volumes[0].Err != nil || volumes[1].Err == nil {
		t.Fatalf("expected per-resource non-force results, networks=%#v volumes=%#v", networks, volumes)
	}
	if got := runtime.actions; len(got) != 4 || got[0] != "remove-network:network-one" || got[1] != "remove-network:network-two" || got[2] != "remove-volume:volume-one" || got[3] != "remove-volume:volume-two" {
		t.Fatalf("expected sequential non-force removals, got %v", got)
	}
}

func TestContainerServiceReturnsRuntimeError(t *testing.T) {
	wantErr := errors.New("boom")
	service := NewContainerService(&fakeRuntime{err: wantErr})

	_, err := service.Load(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestContainerServiceDelegatesNetworkAndVolumeReads(t *testing.T) {
	service := NewContainerService(&fakeRuntime{networks: []domain.Network{{ID: "network"}}, volumes: []domain.Volume{{Name: "volume"}}})
	networks, err := service.Networks(context.Background())
	if err != nil || len(networks) != 1 || networks[0].ID != "network" {
		t.Fatalf("unexpected networks: %#v err=%v", networks, err)
	}
	volumes, err := service.Volumes(context.Background())
	if err != nil || len(volumes) != 1 || volumes[0].Name != "volume" {
		t.Fatalf("unexpected volumes: %#v err=%v", volumes, err)
	}
}

func TestContainerServiceDelegatesStackReads(t *testing.T) {
	service := NewContainerService(&fakeRuntime{stacks: []domain.Stack{{Name: "project"}}})
	stacks, err := service.Stacks(context.Background())
	if err != nil || len(stacks) != 1 || stacks[0].Name != "project" {
		t.Fatalf("unexpected stacks: %#v err=%v", stacks, err)
	}
}

func TestMergeStacksPreservesDetectedContainersAndAddsRegisteredDownProject(t *testing.T) {
	stacks := MergeStacks([]domain.Stack{{Name: "detected", State: "running", Containers: 1, Services: []domain.StackService{{Name: "web", Containers: 1}}}}, []ComposeProject{
		{Name: "detected", WorkingDir: "/srv/detected", Files: []string{"/srv/detected/compose.yaml"}},
		{Name: "down", WorkingDir: "/srv/down", Files: []string{"/srv/down/compose.yaml"}},
	})
	if len(stacks) != 2 || stacks[0].Name != "detected" || stacks[0].State != "running" || stacks[0].Containers != 1 || stacks[0].WorkingDir != "/srv/detected" {
		t.Fatalf("detected stack was not preserved: %#v", stacks)
	}
	if stacks[1].Name != "down" || stacks[1].State != "down" || stacks[1].Containers != 0 {
		t.Fatalf("expected registered down project, got %#v", stacks[1])
	}
}

func TestMergeStacksShowsMissingManifestAndNeverAssumesNeverDeployed(t *testing.T) {
	stacks := MergeStacks(nil, []ComposeProject{{Name: "missing", WorkingDir: "/srv/missing", Files: []string{"/srv/missing/compose.yaml"}, MissingFiles: []string{"/srv/missing/compose.yaml"}}})
	if len(stacks) != 1 || stacks[0].State != "missing compose file" {
		t.Fatalf("expected missing compose file state, got %#v", stacks)
	}
}

func TestMergeStacksRegistrationOverridesDetectedMetadata(t *testing.T) {
	stacks := MergeStacks([]domain.Stack{{Name: "app", WorkingDir: "/detected", Files: []string{"/detected/compose.yaml"}}}, []ComposeProject{{Name: "app", WorkingDir: "/registered", Files: []string{"/registered/compose.yaml"}}})
	if stacks[0].WorkingDir != "/registered" || stacks[0].Files[0] != "/registered/compose.yaml" || stacks[0].MetadataReason != "" {
		t.Fatalf("registration did not override detected metadata: %#v", stacks[0])
	}
}

func TestEnrichStacksAggregatesRunningComposeMetrics(t *testing.T) {
	stacks := EnrichStacks([]domain.Stack{{Name: "app"}}, domain.Snapshot{Containers: []domain.Container{
		{ComposeProject: "app", State: "running", CPUAvailable: true, CPUPercent: 12.5, MemoryAvailable: true, MemoryUsage: 10, MemoryLimit: 20},
		{ComposeProject: "app", State: "running", CPUAvailable: false, MemoryAvailable: true, MemoryUsage: 5, MemoryLimit: 10},
		{ComposeProject: "app", State: "exited", CPUAvailable: true, CPUPercent: 90, MemoryAvailable: true, MemoryUsage: 100},
	}})
	if !stacks[0].CPUAvailable || stacks[0].CPUPercent != 12.5 || !stacks[0].MemoryAvailable || stacks[0].MemoryUsage != 15 || stacks[0].MemoryLimit != 30 {
		t.Fatalf("unexpected aggregate stack metrics: %#v", stacks[0])
	}
}

func TestRebuildStacksDerivesCurrentComposeProjectsAndMergesRegistrations(t *testing.T) {
	projects := []ComposeProject{
		{Name: "app", WorkingDir: "/registered/app", Files: []string{"/registered/app/compose.yaml"}},
		{Name: "down", WorkingDir: "/registered/down", Files: []string{"/registered/down/compose.yaml"}},
	}
	snapshot := domain.Snapshot{Containers: []domain.Container{
		{ID: "worker", Name: "worker-1", ComposeProject: "app", ComposeService: "worker", State: "exited", ComposeWorkingDir: "/detected/app", ComposeConfigFiles: "compose.yaml"},
		{ID: "web", Name: "web-1", ComposeProject: "app", ComposeService: "web", State: "running", ComposeWorkingDir: "/detected/app", ComposeConfigFiles: "compose.yaml", CPUAvailable: true, CPUPercent: 12.5, MemoryAvailable: true, MemoryUsage: 10, MemoryLimit: 20},
	}}

	stacks := RebuildStacks(snapshot, projects)
	if len(stacks) != 2 || stacks[0].Name != "app" || stacks[0].State != "mixed" || stacks[0].Containers != 2 || len(stacks[0].Services) != 2 || len(stacks[0].ContainerItems) != 2 {
		t.Fatalf("expected detected stack and children, got %#v", stacks)
	}
	if stacks[0].WorkingDir != "/registered/app" || stacks[0].Files[0] != "/registered/app/compose.yaml" || !stacks[0].CPUAvailable || stacks[0].CPUPercent != 12.5 || stacks[0].MemoryUsage != 10 {
		t.Fatalf("expected registration metadata precedence and metrics, got %#v", stacks[0])
	}
	if stacks[1].Name != "down" || stacks[1].State != "down" {
		t.Fatalf("expected registered Down stack, got %#v", stacks[1])
	}

	if stacks = RebuildStacks(domain.Snapshot{}, nil); len(stacks) != 0 {
		t.Fatalf("expected disappeared detected stack to be removed, got %#v", stacks)
	}
	stacks = RebuildStacks(domain.Snapshot{}, projects)
	if len(stacks) != 2 || stacks[0].Name != "app" || stacks[0].State != "down" || stacks[1].Name != "down" {
		t.Fatalf("expected registered stacks to remain after containers disappear, got %#v", stacks)
	}
}

func TestDownStacksReturnsUnavailableResultWithoutCallingRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	results := NewContainerService(runtime).DownStacks(context.Background(), []domain.Stack{{Name: "app"}})
	if len(results) != 1 || results[0].Err == nil || len(runtime.actions) != 0 {
		t.Fatalf("expected unavailable result without runtime call: %#v actions=%v", results, runtime.actions)
	}
}

type fakeRuntime struct {
	snapshot  domain.Snapshot
	images    []domain.Image
	stacks    []domain.Stack
	networks  []domain.Network
	volumes   []domain.Volume
	resources ports.ResourceLoad
	err       error
	errors    map[string]error
	actions   []string
}

func (f *fakeRuntime) Stacks(context.Context) ([]domain.Stack, error) { return f.stacks, f.err }

func (f *fakeRuntime) LoadResources(context.Context) (ports.ResourceLoad, error) {
	return f.resources, f.err
}

func (f *fakeRuntime) Snapshot(context.Context) (domain.Snapshot, error) {
	return f.snapshot, f.err
}

func (f *fakeRuntime) Images(context.Context) ([]domain.Image, error) {
	return f.images, f.err
}

func (f *fakeRuntime) ImageDetails(context.Context, string) (domain.ImageDetails, error) {
	return domain.ImageDetails{}, f.err
}

func (f *fakeRuntime) Networks(context.Context) ([]domain.Network, error) { return f.networks, f.err }
func (f *fakeRuntime) NetworkDetails(context.Context, string) (domain.NetworkDetails, error) {
	return domain.NetworkDetails{}, f.err
}
func (f *fakeRuntime) Volumes(context.Context) ([]domain.Volume, error) { return f.volumes, f.err }
func (f *fakeRuntime) VolumeDetails(context.Context, string) (domain.VolumeDetails, error) {
	return domain.VolumeDetails{}, f.err
}

func (f *fakeRuntime) Details(context.Context, string) (domain.ContainerDetails, error) {
	return domain.ContainerDetails{}, f.err
}

func (f *fakeRuntime) Logs(context.Context, string, int) (ports.LogStream, error) {
	return ports.LogStream{}, f.err
}

func (f *fakeRuntime) Stop(_ context.Context, id string) error {
	f.actions = append(f.actions, "stop:"+id)
	return f.errors[id]
}

func (f *fakeRuntime) Restart(_ context.Context, id string) error {
	f.actions = append(f.actions, "restart:"+id)
	return f.errors[id]
}

func (f *fakeRuntime) Remove(_ context.Context, id string, force bool) error {
	f.actions = append(f.actions, "remove:"+id)
	if !force {
		return errors.New("expected forced delete")
	}
	return f.errors[id]
}

func (f *fakeRuntime) RemoveImage(_ context.Context, id string, force bool) error {
	f.actions = append(f.actions, "remove-image:"+id+":"+fmt.Sprint(force))
	return f.errors[id]
}

func (f *fakeRuntime) RemoveNetwork(_ context.Context, id string) error {
	f.actions = append(f.actions, "remove-network:"+id)
	return f.errors[id]
}

func (f *fakeRuntime) RemoveVolume(_ context.Context, name string) error {
	f.actions = append(f.actions, "remove-volume:"+name)
	return f.errors[name]
}

func (f *fakeRuntime) Down(_ context.Context, stack domain.Stack) error {
	f.actions = append(f.actions, "down:"+stack.Name)
	return f.errors[stack.Name]
}
func (f *fakeRuntime) Up(_ context.Context, stack domain.Stack) error {
	f.actions = append(f.actions, "up:"+stack.Name)
	return f.errors[stack.Name]
}
func (f *fakeRuntime) StopStack(_ context.Context, stack domain.Stack) error {
	f.actions = append(f.actions, "stop-stack:"+stack.Name)
	return f.errors[stack.Name]
}
func (f *fakeRuntime) RestartStack(_ context.Context, stack domain.Stack) error {
	f.actions = append(f.actions, "restart-stack:"+stack.Name)
	return f.errors[stack.Name]
}
func (f *fakeRuntime) ComposeLogs(context.Context, domain.Stack, int) (ports.LogStream, error) {
	return ports.LogStream{}, f.err
}
