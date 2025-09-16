package cmd

import (
	"context"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
	"github.com/spf13/cobra"
)

var episodesCmd = &cobra.Command{
	Use:   "episodes",
	Short: "Rename episode files in current directory",
	Long: `Rename episode files in the current directory.
	
This command processes a flat directory of episode files, renaming each file
based on information present in its filename according to your configured format.`,
	RunE: runEpisodesCommand,
}

func runEpisodesCommand(cmd *cobra.Command, args []string) error {
	return RunMediaCommand(cmd, CommandConfig{
		CommandName:   "episodes",
		MaxDepth:      1,
		IncludeDirs:   false,
		TreeAnnotator: annotateEpisodesTree,
	})
}

func annotateEpisodesTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.Metadata) {
	for ni := range t.All(context.Background()) {
		if ni.Node.Data().IsDir() {
			continue
		}

		m := core.EnsureMeta(ni.Node)
		m.Type = core.MediaEpisode

		showName, year, season, episode, found := media.ProcessEpisodeNode(ni.Node)
		if !found {
			continue
		}

		var meta *provider.Metadata
		if metadata != nil {
			key := util.GenerateMetadataKey("episode", showName, year, season, episode)
			meta = metadata[key]

			if meta == nil {
				showKey := util.GenerateMetadataKey("show", showName, year, 0, 0)
				showMeta := metadata[showKey]
				if showMeta != nil {
					meta = showMeta
				}
			}
		}

		ctx := createFormatContext(cfg, showName, "", year, season, episode, meta)
		m.NewName = cfg.ApplyEpisodeTemplate(ctx) + media.ExtractExtension(ni.Node.Name())

		if linkPath != "" {
			m.DestinationPath = linkPath
		}
	}
}

func init() {
	rootCmd.AddCommand(episodesCmd)
}
