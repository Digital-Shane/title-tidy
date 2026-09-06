package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview/v2"
)

func TestEpisodesMetadataFromContainingDirectory(t *testing.T) {
	tests := []struct {
		name     string
		season   string
		filename string
		show     string
		year     string
	}{
		{name: "show folder", filename: "S02E01.mkv", show: "Example Show", year: "2020"},
		{name: "season folder", season: "Season 02", filename: "S02E01.mkv", show: "Example Show", year: "2020"},
		{name: "episode only", season: "Season 02", filename: "E01.mkv", show: "Example Show", year: "2020"},
		{name: "subtitle", season: "Season 02", filename: "E01.srt", show: "Example Show", year: "2020"},
		{name: "inherit year", season: "Season 02", filename: "Example.Show.S02E01.mkv", show: "Example Show", year: "2020"},
		{name: "prefer filename", season: "Season 02", filename: "Other.Show.S02E01.mkv", show: "Other Show"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "Example Show (2020)", tt.season)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, tt.filename), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			indexed, err := treeview.NewTreeFromFileSystem(context.Background(), dir, false,
				treeview.WithMaxDepth[treeview.FileInfo](1),
				treeview.WithFilterFunc(createIndexFilter()),
			)
			if err != nil {
				t.Fatal(err)
			}
			tree := treeview.NewTree(unwrapRoot(indexed), treeview.WithFilterFunc(createMediaFilter(false)))
			if len(tree.Nodes()) != 1 || tree.Nodes()[0].Parent() != nil {
				t.Fatal("episode should remain a root node in the rename preview")
			}

			key := provider.GenerateMetadataKey("episode", tt.show, tt.year, 2, 1)
			items := core.CollectMetadataItems(tree, local.New())
			found := false
			for _, item := range items {
				if item.Key == key && item.MediaType == provider.MediaTypeEpisode {
					found = true
				}
			}
			if !found {
				t.Fatalf("metadata requests = %+v, want episode key %q", items, key)
			}

			cfg := config.DefaultConfig()
			cfg.Episode = "{title} - S{season}E{episode} - {episode_title}"
			annotateEpisodesTree(tree, cfg, map[string]*provider.Metadata{
				key: {Core: provider.CoreMetadata{EpisodeName: "Pilot"}},
			})
			want := tt.show + " - S02E01 - Pilot" + filepath.Ext(tt.filename)
			if got := core.GetMeta(tree.Nodes()[0]).NewName; got != want {
				t.Errorf("rename = %q, want %q", got, want)
			}
		})
	}
}

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
