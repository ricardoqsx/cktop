package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

type SortMode string

const (
	SortState  SortMode = "state"
	SortCPU    SortMode = "cpu"
	SortMemory SortMode = "memory"
	SortName   SortMode = "name"
)

type Action string

const (
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionDelete  Action = "delete"
	ActionDown    Action = "down"
	ActionUp      Action = "up"
)

type ActionResult struct {
	ID     string
	Action Action
	Err    error
}

type ContainerService struct {
	runtime         ports.Runtime
	composeProjects []ComposeProject
}

type ComposeProject struct {
	Name         string
	WorkingDir   string
	Files        []string
	MissingFiles []string
}

func NewContainerService(runtime ports.Runtime, composeProjects ...ComposeProject) ContainerService {
	return ContainerService{runtime: runtime, composeProjects: composeProjects}
}

func (s ContainerService) Load(ctx context.Context) (domain.Snapshot, error) {
	return s.runtime.Snapshot(ctx)
}

func (s ContainerService) LoadResources(ctx context.Context) (ports.ResourceLoad, error) {
	resources, err := s.runtime.LoadResources(ctx)
	resources.Stacks = MergeStacks(resources.Stacks, s.composeProjects)
	return resources, err
}

func EnrichStacks(stacks []domain.Stack, snapshot domain.Snapshot) []domain.Stack {
	result := append([]domain.Stack(nil), stacks...)
	byProject := make(map[string]int, len(result))
	for index := range result {
		result[index].CPUPercent = 0
		result[index].CPUAvailable = false
		result[index].MemoryUsage = 0
		result[index].MemoryLimit = 0
		result[index].MemoryAvailable = false
		result[index].ContainerItems = nil
		byProject[result[index].Name] = index
	}
	for _, container := range snapshot.Containers {
		index, found := byProject[container.ComposeProject]
		if !found {
			continue
		}
		result[index].ContainerItems = append(result[index].ContainerItems, container)
		if container.State != "running" {
			continue
		}
		if container.CPUAvailable {
			result[index].CPUPercent += container.CPUPercent
			result[index].CPUAvailable = true
		}
		if container.MemoryAvailable {
			result[index].MemoryUsage += container.MemoryUsage
			result[index].MemoryLimit += container.MemoryLimit
			result[index].MemoryAvailable = true
		}
	}
	for index := range result {
		sort.SliceStable(result[index].ContainerItems, func(i, j int) bool {
			left, right := result[index].ContainerItems[i], result[index].ContainerItems[j]
			if strings.ToLower(left.ComposeService) != strings.ToLower(right.ComposeService) {
				return strings.ToLower(left.ComposeService) < strings.ToLower(right.ComposeService)
			}
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		})
	}
	return result
}

