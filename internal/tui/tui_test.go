package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestUndoCompleteMsgAccessors(t *testing.T) {
	msg := UndoCompleteMsg{successCount: 3, errorCount: 1}
	if got := msg.SuccessCount(); got != 3 {
		t.Errorf("SuccessCount() = %d, want 3", got)
	}
	if got := msg.ErrorCount(); got != 1 {
		t.Errorf("ErrorCount() = %d, want 1", got)
	}
}

func TestRenameModelGetIconFallback(t *testing.T) {
	model := NewRenameModel(treeWithRoots(t, newTestNode("root", true)))
	model.iconSet = map[string]string{"stats": "STAT"}

	if got := model.getIcon("stats"); got != "STAT" {
		t.Fatalf("getIcon(stats) = %q, want %q", got, "STAT")
	}

	got := model.getIcon("needrename")
	if want := ASCIIIcons["needrename"]; got != want {
		t.Fatalf("getIcon(needrename) = %q, want %q", got, want)
	}
}

func TestRenderHeaderLinkMovie(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmp, err)
	}

	model := NewRenameModel(treeWithRoots(t, newTestNode("root", true)))
	model.IsLinkMode = true
	model.IsMovieMode = true
	model.LinkPath = filepath.Join(tmp, "dest")

	header := model.renderHeader()
	if !strings.Contains(header, "Movie Link") {
		t.Fatalf("renderHeader() missing 'Movie Link': %q", header)
	}
	if !strings.Contains(header, filepath.Base(tmp)) {
		t.Fatalf("renderHeader() missing working dir base %q: %q", filepath.Base(tmp), header)
	}
	if !strings.Contains(header, filepath.Base(model.LinkPath)) {
		t.Fatalf("renderHeader() missing link path base %q: %q", filepath.Base(model.LinkPath), header)
	}
}

func TestRemoveNodeFromTreeVariants(t *testing.T) {
	root := newTestNode("root", true)
	child := newTestNode("child", false)
	root.AddChild(child)
	second := newTestNode("second", false)

	model := NewRenameModel(treeWithRoots(t, root, second))

	model.removeNodeFromTree(nil)

	model.removeNodeFromTree(child)
	if children := root.Children(); len(children) != 0 {
		t.Fatalf("children after removal = %d, want 0", len(children))
	}

	model.removeNodeFromTree(second)
	roots := model.TuiTreeModel.Tree.Nodes()
	if len(roots) != 1 || roots[0] != root {
		t.Fatalf("root nodes after removal = %d (first=%v), want only root", len(roots), roots)
	}
}

func newTestNode(id string, isDir bool) *treeview.Node[treeview.FileInfo] {
	info := treeview.FileInfo{
		FileInfo: core.NewSimpleFileInfo(id, isDir),
		Path:     id,
		Extra:    map[string]any{},
	}
	return treeview.NewNode(id, id, info)
}

func treeWithRoots(t *testing.T, roots ...*treeview.Node[treeview.FileInfo]) *treeview.Tree[treeview.FileInfo] {
	t.Helper()
	return treeview.NewTree(roots)
}
