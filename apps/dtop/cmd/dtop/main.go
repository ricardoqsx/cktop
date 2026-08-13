package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/docker"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
	dtoptui "github.com/ricardoqsx/cktop/apps/dtop/internal/presentation/tui"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %v\n", err)
		os.Exit(1)
	}

	runtime := docker.NewRuntime(docker.ResolverOptions{})
	projects := make([]application.ComposeProject, len(settings.ComposeProjects))
	for index, project := range settings.ComposeProjects {
		projects[index] = application.ComposeProject{Name: project.Name, WorkingDir: project.WorkingDir, Files: project.Files, MissingFiles: project.MissingFiles}
	}
	service := application.NewContainerService(runtime, projects...)
	updates := application.NewImageUpdateService(application.CommandExecutor(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}), application.UpdateOptions{Enabled: settings.Updates.Enabled, Interval: settings.Updates.Interval, Concurrency: settings.Updates.Concurrency})
	model := dtoptui.NewModelWithUpdatesAndLocalizer(service, settings.Display, updates, application.DockerHubLoginConfigured, i18n.NewFromEnvironment(), settings.ComposeDiagnostics...)

	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %v\n", err)
		os.Exit(1)
	}
}
