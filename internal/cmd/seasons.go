package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/treeview"
)

var SeasonsCommand = CommandConfig{
	maxDepth:    2,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
		// Track parent paths for building destination hierarchy
		parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)
		
		for ni := range t.All(context.Background()) {
			m := core.EnsureMeta(ni.Node)
			if ni.Depth == 0 {
				m.Type = core.MediaSeason
				// Extract season number from directory name
				season, found := media.ExtractSeasonNumber(ni.Node.Name())
				if !found {
					continue
				}
				// For seasons command, we don't have the show name context
				m.NewName = cfg.ApplySeasonFolderTemplate("", "", season)
				
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

				// Preserve the file extension (with language code for subtitles)
				ext := media.ExtractExtension(ni.Node.Name())

				// Apply template and add extension - no show/year context
				m.NewName = cfg.ApplyEpisodeTemplate("", "", season, episode) + ext
				
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
