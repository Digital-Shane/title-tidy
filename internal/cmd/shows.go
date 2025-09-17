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

var showsCmd = &cobra.Command{
	Use:   "shows",
	Short: "Rename TV show files and folders",
	Long: `Rename TV show directories with their seasons and episodes.
	
This command processes complete TV show hierarchies, renaming show folders,
season folders, and episode files according to your configured format.`,
	RunE: runShowsCommand,
}

func runShowsCommand(cmd *cobra.Command, args []string) error {
	return RunMediaCommand(cmd, CommandConfig{
		CommandName:   "shows",
		MaxDepth:      3,
		IncludeDirs:   true,
		TreeAnnotator: annotateShowsTree,
	})
}

func annotateShowsTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.Metadata) {
	parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)
	showMetadata := make(map[*treeview.Node[treeview.FileInfo]]*provider.Metadata)
	// Store show info for fallback when episodes don't contain show name
	showInfoCache := make(map[*treeview.Node[treeview.FileInfo]]struct {
		name string
		year string
	})

	ctx := context.Background()

	for ni := range t.BreadthFirst(ctx) {
		m := core.EnsureMeta(ni.Node)
		switch ni.Depth {
		case 0: // Shows
			m.Type = core.MediaShow
			showMeta, err := fetchLocalMetadata(ctx, ni.Node, provider.MediaTypeShow)
			if err != nil || showMeta == nil {
				continue
			}

			if showMeta.Core.Title == "" {
				continue
			}

			// Cache show info for potential fallback
			showInfoCache[ni.Node] = struct {
				name string
				year string
			}{showMeta.Core.Title, showMeta.Core.Year}

			var meta *provider.Metadata
			if metadata != nil {
				key := provider.GenerateMetadataKey("show", showMeta.Core.Title, showMeta.Core.Year, 0, 0)
				meta = metadata[key]
			}

			ctx := createFormatContext(cfg, showMeta.Core.Title, "", showMeta.Core.Year, 0, 0, meta)
			m.NewName = cfg.ApplyShowFolderTemplate(ctx)

			if meta != nil {
				showMetadata[ni.Node] = meta
			}

			if linkPath != "" {
				parentPaths[ni.Node] = linkPath
			}

		case 1: // Seasons
			m.Type = core.MediaSeason
			seasonMeta, err := fetchLocalMetadata(ctx, ni.Node, provider.MediaTypeSeason)
			if err != nil || seasonMeta == nil {
				continue
			}

			if seasonMeta.Core.SeasonNum == 0 {
				continue
			}

			showName := seasonMeta.Core.Title
			year := seasonMeta.Core.Year

			// If not found in season name, use cached show info from parent
			if showName == "" && ni.Node.Parent() != nil {
				if cached, exists := showInfoCache[ni.Node.Parent()]; exists {
					showName = cached.name
					year = cached.year
				}
			}

			var meta *provider.Metadata
			if ni.Node.Parent() != nil {
				meta = showMetadata[ni.Node.Parent()]
			}

			ctx := createFormatContext(cfg, showName, "", year, seasonMeta.Core.SeasonNum, 0, meta)
			m.NewName = cfg.ApplySeasonFolderTemplate(ctx)

			if linkPath != "" {
				if parentPath, exists := parentPaths[ni.Node.Parent()]; exists {
					parentPaths[ni.Node] = filepath.Join(parentPath, m.NewName)
				}
			}

		case 2: // Episodes
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

			// Store the parent show's name for comparison
			var parentShowName string
			var parentShowYear string
			if ni.Node.Parent() != nil && ni.Node.Parent().Parent() != nil {
				if cached, exists := showInfoCache[ni.Node.Parent().Parent()]; exists {
					parentShowName = cached.name
					parentShowYear = cached.year
				}
			}

			// If show name wasn't found in episode, use parent info
			if showName == "" {
				showName = parentShowName
				year = parentShowYear
			}

			// Look up metadata based on the actual show name extracted from the episode
			var meta *provider.Metadata
			if metadata != nil && showName != "" {
				// First try to find metadata for this specific episode
				key := provider.GenerateMetadataKey("episode", showName, year, episodeMeta.Core.SeasonNum, episodeMeta.Core.EpisodeNum)
				meta = metadata[key]

				// If no episode metadata, try show metadata
				if meta == nil {
					key = provider.GenerateMetadataKey("show", showName, year, 0, 0)
					meta = metadata[key]
				}
			}

			// Only use parent metadata if the show names match (not a different show in the folder)
			if meta == nil && showName == parentShowName && ni.Node.Parent() != nil && ni.Node.Parent().Parent() != nil {
				meta = showMetadata[ni.Node.Parent().Parent()]
			}

			ctx := createFormatContext(cfg, showName, "", year, episodeMeta.Core.SeasonNum, episodeMeta.Core.EpisodeNum, meta)
			m.NewName = cfg.ApplyEpisodeTemplate(ctx) + local.ExtractExtension(ni.Node.Name())

			if linkPath != "" {
				if parentPath, exists := parentPaths[ni.Node.Parent()]; exists {
					m.DestinationPath = parentPath
				}
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(showsCmd)
}
