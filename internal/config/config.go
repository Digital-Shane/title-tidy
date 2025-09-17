package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// FormatContext holds all the contextual information needed for formatting media names.
// This allows us to easily extend with new metadata without changing function signatures.
type FormatContext struct {
	// Core identifiers
	ShowName  string
	MovieName string
	Year      string
	Season    int
	Episode   int

	// File information
	OriginalName string
	Node         *treeview.Node[treeview.FileInfo]

	// External metadata (from providers like TMDB, TVDB, etc.)
	Metadata *provider.Metadata

	// Configuration
	Config *FormatConfig
}

// FormatConfig holds the format templates for different media types
type FormatConfig struct {
	ShowFolder       string `json:"show_folder"`
	SeasonFolder     string `json:"season_folder"`
	Episode          string `json:"episode"`
	Movie            string `json:"movie"`
	LogRetentionDays int    `json:"log_retention_days"`
	EnableLogging    bool   `json:"enable_logging"`

	// TMDB Integration settings
	TMDBAPIKey          string `json:"tmdb_api_key"`
	EnableTMDBLookup    bool   `json:"enable_tmdb_lookup"`
	TMDBLanguage        string `json:"tmdb_language"`
	PreferLocalMetadata bool   `json:"prefer_local_metadata"`
	TMDBWorkerCount     int    `json:"tmdb_worker_count"`

	// Template resolver for dynamic variable resolution
	resolver *TemplateResolver
}

// DefaultConfig returns the default format configuration
func DefaultConfig() *FormatConfig {
	return &FormatConfig{
		ShowFolder:          "{title} ({year})",
		SeasonFolder:        "Season {season}",
		Episode:             "S{season}E{episode}",
		Movie:               "{title} ({year})",
		LogRetentionDays:    30,
		EnableLogging:       true,
		TMDBAPIKey:          "",
		EnableTMDBLookup:    false,
		TMDBLanguage:        "en-US",
		PreferLocalMetadata: true,
		TMDBWorkerCount:     10,
		resolver:            NewTemplateResolver(),
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".title-tidy", "config.json"), nil
}

// Load reads the configuration from disk
func Load() (*FormatConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg FormatConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Fill in any missing fields with defaults
	defaults := DefaultConfig()
	if cfg.ShowFolder == "" {
		cfg.ShowFolder = defaults.ShowFolder
	}
	if cfg.SeasonFolder == "" {
		cfg.SeasonFolder = defaults.SeasonFolder
	}
	if cfg.Episode == "" {
		cfg.Episode = defaults.Episode
	}
	if cfg.Movie == "" {
		cfg.Movie = defaults.Movie
	}
	if cfg.LogRetentionDays == 0 {
		cfg.LogRetentionDays = defaults.LogRetentionDays
	}

	// Fill in missing TMDB fields with defaults
	if cfg.TMDBLanguage == "" {
		cfg.TMDBLanguage = defaults.TMDBLanguage
	}
	if cfg.TMDBWorkerCount == 0 {
		cfg.TMDBWorkerCount = defaults.TMDBWorkerCount
	}

	// Initialize the template resolver
	cfg.resolver = NewTemplateResolver()

	return &cfg, nil
}

// NeedsMetadata checks if any template uses variables that would benefit from metadata
func (cfg *FormatConfig) NeedsMetadata() bool {
	// Pattern matches any metadata-related variable
	// Includes both metadata-only vars and vars that benefit from metadata
	metadataVarPattern := `\{(?:episode_title|air_date|rating|genres|runtime|tagline|title)\}`
	re := regexp.MustCompile(metadataVarPattern)

	// Combine all templates to check
	allTemplates := cfg.ShowFolder + cfg.SeasonFolder + cfg.Episode + cfg.Movie

	// Check if any template contains metadata variables
	return re.MatchString(allTemplates)
}

// Save writes the configuration to disk
func (cfg *FormatConfig) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ApplyShowFolderTemplate applies the show folder template using the provided context
func (cfg *FormatConfig) ApplyShowFolderTemplate(ctx *FormatContext) string {
	// Ensure resolver is initialized
	if cfg.resolver == nil {
		cfg.resolver = NewTemplateResolver()
	}

	result, _ := cfg.resolver.Resolve(cfg.ShowFolder, ctx, ctx.Metadata, nil)
	return result
}

// ApplySeasonFolderTemplate applies the season folder template using the provided context
func (cfg *FormatConfig) ApplySeasonFolderTemplate(ctx *FormatContext) string {
	// Ensure resolver is initialized
	if cfg.resolver == nil {
		cfg.resolver = NewTemplateResolver()
	}

	result, _ := cfg.resolver.Resolve(cfg.SeasonFolder, ctx, ctx.Metadata, nil)
	return result
}

// ApplyEpisodeTemplate applies the episode template using the provided context
func (cfg *FormatConfig) ApplyEpisodeTemplate(ctx *FormatContext) string {
	// Ensure resolver is initialized
	if cfg.resolver == nil {
		cfg.resolver = NewTemplateResolver()
	}

	result, _ := cfg.resolver.Resolve(cfg.Episode, ctx, ctx.Metadata, nil)
	return result
}

// ApplyMovieTemplate applies the movie template using the provided context
func (cfg *FormatConfig) ApplyMovieTemplate(ctx *FormatContext) string {
	// Ensure resolver is initialized
	if cfg.resolver == nil {
		cfg.resolver = NewTemplateResolver()
	}

	result, _ := cfg.resolver.Resolve(cfg.Movie, ctx, ctx.Metadata, nil)
	return result
}

// Deprecated: ExtractNameAndYear has been moved to internal/provider/local.
// This stub is kept for documentation purposes only.
// Use local.ExtractNameAndYear instead.
