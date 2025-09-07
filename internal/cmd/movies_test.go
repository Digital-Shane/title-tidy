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

	cfg := &CommandConfig{Config: config.DefaultConfig()}
	out := MoviePreprocess(nodes, cfg)

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

	cfg := &CommandConfig{Config: config.DefaultConfig()}
	out := MoviePreprocess(nodes, cfg)

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

func TestMoviePreprocess_NoDirectories(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantNewName string
	}{
		{
			name:        "video file without directory",
			filename:    "Test.Movie.2024.mkv",
			wantNewName: "Test Movie (2024).mkv",
		},
		{
			name:        "subtitle file without directory",
			filename:    "Test.Movie.2024.en.srt",
			wantNewName: "Test Movie (2024).en.srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := testNewFileNode(tt.filename)
			nodes := []*treeview.Node[treeview.FileInfo]{node}

			cfg := &CommandConfig{
				Config:        config.DefaultConfig(),
				NoDirectories: true,
			}
			out := MoviePreprocess(nodes, cfg)

			// Should return same nodes, not wrapped in virtual directories
			if len(out) != 1 {
				t.Fatalf("MoviePreprocess with NoDirectories = %d nodes, want 1", len(out))
			}

			// Check that metadata was applied directly to the file
			meta := core.GetMeta(out[0])
			if meta == nil {
				t.Fatal("MoviePreprocess with NoDirectories didn't create metadata")
			}
			if meta.Type != core.MediaMovieFile {
				t.Errorf("MoviePreprocess with NoDirectories Type = %v, want %v", meta.Type, core.MediaMovieFile)
			}
			if meta.NewName != tt.wantNewName {
				t.Errorf("MoviePreprocess with NoDirectories NewName = %q, want %q", meta.NewName, tt.wantNewName)
			}
			// Should not be marked as virtual or needing directory
			if meta.IsVirtual {
				t.Error("MoviePreprocess with NoDirectories IsVirtual = true, want false")
			}
			if meta.NeedsDirectory {
				t.Error("MoviePreprocess with NoDirectories NeedsDirectory = true, want false")
			}
		})
	}
}

func TestMoviePreprocess_WithDirectories(t *testing.T) {
	// Test that the default behavior still works (creating directories)
	video := testNewFileNode("Test.Movie.2024.mkv")
	subtitle := testNewFileNode("Test.Movie.2024.en.srt")
	nodes := []*treeview.Node[treeview.FileInfo]{video, subtitle}

	cfg := &CommandConfig{
		Config:        config.DefaultConfig(),
		NoDirectories: false, // Default: create directories
	}
	out := MoviePreprocess(nodes, cfg)

	// Should create one virtual directory containing both files
	if len(out) != 1 {
		t.Fatalf("MoviePreprocess with directories = %d nodes, want 1", len(out))
	}

	// Check that it's a virtual directory
	dirMeta := core.GetMeta(out[0])
	if dirMeta == nil {
		t.Fatal("MoviePreprocess with directories didn't create directory metadata")
	}
	if dirMeta.Type != core.MediaMovie {
		t.Errorf("MoviePreprocess with directories Type = %v, want %v", dirMeta.Type, core.MediaMovie)
	}
	if !dirMeta.IsVirtual {
		t.Error("MoviePreprocess with directories IsVirtual = false, want true")
	}
	if !dirMeta.NeedsDirectory {
		t.Error("MoviePreprocess with directories NeedsDirectory = false, want true")
	}

	// Check that both files are children
	if len(out[0].Children()) != 2 {
		t.Errorf("MoviePreprocess with directories children = %d, want 2", len(out[0].Children()))
	}
}
