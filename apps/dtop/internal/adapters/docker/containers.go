package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	mobycontainer "github.com/moby/moby/api/types/container"
	mobyimage "github.com/moby/moby/api/types/image"
	mobynetwork "github.com/moby/moby/api/types/network"
	mobyvolume "github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

type Runtime struct {
	options  ResolverOptions
	command  string
	mu       sync.Mutex
	previous map[string]mobycontainer.CPUStats
}

func NewRuntime(options ResolverOptions) *Runtime {
	return &Runtime{options: options, command: "docker", previous: make(map[string]mobycontainer.CPUStats)}
}

func (r *Runtime) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	client, info, err := NewClient(ctx, r.options)
	if err != nil {
		return domain.Snapshot{}, err
	}
	defer client.Close()

	containers, err := r.containers(ctx, client, info.NCPU)
	if err != nil {
		return domain.Snapshot{}, err
	}

	snapshot := domain.Snapshot{Engine: toDomainEngine(info), Containers: containers}
	for _, container := range containers {
		if container.CPUAvailable {
			snapshot.ContainerCPUPercent += container.CPUPercent
			snapshot.CPUAvailable = true
		}
		if container.MemoryAvailable {
			snapshot.ContainerMemoryUsage += container.MemoryUsage
			snapshot.MemoryAvailable = true
		}
		if container.SampledAt.After(snapshot.SampledAt) {
			snapshot.SampledAt = container.SampledAt
		}
	}
	if info.NCPU > 0 {
		snapshot.ContainerCPUPercent /= float64(info.NCPU)
	}
	if snapshot.SampledAt.IsZero() {
		snapshot.SampledAt = time.Now()
	}

	return snapshot, nil
}

func (r *Runtime) Stacks(ctx context.Context) ([]domain.Stack, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()

	result, err := apiClient.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list Docker Compose containers: %w", err)
	}
	return composeStacks(result.Items), nil
}

func (r *Runtime) LoadResources(ctx context.Context) (ports.ResourceLoad, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return ports.ResourceLoad{}, err
	}
	defer apiClient.Close()

	resources := ports.ResourceLoad{}
	containers, containersErr := apiClient.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if containersErr != nil {
		resources.StacksErr = fmt.Errorf("list Docker Compose containers: %w", containersErr)
	}

	images, err := apiClient.api.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		resources.ImagesErr = fmt.Errorf("list Docker images: %w", err)
	} else {
		resources.Images = domainImages(images.Items)
	}

	networks, err := apiClient.api.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		resources.NetworksErr = fmt.Errorf("list Docker networks: %w", err)
	} else {
		resources.Networks = domainNetworks(networks.Items, containers.Items, containersErr == nil)
	}

	volumes, err := apiClient.api.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		resources.VolumesErr = fmt.Errorf("list Docker volumes: %w", err)
	} else {
		resources.Volumes = domainVolumes(volumes.Items, containers.Items, containersErr == nil)
	}

	if containersErr == nil {
		resources.Stacks = composeStacks(containers.Items)
	}
	return resources, nil
}

func (r *Runtime) Stop(ctx context.Context, id string) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.ContainerStop(ctx, id, client.ContainerStopOptions{}); err != nil {
		return fmt.Errorf("stop %s: %w", shortID(id), err)
	}

	return nil
}

func (r *Runtime) Images(ctx context.Context) ([]domain.Image, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()

	result, err := apiClient.api.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list Docker images: %w", err)
	}

	return domainImages(result.Items), nil
}

func (r *Runtime) ImageDetails(ctx context.Context, id string) (domain.ImageDetails, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return domain.ImageDetails{}, err
	}
	defer apiClient.Close()

	result, err := apiClient.api.ImageInspect(ctx, id)
	if err != nil {
		return domain.ImageDetails{}, fmt.Errorf("inspect image %s: %w", shortID(id), err)
	}
	return toDomainImageDetails(result.InspectResponse), nil
}

