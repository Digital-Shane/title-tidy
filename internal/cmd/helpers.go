package cmd

import (
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// extractShowNameFromPath extracts show name and year from a file or folder path
// by finding where the season/episode pattern starts and cleaning everything before it
func extractShowNameFromPath(path string, removeExtension bool) (showName, year string) {
	workingPath := path

	// Remove extension if needed (for files)
	if removeExtension {
		ext := media.ExtractExtension(path)
		if ext != "" {
			workingPath = path[:len(path)-len(ext)]
		}
	}

	// Find where the season/episode pattern starts
	if idx := media.FindSeasonEpisodeIndex(workingPath); idx > 0 {
		// Extract everything before the pattern
		showPart := workingPath[:idx]
		// ExtractNameAndYear handles all the complex parsing
		showName, year = config.ExtractNameAndYear(showPart)
	} else {
		// No pattern found, just clean the whole name
		showName, year = config.ExtractNameAndYear(workingPath)
	}

	return showName, year
}

// fetchShowMetadata searches for a TV show and returns its metadata
func fetchShowMetadata(provider *provider.TMDBProvider, showName string) *provider.EnrichedMetadata {
	if provider == nil || showName == "" {
		return nil
	}

	showMeta, err := provider.SearchTVShow(showName)
	if err != nil || showMeta == nil {
		return nil
	}

	return showMeta
}

// fetchSeasonMetadata gets season-specific metadata if show metadata is available
func fetchSeasonMetadata(provider *provider.TMDBProvider, showMeta *provider.EnrichedMetadata, season int) *provider.EnrichedMetadata {
	if provider == nil || showMeta == nil || showMeta.ID == 0 {
		return nil
	}

	seasonMeta, err := provider.GetSeasonInfo(showMeta.ID, season)
	if err != nil || seasonMeta == nil {
		return nil
	}

	// Preserve show information
	seasonMeta.ShowName = showMeta.ShowName
	if seasonMeta.ShowName == "" {
		seasonMeta.ShowName = showMeta.Title
	}

	return seasonMeta
}

// fetchEpisodeMetadata gets episode-specific metadata if show metadata is available
func fetchEpisodeMetadata(provider *provider.TMDBProvider, showMeta *provider.EnrichedMetadata, season, episode int) *provider.EnrichedMetadata {
	if provider == nil || showMeta == nil || showMeta.ID == 0 {
		return nil
	}

	episodeMeta, err := provider.GetEpisodeInfo(showMeta.ID, season, episode)
	if err != nil || episodeMeta == nil {
		return nil
	}

	// Preserve show information
	episodeMeta.ShowName = showMeta.ShowName
	if episodeMeta.ShowName == "" {
		episodeMeta.ShowName = showMeta.Title
	}

	return episodeMeta
}

// createFormatContext creates a FormatContext with all the provided information
func createFormatContext(cfg *config.FormatConfig, showName, movieName, year string, season, episode int, metadata *provider.EnrichedMetadata) *config.FormatContext {
	return &config.FormatContext{
		ShowName:  showName,
		MovieName: movieName,
		Year:      year,
		Season:    season,
		Episode:   episode,
		Metadata:  metadata,
		Config:    cfg,
	}
}

// setDestinationPath sets the destination path for a node if linking is enabled
func setDestinationPath(node *treeview.Node[treeview.FileInfo], linkPath, parentPath, newName string) {
	if linkPath == "" {
		return
	}

	meta := core.GetMeta(node)
	if meta == nil {
		return
	}

	if parentPath != "" {
		meta.DestinationPath = filepath.Join(parentPath, newName)
	} else {
		meta.DestinationPath = filepath.Join(linkPath, newName)
	}
}

// applyEpisodeRename handles the common episode renaming logic
func applyEpisodeRename(node *treeview.Node[treeview.FileInfo], cfg *config.FormatConfig, tmdbProvider *provider.TMDBProvider, showMeta *provider.EnrichedMetadata, showName, year string, season, episode int) string {
	// Preserve the file extension
	ext := media.ExtractExtension(node.Name())

	// Try to fetch episode metadata if we have show metadata
	var metadata *provider.EnrichedMetadata
	if showMeta != nil {
		metadata = fetchEpisodeMetadata(tmdbProvider, showMeta, season, episode)
	}

	// Create context
	ctx := createFormatContext(cfg, showName, "", year, season, episode, metadata)

	// Apply template and add extension
	return cfg.ApplyEpisodeTemplate(ctx) + ext
}

// applySeasonRename handles the common season renaming logic
func applySeasonRename(cfg *config.FormatConfig, tmdbProvider *provider.TMDBProvider, showMeta *provider.EnrichedMetadata, showName, year string, season int) string {
	// Try to fetch season metadata if we have show metadata
	var metadata *provider.EnrichedMetadata
	if showMeta != nil {
		metadata = fetchSeasonMetadata(tmdbProvider, showMeta, season)
		if metadata == nil {
			// Fall back to show metadata if season fetch failed
			metadata = showMeta
		}
	}

	// Create context
	ctx := createFormatContext(cfg, showName, "", year, season, 0, metadata)

	// Apply template
	return cfg.ApplySeasonFolderTemplate(ctx)
}

// applyShowRename handles the common show renaming logic
func applyShowRename(cfg *config.FormatConfig, tmdbProvider *provider.TMDBProvider, showName, year string) (string, *provider.EnrichedMetadata) {
	// Try to fetch show metadata
	showMeta := fetchShowMetadata(tmdbProvider, showName)

	// Create context
	ctx := createFormatContext(cfg, showName, "", year, 0, 0, showMeta)

	// Apply template
	return cfg.ApplyShowFolderTemplate(ctx), showMeta
}
