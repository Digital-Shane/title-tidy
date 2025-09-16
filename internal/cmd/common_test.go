package cmd

import (
	"strings"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

func TestCreateMediaFilter(t *testing.T) {
	tests := []struct {
		name               string
		includeDirectories bool
		fileInfo           treeview.FileInfo
		want               bool
	}{
		{
			name:               "video_file_with_dirs_included",
			includeDirectories: true,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("movie.mp4", false)},
			want:               true,
		},
		{
			name:               "video_file_without_dirs",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("movie.mp4", false)},
			want:               true,
		},
		{
			name:               "subtitle_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("movie.srt", false)},
			want:               true,
		},
		{
			name:               "nfo_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("movie.nfo", false)},
			want:               true,
		},
		{
			name:               "image_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("poster.jpg", false)},
			want:               true,
		},
		{
			name:               "directory_with_dirs_included",
			includeDirectories: true,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("Season 01", true)},
			want:               true,
		},
		{
			name:               "directory_without_dirs",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("Season 01", true)},
			want:               false,
		},
		{
			name:               "hidden_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(".hidden", false)},
			want:               false,
		},
		{
			name:               "DS_Store_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(".DS_Store", false)},
			want:               false,
		},
		{
			name:               "regular_file",
			includeDirectories: false,
			fileInfo:           treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("readme.txt", false)},
			want:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := createMediaFilter(tt.includeDirectories)
			got := filter(tt.fileInfo)
			if got != tt.want {
				t.Errorf("createMediaFilter(%v)(%v) = %v, want %v", tt.includeDirectories, tt.fileInfo.Name(), got, tt.want)
			}
		})
	}
}

func TestUnwrapRoot(t *testing.T) {
	tests := []struct {
		name     string
		tree     *treeview.Tree[treeview.FileInfo]
		wantLen  int
		wantRoot bool
	}{
		{
			name: "single_root_directory",
			tree: func() *treeview.Tree[treeview.FileInfo] {
				root := treeview.NewNode("root", "Show", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("Show", true),
					Extra:    make(map[string]any),
				})
				child1 := treeview.NewNode("c1", "Season 01", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("Season 01", true),
					Extra:    make(map[string]any),
				})
				child2 := treeview.NewNode("c2", "Season 02", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("Season 02", true),
					Extra:    make(map[string]any),
				})
				root.SetChildren([]*treeview.Node[treeview.FileInfo]{child1, child2})
				return treeview.NewTree([]*treeview.Node[treeview.FileInfo]{root})
			}(),
			wantLen:  2,
			wantRoot: false,
		},
		{
			name: "multiple_roots",
			tree: func() *treeview.Tree[treeview.FileInfo] {
				node1 := treeview.NewNode("n1", "Show1", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("Show1", true),
					Extra:    make(map[string]any),
				})
				node2 := treeview.NewNode("n2", "Show2", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("Show2", true),
					Extra:    make(map[string]any),
				})
				return treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node1, node2})
			}(),
			wantLen:  2,
			wantRoot: true,
		},
		{
			name: "single_file_root",
			tree: func() *treeview.Tree[treeview.FileInfo] {
				node := treeview.NewNode("f1", "movie.mp4", treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo("movie.mp4", false),
					Extra:    make(map[string]any),
				})
				return treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node})
			}(),
			wantLen:  1,
			wantRoot: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unwrapRoot(tt.tree)
			if len(got) != tt.wantLen {
				t.Errorf("unwrapRoot() returned %d nodes, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestMarkFilesForDeletion(t *testing.T) {
	tests := []struct {
		name          string
		deleteNFO     bool
		deleteImages  bool
		deleteSamples bool
		filename      string
		wantDeleted   bool
	}{
		{
			name:         "delete_nfo_file",
			deleteNFO:    true,
			deleteImages: false,
			filename:     "movie.nfo",
			wantDeleted:  true,
		},
		{
			name:         "keep_nfo_file",
			deleteNFO:    false,
			deleteImages: false,
			filename:     "movie.nfo",
			wantDeleted:  false,
		},
		{
			name:         "delete_image_file",
			deleteNFO:    false,
			deleteImages: true,
			filename:     "poster.jpg",
			wantDeleted:  true,
		},
		{
			name:          "delete_sample_file",
			deleteNFO:     false,
			deleteImages:  false,
			deleteSamples: true,
			filename:      "sample.mp4",
			wantDeleted:   true,
		},
		{
			name:         "keep_video_file",
			deleteNFO:    true,
			deleteImages: true,
			filename:     "movie.mp4",
			wantDeleted:  false,
		},
		{
			name:         "skip_directory",
			deleteNFO:    true,
			deleteImages: true,
			filename:     "Season 01",
			wantDeleted:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDir := tt.filename == "Season 01"
			node := treeview.NewNode("test", tt.filename, treeview.FileInfo{
				FileInfo: core.NewSimpleFileInfo(tt.filename, isDir),
				Extra:    make(map[string]any),
			})
			tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node})

			noNfo = tt.deleteNFO
			noImg = tt.deleteImages
			noSample = tt.deleteSamples

			markFilesForDeletion(tree)

			meta := core.GetMeta(node)
			gotDeleted := meta != nil && meta.MarkedForDeletion

			if gotDeleted != tt.wantDeleted {
				t.Errorf("markFilesForDeletion() for %s: MarkedForDeletion = %v, want %v",
					tt.filename, gotDeleted, tt.wantDeleted)
			}
		})
	}
}

func TestCreateFormatContext(t *testing.T) {
	cfg := &config.FormatConfig{
		ShowFolder:   "{title} ({year})",
		SeasonFolder: "Season {season}",
		Episode:      "S{season}E{episode}",
		Movie:        "{title} ({year})",
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:  "Enhanced Title",
			Year:   "2023",
			Rating: 8.5,
		},
	}

	tests := []struct {
		name      string
		showName  string
		movieName string
		year      string
		season    int
		episode   int
		metadata  *provider.Metadata
		want      *config.FormatContext
	}{
		{
			name:     "tv_show_context",
			showName: "Breaking Bad",
			year:     "2008",
			season:   1,
			episode:  2,
			metadata: metadata,
			want: &config.FormatContext{
				ShowName:  "Breaking Bad",
				MovieName: "",
				Year:      "2008",
				Season:    1,
				Episode:   2,
				Metadata:  metadata,
				Config:    cfg,
			},
		},
		{
			name:      "movie_context",
			movieName: "The Matrix",
			year:      "1999",
			metadata:  nil,
			want: &config.FormatContext{
				ShowName:  "",
				MovieName: "The Matrix",
				Year:      "1999",
				Season:    0,
				Episode:   0,
				Metadata:  nil,
				Config:    cfg,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createFormatContext(cfg, tt.showName, tt.movieName, tt.year, tt.season, tt.episode, tt.metadata)
			if diff := cmp.Diff(tt.want, got, cmp.FilterPath(func(p cmp.Path) bool {
				// Ignore unexported fields in FormatConfig like 'resolver'
				return strings.Contains(p.String(), ".resolver")
			}, cmp.Ignore())); diff != "" {
				t.Errorf("createFormatContext() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
