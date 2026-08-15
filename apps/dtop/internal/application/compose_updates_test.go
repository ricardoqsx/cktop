package application

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	stateadapter "github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/state"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

func TestComposePullPersistsVerifiedDownloadedDigest(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()

	results := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})
	if len(results) != 1 || results[0].Err != nil || !results[0].Pulled || results[0].Applied {
		t.Fatalf("pull results = %#v", results)
	}
	project, found := store.Get(context.Background(), stack.Name)
	if !found {
		t.Fatal("verified pull did not persist project state")
	}
	web := project.Services["web"]
	if web.Reference != "docker.io/library/app:latest" || web.DownloadedDigest != "sha256:new" || web.DownloadedImageID != "sha256:new-image" || web.AppliedDigest != "sha256:old" || web.AppliedImageID != "sha256:old-image" || web.PendingUnknown {
		t.Fatalf("persisted service = %#v", web)
	}
	if !project.Pending() {
		t.Fatal("pull-only state is not pending")
	}
}

func TestComposeStateSurvivesServiceRestartAndGuardsDownUpUntilApply(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	stack := registeredStack()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	if result := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatalf("pull: %v", result.Err)
	}
	if result := service.ActStacks(context.Background(), ActionDown, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatalf("down: %v", result.Err)
	}

	restarted := NewContainerServiceWithComposeUpdates(runtime, store)
	up := restarted.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]
	if up.Err == nil || !strings.Contains(up.Err.Error(), "Apply downloaded update") {
		t.Fatalf("plain Up was not guarded: %#v", up)
	}
	if countAction(runtime.actions, "up:app") != 0 {
		t.Fatalf("guarded Up reached runtime: %v", runtime.actions)
	}

	apply := restarted.ActStacks(context.Background(), ActionApply, []domain.Stack{stack})[0]
	if apply.Err != nil || !apply.Applied {
		t.Fatalf("apply result = %#v", apply)
	}
	project, _ := store.Get(context.Background(), stack.Name)
	web := project.Services["web"]
	if web.DownloadedDigest != "sha256:new" || web.AppliedDigest != web.DownloadedDigest || project.Pending() {
		t.Fatalf("applied state = %#v", project)
	}
	if countAction(runtime.actions, "up:app") != 1 {
		t.Fatalf("Apply did not execute exactly one Up: %v", runtime.actions)
	}
}

func TestComposeStateFileSurvivesReopenAcrossApplicationBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "compose-updates.json")
	store, err := stateadapter.NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime := composeUpdateRuntime()
	stack := registeredStack()
	if result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatalf("pull: %v", result.Err)
	}

	reopened, err := stateadapter.NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewContainerServiceWithComposeUpdates(runtime, reopened)
	if result := restarted.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]; result.Err == nil {
		t.Fatal("reopened state did not guard Up")
	}
	if result := restarted.ActStacks(context.Background(), ActionApply, []domain.Stack{stack})[0]; result.Err != nil || !result.Applied {
		t.Fatalf("reopened Apply = %#v", result)
	}
}

func TestUnregisteredViewOfProjectCannotBypassRegisteredPendingState(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	stack := registeredStack()
	if result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatal(result.Err)
	}
	stack.Registered = false
	result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]
	if result.Err == nil {
		t.Fatal("unregistered view bypassed registered pending state")
	}
	if countAction(runtime.actions, "up:app") != 0 {
		t.Fatalf("unregistered Up reached runtime: %v", runtime.actions)
	}
}

func TestComposeServiceUpdateCorrelatesEffectiveConfiguration(t *testing.T) {
	runtime := composeUpdateRuntime()
	service := NewContainerServiceWithComposeUpdates(runtime, newFakeComposeUpdateStore())
	stack := registeredStack()
	target := StackUpdateTarget{Stack: stack, Services: []string{"web"}, References: map[string]string{"web": "app:old"}}

	results := service.UpdateStackServices(context.Background(), ActionPull, []StackUpdateTarget{target})
	if len(results) != 1 || results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "image changed") {
		t.Fatalf("stale correlation result = %#v", results)
	}
	if countAction(runtime.actions, "pull-stack-services:app:web") != 0 {
		t.Fatalf("stale target reached Compose pull: %v", runtime.actions)
	}
}

