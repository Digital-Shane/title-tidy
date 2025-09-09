package cmd

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure custom naming formats",
	Long: `Launch the interactive configuration UI to customize naming formats.
	
This command opens a terminal interface for configuring show, season, episode, 
and movie naming templates, as well as TMDB API settings and other options.`,
	RunE: runConfigCommand,
}

func runConfigCommand(cmd *cobra.Command, args []string) error {
	model, err := tui.New()
	if err != nil {
		return fmt.Errorf("failed to initialize config UI: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run config UI: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
}
