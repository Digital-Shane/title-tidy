package cmd

import (
	"fmt"
	configui "github.com/Digital-Shane/title-tidy/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// RunConfigCommand launches the configuration UI
func RunConfigCommand(args []string) error {
	model, err := configui.New()
	if err != nil {
		return fmt.Errorf("failed to initialize config UI: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run config UI: %w", err)
	}

	return nil
}
