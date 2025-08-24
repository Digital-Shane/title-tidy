package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

func TestLinkRegular(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	
	// Save and change to temp directory for relative path calculations
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)
	
	// Create a test file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tests := []struct {
		name        string
		linkMode    core.LinkMode
		linkTarget  string
		newName     string
		wantErr     bool
		checkLink   func(t *testing.T, linkPath string)
	}{
		{
			name:       "HardLinkSameDir",
			linkMode:   core.LinkModeHard,
			linkTarget: "",
			newName:    "hardlink.txt",
			wantErr:    false,
			checkLink: func(t *testing.T, linkPath string) {
				// Check that it's a hard link by comparing inodes
				origStat, _ := os.Stat(testFile)
				linkStat, _ := os.Stat(linkPath)
				if !os.SameFile(origStat, linkStat) {
					t.Error("Expected hard link, but files have different inodes")
				}
			},
		},
		{
			name:       "SoftLinkSameDir",
			linkMode:   core.LinkModeSoft,
			linkTarget: "",
			newName:    "softlink.txt",
			wantErr:    false,
			checkLink: func(t *testing.T, linkPath string) {
				// Check that it's a symbolic link
				linkInfo, err := os.Lstat(linkPath)
				if err != nil {
					t.Fatalf("Failed to stat link: %v", err)
				}
				if linkInfo.Mode()&os.ModeSymlink == 0 {
					t.Error("Expected symbolic link")
				}
			},
		},
		{
			name:       "AutoModeSameFS",
			linkMode:   core.LinkModeAuto,
			linkTarget: "",
			newName:    "autolink.txt",
			wantErr:    false,
			checkLink: func(t *testing.T, linkPath string) {
				// Auto mode should create hard link on same filesystem
				origStat, _ := os.Stat(testFile)
				linkStat, _ := os.Stat(linkPath)
				if !os.SameFile(origStat, linkStat) {
					t.Error("Expected hard link in auto mode on same filesystem")
				}
			},
		},
		{
			name:       "LinkToTargetDir",
			linkMode:   core.LinkModeAuto,
			linkTarget: "target",
			newName:    "targetlink.txt",
			wantErr:    false,
			checkLink: func(t *testing.T, linkPath string) {
				// The link should be in the target directory
				expectedPath := filepath.Join("target", "targetlink.txt")
				if linkPath != expectedPath {
					t.Errorf("Link path = %s, expected %s", linkPath, expectedPath)
				}
				// Verify it's accessible
				content, err := os.ReadFile(linkPath)
				if err != nil {
					t.Fatalf("Failed to read through link: %v", err)
				}
				if string(content) != "test content" {
					t.Error("Link doesn't point to correct file")
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create node with file info
			fileInfo, _ := os.Stat(testFile)
			node := treeview.NewNode("test.txt", "test.txt", treeview.FileInfo{
				FileInfo: fileInfo,
				Path:     testFile,
				Extra:    make(map[string]any),
			})
			
			// Setup metadata
			mm := core.EnsureMeta(node)
			mm.NewName = tt.newName
			mm.LinkMode = tt.linkMode
			mm.LinkTarget = tt.linkTarget
			
			// Perform link operation
			operated, err := LinkRegular(node, mm)
			
			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkRegular() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && !operated {
				t.Error("LinkRegular() returned false but expected operation")
				return
			}
			
			// Check the link if operation succeeded
			if !tt.wantErr && tt.checkLink != nil {
				var linkPath string
				if tt.linkTarget != "" {
					linkPath = filepath.Join(tt.linkTarget, tt.newName)
				} else {
					linkPath = tt.newName
				}
				tt.checkLink(t, linkPath)
				
				// Cleanup the link for next test
				os.Remove(linkPath)
			}
		})
	}
}

