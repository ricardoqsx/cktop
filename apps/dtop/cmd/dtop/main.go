package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/adapters/docker"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/application"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/config"
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
	model := dtoptui.NewModel(service, settings.Display.MemoryMode, settings.ComposeDiagnostics...)

	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dtop: %v\n", err)
		os.Exit(1)
	}
}