func (r *Runtime) Networks(ctx context.Context) ([]domain.Network, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()
	result, err := apiClient.api.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker networks: %w", err)
	}
	containers, err := apiClient.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	return domainNetworks(result.Items, containers.Items, err == nil), nil
}

func (r *Runtime) NetworkDetails(ctx context.Context, id string) (domain.NetworkDetails, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return domain.NetworkDetails{}, err
	}
	defer apiClient.Close()
	result, err := apiClient.api.NetworkInspect(ctx, id, client.NetworkInspectOptions{})
	if err != nil {
		return domain.NetworkDetails{}, fmt.Errorf("inspect network %s: %w", shortID(id), err)
	}
	return toDomainNetworkDetails(result.Network), nil
}

func (r *Runtime) Volumes(ctx context.Context) ([]domain.Volume, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()
	result, err := apiClient.api.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker volumes: %w", err)
	}
	containers, err := apiClient.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	return domainVolumes(result.Items, containers.Items, err == nil), nil
}

func (r *Runtime) VolumeDetails(ctx context.Context, name string) (domain.VolumeDetails, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return domain.VolumeDetails{}, err
	}
	defer apiClient.Close()
	result, err := apiClient.api.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if err != nil {
		return domain.VolumeDetails{}, fmt.Errorf("inspect volume %s: %w", name, err)
	}
	return toDomainVolumeDetails(result.Volume), nil
}

func (r *Runtime) Details(ctx context.Context, id string) (domain.ContainerDetails, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return domain.ContainerDetails{}, err
	}
	defer apiClient.Close()

	result, err := apiClient.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return domain.ContainerDetails{}, fmt.Errorf("inspect %s: %w", shortID(id), err)
	}

	return toDomainDetails(result.Container), nil
}

func (r *Runtime) Logs(ctx context.Context, id string, tail int) (ports.LogStream, error) {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return ports.LogStream{}, err
	}

	inspect, err := apiClient.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		apiClient.Close()
		return ports.LogStream{}, fmt.Errorf("inspect %s for logs: %w", shortID(id), err)
	}
	result, err := apiClient.api.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       strconv.Itoa(tail),
	})
	if err != nil {
		apiClient.Close()
		return ports.LogStream{}, fmt.Errorf("load logs for %s: %w", shortID(id), err)
	}

	lines := make(chan string, 32)
	errs := make(chan error, 1)
	go func() {
		defer apiClient.Close()
		defer result.Close()
		defer close(lines)
		defer close(errs)

		writer := lineWriter{ctx: ctx, lines: lines}
		var copyErr error
		if inspect.Container.Config != nil && inspect.Container.Config.Tty {
			_, copyErr = io.Copy(&writer, result)
		} else {
			_, copyErr = stdcopy.StdCopy(&writer, &writer, result)
		}
		if flushErr := writer.flush(); copyErr == nil {
			copyErr = flushErr
		}
		if copyErr != nil && ctx.Err() == nil {
			errs <- copyErr
		}
	}()

	return ports.LogStream{Lines: lines, Errors: errs}, nil
}

func (r *Runtime) Restart(ctx context.Context, id string) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.ContainerRestart(ctx, id, client.ContainerRestartOptions{}); err != nil {
		return fmt.Errorf("restart %s: %w", shortID(id), err)
	}

	return nil
}

func (r *Runtime) Remove(ctx context.Context, id string, force bool) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("remove %s: %w", shortID(id), err)
	}

	return nil
}

func (r *Runtime) RemoveImage(ctx context.Context, id string, force bool) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.ImageRemove(ctx, id, client.ImageRemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("remove image %s: %w", shortID(id), err)
	}

	return nil
}

func (r *Runtime) RemoveNetwork(ctx context.Context, id string) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.NetworkRemove(ctx, id, client.NetworkRemoveOptions{}); err != nil {
		return fmt.Errorf("remove network %s: %w", shortID(id), err)
	}

	return nil
}

