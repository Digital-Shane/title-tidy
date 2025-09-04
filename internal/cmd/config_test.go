package cmd

import (
	"context"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestCreateMediaFilter(t *testing.T) {
	t.Run("exclude junk files", func(t *testing.T) {
		f := CreateMediaFilter(false)
		// .DS_Store always filtered
		assertBool(t, f(testTreeviewFileInfo(".DS_Store", false)), false, "CreateMediaFilter(.DS_Store)")
		assertBool(t, f(testTreeviewFileInfo("._thumbs", false)), false, "CreateMediaFilter(._ prefix)")
	})
}

func TestUnwrapRoot(t *testing.T) {
	// single root directory => unwrap children (cloned to clear parent refs)
	rootDir := testNewDirNode("Root")
	childA := testNewFileNode("a.mkv")
	childB := testNewFileNode("b.srt")
	rootDir.AddChild(childA)
	rootDir.AddChild(childB)
	tr1 := testNewTree(rootDir)
	got := UnwrapRoot(tr1)
	if len(got) != 2 {
		t.Errorf("UnwrapRoot(single) returned %d nodes, want 2", len(got))
	}
	// Check that we got clones with same data but no parent
	if got[0].Name() != childA.Name() || got[0].Data().Name() != childA.Data().Name() {
		t.Errorf("UnwrapRoot(single) first node = %v, want clone of %v", got[0].Name(), childA.Name())
	}
	if got[1].Name() != childB.Name() || got[1].Data().Name() != childB.Data().Name() {
		t.Errorf("UnwrapRoot(single) second node = %v, want clone of %v", got[1].Name(), childB.Name())
	}
	// Verify parent references are cleared
	if got[0].Parent() != nil {
		t.Errorf("UnwrapRoot(single) first node still has parent reference")
	}
	if got[1].Parent() != nil {
		t.Errorf("UnwrapRoot(single) second node still has parent reference")
	}

	// multiple top nodes => unchanged
	a := testNewFileNode("a.mkv")
	b := testNewFileNode("b.mkv")
	tr2 := testNewTree(a, b)
	got2 := UnwrapRoot(tr2)
	if len(got2) != 2 || got2[0] != a || got2[1] != b {
		t.Errorf("UnwrapRoot(multi) = %v, want originals [%v %v]", got2, a, b)
	}
}

func TestMarkFilesForDeletion(t *testing.T) {
	tests := []struct {
		name          string
		deleteNFO     bool
		deleteImages  bool
		deleteSamples bool
		setupTree     func() *treeview.Tree[treeview.FileInfo]
		wantDeleted   []string
		wantKept      []string
	}{
		{
			name:          "no deletion flags",
			deleteNFO:     false,
			deleteImages:  false,
			deleteSamples: false,
			setupTree: func() *treeview.Tree[treeview.FileInfo] {
				return testNewTree(
					testNewFileNode("movie.mkv"),
					testNewFileNode("movie.nfo"),
					testNewFileNode("poster.jpg"),
					testNewFileNode("sample.mkv"),
				)
			},
			wantKept: []string{"movie.mkv", "movie.nfo", "poster.jpg", "sample.mkv"},
		},
		{
			name:          "delete both NFO and images",
			deleteNFO:     true,
			deleteImages:  true,
			deleteSamples: false,
			setupTree: func() *treeview.Tree[treeview.FileInfo] {
				return testNewTree(
					testNewFileNode("movie.mkv"),
					testNewFileNode("movie.nfo"),
					testNewFileNode("poster.jpg"),
				)
			},
			wantDeleted: []string{"movie.nfo", "poster.jpg"},
			wantKept:    []string{"movie.mkv"},
		},
		{
			name:          "delete sample files",
			deleteNFO:     false,
			deleteImages:  false,
			deleteSamples: true,
			setupTree: func() *treeview.Tree[treeview.FileInfo] {
				sampleDir := testNewDirNode("Sample")
				sampleDir.AddChild(testNewFileNode("preview.mkv"))
				return testNewTree(
					testNewFileNode("movie.mkv"),
					testNewFileNode("sample.mp4"),
					testNewFileNode("movie-sample.avi"),
					sampleDir,
					testNewFileNode("sample.txt"),
				)
			},
			wantDeleted: []string{"sample.mp4", "movie-sample.avi", "Sample", "sample.txt"},
			wantKept:    []string{"movie.mkv"},
		},
		{
			name:          "delete all types",
			deleteNFO:     true,
			deleteImages:  true,
			deleteSamples: true,
			setupTree: func() *treeview.Tree[treeview.FileInfo] {
				return testNewTree(
					testNewFileNode("movie.mkv"),
					testNewFileNode("movie.nfo"),
					testNewFileNode("poster.jpg"),
					testNewFileNode("sample.mkv"),
				)
			},
			wantDeleted: []string{"movie.nfo", "poster.jpg", "sample.mkv"},
			wantKept:    []string{"movie.mkv"},
		},
		{
			name:          "skip directories",
			deleteNFO:     true,
			deleteImages:  true,
			deleteSamples: false,
			setupTree: func() *treeview.Tree[treeview.FileInfo] {
				dir := testNewDirNode("Season 01")
				dir.AddChild(testNewFileNode("episode.nfo"))
				return testNewTree(
					dir,
					testNewFileNode("show.nfo"),
				)
			},
			wantDeleted: []string{"show.nfo", "episode.nfo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := tt.setupTree()
			MarkFilesForDeletion(tree, tt.deleteNFO, tt.deleteImages, tt.deleteSamples)

			// Check deleted files
			for _, filename := range tt.wantDeleted {
				node := findNodeByName(tree, filename)
				if node == nil {
					t.Errorf("Could not find node %q in tree", filename)
					continue
				}
				meta := core.GetMeta(node)
				if meta == nil || !meta.MarkedForDeletion {
					t.Errorf("File %q should be marked for deletion but wasn't", filename)
				}
			}

			// Check kept files
			for _, filename := range tt.wantKept {
				node := findNodeByName(tree, filename)
				if node == nil {
					t.Errorf("Could not find node %q in tree", filename)
					continue
				}
				meta := core.GetMeta(node)
				if meta != nil && meta.MarkedForDeletion {
					t.Errorf("File %q should not be marked for deletion but was", filename)
				}
			}
		})
	}
}

func TestCreateMediaFilterWithNFOAndImages(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		isDir       bool
		includeDirs bool
		want        bool
	}{
		{"nfo file", "movie.nfo", false, false, true},
		{"jpg image", "poster.jpg", false, false, true},
		{"png image", "fanart.png", false, false, true},
		{"jpeg image", "backdrop.jpeg", false, false, true},
		{"NFO uppercase", "MOVIE.NFO", false, false, true},
		{"image with dirs", "poster.jpg", false, true, true},
		{"directory with dirs", "Season 01", true, true, true},
		{"directory without dirs", "Season 01", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := CreateMediaFilter(tt.includeDirs)
			info := testTreeviewFileInfo(tt.filename, tt.isDir)
			got := filter(info)
			if got != tt.want {
				t.Errorf("CreateMediaFilter(%v)(%q, isDir=%v) = %v, want %v",
					tt.includeDirs, tt.filename, tt.isDir, got, tt.want)
			}
		})
	}
}

// Helper function to find a node by name in the tree
func findNodeByName(tree *treeview.Tree[treeview.FileInfo], name string) *treeview.Node[treeview.FileInfo] {
	for ni := range tree.All(context.Background()) {
		if ni.Node.Name() == name {
			return ni.Node
		}
	}
	return nil
}

// Shared helpers for cmd package tests
func testNewFileNode(name string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(name, false), Path: name})
}
func testNewDirNode(name string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(name, true), Path: name})
}
func testNewTree(nodes ...*treeview.Node[treeview.FileInfo]) *treeview.Tree[treeview.FileInfo] {
	return treeview.NewTree(nodes)
}
func testTreeviewFileInfo(name string, isDir bool) treeview.FileInfo {
	return treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(name, isDir), Path: name}
}
func assertBool(t *testing.T, got bool, want bool, desc string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", desc, got, want)
	}
}
