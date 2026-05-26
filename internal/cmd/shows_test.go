package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview/v2"
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

func TestAnnotateShowsTreeIgnoresEpisodeBracketHash(t *testing.T) {
	cfg := config.DefaultConfig()

	show := newShowsTestNode("Kaichou wa Maid-sama!", true, "Kaichou wa Maid-sama!")
	season := newShowsTestNode("Season 01", true, "Kaichou wa Maid-sama!/Season 01")
	episode := newShowsTestNode("[sam] Kaichou wa Maid-sama! - 17 [BD 1080p FLAC] [0E123677].mkv", false, "Kaichou wa Maid-sama!/Season 01/[sam] Kaichou wa Maid-sama! - 17 [BD 1080p FLAC] [0E123677].mkv")

	season.SetChildren([]*treeview.Node[treeview.FileInfo]{episode})
	show.SetChildren([]*treeview.Node[treeview.FileInfo]{season})

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{show})
	annotateShowsTree(tree, cfg, nil)

	episodeMeta := core.GetMeta(episode)
	if episodeMeta == nil {
		t.Fatal("episode metadata missing")
	}
	if episodeMeta.NewName != "S01E17.mkv" {
		t.Errorf("episode rename = %q, want %q", episodeMeta.NewName, "S01E17.mkv")
	}
}
