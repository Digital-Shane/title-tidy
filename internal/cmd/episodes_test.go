package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestEpisodesCommandAnnotate(t *testing.T) {
	f1 := testNewFileNode("S01E02.mkv")
	f2 := testNewFileNode("S01E03.en.srt")
	dir := testNewDirNode("ignoredDir")
	dir.AddChild(testNewFileNode("S01E04.mkv"))
	tr := testNewTree(f1, f2, dir)

	EpisodesCommand.annotate(tr, config.DefaultConfig(), "", nil)

	for _, n := range []*treeview.Node[treeview.FileInfo]{f1, f2} {
		mm := core.GetMeta(n)
		if mm == nil || mm.Type != core.MediaEpisode || mm.NewName == "" {
			t.Errorf("EpisodesCommand annotate meta (%s) = %#v, want episode with NewName", n.Name(), mm)
		}
	}
	if core.GetMeta(dir) != nil {
		t.Errorf("EpisodesCommand annotate directory meta unexpectedly set")
	}
}