func TestComposeServiceApplyChangesOnlySelectedPersistentState(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.composeConfig = append(runtime.composeConfig, ports.ComposeServiceImage{Service: "worker", Reference: "worker:latest"})
	runtime.images = append(runtime.images, domain.Image{ID: "sha256:worker-old", Tags: []string{"worker:latest"}, RepoDigests: []string{"docker.io/library/worker@sha256:worker-old"}})
	runtime.pullImages = append(runtime.pullImages, domain.Image{ID: "sha256:worker-image", Tags: []string{"worker:latest"}, RepoDigests: []string{"docker.io/library/worker@sha256:worker"}})
	runtime.snapshot.Containers = append(runtime.snapshot.Containers, domain.Container{ComposeProject: "app", ComposeService: "worker", ImageID: "sha256:worker-old"})
	runtime.upSnapshot.Containers = append(runtime.upSnapshot.Containers, domain.Container{ComposeProject: "app", ComposeService: "worker", ImageID: "sha256:worker-image"})
	store := newFakeComposeUpdateStore()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()

	pulls := []StackUpdateTarget{
		{Stack: stack, Services: []string{"web"}, References: map[string]string{"web": "app:latest"}},
		{Stack: stack, Services: []string{"worker"}, References: map[string]string{"worker": "worker:latest"}},
	}
	for _, target := range pulls {
		if result := service.UpdateStackServices(context.Background(), ActionPull, []StackUpdateTarget{target})[0]; result.Err != nil {
			t.Fatalf("pull %v: %v", target.Services, result.Err)
		}
	}
	apply := service.UpdateStackServices(context.Background(), ActionApply, []StackUpdateTarget{pulls[0]})[0]
	if apply.Err != nil || !apply.Applied {
		t.Fatalf("web apply = %#v", apply)
	}
	project, _ := store.Get(context.Background(), stack.Name)
	if project.Services["web"].AppliedDigest != "sha256:new" {
		t.Fatalf("web was not applied: %#v", project.Services["web"])
	}
	if worker := project.Services["worker"]; worker.AppliedDigest != "" || worker.DownloadedDigest != "sha256:worker" {
		t.Fatalf("worker state was changed by web Apply: %#v", worker)
	}
}

func TestComposePullFailureCreatesUnknownGuardAndRetryRecovers(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.actionErrors = map[string]error{"pull-stack-services:app": errors.New("partial pull failure")}
	store := newFakeComposeUpdateStore()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()

	failed := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]
	if failed.Err == nil || failed.Pulled {
		t.Fatalf("failed pull result = %#v", failed)
	}
	project, _ := store.Get(context.Background(), stack.Name)
	if !project.PendingUnknown() || !project.Pending() {
		t.Fatalf("failed pull was not recorded conservatively: %#v", project)
	}
	if up := service.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]; up.Err == nil {
		t.Fatal("plain Up bypassed unknown pull state")
	}
	if apply := service.ActStacks(context.Background(), ActionApply, []domain.Stack{stack})[0]; apply.Err == nil {
		t.Fatal("Apply accepted an unverified pull")
	}

	delete(runtime.actionErrors, "pull-stack-services:app")
	retried := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]
	if retried.Err != nil || !retried.Pulled {
		t.Fatalf("retry result = %#v", retried)
	}
	project, _ = store.Get(context.Background(), stack.Name)
	if project.PendingUnknown() || project.Services["web"].DownloadedDigest != "sha256:new" {
		t.Fatalf("retry did not recover verified state: %#v", project)
	}
}

