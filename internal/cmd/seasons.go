package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
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

func annotateSeasonsTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.Metadata) {
	parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)
	var seasonShowMeta *provider.Metadata
	var seasonShowName string
	var seasonYear string

	ctx := context.Background()

	for ni := range t.All(ctx) {
		m := core.EnsureMeta(ni.Node)
		if ni.Depth == 0 {
			m.Type = core.MediaSeason

			seasonMeta, err := fetchLocalMetadata(ctx, ni.Node, provider.MediaTypeSeason)
			if err != nil || seasonMeta == nil {
				continue
			}

			if seasonMeta.Core.SeasonNum == 0 {
				continue
			}

			seasonShowName = seasonMeta.Core.Title
			seasonYear = seasonMeta.Core.Year

			var meta *provider.Metadata
			if metadata != nil {
				showKey := provider.GenerateMetadataKey("show", seasonMeta.Core.Title, seasonMeta.Core.Year, 0, 0)
				seasonShowMeta = metadata[showKey]

				seasonKey := provider.GenerateMetadataKey("season", seasonMeta.Core.Title, seasonMeta.Core.Year, seasonMeta.Core.SeasonNum, 0)
				meta = metadata[seasonKey]
				if meta == nil {
					meta = seasonShowMeta
				}
			}

			ctx := createFormatContext(cfg, seasonMeta.Core.Title, "", seasonMeta.Core.Year, seasonMeta.Core.SeasonNum, 0, meta)
			generated := cfg.ApplySeasonFolderTemplate(ctx)
			m.NewName = core.PreserveExistingBracketTags(generated, ni.Node.Name(), cfg.PreserveExistingTags)

			if linkPath != "" {
				dirName := m.NewName
				if dirName == "" {
					dirName = ni.Node.Name()
				}
				dest := filepath.Join(linkPath, dirName)
				m.DestinationPath = dest
				parentPaths[ni.Node] = dest
			}

		} else if ni.Depth == 1 {
			m.Type = core.MediaEpisode

			episodeMeta, err := fetchLocalMetadata(ctx, ni.Node, provider.MediaTypeEpisode)
			if err != nil || episodeMeta == nil {
				continue
			}

			showName := episodeMeta.Core.Title
			year := episodeMeta.Core.Year
			if episodeMeta.Core.SeasonNum == 0 || episodeMeta.Core.EpisodeNum == 0 {
				continue
			}

			// If show name wasn't found in episode, use the one from season folder
			if showName == "" {
				showName = seasonShowName
				year = seasonYear
			}

			var meta *provider.Metadata
			if metadata != nil && showName != "" {
				// First try to find metadata for this specific episode
				episodeKey := provider.GenerateMetadataKey("episode", showName, year, episodeMeta.Core.SeasonNum, episodeMeta.Core.EpisodeNum)
				meta = metadata[episodeKey]

				// If no episode metadata, try show metadata
				if meta == nil {
					showKey := provider.GenerateMetadataKey("show", showName, year, 0, 0)
					meta = metadata[showKey]
				}

				// Only use parent metadata if the show names match (not a different show in the folder)
				if meta == nil && showName == seasonShowName {
					meta = seasonShowMeta
				}
			}

			ctx := createFormatContext(cfg, showName, "", year, episodeMeta.Core.SeasonNum, episodeMeta.Core.EpisodeNum, meta)
			ext := local.ExtractExtension(ni.Node.Name())
			sourceBase := ni.Node.Name()
			if ext != "" {
				sourceBase = sourceBase[:len(sourceBase)-len(ext)]
			}
			generated := cfg.ApplyEpisodeTemplate(ctx)
			generated = core.PreserveExistingBracketTags(generated, sourceBase, cfg.PreserveExistingTags)
			m.NewName = generated + ext

			if linkPath != "" {
				fileName := m.NewName
				if fileName == "" {
					fileName = ni.Node.Name()
				}
				destBase := linkPath
				if parentPath, exists := parentPaths[ni.Node.Parent()]; exists && parentPath != "" {
					destBase = parentPath
				}
				m.DestinationPath = filepath.Join(destBase, fileName)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(seasonsCmd)
}
