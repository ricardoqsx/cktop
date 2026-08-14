package docker

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	mobycontainer "github.com/moby/moby/api/types/container"
	mobyimage "github.com/moby/moby/api/types/image"
	mobymount "github.com/moby/moby/api/types/mount"
	mobynetwork "github.com/moby/moby/api/types/network"
	mobyvolume "github.com/moby/moby/api/types/volume"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

func TestRuntimeSnapshotIntegration(t *testing.T) {
	if os.Getenv("DTOP_INTEGRATION") != "1" {
		t.Skip("set DTOP_INTEGRATION=1 to test against the configured Docker Engine")
	}

	runtime := NewRuntime(ResolverOptions{})
	first, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("first Docker snapshot: %v", err)
	}
	if first.Engine.Name == "" || first.Engine.MemoryTotal == 0 {
		t.Fatalf("expected Engine identity and memory, got %#v", first.Engine)
	}
	if len(first.Containers) == 0 {
		t.Skip("configured Docker Engine has no containers")
	}

	time.Sleep(250 * time.Millisecond)
	second, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("second Docker snapshot: %v", err)
	}
	memoryAvailable := false
	cpuAvailable := false
	uptimeAvailable := false
	for _, container := range second.Containers {
		memoryAvailable = memoryAvailable || container.MemoryAvailable
		cpuAvailable = cpuAvailable || container.CPUAvailable
		if container.State == "running" && !container.StartedAt.IsZero() {
			uptimeAvailable = true
		}
	}
	if !memoryAvailable {
		t.Fatal("expected memory metrics for at least one running container")
	}
	if !cpuAvailable {
		t.Fatal("expected CPU metrics after the second sample")
	}
	if !uptimeAvailable {
		t.Fatal("expected uptime for at least one running container")
	}
}

func TestRuntimeImagesIntegration(t *testing.T) {
	if os.Getenv("DTOP_INTEGRATION") != "1" {
		t.Skip("set DTOP_INTEGRATION=1 to test against the configured Docker Engine")
	}

	runtime := NewRuntime(ResolverOptions{})
	images, err := runtime.Images(context.Background())
	if err != nil {
		t.Fatalf("list Docker images: %v", err)
	}
	if len(images) == 0 {
		t.Skip("configured Docker Engine has no images")
	}

	details, err := runtime.ImageDetails(context.Background(), images[0].ID)
	if err != nil {
		t.Fatalf("inspect Docker image: %v", err)
	}
	if details.ID == "" {
		t.Fatalf("expected inspected image identity, got %#v", details)
	}
}

func TestRuntimeStacksIntegration(t *testing.T) {
	if os.Getenv("DTOP_INTEGRATION") != "1" {
		t.Skip("set DTOP_INTEGRATION=1 to test against the configured Docker Engine")
	}
	if _, err := NewRuntime(ResolverOptions{}).Stacks(context.Background()); err != nil {
		t.Fatalf("list Docker Compose stacks: %v", err)
	}
}

func TestRuntimeLoadResourcesIntegration(t *testing.T) {
	if os.Getenv("DTOP_INTEGRATION") != "1" {
		t.Skip("set DTOP_INTEGRATION=1 to test against the configured Docker Engine")
	}

	resources, err := NewRuntime(ResolverOptions{}).LoadResources(context.Background())
	if err != nil {
		t.Fatalf("load Docker resources: %v", err)
	}
	if resources.StacksErr != nil || resources.ImagesErr != nil || resources.NetworksErr != nil || resources.VolumesErr != nil {
		t.Fatalf("unexpected partial Docker resource errors: %#v", resources)
	}
}

func TestToDomainContainerTrimsDockerName(t *testing.T) {
	container := toDomainContainer(mobycontainer.Summary{
		ID:      "1234567890abcdef",
		Names:   []string{"/web"},
		Image:   "nginx:latest",
		State:   "running",
		Status:  "Up 1 minute",
		Created: 100,
		Labels:  map[string]string{"com.docker.compose.project": "app", "com.docker.compose.service": "web"},
	})

	if container.Name != "web" {
		t.Fatalf("expected name web, got %q", container.Name)
	}
	if container.ShortID != "1234567890ab" {
		t.Fatalf("expected short id, got %q", container.ShortID)
	}
	if !container.Created.Equal(time.Unix(100, 0)) {
		t.Fatalf("expected created time from unix timestamp, got %v", container.Created)
	}
	if container.ComposeProject != "app" || container.ComposeService != "web" {
		t.Fatalf("expected Compose identity labels, got %#v", container)
	}
}

