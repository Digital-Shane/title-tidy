package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func newShowsTestNode(name string, isDir bool, path string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{
		FileInfo: core.NewSimpleFileInfo(name, isDir),
		Path:     path,
		Extra:    map[string]any{},
	})
}

func TestAnnotateShowsTreeRenamesSeasonZeroSpecials(t *testing.T) {
	cfg := config.DefaultConfig()

	show := newShowsTestNode("Game.of.Thrones", true, "Game.of.Thrones")
	season := newShowsTestNode("Season 0", true, "Game.of.Thrones/Season 0")
	episode := newShowsTestNode("S00E00 test.mkv", false, "Game.of.Thrones/Season 0/S00E00 test.mkv")

	season.SetChildren([]*treeview.Node[treeview.FileInfo]{episode})
	show.SetChildren([]*treeview.Node[treeview.FileInfo]{season})

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{show})
	annotateShowsTree(tree, cfg, nil)

	seasonMeta := core.GetMeta(season)
	if seasonMeta == nil {
		t.Fatal("season metadata missing")
	}
	if seasonMeta.NewName != "Season 00" {
		t.Errorf("season rename = %q, want %q", seasonMeta.NewName, "Season 00")
	}

	episodeMeta := core.GetMeta(episode)
	if episodeMeta == nil {
		t.Fatal("episode metadata missing")
	}
	if episodeMeta.NewName != "S00E00.mkv" {
		t.Errorf("episode rename = %q, want %q", episodeMeta.NewName, "S00E00.mkv")
	}
}