func TestComposePersistenceFailureIsWarningAndRetainsSessionGuard(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	store.err = errors.New("disk full")
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()

	pull := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]
	if pull.Err == nil || pull.Warning != nil || pull.Pulled || !strings.Contains(pull.Err.Error(), "prepare Compose update state") {
		t.Fatalf("persistence failure result = %#v", pull)
	}
	if countAction(runtime.actions, "pull-stack-services:app:web") != 0 {
		t.Fatalf("pull executed without a durable write-ahead marker: %v", runtime.actions)
	}
	if up := service.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]; up.Err == nil {
		t.Fatal("in-memory guard was lost after persistence failure")
	}
}

func TestComposePostPullPersistenceFailureKeepsDurableUnknownMarker(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	store.failAt = 2
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()

	pull := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]
	if pull.Err != nil || pull.Warning == nil || !pull.Pulled {
		t.Fatalf("post-pull persistence result = %#v", pull)
	}
	persisted, found := store.Get(context.Background(), "app")
	if !found || !persisted.PendingUnknown() {
		t.Fatalf("write-ahead marker was not preserved: %#v", persisted)
	}
	restarted := NewContainerServiceWithComposeUpdates(runtime, store)
	if up := restarted.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]; up.Err == nil {
		t.Fatal("restarted service bypassed durable unknown marker")
	}
}

func TestUnavailableComposeStateFailsClosedAndDecoratesRegisteredStack(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	store.err = errors.New("state file is corrupt")
	project := ComposeProject{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	service := NewContainerServiceWithComposeUpdates(runtime, store, project)

	resources, err := service.LoadResources(context.Background())
	if err != nil || len(resources.Stacks) != 1 {
		t.Fatalf("resources = %#v, %v", resources, err)
	}
	stack := resources.Stacks[0]
	if !stack.UpdatePending || !stack.UpdateUnknown || !strings.Contains(stack.UpdateReason, "state file is corrupt") {
		t.Fatalf("unavailable state was not surfaced: %#v", stack)
	}
	if up := service.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]; up.Err == nil || !strings.Contains(up.Err.Error(), "refusing Up") {
		t.Fatalf("unavailable state did not fail closed: %#v", up)
	}
}

func TestComposePullWithoutLocalDigestRemainsUnverified(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.pullImages[0].RepoDigests = nil
	store := newFakeComposeUpdateStore()
	result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionPull, []domain.Stack{registeredStack()})[0]
	if result.Err == nil || !result.Pulled || !strings.Contains(result.Err.Error(), "RepoDigest") {
		t.Fatalf("missing digest result = %#v", result)
	}
	project, _ := store.Get(context.Background(), "app")
	if !project.PendingUnknown() {
		t.Fatalf("missing digest was not retained as unknown: %#v", project)
	}
}

func TestComposeConfigFingerprintIgnoresStaleStateAfterImageChange(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()
	if result := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatal(result.Err)
	}
	runtime.composeConfig[0].Reference = "app:next"
	before := append([]string(nil), runtime.actions...)
	up := service.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]
	if up.Err != nil {
		t.Fatalf("stale state blocked changed configuration: %v", up.Err)
	}
	if reflect.DeepEqual(before, runtime.actions) || runtime.actions[len(runtime.actions)-1] != "up:app" {
		t.Fatalf("changed configuration did not reach Up: %v", runtime.actions)
	}
}

func TestComposeConfigChangePreservesPendingMatchingService(t *testing.T) {
	runtime := composeUpdateRuntime()
	store := newFakeComposeUpdateStore()
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	stack := registeredStack()
	if result := service.ActStacks(context.Background(), ActionPull, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatal(result.Err)
	}
	runtime.composeConfig = append(runtime.composeConfig, ports.ComposeServiceImage{Service: "sidecar", Reference: "sidecar:latest"})
	stack.Files = append(stack.Files, "/srv/app/compose.override.yaml")

	up := service.ActStacks(context.Background(), ActionUp, []domain.Stack{stack})[0]
	if up.Err == nil || !strings.Contains(up.Err.Error(), "Apply downloaded update") {
		t.Fatalf("matching pending service was discarded after config change: %#v", up)
	}
	if countAction(runtime.actions, "up:app") != 0 {
		t.Fatalf("changed registration bypassed pending guard: %v", runtime.actions)
	}
}

