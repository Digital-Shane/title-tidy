package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
)

func TestMovieAnnotate_FileTypes(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantNewName string
		wantType    core.MediaType
	}{
		{
			name:        "video file",
			filename:    "movie.mkv",
			wantNewName: "Test Movie (2024).mkv",
			wantType:    core.MediaMovieFile,
		},
		{
			name:        "subtitle file",
			filename:    "movie.en.srt",
			wantNewName: "Test Movie (2024).en.srt",
			wantType:    core.MediaMovieFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testNewDirNode("Test.Movie.2024")
			file := testNewFileNode(tt.filename)
			dir.AddChild(file)
			tr := testNewTree(dir)

			MovieAnnotate(tr, config.DefaultConfig(), "", nil)

			fm := core.GetMeta(file)
			if fm == nil {
				t.Fatalf("MovieAnnotate didn't create meta for %s", tt.filename)
			}
			if fm.Type != tt.wantType {
				t.Errorf("MovieAnnotate(%s) type = %v, want %v", tt.filename, fm.Type, tt.wantType)
			}
			if fm.NewName != tt.wantNewName {
				t.Errorf("MovieAnnotate(%s) NewName = %q, want %q", tt.filename, fm.NewName, tt.wantNewName)
			}
		})
	}
}

func TestMovieAnnotateWithLinking(t *testing.T) {
	dir := testNewDirNode("Test.Movie.2024")
	videoFile := testNewFileNode("movie.mkv")
	subtitleFile := testNewFileNode("movie.en.srt")
	dir.AddChild(videoFile)
	dir.AddChild(subtitleFile)
	tr := testNewTree(dir)

	linkPath := "/test/destination"
	MovieAnnotate(tr, config.DefaultConfig(), linkPath, nil)

	// Verify directory metadata and destination path
	dm := core.GetMeta(dir)
	if dm == nil {
		t.Fatal("MovieAnnotateWithLinking(directory) meta = nil, want metadata")
	}
	if dm.Type != core.MediaMovie {
		t.Errorf("MovieAnnotateWithLinking(directory) Type = %v, want %v", dm.Type, core.MediaMovie)
	}
	if dm.NewName == "" {
		t.Error("MovieAnnotateWithLinking(directory) NewName = empty, want non-empty")
	}
	wantDirDest := "/test/destination/Test Movie (2024)"
	if dm.DestinationPath != wantDirDest {
		t.Errorf("MovieAnnotateWithLinking(directory) DestinationPath = %q, want %q", dm.DestinationPath, wantDirDest)
	}

	// Verify file metadata and destination paths
	tests := []struct {
		name     string
		node     *treeview.Node[treeview.FileInfo]
		wantDest string
	}{
		{
			name:     "video file",
			node:     videoFile,
			wantDest: "/test/destination/Test Movie (2024)/Test Movie (2024).mkv",
		},
		{
			name:     "subtitle file",
			node:     subtitleFile,
			wantDest: "/test/destination/Test Movie (2024)/Test Movie (2024).en.srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := core.GetMeta(tt.node)
			if fm == nil {
				t.Fatalf("MovieAnnotateWithLinking(%s) meta = nil, want metadata", tt.name)
			}
			if fm.Type != core.MediaMovieFile {
				t.Errorf("MovieAnnotateWithLinking(%s) Type = %v, want %v", tt.name, fm.Type, core.MediaMovieFile)
			}
			if fm.NewName == "" {
				t.Errorf("MovieAnnotateWithLinking(%s) NewName = empty, want non-empty", tt.name)
			}
			if fm.DestinationPath != tt.wantDest {
				t.Errorf("MovieAnnotateWithLinking(%s) DestinationPath = %q, want %q", tt.name, fm.DestinationPath, tt.wantDest)
			}
		})
	}
}

func TestMovieAnnotateWithoutLinking(t *testing.T) {
	dir := testNewDirNode("Test.Movie.2024")
	file := testNewFileNode("movie.mkv")
	dir.AddChild(file)
	tr := testNewTree(dir)

	MovieAnnotate(tr, config.DefaultConfig(), "", nil)

	// Verify no destination paths are set when not linking
	dm := core.GetMeta(dir)
	if dm.DestinationPath != "" {
		t.Errorf("MovieAnnotateWithoutLinking(directory) DestinationPath = %q, want empty", dm.DestinationPath)
	}

	fm := core.GetMeta(file)
	if fm.DestinationPath != "" {
		t.Errorf("MovieAnnotateWithoutLinking(file) DestinationPath = %q, want empty", fm.DestinationPath)
	}
}

func TestMoviePreprocess_DefensiveEmptyExtension(t *testing.T) {
	// Test the defensive check for empty suffix
	nodeWithEmptyExt := testNewFileNode("movie") // no extension
	video := testNewFileNode("movie.mkv")
	nodes := []*treeview.Node[treeview.FileInfo]{nodeWithEmptyExt, video}

	out := MoviePreprocess(nodes, config.DefaultConfig())

	// The file with no extension should be left alone or bundled
	foundOriginal := false
	foundInVirtual := false
	for _, n := range out {
		if n.Name() == "movie" {
			foundOriginal = true
		}
		// Check if it's inside a virtual directory
		for _, child := range n.Children() {
			if child.Name() == "movie" {
				foundInVirtual = true
			}
		}
	}
	if !foundOriginal && !foundInVirtual {
		t.Errorf("MoviePreprocess lost file with no extension")
	}
}

func TestMoviePreprocess_SubtitleDefensiveEmptySuffix(t *testing.T) {
	// Test defensive check for subtitles with empty suffix (lines 60-61)
	video := testNewFileNode("movie.mkv")
	// Create a subtitle that would return empty suffix
	badSubtitle := testNewFileNode("movie.srt") // This should return empty suffix from ExtractSubtitleSuffix
	nodes := []*treeview.Node[treeview.FileInfo]{video, badSubtitle}

	out := MoviePreprocess(nodes, config.DefaultConfig())

	// Should create one virtual directory for the video
	virtualCount := 0
	for _, n := range out {
		if m := core.GetMeta(n); m != nil && m.IsVirtual {
			virtualCount++
		}
	}

	// Should have one virtual directory for the video
	if virtualCount != 1 {
		t.Errorf("MoviePreprocess with empty suffix subtitle = %d virtual dirs, want 1", virtualCount)
	}
}

func TestMovieAnnotate_ChildWithoutParentNewName(t *testing.T) {
	// Test lines 105-106: parent without NewName should be skipped
	dir := testNewDirNode("Movie.Directory")
	child := testNewFileNode("movie.mkv")
	dir.AddChild(child)
	tr := testNewTree(dir)

	// Pre-annotate directory but don't set NewName
	dirMeta := core.EnsureMeta(dir)
	dirMeta.Type = core.MediaMovie
	// Don't set NewName - should cause child to be skipped

	MovieAnnotate(tr, config.DefaultConfig(), "", nil)

	// Child should not have been annotated
	childMeta := core.GetMeta(child)
	if childMeta != nil && childMeta.Type == core.MediaMovieFile {
		t.Errorf("MovieAnnotate should have skipped child when parent has no NewName")
	}
}
