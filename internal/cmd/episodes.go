package cmd

import (
	"context"
	"path/filepath"
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/treeview"
)

// EpisodesCommand processes a flat directory of episode files (no parent season folder).
// Each top-level media file is classified as an episode and renamed solely based on
// information present in its own filename (no contextual season inference).
var EpisodesCommand = CommandConfig{
	maxDepth:    1,
	includeDirs: false,
	annotate: func(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
		for ni := range t.All(context.Background()) {
			// Only operate on leaf nodes (files) at depth 0; directories are excluded by includeDirs=false.
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

			// Preserve the file extension (with language code for subtitles)
			ext := media.ExtractExtension(ni.Node.Name())

			// Apply template and add extension - Episodes command has no show/year context
			m.NewName = cfg.ApplyEpisodeTemplate("", "", season, episode) + ext
			
			// Set destination path if linking
			if linkPath != "" {
				m.DestinationPath = filepath.Join(linkPath, m.NewName)
			}
		}
	},
}