func (r *Runtime) RemoveVolume(ctx context.Context, name string) error {
	apiClient, _, err := NewClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if _, err := apiClient.api.VolumeRemove(ctx, name, client.VolumeRemoveOptions{Force: false}); err != nil {
		return fmt.Errorf("remove volume %s: %w", name, err)
	}

	return nil
}

func (r *Runtime) containers(ctx context.Context, c *Client, onlineCPUs int) ([]domain.Container, error) {
	result, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list Docker containers: %w", err)
	}

	containers := make([]domain.Container, 0, len(result.Items))
	for _, item := range result.Items {
		containers = append(containers, toDomainContainer(item))
	}
	r.loadMetrics(ctx, c, containers, onlineCPUs)

	return containers, nil
}

func (r *Runtime) loadMetrics(ctx context.Context, c *Client, containers []domain.Container, onlineCPUs int) {
	const maxConcurrentStats = 8
	semaphore := make(chan struct{}, maxConcurrentStats)
	var wait sync.WaitGroup

	for index := range containers {
		if containers[index].State != "running" {
			continue
		}
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			container := containers[index]
			if startedAt, err := c.containerStartedAt(ctx, container.ID); err == nil {
				container.StartedAt = startedAt
			}
			stats, err := c.containerStats(ctx, container.ID)
			if err == nil {
				container = r.applyMetrics(container, stats, onlineCPUs)
			}
			containers[index] = container
		}(index)
	}
	wait.Wait()
}

func (c *Client) containerStartedAt(ctx context.Context, id string) (time.Time, error) {
	result, err := c.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return time.Time{}, err
	}
	if result.Container.State == nil || result.Container.State.StartedAt == "" {
		return time.Time{}, nil
	}

	startedAt, err := parseStartedAt(result.Container.State.StartedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse started time for %s: %w", shortID(id), err)
	}

	return startedAt, nil
}

func parseStartedAt(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339Nano, value)
}

func (c *Client) containerStats(ctx context.Context, id string) (mobycontainer.StatsResponse, error) {
	result, err := c.api.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: false})
	if err != nil {
		return mobycontainer.StatsResponse{}, err
	}
	defer result.Body.Close()

	var stats mobycontainer.StatsResponse
	if err := json.NewDecoder(result.Body).Decode(&stats); err != nil {
		return mobycontainer.StatsResponse{}, fmt.Errorf("decode stats for %s: %w", shortID(id), err)
	}

	return stats, nil
}

func (r *Runtime) applyMetrics(container domain.Container, stats mobycontainer.StatsResponse, engineCPUs int) domain.Container {
	r.mu.Lock()
	previous, found := r.previous[container.ID]
	r.previous[container.ID] = stats.CPUStats
	r.mu.Unlock()

	if stats.PreCPUStats.SystemUsage > 0 {
		previous = stats.PreCPUStats
		found = true
	}
	container.CPUPercent, container.CPUAvailable = calculateCPUPercent(stats.CPUStats, previous, engineCPUs, found)
	container.MemoryUsage = memoryUsage(stats.MemoryStats)
	container.MemoryLimit = stats.MemoryStats.Limit
	if container.MemoryLimit > 0 {
		container.MemoryPercent = float64(container.MemoryUsage) / float64(container.MemoryLimit) * 100
	}
	container.MemoryAvailable = true
	container.SampledAt = stats.Read
	if container.SampledAt.IsZero() {
		container.SampledAt = time.Now()
	}

	return container
}