func TestCalculateTargetPath(t *testing.T) {
	// Save and restore working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	
	// Create temp directory and change to it
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	
	tests := []struct {
		name       string
		sourcePath string
		targetRoot string
		newName    string
		wantPath   string
	}{
		{
			name:       "SimpleFile",
			sourcePath: "file.txt",
			targetRoot: "/target",
			newName:    "renamed.txt",
			wantPath:   "/target/renamed.txt",
		},
		{
			name:       "FileInSubdir",
			sourcePath: "subdir/file.txt",
			targetRoot: "/target",
			newName:    "renamed.txt",
			wantPath:   "/target/subdir/renamed.txt",
		},
		{
			name:       "DeepNesting",
			sourcePath: "a/b/c/file.txt",
			targetRoot: "/target",
			newName:    "renamed.txt",
			wantPath:   "/target/a/b/c/renamed.txt",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := calculateTargetPath(tt.sourcePath, tt.targetRoot, tt.newName)
			if err != nil {
				t.Fatalf("calculateTargetPath() error = %v", err)
			}
			
			if gotPath != tt.wantPath {
				t.Errorf("calculateTargetPath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestCreateVirtualDirWithLinks(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(tmpDir)
	
	// Create test files
	file1 := "movie1.mkv"
	file2 := "movie2.mkv"
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)
	
	// Create virtual directory node with children
	virtualNode := treeview.NewNode(".", ".", treeview.FileInfo{
		Path:  ".",
		Extra: make(map[string]any),
	})
	
	// Add metadata for virtual directory
	dirMeta := core.EnsureMeta(virtualNode)
	dirMeta.NewName = "Movie Collection"
	dirMeta.IsVirtual = true
	dirMeta.NeedsDirectory = true
	dirMeta.LinkMode = core.LinkModeAuto
	
	// Create child nodes
	fileInfo1, _ := os.Stat(file1)
	child1 := treeview.NewNode(file1, file1, treeview.FileInfo{
		FileInfo: fileInfo1,
		Path:     file1,
		Extra:    make(map[string]any),
	})
	child1Meta := core.EnsureMeta(child1)
	child1Meta.NewName = "Movie 1.mkv"
	child1Meta.LinkMode = core.LinkModeAuto
	
	fileInfo2, _ := os.Stat(file2)
	child2 := treeview.NewNode(file2, file2, treeview.FileInfo{
		FileInfo: fileInfo2,
		Path:     file2,
		Extra:    make(map[string]any),
	})
	child2Meta := core.EnsureMeta(child2)
	child2Meta.NewName = "Movie 2.mkv"
	child2Meta.LinkMode = core.LinkModeAuto
	
	virtualNode.SetChildren([]*treeview.Node[treeview.FileInfo]{child1, child2})
	
	// Execute the function
	successes, errs := CreateVirtualDirWithLinks(virtualNode, dirMeta)
	
	// Check results
	if len(errs) > 0 {
		t.Errorf("CreateVirtualDirWithLinks() returned errors: %v", errs)
	}
	
	if successes != 3 { // 1 directory + 2 links
		t.Errorf("CreateVirtualDirWithLinks() successes = %d, want 3", successes)
	}
	
	// Verify directory was created
	if _, err := os.Stat("Movie Collection"); err != nil {
		t.Errorf("Virtual directory not created: %v", err)
	}
	
	// Verify links were created
	link1Path := filepath.Join("Movie Collection", "Movie 1.mkv")
	link2Path := filepath.Join("Movie Collection", "Movie 2.mkv")
	
	content1, err := os.ReadFile(link1Path)
	if err != nil || string(content1) != "content1" {
		t.Errorf("Link 1 not working correctly")
	}
	
	content2, err := os.ReadFile(link2Path)
	if err != nil || string(content2) != "content2" {
		t.Errorf("Link 2 not working correctly")
	}
}

func TestLinkModeWithTargetDirectory(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(filepath.Join(sourceDir, "show", "season"), 0755)
	os.Chdir(sourceDir)
	
	// Create a test file in nested structure
	testFile := filepath.Join("show", "season", "episode.mkv")
	os.WriteFile(testFile, []byte("episode content"), 0644)
	
	// Create node
	fileInfo, _ := os.Stat(testFile)
	node := treeview.NewNode("episode.mkv", "episode.mkv", treeview.FileInfo{
		FileInfo: fileInfo,
		Path:     testFile,
		Extra:    make(map[string]any),
	})
	
	// Setup metadata with target directory
	mm := core.EnsureMeta(node)
	mm.NewName = "S01E01.mkv"
	mm.LinkMode = core.LinkModeAuto
	mm.LinkTarget = targetDir
	
	// Perform link operation
	operated, err := LinkRegular(node, mm)
	if err != nil {
		t.Fatalf("LinkRegular() error = %v", err)
	}
	
	if !operated {
		t.Error("LinkRegular() returned false but expected operation")
	}
	
	// Check that directory structure was replicated
	expectedPath := filepath.Join(targetDir, "show", "season", "S01E01.mkv")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Expected file at %s not found: %v", expectedPath, err)
	}
	
	// Verify content is accessible
	content, err := os.ReadFile(expectedPath)
	if err != nil || string(content) != "episode content" {
		t.Error("Link doesn't provide access to correct content")
	}
}

func TestLinkFailureHandling(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	// Create an existing file that will conflict
	conflictFile := filepath.Join(tmpDir, "conflict.txt")
	os.WriteFile(conflictFile, []byte("existing"), 0644)
	
	// Create node
	fileInfo, _ := os.Stat(testFile)
	node := treeview.NewNode("test.txt", "test.txt", treeview.FileInfo{
		FileInfo: fileInfo,
		Path:     testFile,
		Extra:    make(map[string]any),
	})
	
	// Try to link with conflicting name
	mm := core.EnsureMeta(node)
	mm.NewName = "conflict.txt"
	mm.LinkMode = core.LinkModeHard
	
	operated, err := LinkRegular(node, mm)
	
	// Should fail due to existing file
	if err == nil {
		t.Error("Expected error for conflicting file, got nil")
	}
	
	if operated {
		t.Error("Operation should have failed")
	}
	
	// Check that metadata reflects the error
	if mm.RenameStatus != core.RenameStatusError {
		t.Error("Metadata should indicate error status")
	}
	
	if !cmp.Equal(mm.RenameError, "destination already exists") {
		t.Errorf("Unexpected error message: %s", mm.RenameError)
	}
}