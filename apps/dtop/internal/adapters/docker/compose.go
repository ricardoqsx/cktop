package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

const (
	composeTimeout           = 60 * time.Second
	composeConfigOutputLimit = 1 << 20
)

var commandContext = exec.CommandContext
var composeClient = NewClient

// compose executes a local Compose lifecycle command directly, never through a shell.
func (r *Runtime) compose(ctx context.Context, stack domain.Stack, operation string, extra ...string) error {
	if reason := stack.DownUnavailableReason(); reason != "" {
		return fmt.Errorf("%s %s: %s", operation, stack.Name, reason)
	}
	client, info, err := composeClient(ctx, r.options)
	if err != nil {
		return err
	}
	defer client.Close()
	if info.Remote {
		return domain.ErrRemoteUnsupported
	}

	commandCtx, cancel := context.WithTimeout(ctx, composeTimeout)
	defer cancel()
	args := composeArgs(stack, operation, extra...)
	path := r.command
	if path == "" {
		path = "docker"
	}
	command := commandContext(commandCtx, path, args...)
	command.Dir = stack.WorkingDir
	var output limitedOutput
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if commandCtx.Err() != nil {
			return commandCtx.Err()
		}
		message := output.String()
		if message != "" {
			return fmt.Errorf("docker compose %s %q: %w: %s", operation, stack.Name, err, message)
		}
		return fmt.Errorf("docker compose %s %q: %w", operation, stack.Name, err)
	}
	return nil
}

func (r *Runtime) Up(ctx context.Context, stack domain.Stack) error {
	return r.compose(ctx, stack, "up", "-d")
}
func (r *Runtime) StopStack(ctx context.Context, stack domain.Stack) error {
	return r.compose(ctx, stack, "stop")
}
func (r *Runtime) RestartStack(ctx context.Context, stack domain.Stack) error {
	return r.compose(ctx, stack, "restart")
}
func (r *Runtime) PullStack(ctx context.Context, stack domain.Stack) error {
	return r.compose(ctx, stack, "pull")
}
func (r *Runtime) PullStackServices(ctx context.Context, stack domain.Stack, services []string) error {
	return r.compose(ctx, stack, "pull", services...)
}
func (r *Runtime) UpStackServices(ctx context.Context, stack domain.Stack, services []string) error {
	extra := append([]string{"-d", "--no-deps"}, services...)
	return r.compose(ctx, stack, "up", extra...)
}
func (r *Runtime) Down(ctx context.Context, stack domain.Stack) error {
	return r.compose(ctx, stack, "down")
}

func composeArgs(stack domain.Stack, operation string, extra ...string) []string {
	args := []string{"compose", "--project-name", stack.Name, "--project-directory", stack.WorkingDir}
	for _, file := range stack.Files {
		args = append(args, "-f", file)
	}
	return append(args, append([]string{operation}, extra...)...)
}

func composeDownArgs(stack domain.Stack) []string {
	return composeArgs(stack, "down")
}

func composeLogArgs(stack domain.Stack, tail int) []string {
	args := composeArgs(stack, "logs", "--tail", fmt.Sprintf("%d", tail), "--follow", "--no-color")
	return args
}

func composeConfigArgs(stack domain.Stack) []string {
	return composeArgs(stack, "config", "--format", "json")
}

func (r *Runtime) ComposeConfig(ctx context.Context, stack domain.Stack) ([]ports.ComposeServiceImage, error) {
	if reason := stack.DownUnavailableReason(); reason != "" {
		return nil, fmt.Errorf("config %s: %s", stack.Name, reason)
	}
	client, info, err := composeClient(ctx, r.options)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if info.Remote {
		return nil, domain.ErrRemoteUnsupported
	}

	commandCtx, cancel := context.WithTimeout(ctx, composeTimeout)
	defer cancel()
	path := r.command
	if path == "" {
		path = "docker"
	}
	command := commandContext(commandCtx, path, composeConfigArgs(stack)...)
	command.Dir = stack.WorkingDir
	stdout := boundedOutput{limit: composeConfigOutputLimit}
	var stderr limitedOutput
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if commandCtx.Err() != nil {
			return nil, commandCtx.Err()
		}
		if stdout.exceeded {
			return nil, fmt.Errorf("docker compose config %q: output exceeds %d bytes", stack.Name, composeConfigOutputLimit)
		}
		message := stderr.String()
		if message != "" {
			return nil, fmt.Errorf("docker compose config %q: %w: %s", stack.Name, err, message)
		}
		return nil, fmt.Errorf("docker compose config %q: %w", stack.Name, err)
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("docker compose config %q: output exceeds %d bytes", stack.Name, composeConfigOutputLimit)
	}
	images, err := parseComposeConfig(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("docker compose config %q: %w", stack.Name, err)
	}
	return images, nil
}

