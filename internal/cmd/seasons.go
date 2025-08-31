package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
)

var SeasonsCommand = CommandConfig{
	maxDepth:    2,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string, metadata map[string]*provider.EnrichedMetadata) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

		// Track show metadata for passing to episodes
		var seasonShowMeta *provider.EnrichedMetadata
		var seasonShowName, seasonYear string

		for ni := range t.All(context.Background()) {
			m := core.EnsureMeta(ni.Node)
			if ni.Depth == 0 {
				m.Type = core.MediaSeason
				// Extract season number from directory name
				season, found := media.ExtractSeasonNumber(ni.Node.Name())
				if !found {
					continue
				}

				// Extract show name from the season folder name
				showName, year := extractShowNameFromPath(ni.Node.Name(), false)

				// Store for episodes to use
				seasonShowName = showName
				seasonYear = year

				// Get pre-fetched metadata if available
				var meta *provider.EnrichedMetadata
				if metadata != nil {
					// First try to get show metadata
					showKey := util.GenerateMetadataKey("show", showName, year, 0, 0)
					seasonShowMeta = metadata[showKey]

					// Then try to get season-specific metadata
					seasonKey := util.GenerateMetadataKey("season", showName, year, season, 0)
					meta = metadata[seasonKey]
					if meta == nil && seasonShowMeta != nil {
						// Fall back to show metadata if season not found
						meta = seasonShowMeta
					}
				}

				// Apply season rename with metadata
				ctx := createFormatContext(cfg, showName, "", year, season, 0, meta)
				m.NewName = cfg.ApplySeasonFolderTemplate(ctx)

				// Set destination path if linking
				if linkPath != "" {
					m.DestinationPath = filepath.Join(linkPath, m.NewName)
					parentPaths[ni.Node] = m.DestinationPath
				}
			} else {
				m.Type = core.MediaEpisode
				// Parse season/episode from filename
				season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node)
				if !found {
					continue
				}

				// Get pre-fetched episode metadata if available
				var meta *provider.EnrichedMetadata
				if metadata != nil {
					key := util.GenerateMetadataKey("episode", seasonShowName, seasonYear, season, episode)
					meta = metadata[key]
				}

				// Preserve file extension
				ext := media.ExtractExtension(ni.Node.Name())

				// Apply episode rename with metadata
				ctx := createFormatContext(cfg, seasonShowName, "", seasonYear, season, episode, meta)
				m.NewName = cfg.ApplyEpisodeTemplate(ctx) + ext

				// Set destination path if linking
				if linkPath != "" && ni.Node.Parent() != nil {
					parentPath := parentPaths[ni.Node.Parent()]
					setDestinationPath(ni.Node, linkPath, parentPath, m.NewName)
				}
			}
		}
	},
}
