package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/tui"

	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandConfig describes how to construct and annotate a tree for a given subcommand. Fields:
//   - maxDepth: depth budget for filesystem enumeration.
//   - includeDirs: whether directory entries pass the filter.
//   - preprocess: optional in-memory node transformation prior to tree
//     construction (e.g. injecting virtual directories around loose movie files).
//   - annotate: optional pass to attach MediaMeta (type + proposed name).
//   - movieMode: toggles movie-oriented statistics & wording in the TUI.
//   - InstantMode: apply renames immediately without interactive preview.
//   - DeleteNFO: mark NFO files for deletion during rename.
//   - DeleteImages: mark image files for deletion during rename.
//   - LinkMode: type of file system link to create instead of renaming.
//   - LinkTarget: root directory for creating linked file structure.
type CommandConfig struct {
	maxDepth     int
	includeDirs  bool
	preprocess   func([]*treeview.Node[treeview.FileInfo], *config.FormatConfig) []*treeview.Node[treeview.FileInfo]
	annotate     func(*treeview.Tree[treeview.FileInfo], *config.FormatConfig)
	movieMode    bool
	InstantMode  bool
	DeleteNFO    bool
	DeleteImages bool
	Config       *config.FormatConfig
	LinkMode     core.LinkMode
	LinkTarget   string
}

func RunCommand(cfg CommandConfig) error {
	// 1. Run indexing (filesystem scan + progress UI) once.
	idxModel := tui.NewIndexProgressModel(".", tui.IndexConfig{
		MaxDepth:    cfg.maxDepth,
		IncludeDirs: cfg.includeDirs,
		Filter:      CreateMediaFilter(cfg.includeDirs),
	})
	finalModel, err := tea.NewProgram(idxModel, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	// Retrieve built tree from progress model.
	im, ok := finalModel.(*tui.IndexProgressModel)
	if !ok {
		return fmt.Errorf("unexpected model type %T after indexing", finalModel)
	}
	t := im.Tree()
	if t == nil {
		return fmt.Errorf("indexing produced no tree")
	}

	// 2. Prepare nodes
	nodes := UnwrapRoot(t)
	if cfg.preprocess != nil {
		nodes = cfg.preprocess(nodes, cfg.Config)
	}

	// 3. Rebuild application tree with provider and expansion.
	t = treeview.NewTree(nodes,
		treeview.WithExpandAll[treeview.FileInfo](),
		treeview.WithProvider(tui.CreateRenameProvider()),
	)
	if cfg.annotate != nil {
		cfg.annotate(t, cfg.Config)
	}

	// Mark files for deletion based on flags
	MarkFilesForDeletion(t, cfg.DeleteNFO, cfg.DeleteImages)

	// Propagate link mode to all nodes if linking is enabled
	if cfg.LinkMode != core.LinkModeNone {
		SetLinkMode(t, cfg.LinkMode, cfg.LinkTarget)
	}

	// Create model
	model := tui.NewRenameModel(t)
	model.IsMovieMode = cfg.movieMode
	model.DeleteNFO = cfg.DeleteNFO
	model.DeleteImages = cfg.DeleteImages
	model.LinkMode = cfg.LinkMode
	model.LinkTarget = cfg.LinkTarget

	// If instant mode, perform renames immediately
	if cfg.InstantMode {
		cmd := model.PerformRenames()
		if cmd != nil {
			msg := cmd()
			if result, ok := msg.(tui.RenameCompleteMsg); ok && result.ErrorCount() > 0 {
				return fmt.Errorf("%d errors occurred during renaming", result.ErrorCount())
			}
		}
		return nil
	}
	// 4. Launch rename TUI
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// CreateMediaFilter returns a filter function that excludes common junk files
// and optionally filters for specific file types based on the includeDirectories parameter.
func CreateMediaFilter(includeDirectories bool) func(info treeview.FileInfo) bool {
	return func(info treeview.FileInfo) bool {
		if info.Name() == ".DS_Store" || strings.HasPrefix(info.Name(), "._") {
			return false
		}
		if includeDirectories {
			return info.IsDir() || media.IsSubtitle(info.Name()) || media.IsVideo(info.Name()) || media.IsNFO(info.Name()) || media.IsImage(info.Name())
		}
		return media.IsSubtitle(info.Name()) || media.IsVideo(info.Name()) || media.IsNFO(info.Name()) || media.IsImage(info.Name())
	}
}

// UnwrapRoot returns children of single root directory, otherwise original nodes.
// When unwrapping, it clones just the immediate children to clear their parent references.
func UnwrapRoot(t *treeview.Tree[treeview.FileInfo]) []*treeview.Node[treeview.FileInfo] {
	ns := t.Nodes()
	if len(ns) == 1 && ns[0].Data().IsDir() {
		children := ns[0].Children()
		// Clone just the immediate children to clear their parent references
		cloned := make([]*treeview.Node[treeview.FileInfo], len(children))
		for i, child := range children {
			// Clone the node to clear its parent reference
			clone := treeview.NewNodeClone(child)
			// Keep the original children
			clone.SetChildren(child.Children())
			cloned[i] = clone
		}
		return cloned
	}
	return ns
}

// MarkFilesForDeletion traverses the tree and marks NFO and/or image files for deletion
func MarkFilesForDeletion(t *treeview.Tree[treeview.FileInfo], deleteNFO, deleteImages bool) {
	if !deleteNFO && !deleteImages {
		return
	}

	for ni := range t.All(context.Background()) {
		if ni.Node.Data().IsDir() {
			continue
		}

		filename := ni.Node.Name()
		shouldDelete := false

		if deleteNFO && media.IsNFO(filename) {
			shouldDelete = true
		}

		if deleteImages && media.IsImage(filename) {
			shouldDelete = true
		}

		if shouldDelete {
			meta := core.EnsureMeta(ni.Node)
			meta.MarkedForDeletion = true
		}
	}
}

// SetLinkMode propagates link mode configuration to all nodes in the tree
func SetLinkMode(t *treeview.Tree[treeview.FileInfo], linkMode core.LinkMode, linkTarget string) {
	for ni := range t.All(context.Background()) {
		meta := core.EnsureMeta(ni.Node)
		meta.LinkMode = linkMode
		meta.LinkTarget = linkTarget
	}
}
