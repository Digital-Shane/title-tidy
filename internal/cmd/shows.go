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

var ShowsCommand = CommandConfig{
	maxDepth:    3,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string, metadata map[string]*provider.EnrichedMetadata) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

		// Track show metadata for reuse in seasons/episodes
		showMetadata := make(map[*treeview.Node[treeview.FileInfo]]*provider.EnrichedMetadata)

		// Use BreadthFirst to process shows first, then seasons, then episodes
		for ni := range t.BreadthFirst(context.Background()) {
			m := core.EnsureMeta(ni.Node)
			switch ni.Depth {
			case 0: // Shows
				m.Type = core.MediaShow
				// Extract name and year from original filename
				showName, year := config.ExtractNameAndYear(ni.Node.Name())
				if showName == "" {
					continue
				}

				// Get pre-fetched metadata if available
				var meta *provider.EnrichedMetadata
				if metadata != nil {
					key := util.GenerateMetadataKey("show", showName, year, 0, 0)
					meta = metadata[key]
				}

				// Apply show rename with metadata
				ctx := createFormatContext(cfg, showName, "", year, 0, 0, meta)
				m.NewName = cfg.ApplyShowFolderTemplate(ctx)

				// Store metadata for children
				if meta != nil {
					showMetadata[ni.Node] = meta
				}

				// Set destination path if linking
				if linkPath != "" {
					m.DestinationPath = filepath.Join(linkPath, m.NewName)
					parentPaths[ni.Node] = m.DestinationPath
				}

			case 1: // Seasons
				m.Type = core.MediaSeason
				// Get show context from parent show node
				var showName, year string
				var parentMetadata *provider.EnrichedMetadata
				if showNode := ni.Node.Parent(); showNode != nil {
					showName, year = config.ExtractNameAndYear(showNode.Name())
					parentMetadata = showMetadata[showNode]
				}

				// Extract season number from directory name
				season, found := media.ExtractSeasonNumber(ni.Node.Name())
				if !found {
					continue
				}

				// Get pre-fetched season metadata if available
				var meta *provider.EnrichedMetadata
				if metadata != nil {
					key := util.GenerateMetadataKey("season", showName, year, season, 0)
					meta = metadata[key]
					if meta == nil && parentMetadata != nil {
						// Fall back to show metadata if season not found
						meta = parentMetadata
					}
				}

				// Apply season rename with metadata
				ctx := createFormatContext(cfg, showName, "", year, season, 0, meta)
				m.NewName = cfg.ApplySeasonFolderTemplate(ctx)

				// Set destination path if linking
				if linkPath != "" && ni.Node.Parent() != nil {
					parentPath := parentPaths[ni.Node.Parent()]
					if parentPath != "" {
						m.DestinationPath = filepath.Join(parentPath, m.NewName)
						parentPaths[ni.Node] = m.DestinationPath
					}
				}

			case 2: // Episodes
				m.Type = core.MediaEpisode
				// Get show context from grandparent show node
				var showName, year string
				if seasonNode := ni.Node.Parent(); seasonNode != nil {
					if showNode := seasonNode.Parent(); showNode != nil {
						showName, year = config.ExtractNameAndYear(showNode.Name())
					}
				}

				// Parse season/episode from filename
				season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node)
				if !found {
					continue
				}

				// Get pre-fetched episode metadata if available
				var meta *provider.EnrichedMetadata
				if metadata != nil {
					key := util.GenerateMetadataKey("episode", showName, year, season, episode)
					meta = metadata[key]
				}

				// Preserve file extension
				ext := media.ExtractExtension(ni.Node.Name())

				// Apply episode rename with metadata
				ctx := createFormatContext(cfg, showName, "", year, season, episode, meta)
				m.NewName = cfg.ApplyEpisodeTemplate(ctx) + ext

				// Set destination path if linking
				if linkPath != "" && ni.Node.Parent() != nil {
					parentPath := parentPaths[ni.Node.Parent()]
					if parentPath != "" {
						m.DestinationPath = filepath.Join(parentPath, m.NewName)
					}
				}
			}
		}
	},
}
