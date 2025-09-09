package cmd

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
	"github.com/spf13/cobra"
)

var moviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Rename movie files and folders",
	Long: `Rename movie files and their containing directories.
	
This command processes movie files, creating directories for loose files (unless --no-dir is specified)
and renaming according to your configured format with optional TMDB metadata lookup.`,
	RunE: runMoviesCommand,
}

func runMoviesCommand(cmd *cobra.Command, args []string) error {
	preprocessFunc := func(nodes []*treeview.Node[treeview.FileInfo], cfg *config.FormatConfig) []*treeview.Node[treeview.FileInfo] {
		return moviePreprocess(nodes, cfg, noDir)
	}

	return RunMediaCommand(cmd, CommandConfig{
		CommandName:    "movies",
		MaxDepth:       2,
		IncludeDirs:    true,
		IsMovieMode:    true,
		TreeAnnotator:  annotateMoviesTree,
		TreePreprocess: preprocessFunc,
	})
}

func moviePreprocess(nodes []*treeview.Node[treeview.FileInfo], cfg *config.FormatConfig, noDir bool) []*treeview.Node[treeview.FileInfo] {
	if noDir {
		for _, n := range nodes {
			if n.Data().IsDir() {
				continue
			}
			if media.IsVideo(n.Name()) {
				m := core.EnsureMeta(n)
				m.Type = core.MediaMovieFile

				base := n.Name()
				if ext := media.ExtractExtension(base); ext != "" {
					base = base[:len(base)-len(ext)]
				}
				formatted, year := config.ExtractNameAndYear(base)

				ctx := createFormatContext(cfg, "", formatted, year, 0, 0, nil)
				m.NewName = cfg.ApplyMovieTemplate(ctx) + media.ExtractExtension(n.Name())
			} else if media.IsSubtitle(n.Name()) {
				m := core.EnsureMeta(n)
				m.Type = core.MediaMovieFile
			}
		}
		return nodes
	}

	// Group video files with their subtitles into virtual directories
	result := make([]*treeview.Node[treeview.FileInfo], 0)
	usedNodes := make(map[*treeview.Node[treeview.FileInfo]]bool)

	for _, n := range nodes {
		if n.Data().IsDir() {
			result = append(result, n)
			continue
		}

		if usedNodes[n] || !media.IsVideo(n.Name()) {
			continue
		}

		videoBaseName := n.Name()
		if ext := media.ExtractExtension(videoBaseName); ext != "" {
			videoBaseName = videoBaseName[:len(videoBaseName)-len(ext)]
		}

		children := []*treeview.Node[treeview.FileInfo]{n}
		usedNodes[n] = true

		for _, other := range nodes {
			if other == n || usedNodes[other] || other.Data().IsDir() {
				continue
			}

			if media.IsSubtitle(other.Name()) {
				otherBaseName := other.Name()
				if ext := media.ExtractExtension(otherBaseName); ext != "" {
					otherBaseName = otherBaseName[:len(otherBaseName)-len(ext)]
				}
				// Remove language codes and subtitle suffixes like .en.srt -> base name
				if idx := strings.LastIndex(otherBaseName, "."); idx != -1 {
					otherBaseName = otherBaseName[:idx]
				}
				if otherBaseName == videoBaseName {
					children = append(children, other)
					usedNodes[other] = true
				}
			}
		}

		// Create a mock FileInfo for the virtual directory
		mockInfo := core.NewSimpleFileInfo(videoBaseName, true)
		virtualDir := treeview.NewNode("virtual_movie_dir", videoBaseName, treeview.FileInfo{
			FileInfo: mockInfo,
			Path:     videoBaseName,
			Extra:    make(map[string]any),
		})
		virtualDir.SetChildren(children)

		result = append(result, virtualDir)
	}

	for _, n := range nodes {
		if !usedNodes[n] && !n.Data().IsDir() {
			result = append(result, n)
		}
	}

	return result
}

func annotateMoviesTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.EnrichedMetadata) {
	for ni := range t.All(context.Background()) {
		m := core.EnsureMeta(ni.Node)

		if ni.Depth == 0 {
			if ni.Node.Data().IsDir() {
				m.Type = core.MediaMovie
				movieName, year := config.ExtractNameAndYear(ni.Node.Name())

				var meta *provider.EnrichedMetadata
				if metadata != nil {
					key := util.GenerateMetadataKey("movie", movieName, year, 0, 0)
					meta = metadata[key]
				}

				ctx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
				m.NewName = cfg.ApplyMovieTemplate(ctx)

				if linkPath != "" {
					m.DestinationPath = linkPath
				}
			} else {
				if media.IsVideo(ni.Node.Name()) {
					m.Type = core.MediaMovieFile
				} else if media.IsSubtitle(ni.Node.Name()) {
					m.Type = core.MediaMovieFile
				}
			}
		} else if ni.Depth == 1 {
			if media.IsVideo(ni.Node.Name()) {
				m.Type = core.MediaMovieFile

				if ni.Node.Parent() != nil {
					movieName, year := config.ExtractNameAndYear(ni.Node.Parent().Name())

					var meta *provider.EnrichedMetadata
					if metadata != nil {
						key := util.GenerateMetadataKey("movie", movieName, year, 0, 0)
						meta = metadata[key]
					}

					ctx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
					m.NewName = cfg.ApplyMovieTemplate(ctx) + media.ExtractExtension(ni.Node.Name())
				}

				if linkPath != "" {
					if parentMeta := core.GetMeta(ni.Node.Parent()); parentMeta != nil && parentMeta.NewName != "" {
						m.DestinationPath = filepath.Join(linkPath, parentMeta.NewName)
					}
				}
			} else if media.IsSubtitle(ni.Node.Name()) {
				m.Type = core.MediaMovieFile

				if linkPath != "" {
					if parentMeta := core.GetMeta(ni.Node.Parent()); parentMeta != nil && parentMeta.NewName != "" {
						m.DestinationPath = filepath.Join(linkPath, parentMeta.NewName)
					}
				}
			}
		}
	}
}

var (
	noDir bool
)

func init() {
	rootCmd.AddCommand(moviesCmd)
	moviesCmd.Flags().BoolVar(&noDir, "no-dir", false, "Don't create directories for movies (keep files in current location)")
}
