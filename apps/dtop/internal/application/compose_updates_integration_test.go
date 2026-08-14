package application

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/docker"
	stateadapter "github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/state"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

func TestComposeUpdatePersistenceMutationIntegration(t *testing.T) {
	if os.Getenv("DTOP_MUTATION_INTEGRATION") != "1" {
		t.Skip("set DTOP_MUTATION_INTEGRATION=1 and DTOP_MUTATION_ENGINE_ID for destructive integration")
	}
	expectedEngine := strings.TrimSpace(os.Getenv("DTOP_MUTATION_ENGINE_ID"))
	actualEngine := strings.TrimSpace(runDockerIntegrationCommand(t, "info", "--format", "{{.ID}}"))
	if expectedEngine == "" || actualEngine != expectedEngine {
		t.Fatalf("refusing mutations: DTOP_MUTATION_ENGINE_ID=%q, active Engine ID=%q", expectedEngine, actualEngine)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	name := fmt.Sprintf("dtop-d4-%d", time.Now().UnixNano())
	reference := "alpine:3.20"
	originalReferenceID, _ := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Id}}", reference).Output()
	runDockerIntegrationCommandContext(t, ctx, "pull", "alpine:3.19")
	runDockerIntegrationCommandContext(t, ctx, "tag", "alpine:3.19", reference)
	t.Cleanup(func() {
		if original := strings.TrimSpace(string(originalReferenceID)); original != "" {
			_ = exec.Command("docker", "tag", original, reference).Run()
		} else {
			_ = exec.Command("docker", "image", "rm", reference).Run()
		}
	})

	workingDir := t.TempDir()
	composeFile := filepath.Join(workingDir, "compose.yaml")
	compose := []byte("services:\n  app:\n    image: " + reference + "\n    pull_policy: never\n    command: [\"sh\", \"-c\", \"while true; do sleep 3600; done\"]\n")
	if err := os.WriteFile(composeFile, compose, 0o600); err != nil {
		t.Fatal(err)
	}
	runDockerIntegrationCommandContext(t, ctx, "compose", "--project-name", name, "--project-directory", workingDir, "-f", composeFile, "up", "-d")
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", name, "--project-directory", workingDir, "-f", composeFile, "down", "--remove-orphans").Run()
	})
	compose = []byte("services:\n  app:\n    image: " + reference + "\n    command: [\"sh\", \"-c\", \"while true; do sleep 3600; done\"]\n")
	if err := os.WriteFile(composeFile, compose, 0o600); err != nil {
		t.Fatal(err)
	}

	stack := domain.Stack{Name: name, Registered: true, State: "running", WorkingDir: workingDir, Files: []string{composeFile}}
	statePath := filepath.Join(t.TempDir(), "compose-updates.json")
	store, err := stateadapter.NewComposeUpdates(statePath)
	if err != nil {
		t.Fatal(err)
	}
	runtime := docker.NewRuntime(docker.ResolverOptions{})
	service := NewContainerServiceWithComposeUpdates(runtime, store)
	if result := service.ActStacks(ctx, ActionPull, []domain.Stack{stack})[0]; result.Err != nil || !result.Pulled {
		t.Fatalf("pull result = %#v", result)
	}
	if result := service.ActStacks(ctx, ActionDown, []domain.Stack{stack})[0]; result.Err != nil {
		t.Fatalf("down result = %#v", result)
	}

	reopened, err := stateadapter.NewComposeUpdates(statePath)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewContainerServiceWithComposeUpdates(runtime, reopened)
	if result := restarted.ActStacks(ctx, ActionUp, []domain.Stack{stack})[0]; result.Err == nil {
		t.Fatal("plain Up bypassed persisted pending update")
	}
	if result := restarted.ActStacks(ctx, ActionApply, []domain.Stack{stack})[0]; result.Err != nil || !result.Applied {
		t.Fatalf("apply result = %#v", result)
	}
	project, found := reopened.Get(name)
	if !found || project.Pending() {
		t.Fatalf("final persisted state = %#v", project)
	}
}

func runDockerIntegrationCommand(t *testing.T, args ...string) string {
	t.Helper()
	return runDockerIntegrationCommandContext(t, context.Background(), args...)
}

func runDockerIntegrationCommandContext(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output)
}
