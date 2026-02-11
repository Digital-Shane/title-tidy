package cmd

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview"
	"github.com/spf13/cobra"
)

var bracketTagPattern = regexp.MustCompile(`\[[^\[\]]+\]`)

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

func detectMovieNameAndYear(node *treeview.Node[treeview.FileInfo]) (string, string) {
	if node == nil {
		return "", ""
	}

	mediaType, meta, err := detectLocalMedia(node)
	if err == nil && mediaType == provider.MediaTypeMovie && meta != nil {
		return meta.Core.Title, meta.Core.Year
	}

	return fallbackMovieNameYear(node)
}

func fallbackMovieNameYear(node *treeview.Node[treeview.FileInfo]) (string, string) {
	if node == nil {
		return "", ""
	}

	name := node.Name()
	if !node.Data().IsDir() {
		if ext := local.ExtractExtension(name); ext != "" {
			name = name[:len(name)-len(ext)]
		}
	}

	return local.ExtractNameAndYear(name)
}

func extractBracketTags(baseName string) []string {
	if baseName == "" {
		return nil
	}
	matches := bracketTagPattern.FindAllString(baseName, -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func normalizeBracketTag(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	return strings.ToLower(strings.TrimSpace(trimmed))
}

func appendPreservedTags(baseName string, sourceFileName string, preserve bool) string {
	if !preserve {
		return baseName
	}

	sourceBase := sourceFileName
	if ext := local.ExtractExtension(sourceBase); ext != "" {
		sourceBase = sourceBase[:len(sourceBase)-len(ext)]
	}

	tagsToPreserve := extractBracketTags(sourceBase)
	if len(tagsToPreserve) == 0 {
		return baseName
	}

	existingTags := extractBracketTags(baseName)
	existing := make(map[string]struct{}, len(existingTags))
	for _, tag := range existingTags {
		norm := normalizeBracketTag(tag)
		if norm == "" {
			continue
		}
		existing[norm] = struct{}{}
	}

	result := baseName
	for _, tag := range tagsToPreserve {
		norm := normalizeBracketTag(tag)
		if norm == "" {
			continue
		}
		if _, ok := existing[norm]; ok {
			continue
		}
		result += tag
		existing[norm] = struct{}{}
	}

	return result
}

func moviePreprocess(nodes []*treeview.Node[treeview.FileInfo], cfg *config.FormatConfig, noDir bool) []*treeview.Node[treeview.FileInfo] {
	if noDir {
		// First pass: process video files and build a map of base names to new names
		videoRenames := make(map[string]string) // maps original base name to new base name

		for _, n := range nodes {
			if n.Data().IsDir() {
				continue
			}
			if local.IsVideo(n.Name()) {
				m := core.EnsureMeta(n)
				m.Type = core.MediaMovieFile

				fileExt := local.ExtractExtension(n.Name())
				base := n.Name()
				if fileExt != "" {
					base = base[:len(base)-len(fileExt)]
				}

				movieName, year := detectMovieNameAndYear(n)
				ctx := createFormatContext(cfg, "", movieName, year, 0, 0, nil)
				newBase := cfg.ApplyMovieTemplate(ctx)
				newBase = appendPreservedTags(newBase, n.Name(), cfg.PreserveExistingTags)
				m.NewName = newBase + fileExt

				// Store the rename mapping for subtitle matching
				videoRenames[base] = newBase
			}
		}

		// Second pass: process subtitle files and match them to videos
		for _, n := range nodes {
			if n.Data().IsDir() {
				continue
			}
			if local.IsSubtitle(n.Name()) {
				m := core.EnsureMeta(n)
				m.Type = core.MediaMovieFile

				// Extract base name and try to match with a video
				subBase := n.Name()
				subExt := local.ExtractExtension(subBase)
				if subExt != "" {
					subBase = subBase[:len(subBase)-len(subExt)]
				}

				// Try to find matching video rename
				if newBase, found := videoRenames[subBase]; found {
					// Found a matching video, rename subtitle to match
					m.NewName = newBase + subExt
				}
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

		if usedNodes[n] || !local.IsVideo(n.Name()) {
			continue
		}

		videoBaseName := n.Name()
		if ext := local.ExtractExtension(videoBaseName); ext != "" {
			videoBaseName = videoBaseName[:len(videoBaseName)-len(ext)]
		}

		children := []*treeview.Node[treeview.FileInfo]{n}
		usedNodes[n] = true

		for _, other := range nodes {
			if other == n || usedNodes[other] || other.Data().IsDir() {
				continue
			}

			if local.IsSubtitle(other.Name()) {
				otherBaseName := other.Name()
				if ext := local.ExtractExtension(otherBaseName); ext != "" {
					otherBaseName = otherBaseName[:len(otherBaseName)-len(ext)]
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

		// Mark virtual directory with proper metadata flags
		virtualMeta := core.EnsureMeta(virtualDir)
		virtualMeta.IsVirtual = true
		virtualMeta.NeedsDirectory = true

		result = append(result, virtualDir)
	}

	for _, n := range nodes {
		if !usedNodes[n] && !n.Data().IsDir() {
			result = append(result, n)
		}
	}

	return result
}

func annotateMoviesTree(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, metadata map[string]*provider.Metadata) {
	ctx := context.Background()

	videoRenameMap := make(map[string]string)
	pendingSubtitles := make(map[string][]*treeview.Node[treeview.FileInfo])

	stripExtension := func(name string) string {
		base := name
		if ext := local.ExtractExtension(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
		return base
	}

	for ni := range t.All(ctx) {
		m := core.EnsureMeta(ni.Node)

		switch ni.Depth {
		case 0:
			if ni.Node.Data().IsDir() {
				m.Type = core.MediaMovie
				movieName, year := detectMovieNameAndYear(ni.Node)

				var meta *provider.Metadata
				if metadata != nil {
					key := provider.GenerateMetadataKey("movie", movieName, year, 0, 0)
					meta = metadata[key]
				}

				ctx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
				m.NewName = cfg.ApplyMovieTemplate(ctx)

				if linkPath != "" {
					m.DestinationPath = linkPath
				}
			} else {
				if local.IsVideo(ni.Node.Name()) {
					m.Type = core.MediaMovieFile

					movieName, year := detectMovieNameAndYear(ni.Node)

					var meta *provider.Metadata
					if metadata != nil {
						key := provider.GenerateMetadataKey("movie", movieName, year, 0, 0)
						meta = metadata[key]
					}

					formatCtx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
					newBase := cfg.ApplyMovieTemplate(formatCtx)
					newBase = appendPreservedTags(newBase, ni.Node.Name(), cfg.PreserveExistingTags)
					m.NewName = newBase + local.ExtractExtension(ni.Node.Name())

					if linkPath != "" {
						m.DestinationPath = linkPath
					}

					if noDir {
						base := stripExtension(ni.Node.Name())
						videoRenameMap[base] = newBase
						if subs := pendingSubtitles[base]; len(subs) > 0 {
							for _, subNode := range subs {
								subMeta := core.EnsureMeta(subNode)
								subMeta.NewName = newBase + local.ExtractExtension(subNode.Name())
								if linkPath != "" {
									subMeta.DestinationPath = linkPath
								}
							}
							delete(pendingSubtitles, base)
						}
					}
				} else if local.IsSubtitle(ni.Node.Name()) {
					m.Type = core.MediaMovieFile

					if linkPath != "" {
						m.DestinationPath = linkPath
					}

					if noDir {
						base := stripExtension(ni.Node.Name())
						if newBase, found := videoRenameMap[base]; found {
							m.NewName = newBase + local.ExtractExtension(ni.Node.Name())
						} else {
							pendingSubtitles[base] = append(pendingSubtitles[base], ni.Node)
						}
					}
				}
			}
		case 1:
			if local.IsVideo(ni.Node.Name()) {
				m.Type = core.MediaMovieFile

				if ni.Node.Parent() != nil {
					movieName, year := detectMovieNameAndYear(ni.Node.Parent())

					var meta *provider.Metadata
					if metadata != nil {
						key := provider.GenerateMetadataKey("movie", movieName, year, 0, 0)
						meta = metadata[key]
					}

					ctx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
					baseNewName := cfg.ApplyMovieTemplate(ctx)
					baseNewName = appendPreservedTags(baseNewName, ni.Node.Name(), cfg.PreserveExistingTags)
					m.NewName = baseNewName + local.ExtractExtension(ni.Node.Name())
				}

				if linkPath != "" {
					if parentMeta := core.GetMeta(ni.Node.Parent()); parentMeta != nil && parentMeta.NewName != "" {
						m.DestinationPath = filepath.Join(linkPath, parentMeta.NewName)
					}
				}
			} else if local.IsSubtitle(ni.Node.Name()) {
				m.Type = core.MediaMovieFile

				// Match subtitle naming to the parent movie
				if ni.Node.Parent() != nil {
					movieName, year := detectMovieNameAndYear(ni.Node.Parent())

					var meta *provider.Metadata
					if metadata != nil {
						key := provider.GenerateMetadataKey("movie", movieName, year, 0, 0)
						meta = metadata[key]
					}

					ctx := createFormatContext(cfg, "", movieName, year, 0, 0, meta)
					baseNewName := cfg.ApplyMovieTemplate(ctx)
					baseNewName = appendPreservedTags(baseNewName, ni.Node.Name(), cfg.PreserveExistingTags)

					// Preserve the subtitle extension including language codes
					m.NewName = baseNewName + local.ExtractExtension(ni.Node.Name())
				}

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
