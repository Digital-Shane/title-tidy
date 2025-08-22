package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
)

func TestShowsCommandAnnotate(t *testing.T) {
	show := testNewDirNode("Some.Show.2024")
	season := testNewDirNode("Season 2")
	ep := testNewFileNode("S02E03.mkv")
	season.AddChild(ep)
	show.AddChild(season)
	tr := testNewTree(show)

	ShowsCommand.annotate(tr, config.DefaultConfig())

	sm := core.GetMeta(show)
	if sm == nil || sm.Type != core.MediaShow || sm.NewName == "" {
		t.Fatalf("show meta = %#v, want show with NewName", sm)
	}
	sem := core.GetMeta(season)
	if sem == nil || sem.Type != core.MediaSeason || sem.NewName == "" {
		t.Fatalf("season meta = %#v, want season with NewName", sem)
	}
	em := core.GetMeta(ep)
	if em == nil || em.Type != core.MediaEpisode || em.NewName == "" {
		t.Fatalf("episode meta = %#v, want episode with NewName", em)
	}
}