type composeServiceImages []ports.ComposeServiceImage

func (images *composeServiceImages) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return fmt.Errorf("services must be an object")
	}

	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok || strings.TrimSpace(name) == "" {
			return fmt.Errorf("service name must not be empty")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate service name")
		}
		seen[name] = struct{}{}

		var service struct {
			Image      string          `json:"image"`
			Build      json.RawMessage `json:"build"`
			PullPolicy string          `json:"pull_policy"`
		}
		if err := decoder.Decode(&service); err != nil {
			return err
		}
		hasBuild := len(service.Build) > 0 && string(service.Build) != "null"
		*images = append(*images, ports.ComposeServiceImage{Service: name, Reference: service.Image, Build: hasBuild, PullPolicy: service.PullPolicy})
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}

func parseComposeConfig(data []byte) ([]ports.ComposeServiceImage, error) {
	config := struct {
		Services composeServiceImages `json:"services"`
	}{Services: make(composeServiceImages, 0)}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid Docker Compose config JSON: %w", err)
	}
	images := []ports.ComposeServiceImage(config.Services)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Service < images[j].Service
	})
	return images, nil
}

func (r *Runtime) ComposeLogs(ctx context.Context, stack domain.Stack, tail int) (ports.LogStream, error) {
	if reason := stack.DownUnavailableReason(); reason != "" {
		return ports.LogStream{}, fmt.Errorf("logs %s: %s", stack.Name, reason)
	}
	client, info, err := composeClient(ctx, r.options)
	if err != nil {
		return ports.LogStream{}, err
	}
	client.Close()
	if info.Remote {
		return ports.LogStream{}, domain.ErrRemoteUnsupported
	}
	path := r.command
	if path == "" {
		path = "docker"
	}
	command := commandContext(ctx, path, composeLogArgs(stack, tail)...)
	command.Dir = stack.WorkingDir
	stdout, err := command.StdoutPipe()
	if err != nil {
		return ports.LogStream{}, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return ports.LogStream{}, err
	}
	if err := command.Start(); err != nil {
		return ports.LogStream{}, fmt.Errorf("docker compose logs %q: %w", stack.Name, err)
	}
	lines := make(chan string, 32)
	errs := make(chan error, 1)
	go func() {
		defer close(lines)
		defer close(errs)
		reportError := func(err error) {
			select {
			case errs <- err:
			default:
			}
		}
		var readers sync.WaitGroup
		readers.Add(2)
		for _, reader := range []io.Reader{stdout, stderr} {
			go func(reader io.Reader) {
				defer readers.Done()
				scanner := bufio.NewScanner(reader)
				scanner.Buffer(make([]byte, 1024), 64*1024)
				for scanner.Scan() {
					select {
					case lines <- scanner.Text():
					case <-ctx.Done():
						return
					}
				}
				if err := scanner.Err(); err != nil && ctx.Err() == nil {
					reportError(err)
				}
			}(reader)
		}
		readers.Wait()
		if err := command.Wait(); err != nil && ctx.Err() == nil {
			reportError(fmt.Errorf("docker compose logs %q: %w", stack.Name, err))
		}
	}()
	return ports.LogStream{Lines: lines, Errors: errs}, nil
}

type limitedOutput struct{ bytes.Buffer }

func (w *limitedOutput) Write(data []byte) (int, error) {
	const limit = 8192
	originalLength := len(data)
	remaining := limit - w.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = w.Buffer.Write(data)
	}
	return originalLength, nil
}

func (w *limitedOutput) String() string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, w.Buffer.String()))
}

type boundedOutput struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (w *boundedOutput) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := w.limit - w.Len()
	if len(data) > remaining {
		w.exceeded = true
	}
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = w.Buffer.Write(data)
	}
	return originalLength, nil
}
