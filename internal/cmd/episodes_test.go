package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview/v2"
)

func newEpisodesTestNode(name string, path string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{
		FileInfo: core.NewSimpleFileInfo(name, false),
		Path:     path,
		Extra:    map[string]any{},
	})
}

func TestAnnotateEpisodesTreeRenamesSeasonZeroEpisodeZero(t *testing.T) {
	cfg := config.DefaultConfig()
	episode := newEpisodesTestNode("Breaking.Bad.0.00.mkv", "Breaking.Bad.0.00.mkv")
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{episode})

	annotateEpisodesTree(tree, cfg, nil)

	meta := core.GetMeta(episode)
	if meta == nil {
		t.Fatal("episode metadata missing")
	}
	if meta.NewName != "S00E00.mkv" {
		t.Errorf("episode rename = %q, want %q", meta.NewName, "S00E00.mkv")
	}
}

func TestAnnotateEpisodesTreeIgnoresBracketHash(t *testing.T) {
	cfg := config.DefaultConfig()
	episode := newEpisodesTestNode("[sam] Kaichou wa Maid-sama! - 17 [BD 1080p FLAC] [0E123677].mkv", "[sam] Kaichou wa Maid-sama! - 17 [BD 1080p FLAC] [0E123677].mkv")
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{episode})

	annotateEpisodesTree(tree, cfg, nil)

	meta := core.GetMeta(episode)
	if meta == nil {
		t.Fatal("episode metadata missing")
	}
	if meta.NewName != "S00E17.mkv" {
		t.Errorf("episode rename = %q, want %q", meta.NewName, "S00E17.mkv")
	}
}
