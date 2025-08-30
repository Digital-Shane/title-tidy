package cmd

import (
	"context"

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
		// Initialize TMDB provider if enabled and needed
		tmdbProvider := initializeTMDBProvider(cfg)

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

			// Fetch show metadata if available
			showMeta := fetchShowMetadata(tmdbProvider, showName)

			// Apply episode rename
			m.NewName = applyEpisodeRename(ni.Node, cfg, tmdbProvider, showMeta, showName, year, season, episode)

			// Set destination path if linking
			setDestinationPath(ni.Node, linkPath, "", m.NewName)
		}
	},
}