// RebuildStacks derives Compose projects from the current snapshot, then applies
// registered project metadata so stopped registered projects remain visible.
func RebuildStacks(snapshot domain.Snapshot, projects []ComposeProject) []domain.Stack {
	type serviceCounts struct {
		running int
		stopped int
	}
	type stackCounts struct {
		running    int
		stopped    int
		services   map[string]*serviceCounts
		workingDir string
		files      string
	}

	containers := append([]domain.Container(nil), snapshot.Containers...)
	sort.SliceStable(containers, func(i, j int) bool {
		left, right := containers[i], containers[j]
		if strings.ToLower(left.ComposeProject) != strings.ToLower(right.ComposeProject) {
			return strings.ToLower(left.ComposeProject) < strings.ToLower(right.ComposeProject)
		}
		if strings.ToLower(left.ComposeService) != strings.ToLower(right.ComposeService) {
			return strings.ToLower(left.ComposeService) < strings.ToLower(right.ComposeService)
		}
		if strings.ToLower(left.Name) != strings.ToLower(right.Name) {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		return left.ID < right.ID
	})

	projectsByName := make(map[string]*stackCounts)
	for _, container := range containers {
		project := strings.TrimSpace(container.ComposeProject)
		if project == "" {
			continue
		}
		stack := projectsByName[project]
		if stack == nil {
			stack = &stackCounts{services: make(map[string]*serviceCounts)}
			projectsByName[project] = stack
		}
		if stack.workingDir == "" {
			stack.workingDir = container.ComposeWorkingDir
		}
		if stack.files == "" {
			stack.files = container.ComposeConfigFiles
		}
		service := strings.TrimSpace(container.ComposeService)
		if service == "" {
			service = container.Name
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

	detected := make([]domain.Stack, 0, len(projectsByName))
	for name, counts := range projectsByName {
		workingDir, files, reason := domain.NormalizeComposeMetadata(counts.workingDir, counts.files)
		stack := domain.Stack{Name: name, State: aggregateStackState(counts.running, counts.stopped), Containers: counts.running + counts.stopped, WorkingDir: workingDir, Files: files, MetadataReason: reason}
		for serviceName, service := range counts.services {
			stack.Services = append(stack.Services, domain.StackService{Name: serviceName, State: aggregateStackState(service.running, service.stopped), Containers: service.running + service.stopped})
		}
		sort.SliceStable(stack.Services, func(i, j int) bool {
			return strings.ToLower(stack.Services[i].Name) < strings.ToLower(stack.Services[j].Name)
		})
		detected = append(detected, stack)
	}

	return EnrichStacks(MergeStacks(detected, projects), snapshot)
}

func (s ContainerService) RebuildStacks(snapshot domain.Snapshot) []domain.Stack {
	return RebuildStacks(snapshot, s.composeProjects)
}

func (s ContainerService) Stacks(ctx context.Context) ([]domain.Stack, error) {
	stacks, err := s.runtime.Stacks(ctx)
	return MergeStacks(stacks, s.composeProjects), err
}

func MergeStacks(detected []domain.Stack, projects []ComposeProject) []domain.Stack {
	stacks := append([]domain.Stack(nil), detected...)
	byName := make(map[string]int, len(stacks))
	for index, stack := range stacks {
		byName[stack.Name] = index
	}
	for _, project := range projects {
		if index, found := byName[project.Name]; found {
			stacks[index].WorkingDir = project.WorkingDir
			stacks[index].Files = append([]string(nil), project.Files...)
			stacks[index].MetadataReason = ""
			if len(project.MissingFiles) > 0 {
				stacks[index].MetadataReason = "registered Compose config file is unavailable"
			}
			continue
		}
		state := "down"
		if len(project.MissingFiles) > 0 {
			state = "missing compose file"
		}
		reason := ""
		if len(project.MissingFiles) > 0 {
			reason = "registered Compose config file is unavailable"
		}
		stacks = append(stacks, domain.Stack{Name: project.Name, State: state, WorkingDir: project.WorkingDir, Files: append([]string(nil), project.Files...), MetadataReason: reason})
	}
	sort.SliceStable(stacks, func(i, j int) bool { return strings.ToLower(stacks[i].Name) < strings.ToLower(stacks[j].Name) })
	return stacks
}

func (s ContainerService) DownStacks(ctx context.Context, stacks []domain.Stack) []ActionResult {
	return s.actStacks(ctx, ActionDown, stacks)
}

func (s ContainerService) ActStacks(ctx context.Context, action Action, stacks []domain.Stack) []ActionResult {
	return s.actStacks(ctx, action, stacks)
}

func (s ContainerService) actStacks(ctx context.Context, action Action, stacks []domain.Stack) []ActionResult {
	results := make([]ActionResult, 0, len(stacks))
	for _, stack := range stacks {
		result := ActionResult{ID: stack.Name, Action: action}
		if reason := stack.DownUnavailableReason(); reason != "" {
			result.Err = fmt.Errorf("%s unavailable: %s", action, reason)
		} else {
			switch action {
			case ActionUp:
				result.Err = s.runtime.Up(ctx, stack)
			case ActionStop:
				result.Err = s.runtime.StopStack(ctx, stack)
			case ActionRestart:
				result.Err = s.runtime.RestartStack(ctx, stack)
			case ActionDown:
				result.Err = s.runtime.Down(ctx, stack)
			default:
				result.Err = fmt.Errorf("unsupported stack action %q", action)
			}
		}
		results = append(results, result)
	}
	return results
}

func (s ContainerService) Images(ctx context.Context) ([]domain.Image, error) {
	return s.runtime.Images(ctx)
}

func (s ContainerService) ImageDetails(ctx context.Context, id string) (domain.ImageDetails, error) {
	return s.runtime.ImageDetails(ctx, id)
}

func (s ContainerService) Networks(ctx context.Context) ([]domain.Network, error) {
	return s.runtime.Networks(ctx)
}

func (s ContainerService) NetworkDetails(ctx context.Context, id string) (domain.NetworkDetails, error) {
	return s.runtime.NetworkDetails(ctx, id)
}

func (s ContainerService) Volumes(ctx context.Context) ([]domain.Volume, error) {
	return s.runtime.Volumes(ctx)
}

func (s ContainerService) VolumeDetails(ctx context.Context, name string) (domain.VolumeDetails, error) {
	return s.runtime.VolumeDetails(ctx, name)
}

func (s ContainerService) Details(ctx context.Context, id string) (domain.ContainerDetails, error) {
	return s.runtime.Details(ctx, id)
}

func (s ContainerService) Logs(ctx context.Context, id string, tail int) (ports.LogStream, error) {
	return s.runtime.Logs(ctx, id, tail)
}

func (s ContainerService) ComposeLogs(ctx context.Context, stack domain.Stack, tail int) (ports.LogStream, error) {
	return s.runtime.ComposeLogs(ctx, stack, tail)
}

func (s ContainerService) Act(ctx context.Context, action Action, ids []string) []ActionResult {
	results := make([]ActionResult, 0, len(ids))
	for _, id := range ids {
		result := ActionResult{ID: id, Action: action}
		switch action {
		case ActionStop:
			result.Err = s.runtime.Stop(ctx, id)
		case ActionRestart:
			result.Err = s.runtime.Restart(ctx, id)
		case ActionDelete:
			result.Err = s.runtime.Remove(ctx, id, true)
		default:
			result.Err = fmt.Errorf("unsupported action %q", action)
		}
		results = append(results, result)
	}

	return results
}

func (s ContainerService) RemoveImages(ctx context.Context, ids []string) []ActionResult {
	results := make([]ActionResult, 0, len(ids))
	for _, id := range ids {
		results = append(results, ActionResult{ID: id, Action: ActionDelete, Err: s.runtime.RemoveImage(ctx, id, false)})
	}

	return results
}

func (s ContainerService) RemoveNetworks(ctx context.Context, ids []string) []ActionResult {
	results := make([]ActionResult, 0, len(ids))
	for _, id := range ids {
		results = append(results, ActionResult{ID: id, Action: ActionDelete, Err: s.runtime.RemoveNetwork(ctx, id)})
	}

	return results
}

func (s ContainerService) RemoveVolumes(ctx context.Context, names []string) []ActionResult {
	results := make([]ActionResult, 0, len(names))
	for _, name := range names {
		results = append(results, ActionResult{ID: name, Action: ActionDelete, Err: s.runtime.RemoveVolume(ctx, name)})
	}

	return results
}

func (s ContainerService) Sort(snapshot domain.Snapshot, mode SortMode) domain.Snapshot {
	containers := append([]domain.Container(nil), snapshot.Containers...)
	sort.SliceStable(containers, func(i, j int) bool {
		left, right := containers[i], containers[j]
		switch mode {
		case SortCPU:
			if left.CPUAvailable != right.CPUAvailable {
				return left.CPUAvailable
			}
			if left.CPUPercent != right.CPUPercent {
				return left.CPUPercent > right.CPUPercent
			}
		case SortMemory:
			if left.MemoryAvailable != right.MemoryAvailable {
				return left.MemoryAvailable
			}
			if left.MemoryUsage != right.MemoryUsage {
				return left.MemoryUsage > right.MemoryUsage
			}
		case SortName:
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		default:
			if stateRank(left.State) != stateRank(right.State) {
				return stateRank(left.State) < stateRank(right.State)
			}
		}

		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	snapshot.Containers = containers

	return snapshot
}

func NextSortMode(mode SortMode) SortMode {
	switch mode {
	case SortState:
		return SortCPU
	case SortCPU:
		return SortMemory
	case SortMemory:
		return SortName
	default:
		return SortState
	}
}

func stateRank(state string) int {
	switch strings.ToLower(state) {
	case "running":
		return 0
	case "restarting":
		return 1
	case "paused":
		return 2
	case "created":
		return 3
	case "exited":
		return 4
	case "dead", "removing":
		return 5
	default:
		return 6
	}
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
