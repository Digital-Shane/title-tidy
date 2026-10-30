package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview/v2"
)

func regexCutConfig() *config.FormatConfig {
	cfg := config.DefaultConfig()
	cfg.PreserveExistingTags = true
	cfg.FilenameReplacementsRegex = true
	cfg.FilenameReplacements = config.FilenameReplacements{
		{Search: `[Ee]xtended(.[Vv]ersion)?`, Replacement: "@Extended@", Regex: true},
		{Search: `\[.+?\]`, Replacement: "", Regex: true},
		{Search: `@Extended@`, Replacement: "[Extended]", Regex: true},
	}
	return cfg
}

func preprocessingNode(name string, isDir bool) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{
		FileInfo: core.NewSimpleFileInfo(name, isDir),
		Path:     filepath.Join("/library", name),
	})
}

func TestRegexPreprocessingMovieWorkflow(t *testing.T) {
	prevNoDir, prevLinkPath := noDir, linkPath
	t.Cleanup(func() { noDir, linkPath = prevNoDir, prevLinkPath })
	for _, flat := range []bool{true, false} {
		for _, link := range []string{"", "/destination"} {
			name := "Argo_Extended_version_(2012)_[H.264].[AAC].[DVDRip]"
			video := preprocessingNode(name+".mp4", false)
			subtitle := preprocessingNode(name+".en.srt", false)
			cfg := regexCutConfig()
			noDir, linkPath = flat, link
			nodes := moviePreprocess([]*treeview.Node[treeview.FileInfo]{video, subtitle}, cfg, flat)
			tree := treeview.NewTree(nodes)
			core.PrepareFilenamePreprocessing(nodes, cfg.FilenamePreprocessor())
			items := core.CollectMetadataItems(tree, local.New())
			if len(items) != 1 || items[0].Name != "Argo" || items[0].Year != "2012" {
				t.Fatalf("metadata requests include release tags: %+v", items)
			}
			annotateMoviesTree(tree, cfg, nil)
			for node, want := range map[*treeview.Node[treeview.FileInfo]]string{
				video: "Argo (2012) [Extended].mp4", subtitle: "Argo (2012) [Extended].en.srt",
			} {
				if got := core.GetMeta(node).NewName; got != want {
					t.Errorf("flat=%v, link=%q: rename=%q, want %q", flat, link, got, want)
				}
				if node.Name() != node.Data().FileInfo.Name() || node.Data().Path != filepath.Join("/library", node.Name()) {
					t.Fatal("preprocessing changed source names/paths")
				}
			}
			if !flat && core.GetMeta(nodes[0]).NewName != "Argo (2012) [Extended]" {
				t.Errorf("virtual movie directory = %q", core.GetMeta(nodes[0]).NewName)
			}
			// Annotation can be refreshed without cascading the preprocessing again.
			annotateMoviesTree(tree, cfg, nil)
			if got := core.GetMeta(video).NewName; got != "Argo (2012) [Extended].mp4" {
				t.Errorf("refreshed rename = %q", got)
			}
		}
	}
}

func TestRegexPreprocessingTVWorkflows(t *testing.T) {
	for _, mode := range []string{"shows", "seasons", "episodes"} {
		t.Run(mode, func(t *testing.T) {
			cfg := regexCutConfig()
			cfg.Episode = "{title} S{season}E{episode}"
			show := preprocessingNode("Argo_Extended_version_(2012)_[AAC]", true)
			season := preprocessingNode("Season 02", true)
			episode := preprocessingNode("Argo_Extended_version_(2012)_S02E03_[AAC].mkv", false)
			show.AddChild(season)
			season.AddChild(episode)
			var tree *treeview.Tree[treeview.FileInfo]
			switch mode {
			case "shows":
				tree = treeview.NewTree([]*treeview.Node[treeview.FileInfo]{show})
				annotateShowsTree(tree, cfg, nil)
			case "seasons":
				tree = treeview.NewTree([]*treeview.Node[treeview.FileInfo]{season})
				annotateSeasonsTree(tree, cfg, nil)
			case "episodes":
				tree = treeview.NewTree([]*treeview.Node[treeview.FileInfo]{episode})
				annotateEpisodesTree(tree, cfg, nil)
			}
			if got := core.GetMeta(episode).NewName; got != "Argo S02E03 [Extended].mkv" {
				t.Errorf("episode rename = %q", got)
			}
			meta, err := fetchLocalMetadata(context.Background(), episode, provider.MediaTypeEpisode)
			if err != nil || meta.Core.Title != "Argo" || meta.Core.Year != "2012" {
				t.Fatalf("parsed TV metadata = %+v, %v", meta, err)
			}
		})
	}
}
