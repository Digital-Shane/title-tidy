package cmd

import (
	"context"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/treeview"
)

var SeasonsCommand = CommandConfig{
	maxDepth:    2,
	includeDirs: true,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig) {
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
			}
		}
	},
}
