package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

var (
	// yearRangeRe extracts a year or year range; only the first year is used in output.
	yearRangeRe = regexp.MustCompile(`\b((19|20)\d{2})(?:[\s\-–—]+(?:19|20)\d{2})?\b`)
	// encodingTagsRe removes codec/resolution/source tags to isolate the series title.
	encodingTagsRe = regexp.MustCompile(`(?i)\b(?:HD|HDR|DV|x265|x264|H\.?264|H\.?265|HEVC|AVC|AAC|AC3|DD|DTS|FLAC|MP3|WEB-?DL|BluRay|BDRip|DVDRip|HDTV|720p|1080p|2160p|4K|UHD|SDR|10bit|8bit|PROPER|REPACK|iNTERNAL|LiMiTED|UNRATED|EXTENDED|DiRECTORS?\.?CUT|THEATRICAL|COMPLETE|SEASON|SERIES|MULTI|DUAL|DUBBED|SUBBED|SUB|RETAIL|WS|FS|NTSC|PAL|R[1-6]|UNCUT|UNCENSORED)\b`)
	// emptyBracketsRe matches any empty brackets (with optional spaces inside)
	emptyBracketsRe = regexp.MustCompile(`\s*[\(\[\{<]\s*[\)\]\}>]`)
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
	Metadata *provider.EnrichedMetadata

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
}

