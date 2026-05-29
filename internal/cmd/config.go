package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/Digital-Shane/title-tidy/internal/config"
	tuiconfig "github.com/Digital-Shane/title-tidy/internal/tui/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure custom naming formats",
	Long: `Launch the interactive configuration UI to customize naming formats.
	
This command opens a terminal interface for configuring show, season, episode, 
and movie naming templates, as well as metadata provider settings and other options.`,
	RunE: runConfigCommand,
}

func runConfigCommand(cmd *cobra.Command, args []string) error {
	if err := config.EnsureBuiltinProviders(); err != nil {
		return fmt.Errorf("failed to load providers: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	for _, err := range cfg.ConfigureProviderRegistry(nil) {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to configure provider: %v\n", err)
	}

	model, err := tuiconfig.New()
	if err != nil {
		return fmt.Errorf("failed to initialize config UI: %w", err)
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run config UI: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
}
