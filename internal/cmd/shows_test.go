package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestShowsCommandAnnotate(t *testing.T) {
	show := testNewDirNode("Some.Show.2024")
	season := testNewDirNode("Season 2")
	ep := testNewFileNode("S02E03.mkv")
	season.AddChild(ep)
	show.AddChild(season)
	tr := testNewTree(show)

	ShowsCommand.annotate(tr, config.DefaultConfig(), "", nil)

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

func TestShowsCommandAnnotateWithLinking(t *testing.T) {
	show := testNewDirNode("Some.Show.2024")
	season := testNewDirNode("Season 2")
	ep1 := testNewFileNode("S02E03.mkv")
	ep2 := testNewFileNode("S02E04.en.srt")
	season.AddChild(ep1)
	season.AddChild(ep2)
	show.AddChild(season)
	tr := testNewTree(show)

	linkPath := "/test/destination"
	ShowsCommand.annotate(tr, config.DefaultConfig(), linkPath, nil)

	// Verify show metadata and destination path
	sm := core.GetMeta(show)
	if sm == nil {
		t.Fatal("ShowsCommandAnnotateWithLinking(show) meta = nil, want metadata")
	}
	if sm.Type != core.MediaShow {
		t.Errorf("ShowsCommandAnnotateWithLinking(show) Type = %v, want %v", sm.Type, core.MediaShow)
	}
	if sm.NewName == "" {
		t.Error("ShowsCommandAnnotateWithLinking(show) NewName = empty, want non-empty")
	}
	wantShowDest := "/test/destination/Some Show (2024)"
	if sm.DestinationPath != wantShowDest {
		t.Errorf("ShowsCommandAnnotateWithLinking(show) DestinationPath = %q, want %q", sm.DestinationPath, wantShowDest)
	}

	// Verify season metadata and destination path
	sem := core.GetMeta(season)
	if sem == nil {
		t.Fatal("ShowsCommandAnnotateWithLinking(season) meta = nil, want metadata")
	}
	if sem.Type != core.MediaSeason {
		t.Errorf("ShowsCommandAnnotateWithLinking(season) Type = %v, want %v", sem.Type, core.MediaSeason)
	}
	if sem.NewName == "" {
		t.Error("ShowsCommandAnnotateWithLinking(season) NewName = empty, want non-empty")
	}
	wantSeasonDest := "/test/destination/Some Show (2024)/Season 02"
	if sem.DestinationPath != wantSeasonDest {
		t.Errorf("ShowsCommandAnnotateWithLinking(season) DestinationPath = %q, want %q", sem.DestinationPath, wantSeasonDest)
	}

	// Verify episode metadata and destination paths
	tests := []struct {
		name     string
		node     *treeview.Node[treeview.FileInfo]
		wantDest string
	}{
		{
			name:     "video episode",
			node:     ep1,
			wantDest: "/test/destination/Some Show (2024)/Season 02/S02E03.mkv",
		},
		{
			name:     "subtitle episode",
			node:     ep2,
			wantDest: "/test/destination/Some Show (2024)/Season 02/S02E04.en.srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			em := core.GetMeta(tt.node)
			if em == nil {
				t.Fatalf("ShowsCommandAnnotateWithLinking(%s) meta = nil, want metadata", tt.name)
			}
			if em.Type != core.MediaEpisode {
				t.Errorf("ShowsCommandAnnotateWithLinking(%s) Type = %v, want %v", tt.name, em.Type, core.MediaEpisode)
			}
			if em.NewName == "" {
				t.Errorf("ShowsCommandAnnotateWithLinking(%s) NewName = empty, want non-empty", tt.name)
			}
			if em.DestinationPath != tt.wantDest {
				t.Errorf("ShowsCommandAnnotateWithLinking(%s) DestinationPath = %q, want %q", tt.name, em.DestinationPath, tt.wantDest)
			}
		})
	}
}

func TestShowsCommandAnnotateWithoutLinking(t *testing.T) {
	show := testNewDirNode("Some.Show.2024")
	season := testNewDirNode("Season 2")
	ep := testNewFileNode("S02E03.mkv")
	season.AddChild(ep)
	show.AddChild(season)
	tr := testNewTree(show)

	ShowsCommand.annotate(tr, config.DefaultConfig(), "", nil)

	// Verify no destination paths are set when not linking
	sm := core.GetMeta(show)
	if sm.DestinationPath != "" {
		t.Errorf("ShowsCommandAnnotateWithoutLinking(show) DestinationPath = %q, want empty", sm.DestinationPath)
	}

	sem := core.GetMeta(season)
	if sem.DestinationPath != "" {
		t.Errorf("ShowsCommandAnnotateWithoutLinking(season) DestinationPath = %q, want empty", sem.DestinationPath)
	}

	em := core.GetMeta(ep)
	if em.DestinationPath != "" {
		t.Errorf("ShowsCommandAnnotateWithoutLinking(episode) DestinationPath = %q, want empty", em.DestinationPath)
	}
}
