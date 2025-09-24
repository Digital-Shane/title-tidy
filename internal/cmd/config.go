package cmd

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	providerInit "github.com/Digital-Shane/title-tidy/internal/provider/init"
	"github.com/Digital-Shane/title-tidy/internal/tui"
	"github.com/charmbracelet/bubbletea"
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
	// Initialize providers
	if err := providerInit.LoadBuiltinProviders(); err != nil {
		return fmt.Errorf("failed to load providers: %w", err)
	}

	// Load config to get TMDB settings
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Configure TMDB if enabled and configured
	if cfg.EnableTMDBLookup && cfg.TMDBAPIKey != "" {
		tmdbConfig := map[string]interface{}{
			"api_key":       cfg.TMDBAPIKey,
			"language":      cfg.TMDBLanguage,
			"cache_enabled": true,
		}
		if err := provider.GlobalRegistry.Configure("tmdb", tmdbConfig); err == nil {
			// Only enable if configuration succeeded
			provider.GlobalRegistry.Enable("tmdb")
		}
	}

	// Configure OMDb if enabled and configured
	if cfg.EnableOMDBLookup && cfg.OMDBAPIKey != "" {
		omdbConfig := map[string]interface{}{
			"api_key": cfg.OMDBAPIKey,
		}
		if err := provider.GlobalRegistry.Configure("omdb", omdbConfig); err == nil {
			provider.GlobalRegistry.Enable("omdb")
		}
	}

	if cfg.EnableFFProbe {
		if err := provider.GlobalRegistry.Enable("ffprobe"); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to enable ffprobe provider: %v\n", err)
		}
	}

	// Create template registry
	templateReg := config.NewTemplateRegistry()

	// Register all enabled providers with template registry
	for _, name := range provider.GlobalRegistry.List() {
		if p, exists := provider.GlobalRegistry.Get(name); exists {
			templateReg.RegisterProvider(p)
		}
	}

	// Create UI model with registry
	model, err := tui.NewWithRegistry(templateReg)
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
