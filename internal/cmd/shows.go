package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/treeview"
)

var ShowsCommand = CommandConfig{
	maxDepth:    3,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

		// Use BreadthFirst to process shows first, then seasons, then episodes
		for ni := range t.BreadthFirst(context.Background()) {
			m := core.EnsureMeta(ni.Node)
			switch ni.Depth {
			case 0: // Shows
				m.Type = core.MediaShow
				// Extract name and year from original filename
				formatted, year := config.ExtractNameAndYear(ni.Node.Name())
				if formatted == "" {
					continue
				}
				// Apply show template
				m.NewName = cfg.ApplyShowFolderTemplate(formatted, year)

				// Set destination path if linking
				if linkPath != "" {
					m.DestinationPath = filepath.Join(linkPath, m.NewName)
					parentPaths[ni.Node] = m.DestinationPath
				}

			case 1: // Seasons
				m.Type = core.MediaSeason
				// Get show context from parent show node
				var showName, year string
				if showNode := ni.Node.Parent(); showNode != nil {
					showName, year = config.ExtractNameAndYear(showNode.Name())
				}

				// Extract season number from directory name
				season, found := media.ExtractSeasonNumber(ni.Node.Name())
				if !found {
					continue
				}

				// Apply season template with full context
				m.NewName = cfg.ApplySeasonFolderTemplate(showName, year, season)

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

				// Preserve the file extension (with language code for subtitles)
				ext := media.ExtractExtension(ni.Node.Name())

				// Apply episode template with full context and add extension
				m.NewName = cfg.ApplyEpisodeTemplate(showName, year, season, episode) + ext

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