func calculateCPUPercent(current, previous mobycontainer.CPUStats, engineCPUs int, hasPrevious bool) (float64, bool) {
	if !hasPrevious || current.CPUUsage.TotalUsage < previous.CPUUsage.TotalUsage || current.SystemUsage <= previous.SystemUsage {
		return 0, false
	}

	onlineCPUs := int(current.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = len(current.CPUUsage.PercpuUsage)
	}
	if onlineCPUs == 0 {
		onlineCPUs = engineCPUs
	}
	if onlineCPUs == 0 {
		return 0, false
	}

	cpuDelta := current.CPUUsage.TotalUsage - previous.CPUUsage.TotalUsage
	systemDelta := current.SystemUsage - previous.SystemUsage
	return float64(cpuDelta) / float64(systemDelta) * float64(onlineCPUs) * 100, true
}

func memoryUsage(memory mobycontainer.MemoryStats) uint64 {
	cache, ok := memory.Stats["total_inactive_file"]
	if !ok {
		cache = memory.Stats["inactive_file"]
	}
	if cache < memory.Usage {
		return memory.Usage - cache
	}

	return memory.Usage
}

func toDomainEngine(info ConnectionInfo) domain.EngineInfo {
	return domain.EngineInfo{
		Name:            info.Name,
		Endpoint:        info.Endpoint,
		Transport:       info.Transport,
		Remote:          info.Remote,
		Secure:          info.Secure,
		Source:          info.Source,
		ServerVersion:   info.ServerVersion,
		APIVersion:      info.APIVersion,
		OperatingSystem: info.OperatingSystem,
		NCPU:            info.NCPU,
		MemoryTotal:     info.MemoryTotal,
	}
}

func toDomainContainer(summary mobycontainer.Summary) domain.Container {
	return domain.Container{
		ID:                 summary.ID,
		ShortID:            shortID(summary.ID),
		Name:               containerName(summary),
		Image:              summary.Image,
		State:              string(summary.State),
		Status:             summary.Status,
		Health:             containerHealth(summary),
		Created:            time.Unix(summary.Created, 0),
		ComposeProject:     strings.TrimSpace(summary.Labels["com.docker.compose.project"]),
		ComposeService:     strings.TrimSpace(summary.Labels["com.docker.compose.service"]),
		ComposeWorkingDir:  strings.TrimSpace(summary.Labels["com.docker.compose.project.working_dir"]),
		ComposeConfigFiles: strings.TrimSpace(summary.Labels["com.docker.compose.project.config_files"]),
	}
}

func toDomainImage(summary mobyimage.Summary) domain.Image {
	tags := append([]string(nil), summary.RepoTags...)
	sort.Strings(tags)
	name := "<untagged>"
	if len(tags) > 0 {
		name = tags[0]
	}
	return domain.Image{
		ID:         summary.ID,
		ShortID:    shortID(strings.TrimPrefix(summary.ID, "sha256:")),
		Name:       name,
		Tags:       tags,
		Size:       nonNegativeSize(summary.Size),
		Created:    time.Unix(summary.Created, 0),
		Containers: summary.Containers,
		UsageKnown: summary.Containers >= 0,
		Dangling:   len(tags) == 0,
	}
}

func domainImages(summaries []mobyimage.Summary) []domain.Image {
	images := make([]domain.Image, 0, len(summaries))
	for _, image := range summaries {
		images = append(images, toDomainImage(image))
	}
	sort.SliceStable(images, func(i, j int) bool {
		if images[i].Dangling != images[j].Dangling {
			return !images[i].Dangling
		}
		return strings.ToLower(images[i].Name) < strings.ToLower(images[j].Name)
	})
	return images
}

func toDomainImageDetails(inspect mobyimage.InspectResponse) domain.ImageDetails {
	created, _ := parseStartedAt(inspect.Created)
	return domain.ImageDetails{
		ID:           inspect.ID,
		Tags:         append([]string(nil), inspect.RepoTags...),
		Digests:      append([]string(nil), inspect.RepoDigests...),
		Size:         nonNegativeSize(inspect.Size),
		Created:      created,
		Architecture: inspect.Architecture,
		OS:           inspect.Os,
	}
}

