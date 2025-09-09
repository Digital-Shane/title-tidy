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

var seasonsCmd = &cobra.Command{
	Use:   "seasons",
	Short: "Rename season folders and episodes within",
	Long: `Rename season folders and their contained episode files.
	
This command processes season directories, renaming the season folder itself
and all episode files within according to your configured format.`,
	RunE: runSeasonsCommand,
}

func runSeasonsCommand(cmd *cobra.Command, args []string) error {
	return RunMediaCommand(cmd, CommandConfig{
		CommandName:   "seasons",
		MaxDepth:      2,
		IncludeDirs:   true,
		TreeAnnotator: annotateSeasonsTree,
	})
}

func annotateSeasonsTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.EnrichedMetadata) {
	parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)
	var seasonShowMeta *provider.EnrichedMetadata
	var seasonShowName string
	var seasonYear string

	for ni := range t.All(context.Background()) {
		m := core.EnsureMeta(ni.Node)
		if ni.Depth == 0 {
			m.Type = core.MediaSeason
			seasonNumber, found := media.ExtractSeasonNumber(ni.Node.Name())
			if !found {
				continue
			}

			showName, year := media.ExtractShowNameFromPath(ni.Node.Name(), false)
			seasonShowName = showName
			seasonYear = year

			var meta *provider.EnrichedMetadata
			if metadata != nil {
				showKey := util.GenerateMetadataKey("show", showName, year, 0, 0)
				seasonShowMeta = metadata[showKey]

				seasonKey := util.GenerateMetadataKey("season", showName, year, seasonNumber, 0)
				meta = metadata[seasonKey]
				if meta == nil {
					meta = seasonShowMeta
				}
			}

			ctx := createFormatContext(cfg, showName, "", year, seasonNumber, 0, meta)
			m.NewName = cfg.ApplySeasonFolderTemplate(ctx)

			if linkPath != "" {
				parentPaths[ni.Node] = linkPath
			}

		} else if ni.Depth == 1 {
			m.Type = core.MediaEpisode
			seasonNumber, episodeNumber, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node)
			if !found || seasonNumber == 0 || episodeNumber == 0 {
				continue
			}

			var meta *provider.EnrichedMetadata
			if metadata != nil {
				episodeKey := util.GenerateMetadataKey("episode", seasonShowName, seasonYear, seasonNumber, episodeNumber)
				meta = metadata[episodeKey]
				if meta == nil {
					meta = seasonShowMeta
				}
			}

			ctx := createFormatContext(cfg, seasonShowName, ni.Node.Name(), seasonYear, seasonNumber, episodeNumber, meta)
			m.NewName = cfg.ApplyEpisodeTemplate(ctx) + media.ExtractExtension(ni.Node.Name())

			if linkPath != "" {
				if parentPath, exists := parentPaths[ni.Node.Parent()]; exists {
					m.DestinationPath = parentPath
				}
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(seasonsCmd)
}
