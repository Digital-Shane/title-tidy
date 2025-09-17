package cmd

import (
	"context"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
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
	ctx := context.Background()

	for ni := range t.All(ctx) {
		if ni.Node.Data().IsDir() {
			continue
		}

		m := core.EnsureMeta(ni.Node)
		m.Type = core.MediaEpisode

		fetchMeta, err := fetchLocalMetadata(ctx, ni.Node, provider.MediaTypeEpisode)
		if err != nil || fetchMeta == nil {
			continue
		}

		if fetchMeta.Core.SeasonNum == 0 || fetchMeta.Core.EpisodeNum == 0 {
			continue
		}

		var meta *provider.Metadata
		if metadata != nil {
			key := provider.GenerateMetadataKey(
				"episode",
				fetchMeta.Core.Title,
				fetchMeta.Core.Year,
				fetchMeta.Core.SeasonNum,
				fetchMeta.Core.EpisodeNum,
			)
			meta = metadata[key]

			if meta == nil {
				showKey := provider.GenerateMetadataKey(
					"show",
					fetchMeta.Core.Title,
					fetchMeta.Core.Year,
					0,
					0,
				)
				showMeta := metadata[showKey]
				if showMeta != nil {
					meta = showMeta
				}
			}
		}

		ctx := createFormatContext(
			cfg,
			fetchMeta.Core.Title,
			"",
			fetchMeta.Core.Year,
			fetchMeta.Core.SeasonNum,
			fetchMeta.Core.EpisodeNum,
			meta,
		)
		m.NewName = cfg.ApplyEpisodeTemplate(ctx) + local.ExtractExtension(ni.Node.Name())

		if linkPath != "" {
			m.DestinationPath = linkPath
		}
	}
}

func init() {
	rootCmd.AddCommand(episodesCmd)
}
