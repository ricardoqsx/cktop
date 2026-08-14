package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

func (r *Runtime) Prune(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("docker prune: missing arguments")
	}
	apiClient, info, err := NewClient(ctx, r.options)
	if err != nil {
		return "", err
	}
	apiClient.Close()
	if info.Remote {
		return "", domain.ErrRemoteUnsupported
	}

	commandCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	path := r.command
	if path == "" {
		path = "docker"
	}
	command := commandContext(commandCtx, path, args...)
	var output limitedOutput
	command.Stdout = &output
	command.Stderr = &output
	err = command.Run()
	if commandCtx.Err() != nil {
		return output.String(), commandCtx.Err()
	}
	if err != nil {
		return output.String(), fmt.Errorf("docker %s: %w", args[0], err)
	}
	return output.String(), nil
}