// DefaultConfig returns the default format configuration
func DefaultConfig() *FormatConfig {
	return &FormatConfig{
		ShowFolder:          "{show} ({year})",
		SeasonFolder:        "{season_name}",
		Episode:             "{season_code}{episode_code}",
		Movie:               "{movie} ({year})",
		LogRetentionDays:    30,
		EnableLogging:       true,
		TMDBAPIKey:          "",
		EnableTMDBLookup:    false,
		TMDBLanguage:        "en-US",
		PreferLocalMetadata: true,
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

	return &cfg, nil
}

// NeedsMetadata checks if any template uses variables that would benefit from metadata
func (cfg *FormatConfig) NeedsMetadata() bool {
	// Pattern matches any metadata-related variable
	// Includes both metadata-only vars and vars that benefit from metadata
	metadataVarPattern := `\{(?:episode_title|air_date|rating|genres|overview|runtime|tagline|title)\}`
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

// CleanName removes empty brackets, trims spaces and separators
func CleanName(name string) string {
	result := name

	// Keep removing empty brackets until none remain (handles nested cases)
	for emptyBracketsRe.MatchString(result) {
		result = emptyBracketsRe.ReplaceAllString(result, "")
	}

	// Clean up template artifacts
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "-")
	result = strings.TrimSuffix(result, "-")
	result = strings.TrimSpace(result)

	return result
}

// applyShowVariables replaces show and year variables in a template string
func applyShowVariables(template, show, year string) string {
	result := strings.ReplaceAll(template, "{show}", show)
	result = strings.ReplaceAll(result, "{year}", year)
	return result
}

// applySeasonVariables replaces season variables in a template string
func applySeasonVariables(template string, season int) string {
	result := strings.ReplaceAll(template, "{season}", fmt.Sprintf("%02d", season))
	result = strings.ReplaceAll(result, "{season_code}", fmt.Sprintf("S%02d", season))
	result = strings.ReplaceAll(result, "{season_name}", fmt.Sprintf("Season %02d", season))
	return result
}

// applyEpisodeVariables replaces episode variables in a template string
func applyEpisodeVariables(template string, episode int) string {
	result := strings.ReplaceAll(template, "{episode}", fmt.Sprintf("%02d", episode))
	result = strings.ReplaceAll(result, "{episode_code}", fmt.Sprintf("E%02d", episode))
	return result
}

// resolveVariable resolves a single variable using metadata if available, falling back to local data
func resolveVariable(varName string, ctx *FormatContext) string {
	// Check if we should prefer local metadata
	preferLocal := ctx.Config != nil && ctx.Config.PreferLocalMetadata

	switch varName {
	case "{show}":
		if !preferLocal && ctx.Metadata != nil && ctx.Metadata.ShowName != "" {
			return ctx.Metadata.ShowName
		}
		return ctx.ShowName

	case "{movie}":
		if !preferLocal && ctx.Metadata != nil && ctx.Metadata.Title != "" {
			return ctx.Metadata.Title
		}
		return ctx.MovieName

	case "{title}":
		// Generic title - works for both movies and shows
		if !preferLocal && ctx.Metadata != nil {
			if ctx.Metadata.Title != "" {
				return ctx.Metadata.Title
			}
			if ctx.Metadata.ShowName != "" {
				return ctx.Metadata.ShowName
			}
		}
		if ctx.MovieName != "" {
			return ctx.MovieName
		}
		return ctx.ShowName

	case "{year}":
		if !preferLocal && ctx.Metadata != nil && ctx.Metadata.Year != "" {
			return ctx.Metadata.Year
		}
		return ctx.Year

	case "{season}":
		if ctx.Season > 0 {
			return fmt.Sprintf("%02d", ctx.Season)
		}
		return ""

	case "{season_code}":
		if ctx.Season > 0 {
			return fmt.Sprintf("S%02d", ctx.Season)
		}
		return ""

	case "{season_name}":
		if ctx.Metadata != nil && ctx.Metadata.SeasonName != "" && !preferLocal {
			return ctx.Metadata.SeasonName
		}
		if ctx.Season > 0 {
			return fmt.Sprintf("Season %02d", ctx.Season)
		}
		return ""

	case "{episode}":
		if ctx.Episode > 0 {
			return fmt.Sprintf("%02d", ctx.Episode)
		}
		return ""

	case "{episode_code}":
		if ctx.Episode > 0 {
			return fmt.Sprintf("E%02d", ctx.Episode)
		}
		return ""

	case "{episode_title}":
		if ctx.Metadata != nil && ctx.Metadata.EpisodeName != "" {
			return ctx.Metadata.EpisodeName
		}
		return ""

	case "{air_date}":
		if ctx.Metadata != nil && ctx.Metadata.EpisodeAir != "" {
			return ctx.Metadata.EpisodeAir
		}
		return ""

	case "{rating}":
		if ctx.Metadata != nil && ctx.Metadata.Rating > 0 {
			return fmt.Sprintf("%.1f", ctx.Metadata.Rating)
		}
		return ""

	case "{genres}":
		if ctx.Metadata != nil && len(ctx.Metadata.Genres) > 0 {
			return strings.Join(ctx.Metadata.Genres, ", ")
		}
		return ""

	case "{overview}":
		if ctx.Metadata != nil && ctx.Metadata.Overview != "" {
			// Truncate to reasonable length for filenames
			overview := ctx.Metadata.Overview
			if len(overview) > 100 {
				overview = overview[:97] + "..."
			}
			return overview
		}
		return ""

	case "{runtime}":
		if ctx.Metadata != nil && ctx.Metadata.Runtime > 0 {
			return fmt.Sprintf("%d", ctx.Metadata.Runtime)
		}
		return ""

	case "{tagline}":
		if ctx.Metadata != nil && ctx.Metadata.Tagline != "" {
			return ctx.Metadata.Tagline
		}
		return ""
	}

	return varName // Return unchanged if not recognized
}

// applyMetadataVariables replaces all variables in a template using metadata-aware resolution
func applyMetadataVariables(template string, ctx *FormatContext, cfg *FormatConfig) string {
	// Ensure config is set
	if ctx.Config == nil {
		ctx.Config = cfg
	}

	result := template

	// List of all supported variables
	variables := []string{
		"{show}", "{movie}", "{title}", "{year}",
		"{season}", "{season_code}", "{season_name}",
		"{episode}", "{episode_code}", "{episode_title}",
		"{air_date}", "{rating}", "{genres}",
		"{overview}", "{runtime}", "{tagline}",
	}

	// Replace each variable
	for _, varName := range variables {
		if strings.Contains(result, varName) {
			value := resolveVariable(varName, ctx)
			result = strings.ReplaceAll(result, varName, value)
		}
	}

	return result
}

// ApplyShowFolderTemplate applies the show folder template using the provided context
func (cfg *FormatConfig) ApplyShowFolderTemplate(ctx *FormatContext) string {
	result := applyMetadataVariables(cfg.ShowFolder, ctx, cfg)
	return CleanName(result)
}

// ApplySeasonFolderTemplate applies the season folder template using the provided context
func (cfg *FormatConfig) ApplySeasonFolderTemplate(ctx *FormatContext) string {
	result := applyMetadataVariables(cfg.SeasonFolder, ctx, cfg)
	return CleanName(result)
}

// ApplyEpisodeTemplate applies the episode template using the provided context
func (cfg *FormatConfig) ApplyEpisodeTemplate(ctx *FormatContext) string {
	result := applyMetadataVariables(cfg.Episode, ctx, cfg)
	return CleanName(result)
}

// ApplyMovieTemplate applies the movie template using the provided context
func (cfg *FormatConfig) ApplyMovieTemplate(ctx *FormatContext) string {
	result := applyMetadataVariables(cfg.Movie, ctx, cfg)
	return CleanName(result)
}

// ExtractNameAndYear cleans a filename and extracts the name and year components.
// Returns the cleaned name and year (year may be empty).
func ExtractNameAndYear(name string) (string, string) {
	if name == "" {
		return name, ""
	}

	formatted := name
	year := ""

	// First, look for a year or year range in the name
	yearMatches := yearRangeRe.FindStringSubmatch(formatted)

	if len(yearMatches) > 1 {
		// Extract just the first year from the match (yearMatches[1] is the full first year)
		year = yearMatches[1]

		// Find the position of the year match
		yearIndex := strings.Index(formatted, yearMatches[0])
		if yearIndex != -1 {
			// Keep only the part before the year
			formatted = formatted[:yearIndex]
			formatted = strings.TrimRight(formatted, " ([{-_")
		}
	}

	// Replace separators with spaces
	formatted = strings.ReplaceAll(formatted, ".", " ")
	formatted = strings.ReplaceAll(formatted, "-", " ")
	formatted = strings.ReplaceAll(formatted, "_", " ")

	// Remove common encoding tags
	formatted = encodingTagsRe.ReplaceAllString(formatted, "")

	// Clean up extra spaces
	formatted = strings.TrimSpace(strings.Join(strings.Fields(formatted), " "))

	return formatted, year
}
