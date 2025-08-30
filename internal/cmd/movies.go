package cmd

import (
	"context"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

var MoviesCommand = CommandConfig{
	maxDepth:    2,
	includeDirs: true,
	movieMode:   true,
	preprocess:  MoviePreprocess,
	annotate:    MovieAnnotate,
}

// MoviePreprocess groups standalone movie video files (and matching subtitles) into
// virtual directories, so they can be materialized atomically during rename.
// Matching for subtitles: the filename prefix before language + subtitle suffix must
// exactly match the video filename without its extension.
func MoviePreprocess(nodes []*treeview.Node[treeview.FileInfo], cfg *config.FormatConfig) []*treeview.Node[treeview.FileInfo] {
	// Initialize TMDB provider if enabled and needed
	tmdbProvider := initializeTMDBProvider(cfg)

	type bundle struct {
		dir *treeview.Node[treeview.FileInfo]
	}
	bundles := map[string]*bundle{} // base name (without extension) -> bundle
	var out []*treeview.Node[treeview.FileInfo]

	// First pass: wrap loose video files
	for _, n := range nodes {
		if n.Data().IsDir() || !media.IsVideo(n.Name()) {
			continue
		}
		base := n.Name()
		if ext := media.ExtractExtension(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
		if _, exists := bundles[base]; !exists {
			vd := treeview.NewNode(base, base, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(base, true), Path: base})
			vm := core.EnsureMeta(vd)
			vm.Type = core.MediaMovie

			// Extract year if present for movie formatting
			formatted, year := config.ExtractNameAndYear(base)
			// Apply movie template with TMDB metadata if available
			newName, _ := applyRename(cfg, tmdbProvider, formatted, year, true)
			vm.NewName = newName

			vm.IsVirtual = true
			vm.NeedsDirectory = true
			bundles[base] = &bundle{dir: vd}
		}
		b := bundles[base]
		b.dir.AddChild(n)
		cm := core.EnsureMeta(n)
		cm.Type = core.MediaMovieFile

		// Use parent's formatted name for the movie file
		parentMeta := core.GetMeta(b.dir)
		cm.NewName = parentMeta.NewName + media.ExtractExtension(n.Name())
	}

	// Second pass: attach related subtitle files
	for _, n := range nodes {
		if n.Data().IsDir() || !media.IsSubtitle(n.Name()) {
			continue
		}
		suffix := media.ExtractExtension(n.Name())
		if suffix == "" { // defensive
			continue
		}
		base := n.Name()[:len(n.Name())-len(suffix)]
		if b, ok := bundles[base]; ok {
			b.dir.AddChild(n)
			sm := core.EnsureMeta(n)
			sm.Type = core.MediaMovieFile
			// Use parent's formatted name for subtitle
			parentMeta := core.GetMeta(b.dir)
			sm.NewName = parentMeta.NewName + suffix
		}
	}

	// Build final node list: virtual dirs + untouched originals
	used := map[*treeview.Node[treeview.FileInfo]]bool{}
	for _, b := range bundles {
		out = append(out, b.dir)
		used[b.dir] = true
		for _, c := range b.dir.Children() {
			used[c] = true
		}
	}
	for _, n := range nodes {
		if used[n] {
			continue
		}
		out = append(out, n)
	}
	return out
}

// MovieAnnotate adds metadata to any remaining movie directories / files not handled
// during preprocess (e.g., pre-existing movie directories from the filesystem).
func MovieAnnotate(t *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, linkPath string) {
	// Track parent paths for building destination hierarchy
	parentPaths := make(map[*treeview.Node[treeview.FileInfo]]string)

	// Initialize TMDB provider if enabled and needed
	tmdbProvider := initializeTMDBProvider(cfg)

	// Track movie metadata for reuse
	movieMetadata := make(map[*treeview.Node[treeview.FileInfo]]*provider.EnrichedMetadata)

	for ni := range t.All(context.Background()) {
		if core.GetMeta(ni.Node) != nil { // already annotated but may need destination path
			m := core.GetMeta(ni.Node)
			if linkPath != "" && m.DestinationPath == "" {
				if ni.Depth == 0 {
					// Top-level movie directory
					m.DestinationPath = filepath.Join(linkPath, m.NewName)
					parentPaths[ni.Node] = m.DestinationPath
				} else if ni.Node.Parent() != nil {
					// Child of a movie directory
					parentPath := parentPaths[ni.Node.Parent()]
					if parentPath != "" {
						m.DestinationPath = filepath.Join(parentPath, m.NewName)
					}
				}
			}
			continue
		}
		if ni.Depth == 0 && ni.Node.Data().IsDir() { // only treat directories as movie containers
			m := core.EnsureMeta(ni.Node)
			m.Type = core.MediaMovie

			// Extract year if present for movie formatting
			formatted, year := config.ExtractNameAndYear(ni.Node.Name())
			// Apply movie template with TMDB metadata if available
			newName, metadata := applyRename(cfg, tmdbProvider, formatted, year, true)
			m.NewName = newName

			// Store metadata for potential future use
			if metadata != nil {
				movieMetadata[ni.Node] = metadata
			}

			// Set destination path if linking
			if linkPath != "" {
				m.DestinationPath = filepath.Join(linkPath, m.NewName)
				parentPaths[ni.Node] = m.DestinationPath
			}
			continue
		}
		p := ni.Node.Parent()
		pm := core.GetMeta(p)
		if pm == nil || pm.NewName == "" {
			continue
		}
		m := core.EnsureMeta(ni.Node)
		m.Type = core.MediaMovieFile
		m.NewName = pm.NewName + media.ExtractExtension(ni.Node.Name())

		// Set destination path if linking
		if linkPath != "" && p != nil {
			parentPath := parentPaths[p]
			if parentPath != "" {
				m.DestinationPath = filepath.Join(parentPath, m.NewName)
			}
		}
	}
}