func TestToDomainImageIdentifiesUsageAndDanglingState(t *testing.T) {
	image := toDomainImage(mobyimage.Summary{
		ID:         "sha256:1234567890abcdef",
		RepoTags:   []string{"nginx:latest", "nginx:stable"},
		Size:       1024,
		Created:    100,
		Containers: 2,
	})
	if image.Name != "nginx:latest" || image.ShortID != "1234567890ab" {
		t.Fatalf("unexpected image identity: %#v", image)
	}
	if image.Dangling || !image.UsageKnown || image.Containers != 2 {
		t.Fatalf("unexpected image usage: %#v", image)
	}

	dangling := toDomainImage(mobyimage.Summary{ID: "sha256:abcdef1234567890", Containers: -1})
	if !dangling.Dangling || dangling.UsageKnown || dangling.Name != "<untagged>" {
		t.Fatalf("unexpected dangling image: %#v", dangling)
	}
}

func TestToDomainNetworkAndVolumePreserveUsageState(t *testing.T) {
	network := toDomainNetwork(mobynetwork.Summary{Network: mobynetwork.Network{ID: "1234567890abcdef", Name: "app", Driver: "bridge", Scope: "local"}}, 2, true)
	if network.Name != "app" || network.ShortID != "1234567890ab" || network.Containers != 2 || !network.UsageKnown {
		t.Fatalf("unexpected network: %#v", network)
	}
	volume := toDomainVolume(mobyvolume.Volume{Name: "data", Driver: "local", Scope: "local", CreatedAt: "2026-08-09T12:30:45Z"}, 0, true)
	if volume.Name != "data" || volume.Containers != 0 || !volume.UsageKnown || volume.Created.IsZero() {
		t.Fatalf("unexpected volume: %#v", volume)
	}
}

func TestResourceMappingsReuseOneContainerList(t *testing.T) {
	containers := []mobycontainer.Summary{
		{Labels: map[string]string{"com.docker.compose.project": "app", "com.docker.compose.service": "web"}, State: "running", NetworkSettings: &mobycontainer.NetworkSettingsSummary{Networks: map[string]*mobynetwork.EndpointSettings{"app": {NetworkID: "network"}}}, Mounts: []mobycontainer.MountPoint{{Name: "data", Type: "volume"}}},
	}
	stacks := composeStacks(containers)
	networks := domainNetworks([]mobynetwork.Summary{{Network: mobynetwork.Network{ID: "network", Name: "app"}}}, containers, true)
	volumes := domainVolumes([]mobyvolume.Volume{{Name: "data"}}, containers, true)
	if len(stacks) != 1 || stacks[0].Name != "app" || len(networks) != 1 || networks[0].Containers != 1 || len(volumes) != 1 || volumes[0].Containers != 1 {
		t.Fatalf("unexpected mappings from shared container list: stacks=%#v networks=%#v volumes=%#v", stacks, networks, volumes)
	}

	networks = domainNetworks([]mobynetwork.Summary{{Network: mobynetwork.Network{ID: "network", Name: "app"}}}, nil, false)
	volumes = domainVolumes([]mobyvolume.Volume{{Name: "data"}}, nil, false)
	if networks[0].UsageKnown || volumes[0].UsageKnown {
		t.Fatalf("expected unknown usage after container list failure: networks=%#v volumes=%#v", networks, volumes)
	}
}

func TestRuntimeNetworkAndVolumeIntegration(t *testing.T) {
	if os.Getenv("DTOP_INTEGRATION") != "1" {
		t.Skip("set DTOP_INTEGRATION=1 to test against the configured Docker Engine")
	}
	runtime := NewRuntime(ResolverOptions{})
	networks, err := runtime.Networks(context.Background())
	if err != nil {
		t.Fatalf("list Docker networks: %v", err)
	}
	if len(networks) > 0 {
		details, err := runtime.NetworkDetails(context.Background(), networks[0].ID)
		if err != nil || details.ID == "" {
			t.Fatalf("inspect Docker network: details=%#v err=%v", details, err)
		}
	}
	volumes, err := runtime.Volumes(context.Background())
	if err != nil {
		t.Fatalf("list Docker volumes: %v", err)
	}
	if len(volumes) > 0 {
		details, err := runtime.VolumeDetails(context.Background(), volumes[0].Name)
		if err != nil || details.Name == "" {
			t.Fatalf("inspect Docker volume: details=%#v err=%v", details, err)
		}
	}
}

