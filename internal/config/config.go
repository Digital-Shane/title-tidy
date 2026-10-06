package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview/v2"
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

	// External metadata supplied by metadata providers.
	Metadata *provider.Metadata

	// Configuration
	Config *FormatConfig
}

// FormatConfig holds the format templates for different media types
type FormatConfig struct {
	ShowFolder           string                    `json:"show_folder"`
	SeasonFolder         string                    `json:"season_folder"`
	Episode              string                    `json:"episode"`
	Movie                string                    `json:"movie"`
	PreserveExistingTags bool                      `json:"preserve_existing_tags"`
	FilenameReplacements map[string]string         `json:"filename_replacements"`
	LogRetentionDays     int                       `json:"log_retention_days"`
	EnableLogging        bool                      `json:"enable_logging"`
	MetadataWorkerCount  int                       `json:"metadata_worker_count"`
	EnableManualSearch   bool                      `json:"enable_manual_search"`
	Providers            map[string]ProviderConfig `json:"providers,omitempty"`
}

// ProviderConfig stores persisted settings for a metadata provider.
type ProviderConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// DefaultConfig returns the default format configuration
func DefaultConfig() *FormatConfig {
	return &FormatConfig{
		ShowFolder:           "{title} ({year})",
		SeasonFolder:         "Season {season}",
		Episode:              "S{season}E{episode}",
		Movie:                "{title} ({year})",
		PreserveExistingTags: false,
		FilenameReplacements: map[string]string{},
		LogRetentionDays:     30,
		EnableLogging:        true,
		MetadataWorkerCount:  10,
		EnableManualSearch:   false,
		Providers:            map[string]ProviderConfig{},
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

	var disk struct {
		FormatConfig

		TMDBAPIKey       string `json:"tmdb_api_key"`
		EnableTMDBLookup *bool  `json:"enable_tmdb_lookup"`
		TMDBLanguage     string `json:"tmdb_language"`
		TMDBWorkerCount  int    `json:"tmdb_worker_count"`
		OMDBAPIKey       string `json:"omdb_api_key"`
		EnableOMDBLookup *bool  `json:"enable_omdb_lookup"`
		TVDBAPIKey       string `json:"tvdb_api_key"`
		EnableTVDBLookup *bool  `json:"enable_tvdb_lookup"`
		EnableFFProbe    *bool  `json:"enable_ffprobe"`
	}
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	cfg := disk.FormatConfig
	if err := ValidateFilenameReplacements(cfg.FilenameReplacements); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
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
	if cfg.MetadataWorkerCount == 0 && disk.TMDBWorkerCount > 0 {
		cfg.MetadataWorkerCount = disk.TMDBWorkerCount
	}
	if cfg.MetadataWorkerCount == 0 {
		cfg.MetadataWorkerCount = defaults.MetadataWorkerCount
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	if cfg.FilenameReplacements == nil {
		cfg.FilenameReplacements = make(map[string]string)
	}
	if len(cfg.Providers) == 0 {
		cfg.applyLegacyProviderConfig(disk.TMDBAPIKey, disk.EnableTMDBLookup, disk.TMDBLanguage, disk.OMDBAPIKey, disk.EnableOMDBLookup, disk.TVDBAPIKey, disk.EnableTVDBLookup, disk.EnableFFProbe)
	}
	cfg.Providers = NormalizeProviderConfigs(cfg.Providers, nil)

	return &cfg, nil
}

// NeedsMetadata checks if any template uses variables that would benefit from metadata
func (cfg *FormatConfig) NeedsMetadata() bool {
	metadataVars := metadataVariableNames()
	if len(metadataVars) == 0 {
		return false
	}

	allTemplates := cfg.ShowFolder + cfg.SeasonFolder + cfg.Episode + cfg.Movie
	for _, name := range metadataVars {
		placeholder := "{" + name + "}"
		if strings.Contains(allTemplates, placeholder) {
			return true
		}
	}
	return false
}

var (
	metadataVarOnce  sync.Once
	metadataVarCache []string
)

func metadataVariableNames() []string {
	metadataVarOnce.Do(func() {
		_ = EnsureBuiltinProviders()
		providers := provider.GlobalRegistry.Providers()
		unique := make(map[string]struct{})
		for _, p := range providers {
			if p == nil {
				continue
			}
			for _, v := range p.SupportedVariables() {
				unique[v.Name] = struct{}{}
			}
		}

		metadataVarCache = make([]string, 0, len(unique))
		for name := range unique {
			metadataVarCache = append(metadataVarCache, name)
		}
	})

	return metadataVarCache
}

// Save writes the configuration to disk
func (cfg *FormatConfig) Save() error {
	if err := ValidateFilenameReplacements(cfg.FilenameReplacements); err != nil {
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg.Providers = NormalizeProviderConfigs(cfg.Providers, nil)
	if cfg.MetadataWorkerCount == 0 {
		cfg.MetadataWorkerCount = DefaultConfig().MetadataWorkerCount
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
	return cfg.resolveTemplate(cfg.ShowFolder, ctx)
}

// ApplySeasonFolderTemplate applies the season folder template using the provided context
func (cfg *FormatConfig) ApplySeasonFolderTemplate(ctx *FormatContext) string {
	return cfg.resolveTemplate(cfg.SeasonFolder, ctx)
}

// ApplyEpisodeTemplate applies the episode template using the provided context
func (cfg *FormatConfig) ApplyEpisodeTemplate(ctx *FormatContext) string {
	return cfg.resolveTemplate(cfg.Episode, ctx)
}

// ApplyMovieTemplate applies the movie template using the provided context
func (cfg *FormatConfig) ApplyMovieTemplate(ctx *FormatContext) string {
	return cfg.resolveTemplate(cfg.Movie, ctx)
}

func (cfg *FormatConfig) resolveTemplate(template string, ctx *FormatContext) string {
	if ctx == nil {
		ctx = &FormatContext{}
	}
	result, _ := ResolveTemplate(template, ctx, ctx.Metadata, nil)
	return result
}
