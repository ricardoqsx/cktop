package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/docker"
	stateadapter "github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/state"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/i18n"
	dtoptui "github.com/ricardoqsx/cktop/apps/dtop/internal/presentation/tui"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/sanitize"
)

func main() {
	options, err := parseCLI(os.Args[1:], os.Stdout)
	if isHelp(err) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %s\n", sanitize.TerminalLine(err.Error()))
		os.Exit(2)
	}
	if options.version {
		fmt.Fprintln(os.Stdout, versionText())
		return
	}

	settings, err := config.LoadWithPath(options.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %s\n", sanitize.TerminalLine(err.Error()))
		os.Exit(1)
	}

	runtime := docker.NewRuntime(docker.ResolverOptions{})
	composeUpdates, stateErr := stateadapter.NewComposeUpdates(stateadapter.DefaultComposeUpdatesPath())
	if stateErr != nil {
		settings.ComposeDiagnostics = append(settings.ComposeDiagnostics, fmt.Sprintf("Compose update state unavailable: %v", stateErr))
	}
	projects := make([]application.ComposeProject, len(settings.ComposeProjects))
	for index, project := range settings.ComposeProjects {
		projects[index] = application.ComposeProject{Name: project.Name, WorkingDir: project.WorkingDir, Files: project.Files, MissingFiles: project.MissingFiles}
	}
	service := application.NewContainerServiceWithComposeUpdates(runtime, composeUpdates, projects...)
	updates := application.NewImageUpdateService(application.CommandExecutor(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}), application.UpdateOptions{Enabled: settings.Updates.Enabled, Interval: settings.Updates.Interval, Concurrency: settings.Updates.Concurrency})
	model := dtoptui.NewModelWithUpdatesAndLocalizer(service, settings.Display, updates, application.DockerHubLoginConfigured, i18n.NewFromEnvironment(), settings.ComposeDiagnostics...)

	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %s\n", sanitize.TerminalLine(err.Error()))
		os.Exit(1)
	}
}
