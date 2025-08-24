package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestSeasonsCommandAnnotate(t *testing.T) {
	season1 := testNewDirNode("Season 1")
	ep1 := testNewFileNode("Episode.1.mkv")
	ep2 := testNewFileNode("Episode.2.srt")
	season1.AddChild(ep1)
	season1.AddChild(ep2)
	season2 := testNewDirNode("Season 02")
	tr := testNewTree(season1, season2)

	SeasonsCommand.annotate(tr, config.DefaultConfig())

	for _, s := range []*treeview.Node[treeview.FileInfo]{season1, season2} {
		mm := core.GetMeta(s)
		if mm == nil || mm.Type != core.MediaSeason || mm.NewName == "" {
			t.Errorf("season meta (%s) = %#v, want season with NewName", s.Name(), mm)
		}
	}
	for _, e := range []*treeview.Node[treeview.FileInfo]{ep1, ep2} {
		mm := core.GetMeta(e)
		if mm == nil || mm.Type != core.MediaEpisode || mm.NewName == "" {
			t.Errorf("episode meta (%s) = %#v, want episode with NewName", e.Name(), mm)
		}
	}
}