func TestComposeWholeStackPullExcludesPinnedAndBuildOnlyServices(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.composeConfig = append(runtime.composeConfig,
		ports.ComposeServiceImage{Service: "database", Reference: "postgres@sha256:abc"},
		ports.ComposeServiceImage{Service: "builder", Reference: "", Build: true},
		ports.ComposeServiceImage{Service: "local-image", Reference: "local/app:latest", Build: true},
		ports.ComposeServiceImage{Service: "never-pull", Reference: "private/app:latest", PullPolicy: "never"},
	)
	result := NewContainerServiceWithComposeUpdates(runtime, newFakeComposeUpdateStore()).ActStacks(context.Background(), ActionPull, []domain.Stack{registeredStack()})[0]
	if result.Err != nil || !result.Pulled {
		t.Fatalf("pull result = %#v", result)
	}
	if countAction(runtime.actions, "pull-stack-services:app:web") != 1 {
		t.Fatalf("pull included a pinned or build-only service: %v", runtime.actions)
	}
}

func TestComposePullInitializesAppliedBaselineAndLeavesUnchangedSiblingCurrent(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.composeConfig = append(runtime.composeConfig, ports.ComposeServiceImage{Service: "worker", Reference: "worker:latest"})
	stableWorker := domain.Image{ID: "sha256:worker", Tags: []string{"worker:latest"}, RepoDigests: []string{"docker.io/library/worker@sha256:stable"}}
	runtime.images = append(runtime.images, stableWorker)
	runtime.pullImages = append(runtime.pullImages, stableWorker)
	runtime.snapshot.Containers = append(runtime.snapshot.Containers, domain.Container{ComposeProject: "app", ComposeService: "worker", ImageID: stableWorker.ID})
	store := newFakeComposeUpdateStore()
	result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionPull, []domain.Stack{registeredStack()})[0]
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	project, _ := store.Get(context.Background(), "app")
	if worker := project.Services["worker"]; worker.DownloadedDigest != "sha256:stable" || worker.AppliedDigest != worker.DownloadedDigest || worker.PendingUnknown {
		t.Fatalf("unchanged worker became pending: %#v", worker)
	}
	if web := project.Services["web"]; web.DownloadedDigest == web.AppliedDigest {
		t.Fatalf("updated web was not pending: %#v", web)
	}
}

func TestComposePullTreatsMixedReplicaBaselinesAsPending(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.images = append(runtime.images, domain.Image{ID: "sha256:new-image", Tags: []string{"app:latest"}, RepoDigests: []string{"docker.io/library/app@sha256:new"}})
	runtime.snapshot.Containers = append(runtime.snapshot.Containers, domain.Container{ComposeProject: "app", ComposeService: "web", ImageID: "sha256:new-image"})
	store := newFakeComposeUpdateStore()
	result := NewContainerServiceWithComposeUpdates(runtime, store).ActStacks(context.Background(), ActionPull, []domain.Stack{registeredStack()})[0]
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	project, _ := store.Get(context.Background(), "app")
	web := project.Services["web"]
	if web.AppliedDigest != "" || web.DownloadedDigest != "sha256:new" || !project.Pending() {
		t.Fatalf("mixed baseline was treated as current: %#v", web)
	}
}

func TestComposeApplyRejectsWhenNoUpdateIsPending(t *testing.T) {
	runtime := composeUpdateRuntime()
	service := NewContainerServiceWithComposeUpdates(runtime, newFakeComposeUpdateStore())
	stack := registeredStack()
	update := service.ActStacks(context.Background(), ActionUpdate, []domain.Stack{stack})[0]
	if update.Err != nil || !update.Applied {
		t.Fatalf("update result = %#v", update)
	}
	apply := service.ActStacks(context.Background(), ActionApply, []domain.Stack{stack})[0]
	if apply.Err == nil || !strings.Contains(apply.Err.Error(), "no verified downloaded update pending") {
		t.Fatalf("empty Apply result = %#v", apply)
	}
	if countAction(runtime.actions, "up:app") != 1 {
		t.Fatalf("empty Apply executed another Up: %v", runtime.actions)
	}
}

