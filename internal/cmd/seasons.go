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

var SeasonsCommand = CommandConfig{
	maxDepth:    2,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

		// Initialize TMDB provider if enabled and needed
		tmdbProvider := initializeTMDBProvider(cfg)

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

				// Fetch show metadata if available
				showMeta := fetchShowMetadata(tmdbProvider, showName)
				seasonShowMeta = showMeta

				// Apply season rename
				m.NewName = applySeasonRename(cfg, tmdbProvider, showMeta, showName, year, season)

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

				// Apply episode rename using parent show context
				m.NewName = applyEpisodeRename(ni.Node, cfg, tmdbProvider, seasonShowMeta, seasonShowName, seasonYear, season, episode)

				// Set destination path if linking
				if linkPath != "" && ni.Node.Parent() != nil {
					parentPath := parentPaths[ni.Node.Parent()]
					setDestinationPath(ni.Node, linkPath, parentPath, m.NewName)
				}
			}
		}
	},
}
