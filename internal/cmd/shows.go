package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

var ShowsCommand = CommandConfig{
	maxDepth:    3,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

		// Initialize TMDB provider if enabled and needed
		tmdbProvider := initializeTMDBProvider(cfg)

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

				// Apply show rename and get metadata
				newName, metadata := applyShowRename(cfg, tmdbProvider, showName, year)
				m.NewName = newName

				// Store metadata for children
				if metadata != nil {
					showMetadata[ni.Node] = metadata
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

				// Apply season rename
				m.NewName = applySeasonRename(cfg, tmdbProvider, parentMetadata, showName, year, season)

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
				var grandparentMetadata *provider.EnrichedMetadata
				if seasonNode := ni.Node.Parent(); seasonNode != nil {
					if showNode := seasonNode.Parent(); showNode != nil {
						showName, year = config.ExtractNameAndYear(showNode.Name())
						grandparentMetadata = showMetadata[showNode]
					}
				}

				// Parse season/episode from filename
				season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node)
				if !found {
					continue
				}

				// Apply episode rename
				m.NewName = applyEpisodeRename(ni.Node, cfg, tmdbProvider, grandparentMetadata, showName, year, season, episode)

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