func TestLineWriterSplitsLinesAndRemovesCarriageReturns(t *testing.T) {
	lines := make(chan string, 3)
	writer := lineWriter{ctx: context.Background(), lines: lines}
	if _, err := writer.Write([]byte("stdout\nstderr\r\npartial")); err != nil {
		t.Fatalf("write logs: %v", err)
	}
	if err := writer.flush(); err != nil {
		t.Fatalf("flush logs: %v", err)
	}

	for index, want := range []string{"stdout", "stderr", "partial"} {
		if got := <-lines; got != want {
			t.Fatalf("line %d: expected %q, got %q", index, want, got)
		}
	}
}

func TestLineWriterStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := lineWriter{ctx: ctx, lines: make(chan string)}
	_, err := writer.Write([]byte("line\n"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled context, got %v", err)
	}
}

func TestToDomainContainerFallsBackToShortID(t *testing.T) {
	container := toDomainContainer(mobycontainer.Summary{ID: "abcdef1234567890"})

	if container.Name != "abcdef123456" {
		t.Fatalf("expected short id fallback name, got %q", container.Name)
	}
}

func TestToDomainContainerHandlesUnnamedContainer(t *testing.T) {
	container := toDomainContainer(mobycontainer.Summary{})

	if container.Name != "<unnamed>" {
		t.Fatalf("expected unnamed fallback, got %q", container.Name)
	}
}

func TestToDomainContainerKeepsComposeMetadataLabels(t *testing.T) {
	container := toDomainContainer(mobycontainer.Summary{Labels: map[string]string{
		"com.docker.compose.project":              "app",
		"com.docker.compose.service":              "web",
		"com.docker.compose.project.working_dir":  " /srv/app ",
		"com.docker.compose.project.config_files": " compose.yaml ",
		"com.docker.compose.oneoff":               "True",
	}})
	if container.ComposeProject != "app" || container.ComposeService != "web" || container.ComposeWorkingDir != "/srv/app" || container.ComposeConfigFiles != "compose.yaml" || !container.ComposeOneOff {
		t.Fatalf("expected Compose labels in snapshot container, got %#v", container)
	}
}

func TestComposeStacksGroupsMixedProjectsAndFallsBackToContainerName(t *testing.T) {
	stacks := composeStacks([]mobycontainer.Summary{
		{ID: "one", Names: []string{"/web-1"}, State: "running", Labels: map[string]string{"com.docker.compose.project": "app", "com.docker.compose.service": "web"}},
		{ID: "two", Names: []string{"/worker-1"}, State: "exited", Labels: map[string]string{"com.docker.compose.project": "app", "com.docker.compose.service": "worker"}},
		{ID: "three", Names: []string{"/orphan-1"}, State: "running", Labels: map[string]string{"com.docker.compose.project": "app"}},
	})
	if len(stacks) != 1 || stacks[0].State != "mixed" || stacks[0].Containers != 3 || len(stacks[0].Services) != 3 {
		t.Fatalf("unexpected stacks: %#v", stacks)
	}
	if stacks[0].Services[0].Name != "orphan-1" || stacks[0].Services[0].State != "running" {
		t.Fatalf("expected fallback service, got %#v", stacks[0].Services)
	}
}

func TestComposeStacksReadsAndNormalizesLabelMetadata(t *testing.T) {
	stacks := composeStacks([]mobycontainer.Summary{{State: "running", Labels: map[string]string{"com.docker.compose.project": "app", "com.docker.compose.project.working_dir": "/srv/app", "com.docker.compose.project.config_files": "compose.yaml, compose.prod.yaml"}}})
	if len(stacks) != 1 || stacks[0].DownUnavailableReason() != "" || !reflect.DeepEqual(stacks[0].Files, []string{"/srv/app/compose.yaml", "/srv/app/compose.prod.yaml"}) {
		t.Fatalf("unexpected label metadata: %#v", stacks)
	}
}

