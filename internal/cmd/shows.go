package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
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

func annotateShowsTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.EnrichedMetadata) {
	parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)
	showMetadata := make(map[*treeview.Node[treeview.FileInfo]]*provider.EnrichedMetadata)
	// Store show info for fallback when episodes don't contain show name
	showInfoCache := make(map[*treeview.Node[treeview.FileInfo]]struct {
		name string
		year string
	})

	for ni := range t.BreadthFirst(context.Background()) {
		m := core.EnsureMeta(ni.Node)
		switch ni.Depth {
		case 0: // Shows
			m.Type = core.MediaShow
			showName, year := media.ExtractShowInfo(ni.Node, false)
			if showName == "" {
				continue
			}

			// Cache show info for potential fallback
			showInfoCache[ni.Node] = struct {
				name string
				year string
			}{showName, year}

			var meta *provider.EnrichedMetadata
			if metadata != nil {
				key := util.GenerateMetadataKey("show", showName, year, 0, 0)
				meta = metadata[key]
			}

			ctx := createFormatContext(cfg, showName, "", year, 0, 0, meta)
			m.NewName = cfg.ApplyShowFolderTemplate(ctx)

			if meta != nil {
				showMetadata[ni.Node] = meta
			}

			if linkPath != "" {
				parentPaths[ni.Node] = linkPath
			}

		case 1: // Seasons
			m.Type = core.MediaSeason
			seasonNumber, found := media.ExtractSeasonNumber(ni.Node.Name())
			if !found {
				continue
			}

			showName, year := media.ExtractShowInfo(ni.Node, false)

			// If not found in season name, use cached show info from parent
			if showName == "" && ni.Node.Parent() != nil {
				if cached, exists := showInfoCache[ni.Node.Parent()]; exists {
					showName = cached.name
					year = cached.year
				}
			}

			var meta *provider.EnrichedMetadata
			if ni.Node.Parent() != nil {
				meta = showMetadata[ni.Node.Parent()]
			}

			ctx := createFormatContext(cfg, showName, "", year, seasonNumber, 0, meta)
			m.NewName = cfg.ApplySeasonFolderTemplate(ctx)

			if linkPath != "" {
				if parentPath, exists := parentPaths[ni.Node.Parent()]; exists {
					parentPaths[ni.Node] = filepath.Join(parentPath, m.NewName)
				}
			}

		case 2: // Episodes
			m.Type = core.MediaEpisode

			showName, year, seasonNumber, episodeNumber, found := media.ProcessEpisodeNode(ni.Node)
			if !found || seasonNumber == 0 || episodeNumber == 0 {
				continue
			}

			// If show name wasn't found in episode, use cached info from grandparent (show folder)
			if showName == "" && ni.Node.Parent() != nil && ni.Node.Parent().Parent() != nil {
				if cached, exists := showInfoCache[ni.Node.Parent().Parent()]; exists {
					showName = cached.name
					year = cached.year
				}
			}

			var meta *provider.EnrichedMetadata
			if ni.Node.Parent() != nil && ni.Node.Parent().Parent() != nil {
				meta = showMetadata[ni.Node.Parent().Parent()]
			}

			ctx := createFormatContext(cfg, showName, "", year, seasonNumber, episodeNumber, meta)
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
	rootCmd.AddCommand(showsCmd)
}
