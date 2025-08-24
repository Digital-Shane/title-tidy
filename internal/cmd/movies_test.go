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

			MovieAnnotate(tr, config.DefaultConfig())

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

	MovieAnnotate(tr, config.DefaultConfig())

	// Child should not have been annotated
	childMeta := core.GetMeta(child)
	if childMeta != nil && childMeta.Type == core.MediaMovieFile {
		t.Errorf("MovieAnnotate should have skipped child when parent has no NewName")
	}
}