func TestComposeDownArgsAreExplicitAndNeverRemoveVolumes(t *testing.T) {
	args := composeDownArgs(domain.Stack{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml", "/srv/app/compose.prod.yaml"}})
	want := []string{"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "-f", "/srv/app/compose.prod.yaml", "down"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v want=%v", args, want)
	}
	for _, arg := range args {
		if arg == "--volumes" {
			t.Fatal("down must not remove volumes")
		}
	}
}

func TestComposeLifecycleArgsAreExplicit(t *testing.T) {
	stack := domain.Stack{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	for operation, want := range map[string][]string{
		"up":      {"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "up", "-d"},
		"pull":    {"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "pull"},
		"stop":    {"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "stop"},
		"restart": {"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "restart"},
	} {
		args := composeArgs(stack, operation, map[string][]string{"up": {"-d"}}[operation]...)
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("%s args=%v want=%v", operation, args, want)
		}
		for _, arg := range args {
			if arg == "--volumes" {
				t.Fatalf("%s must not remove volumes", operation)
			}
		}
	}
}

func TestComposeServiceUpdateArgsRemainScoped(t *testing.T) {
	stack := domain.Stack{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	pull := composeArgs(stack, "pull", "web")
	up := composeArgs(stack, "up", "-d", "--no-deps", "web")
	if !reflect.DeepEqual(pull, []string{"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "pull", "web"}) {
		t.Fatalf("scoped pull args=%v", pull)
	}
	if !reflect.DeepEqual(up, []string{"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "up", "-d", "--no-deps", "web"}) {
		t.Fatalf("scoped up args=%v", up)
	}
}

func TestPreserveAnonymousVolumesUsesExistingNames(t *testing.T) {
	configured := []mobymount.Mount{{Type: mobymount.TypeBind, Source: "/srv/config", Target: "/config"}, {Type: mobymount.TypeVolume, Source: "named", Target: "/named"}}
	inspected := []mobycontainer.MountPoint{{Type: mobymount.TypeVolume, Name: "anonymous-id", Destination: "/data", RW: true}, {Type: mobymount.TypeVolume, Name: "named", Destination: "/named", RW: true}}

	got := preserveAnonymousVolumes(configured, []string{"named:/named"}, inspected)
	if len(got) != 3 || got[2].Source != "anonymous-id" || got[2].Target != "/data" || got[2].ReadOnly {
		t.Fatalf("anonymous volume was not preserved: %#v", got)
	}
}

func TestComposeLogArgsUseExplicitMetadataAndNoColor(t *testing.T) {
	args := composeLogArgs(domain.Stack{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}, 100)
	want := []string{"compose", "--project-name", "app", "--project-directory", "/srv/app", "-f", "/srv/app/compose.yaml", "logs", "--tail", "100", "--follow", "--no-color"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v want=%v", args, want)
	}
}

func TestCalculateCPUPercent(t *testing.T) {
	previous := mobycontainer.CPUStats{
		CPUUsage:    mobycontainer.CPUUsage{TotalUsage: 100},
		SystemUsage: 500,
	}
	current := mobycontainer.CPUStats{
		CPUUsage:    mobycontainer.CPUUsage{TotalUsage: 200},
		SystemUsage: 1000,
		OnlineCPUs:  2,
	}

	percent, available := calculateCPUPercent(current, previous, 4, true)
	if !available {
		t.Fatal("expected CPU metric available")
	}
	if percent != 40 {
		t.Fatalf("expected 40%% CPU, got %.2f", percent)
	}
}

func TestCalculateCPUPercentRequiresPreviousSample(t *testing.T) {
	percent, available := calculateCPUPercent(mobycontainer.CPUStats{}, mobycontainer.CPUStats{}, 4, false)
	if available || percent != 0 {
		t.Fatalf("expected unavailable zero CPU, got %.2f available=%v", percent, available)
	}
}

func TestMemoryUsageExcludesCgroupCache(t *testing.T) {
	tests := []struct {
		name  string
		stats map[string]uint64
	}{
		{name: "cgroup v1", stats: map[string]uint64{"total_inactive_file": 200}},
		{name: "cgroup v2", stats: map[string]uint64{"inactive_file": 200}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := memoryUsage(mobycontainer.MemoryStats{Usage: 1000, Stats: test.stats})
			if got != 800 {
				t.Fatalf("expected 800 bytes without cache, got %d", got)
			}
		})
	}
}

func TestParseStartedAt(t *testing.T) {
	startedAt, err := parseStartedAt("2026-08-09T12:30:45.123456789Z")
	if err != nil {
		t.Fatalf("parse started at: %v", err)
	}
	if startedAt.IsZero() {
		t.Fatal("expected non-zero started time")
	}

	if _, err := parseStartedAt("not-a-time"); err == nil {
		t.Fatal("expected invalid timestamp error")
	}
}
