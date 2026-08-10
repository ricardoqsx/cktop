package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricardoqsx/cktop/libs/tui"
)

func main() {
	model := tui.NewShell(tui.ShellOptions{
		Title:    "ktop",
		Subtitle: "Kubernetes Cluster Status TUI",
		Views: []tui.View{
			{
				Title:   "Overview",
				Status:  tui.StatusUnavailable,
				Summary: "El desarrollo funcional de ktop comenzara despues de validar dtop.",
				Sections: []tui.Section{
					{
						Title: "Later",
						Body:  "K1: leer kubeconfig, construir Overview e informar degradacion sin Metrics API.",
					},
				},
			},
			{
				Title:   "Issues",
				Status:  tui.StatusUnavailable,
				Summary: "Pendiente hasta iniciar el producto Kubernetes.",
			},
		},
	})

	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ktop: %v\n", err)
		os.Exit(1)
	}
}
