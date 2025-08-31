package cmd

import (
	"context"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
)

// EpisodesCommand processes a flat directory of episode files (no parent season folder).
// Each top-level media file is classified as an episode and renamed solely based on
// information present in its own filename (no contextual season inference).
var EpisodesCommand = CommandConfig{
	maxDepth:    1,
	includeDirs: false,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string, metadata map[string]*provider.EnrichedMetadata) {
		for ni := range t.All(context.Background()) {
			// Only operate on leaf nodes (files) at depth 0
			if ni.Node.Data().IsDir() {
				continue
			}
			m := core.EnsureMeta(ni.Node)
			m.Type = core.MediaEpisode

			// Parse season/episode from filename
			season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node)
			if !found {
				continue
			}

			// Extract show name from filename
			showName, year := extractShowNameFromPath(ni.Node.Name(), true)

			// Get pre-fetched metadata if available
			var meta *provider.EnrichedMetadata
			if metadata != nil {
				// Try to get episode-specific metadata
				key := util.GenerateMetadataKey("episode", showName, year, season, episode)
				meta = metadata[key]

				// Fall back to show metadata if episode not found
				if meta == nil {
					showKey := util.GenerateMetadataKey("show", showName, year, 0, 0)
					showMeta := metadata[showKey]
					if showMeta != nil {
						meta = showMeta
					}
				}
			}

			// Preserve file extension
			ext := media.ExtractExtension(ni.Node.Name())

			// Apply episode rename with metadata
			ctx := createFormatContext(cfg, showName, "", year, season, episode, meta)
			m.NewName = cfg.ApplyEpisodeTemplate(ctx) + ext

			// Set destination path if linking
			setDestinationPath(ni.Node, linkPath, "", m.NewName)
		}
	},
}
