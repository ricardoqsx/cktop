package application

import (
	"context"
	"fmt"
	"strings"
)

type PruneKind string

const (
	PruneContainers PruneKind = "containers"
	PruneImages     PruneKind = "images"
	PruneNetworks   PruneKind = "networks"
	PruneVolumes    PruneKind = "volumes"
	PruneSystem     PruneKind = "system"
)

type PruneResult struct {
	Kind    PruneKind
	Command []string
	Output  string
	Err     error
}

func PruneCommand(kind PruneKind) ([]string, bool) {
	var args []string
	switch kind {
	case PruneContainers:
		args = []string{"container", "prune", "--force"}
	case PruneImages:
		args = []string{"image", "prune", "--all", "--force"}
	case PruneNetworks:
		args = []string{"network", "prune", "--force"}
	case PruneVolumes:
		args = []string{"volume", "prune", "--force"}
	case PruneSystem:
		args = []string{"system", "prune", "--all", "--force"}
	default:
		return nil, false
	}
	return append([]string{"docker"}, args...), true
}

func PruneCommandText(kind PruneKind) string {
	command, ok := PruneCommand(kind)
	if !ok {
		return ""
	}
	return strings.Join(command, " ")
}

func (s ContainerService) Prune(ctx context.Context, kind PruneKind) PruneResult {
	command, ok := PruneCommand(kind)
	result := PruneResult{Kind: kind, Command: command}
	if !ok {
		result.Err = fmt.Errorf("unsupported prune kind %q", kind)
		return result
	}
	result.Output, result.Err = s.runtime.Prune(ctx, command[1:]...)
	return result
}
