package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview/v2"
)

func TestMultiEpisodeWorkflows(t *testing.T) {
	previousLinkPath := linkPath
	t.Cleanup(func() { linkPath = previousLinkPath })

	for _, mode := range []string{"shows", "seasons", "episodes"} {
		for _, enriched := range []bool{false, true} {
			for _, destination := range []string{"", "/destination"} {
				t.Run(fmt.Sprintf("%s/enriched=%t/link=%t", mode, enriched, destination != ""), func(t *testing.T) {
					linkPath = destination
					cfg := config.DefaultConfig()
					show := preprocessingNode("Salem's Lot (1979)", true)
					season := preprocessingNode("Season 1", true)
					episode := preprocessingNode("Salem's Lot (1979) S01E01E02 1080p BluRay Opus 2.0 AV1-WhiSe.mkv", false)
					subtitle := preprocessingNode("Salem's Lot (1979) S01E01-E02.en.srt", false)
					show.AddChild(season)
					season.AddChild(episode)
					season.AddChild(subtitle)

					var roots []*treeview.Node[treeview.FileInfo]
					switch mode {
					case "shows":
						roots = []*treeview.Node[treeview.FileInfo]{show}
					case "seasons":
						roots = []*treeview.Node[treeview.FileInfo]{season}
					case "episodes":
						roots = []*treeview.Node[treeview.FileInfo]{episode, subtitle}
					}
					tree := treeview.NewTree(roots)
					key := provider.GenerateMetadataKey("episode", "Salem's Lot", "1979", 1, 1)
					episodeRequests := 0
					for _, item := range core.CollectMetadataItems(tree, local.New()) {
						if item.MediaType == provider.MediaTypeEpisode {
							episodeRequests++
							if item.Key != key || item.Season != 1 || item.Episode != 1 {
								t.Errorf("metadata lookup = %+v, want season 1, episode 1", item)
							}
						}
					}
					if episodeRequests != 1 {
						t.Fatalf("episode metadata requests = %d, want 1 shared by video and subtitle", episodeRequests)
					}

					var metadata map[string]*provider.Metadata
					if enriched {
						cfg.Episode = "{title} - S{season}E{episode} - {episode_title}"
						metadata = map[string]*provider.Metadata{
							key: {Core: provider.CoreMetadata{Title: "Salem's Lot", EpisodeName: "Part 1", SeasonNum: 1, EpisodeNum: 1}},
						}
					}
					switch mode {
					case "shows":
						annotateShowsTree(tree, cfg, metadata)
					case "seasons":
						annotateSeasonsTree(tree, cfg, metadata)
					case "episodes":
						annotateEpisodesTree(tree, cfg, metadata)
					}
					base := "S01E01-E02"
					if enriched {
						base = "Salem's Lot - " + base + " - Part 1"
					}
					for node, extension := range map[*treeview.Node[treeview.FileInfo]]string{episode: ".mkv", subtitle: ".en.srt"} {
						meta := core.GetMeta(node)
						if meta == nil || meta.NewName != base+extension {
							t.Fatalf("rename for %q = %+v, want %q", node.Name(), meta, base+extension)
						}
						if destination != "" && filepath.Base(meta.DestinationPath) != base+extension {
							t.Errorf("link destination = %q, want filename %q", meta.DestinationPath, base+extension)
						}
					}
				})
			}
		}
	}
}