func TestComposeApplyIgnoresOneOffContainersDuringVerification(t *testing.T) {
	runtime := composeUpdateRuntime()
	runtime.upSnapshot.Containers = append(runtime.upSnapshot.Containers, domain.Container{ComposeProject: "app", ComposeService: "web", ComposeOneOff: true, ImageID: "sha256:old-image"})
	result := NewContainerServiceWithComposeUpdates(runtime, newFakeComposeUpdateStore()).ActStacks(context.Background(), ActionUpdate, []domain.Stack{registeredStack()})[0]
	if result.Err != nil || !result.Applied {
		t.Fatalf("one-off container invalidated Apply: %#v", result)
	}
}

func TestComposeCacheRejectsStaleRefreshRevision(t *testing.T) {
	coordinator := newComposeUpdateCoordinator(nil)
	pending := domain.ComposeUpdateProject{Name: "app", Services: map[string]domain.ComposeUpdateService{"web": {Reference: "app:latest", DownloadedDigest: "sha256:new"}}}
	revision := coordinator.currentRevision()
	if err := coordinator.put(context.Background(), domain.ComposeUpdateProject{Name: "app", Services: map[string]domain.ComposeUpdateService{}}); err != nil {
		t.Fatal(err)
	}
	coordinator.setCacheAtRevision("app", composeUpdateCache{project: pending, eligible: true}, revision)
	if cache, found := coordinator.cached("app"); found && cache.project.Pending() {
		t.Fatalf("stale refresh overwrote newer cache state: %#v", cache)
	}
}

func composeUpdateRuntime() *fakeRuntime {
	return &fakeRuntime{
		composeConfig: []ports.ComposeServiceImage{{Service: "web", Reference: "app:latest"}},
		images:        []domain.Image{{ID: "sha256:old-image", Tags: []string{"app:latest"}, RepoDigests: []string{"docker.io/library/app@sha256:old"}}},
		pullImages:    []domain.Image{{ID: "sha256:new-image", Tags: []string{"app:latest"}, RepoDigests: []string{"docker.io/library/app@sha256:new"}}},
		snapshot:      domain.Snapshot{Containers: []domain.Container{{ComposeProject: "app", ComposeService: "web", ImageID: "sha256:old-image"}}},
		upSnapshot:    domain.Snapshot{Containers: []domain.Container{{ComposeProject: "app", ComposeService: "web", ImageID: "sha256:new-image"}}},
	}
}

func registeredStack() domain.Stack {
	return domain.Stack{Name: "app", Registered: true, State: "running", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
}

func countAction(actions []string, action string) int {
	count := 0
	for _, value := range actions {
		if value == action {
			count++
		}
	}
	return count
}

type fakeComposeUpdateStore struct {
	mu       sync.Mutex
	projects map[string]domain.ComposeUpdateProject
	err      error
	putCalls int
	failAt   int
}

func newFakeComposeUpdateStore() *fakeComposeUpdateStore {
	return &fakeComposeUpdateStore{projects: make(map[string]domain.ComposeUpdateProject)}
}

func (store *fakeComposeUpdateStore) Get(_ context.Context, project string) (domain.ComposeUpdateProject, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, found := store.projects[project]
	return cloneComposeUpdateProject(value), found
}

func (store *fakeComposeUpdateStore) Put(_ context.Context, project domain.ComposeUpdateProject) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.putCalls++
	if store.err != nil {
		return store.err
	}
	if store.failAt > 0 && store.putCalls == store.failAt {
		return errors.New("transient state write failure")
	}
	store.projects[project.Name] = cloneComposeUpdateProject(project)
	return nil
}

func (store *fakeComposeUpdateStore) Health(context.Context) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.err
}

func (store *fakeComposeUpdateStore) BeginMutation(context.Context) (func(), error) {
	return func() {}, nil
}
