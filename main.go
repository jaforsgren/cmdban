package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/config"
	"cmdban/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.EnsureTaskDirectory(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating task directory: %v\n", err)
		os.Exit(1)
	}

	model := ui.NewModel(cfg)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