func toDomainNetwork(summary mobynetwork.Summary, containers int, usageKnown bool) domain.Network {
	return domain.Network{ID: summary.ID, ShortID: shortID(summary.ID), Name: summary.Name, Driver: summary.Driver, Scope: summary.Scope, Created: summary.Created, Containers: containers, UsageKnown: usageKnown, Internal: summary.Internal, Attachable: summary.Attachable}
}

func domainNetworks(networks []mobynetwork.Summary, containers []mobycontainer.Summary, usageKnown bool) []domain.Network {
	counts := make(map[string]int)
	if usageKnown {
		for _, container := range containers {
			if container.NetworkSettings == nil {
				continue
			}
			for _, endpoint := range container.NetworkSettings.Networks {
				if endpoint.NetworkID != "" {
					counts[endpoint.NetworkID]++
				}
			}
		}
	}
	result := make([]domain.Network, 0, len(networks))
	for _, network := range networks {
		result = append(result, toDomainNetwork(network, counts[network.ID], usageKnown))
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func toDomainNetworkDetails(inspect mobynetwork.Inspect) domain.NetworkDetails {
	containers := make([]string, 0, len(inspect.Containers))
	for id, container := range inspect.Containers {
		name := container.Name
		if name == "" {
			name = shortID(id)
		}
		containers = append(containers, name)
	}
	sort.Strings(containers)
	return domain.NetworkDetails{ID: inspect.ID, Name: inspect.Name, Driver: inspect.Driver, Scope: inspect.Scope, Created: inspect.Created, Internal: inspect.Internal, Attachable: inspect.Attachable, Containers: containers}
}

func toDomainVolume(volume mobyvolume.Volume, containers int, usageKnown bool) domain.Volume {
	created, _ := parseStartedAt(volume.CreatedAt)
	return domain.Volume{Name: volume.Name, Driver: volume.Driver, Scope: volume.Scope, Mountpoint: volume.Mountpoint, Created: created, Containers: containers, UsageKnown: usageKnown}
}

func domainVolumes(volumes []mobyvolume.Volume, containers []mobycontainer.Summary, usageKnown bool) []domain.Volume {
	counts := make(map[string]int)
	if usageKnown {
		for _, container := range containers {
			for _, mount := range container.Mounts {
				if mount.Name != "" && string(mount.Type) == "volume" {
					counts[mount.Name]++
				}
			}
		}
	}
	result := make([]domain.Volume, 0, len(volumes))
	for _, volume := range volumes {
		result = append(result, toDomainVolume(volume, counts[volume.Name], usageKnown))
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func toDomainVolumeDetails(volume mobyvolume.Volume) domain.VolumeDetails {
	created, _ := parseStartedAt(volume.CreatedAt)
	return domain.VolumeDetails{Name: volume.Name, Driver: volume.Driver, Scope: volume.Scope, Mountpoint: volume.Mountpoint, Created: created, Options: volume.Options, Labels: volume.Labels}
}

func composeStacks(containers []mobycontainer.Summary) []domain.Stack {
	type serviceCounts struct {
		running, stopped int
	}
	type stackCounts struct {
		running, stopped int
		services         map[string]*serviceCounts
		workingDir       string
		files            string
	}
	projects := make(map[string]*stackCounts)
	for _, container := range containers {
		project := strings.TrimSpace(container.Labels["com.docker.compose.project"])
		if project == "" {
			continue
		}
		stack := projects[project]
		if stack == nil {
			stack = &stackCounts{services: make(map[string]*serviceCounts)}
			projects[project] = stack
		}
		if stack.workingDir == "" {
			stack.workingDir = container.Labels["com.docker.compose.project.working_dir"]
		}
		if stack.files == "" {
			stack.files = container.Labels["com.docker.compose.project.config_files"]
		}
		service := strings.TrimSpace(container.Labels["com.docker.compose.service"])
		if service == "" {
			service = containerName(container)
		}
		counts := stack.services[service]
		if counts == nil {
			counts = &serviceCounts{}
			stack.services[service] = counts
		}
		if container.State == "running" {
			stack.running++
			counts.running++
		} else {
			stack.stopped++
			counts.stopped++
		}
	}

	stacks := make([]domain.Stack, 0, len(projects))
	for name, counts := range projects {
		workingDir, files, reason := domain.NormalizeComposeMetadata(counts.workingDir, counts.files)
		stack := domain.Stack{Name: name, State: aggregateStackState(counts.running, counts.stopped), Containers: counts.running + counts.stopped, WorkingDir: workingDir, Files: files, MetadataReason: reason}
		for serviceName, service := range counts.services {
			stack.Services = append(stack.Services, domain.StackService{Name: serviceName, State: aggregateStackState(service.running, service.stopped), Containers: service.running + service.stopped})
		}
		sort.Slice(stack.Services, func(i, j int) bool {
			return strings.ToLower(stack.Services[i].Name) < strings.ToLower(stack.Services[j].Name)
		})
		stacks = append(stacks, stack)
	}
	sort.Slice(stacks, func(i, j int) bool { return strings.ToLower(stacks[i].Name) < strings.ToLower(stacks[j].Name) })
	return stacks
}

func aggregateStackState(running, stopped int) string {
	if running == 0 {
		return "stopped"
	}
	if stopped == 0 {
		return "running"
	}
	return "mixed"
}

func nonNegativeSize(size int64) uint64 {
	if size < 0 {
		return 0
	}
	return uint64(size)
}

func toDomainDetails(inspect mobycontainer.InspectResponse) domain.ContainerDetails {
	details := domain.ContainerDetails{ID: inspect.ID, Name: strings.TrimPrefix(inspect.Name, "/")}
	if inspect.Config != nil {
		details.Image = inspect.Config.Image
	}
	if inspect.State != nil {
		details.State = string(inspect.State.Status)
		details.Health = "-"
		if inspect.State.Health != nil {
			details.Health = string(inspect.State.Health.Status)
		}
		details.StartedAt, _ = parseStartedAt(inspect.State.StartedAt)
	}
	if inspect.NetworkSettings != nil {
		for port, bindings := range inspect.NetworkSettings.Ports {
			if len(bindings) == 0 {
				details.Ports = append(details.Ports, port.String())
				continue
			}
			for _, binding := range bindings {
				details.Ports = append(details.Ports, binding.HostIP.String()+":"+binding.HostPort+" -> "+port.String())
			}
		}
		for name := range inspect.NetworkSettings.Networks {
			details.Networks = append(details.Networks, name)
		}
	}
	sort.Strings(details.Ports)
	sort.Strings(details.Networks)
	return details
}

type lineWriter struct {
	ctx       context.Context
	lines     chan<- string
	remainder string
}

func (w *lineWriter) Write(data []byte) (int, error) {
	w.remainder += string(data)
	for {
		index := strings.IndexByte(w.remainder, '\n')
		if index < 0 {
			return len(data), nil
		}
		line := strings.TrimSuffix(w.remainder[:index], "\r")
		w.remainder = w.remainder[index+1:]
		select {
		case w.lines <- line:
		case <-w.ctx.Done():
			return len(data), w.ctx.Err()
		}
	}
}

func (w *lineWriter) flush() error {
	if w.remainder == "" {
		return nil
	}
	select {
	case w.lines <- strings.TrimSuffix(w.remainder, "\r"):
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

func containerName(summary mobycontainer.Summary) string {
	for _, name := range summary.Names {
		clean := strings.TrimPrefix(name, "/")
		if clean != "" {
			return clean
		}
	}

	if summary.ID != "" {
		return shortID(summary.ID)
	}

	return "<unnamed>"
}

func containerHealth(summary mobycontainer.Summary) string {
	if summary.Health == nil {
		return "-"
	}

	return string(summary.Health.Status)
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}

	return id[:12]
}
