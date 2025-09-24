package rename

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

func TestLinkRegular(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	destDir := filepath.Join(tmpDir, "dest")
	destPath := filepath.Join(destDir, "linked.txt")

	// Create source file
	if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test node and metadata
	node := createTestFileNode("source.txt", srcPath)
	mm := core.EnsureMeta(node)
	mm.DestinationPath = destPath

	// Test linking
	linked, err := core.LinkRegular(node, mm)
	if err != nil {
		t.Fatalf("core.LinkRegular(%q, %q) = %v, want nil", srcPath, destPath, err)
	}
	if !linked {
		t.Errorf("core.LinkRegular(%q, %q) = false, want true", srcPath, destPath)
	}

	// Verify hard link was created
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("core.LinkRegular destination file not created: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "test content"
	if got := string(content); got != want {
		t.Errorf("core.LinkRegular destination content = %q, want %q", got, want)
	}

	// Verify same inode (hard link)
	srcStat, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	destStat, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}

	// Check link count increased
	srcSys := srcStat.Sys()
	destSys := destStat.Sys()
	if diff := cmp.Diff(srcSys, destSys); diff != "" {
		t.Errorf("core.LinkRegular inode mismatch (-src +dest):\n%s", diff)
	}

	// Verify metadata status
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("core.LinkRegular metadata status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
}

func TestLinkRegularNoDestinationPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")

	// Create source file
	if err := os.WriteFile(srcPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test node and metadata without destination path
	node := createTestFileNode("source.txt", srcPath)
	mm := core.EnsureMeta(node)
	// Don't set DestinationPath

	// Test linking
	linked, err := core.LinkRegular(node, mm)
	if err == nil {
		t.Errorf("core.LinkRegular(no destination) = nil, want error")
	}
	if linked {
		t.Errorf("core.LinkRegular(no destination) = true, want false")
	}

	// Verify metadata error status
	if mm.RenameStatus != core.RenameStatusError {
		t.Errorf("core.LinkRegular metadata status = %v, want %v", mm.RenameStatus, core.RenameStatusError)
	}
}

func TestLinkRegularDestinationExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Create source and destination files
	if err := os.WriteFile(srcPath, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test node and metadata
	node := createTestFileNode("source.txt", srcPath)
	mm := core.EnsureMeta(node)
	mm.DestinationPath = destPath

	// Test linking - should succeed when destination exists (incremental linking)
	linked, err := core.LinkRegular(node, mm)
	if err != nil {
		t.Errorf("core.LinkRegular(existing destination) = %v, want nil", err)
	}
	if linked {
		t.Errorf("core.LinkRegular(existing destination) = true, want false (no new link created)")
	}

	// Verify metadata success status (existing files treated as success)
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("core.LinkRegular metadata status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
}

func TestLinkVirtualDirWithExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "destination")

	// Create source files
	srcFile1 := filepath.Join(tmpDir, "video.mkv")
	srcFile2 := filepath.Join(tmpDir, "subtitle.srt")
	if err := os.WriteFile(srcFile1, []byte("video content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile2, []byte("subtitle content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create destination directory and one existing file
	dirPath := filepath.Join(linkPath, "Test Movie (2024)")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(dirPath, "Test Movie (2024).mkv")
	if err := os.WriteFile(existingFile, []byte("existing content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create virtual directory node
	virtualDir := createTestDirNode("Test Movie (2024)", "")
	mm := core.EnsureMeta(virtualDir)
	mm.NewName = "Test Movie (2024)"
	mm.IsVirtual = true
	mm.NeedsDirectory = true

	// Create child nodes
	child1 := createTestFileNode("video.mkv", srcFile1)
	child2 := createTestFileNode("subtitle.srt", srcFile2)
	virtualDir.AddChild(child1)
	virtualDir.AddChild(child2)

	// Set up child metadata
	cm1 := core.EnsureMeta(child1)
	cm1.NewName = "Test Movie (2024).mkv" // This one already exists
	cm2 := core.EnsureMeta(child2)
	cm2.NewName = "Test Movie (2024).srt" // This one is new

	// Test virtual directory linking with existing directory and one existing file
	successes, errs := core.LinkVirtualDir(virtualDir, mm, linkPath)

	if len(errs) > 0 {
		t.Fatalf("LinkVirtualDirWithExistingFiles errors = %v, want none", errs)
	}
	if successes != 3 { // 1 directory (already exists) + 1 existing file + 1 new file
		t.Errorf("LinkVirtualDirWithExistingFiles successes = %d, want 3", successes)
	}

	// Verify both children have success status
	if cm1.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("LinkVirtualDirWithExistingFiles existing child status = %v, want %v", cm1.RenameStatus, core.RenameStatusSuccess)
	}
	if cm2.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("LinkVirtualDirWithExistingFiles new child status = %v, want %v", cm2.RenameStatus, core.RenameStatusSuccess)
	}

	// Verify the new file was linked
	linkedFile2 := filepath.Join(dirPath, "Test Movie (2024).srt")
	if _, err := os.Stat(linkedFile2); err != nil {
		t.Errorf("LinkVirtualDirWithExistingFiles new file not linked: %v", err)
	}
}

func TestLinkVirtualDir(t *testing.T) {
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "destination")

	// Create source files
	srcFile1 := filepath.Join(tmpDir, "video.mkv")
	srcFile2 := filepath.Join(tmpDir, "subtitle.srt")
	if err := os.WriteFile(srcFile1, []byte("video content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile2, []byte("subtitle content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create virtual directory node
	virtualDir := createTestDirNode("Test Movie (2024)", "")
	mm := core.EnsureMeta(virtualDir)
	mm.NewName = "Test Movie (2024)"
	mm.IsVirtual = true
	mm.NeedsDirectory = true

	// Create child nodes
	child1 := createTestFileNode("video.mkv", srcFile1)
	child2 := createTestFileNode("subtitle.srt", srcFile2)
	virtualDir.AddChild(child1)
	virtualDir.AddChild(child2)

	// Set up child metadata
	cm1 := core.EnsureMeta(child1)
	cm1.NewName = "Test Movie (2024).mkv"
	cm2 := core.EnsureMeta(child2)
	cm2.NewName = "Test Movie (2024).srt"

	// Test virtual directory creation with linking
	successes, errs := core.LinkVirtualDir(virtualDir, mm, linkPath)

	if len(errs) > 0 {
		t.Fatalf("core.LinkVirtualDir errors = %v, want none", errs)
	}
	if successes != 3 { // 1 directory + 2 files
		t.Errorf("core.LinkVirtualDir successes = %d, want 3", successes)
	}

	// Verify directory was created
	dirPath := filepath.Join(linkPath, "Test Movie (2024)")
	if _, err := os.Stat(dirPath); err != nil {
		t.Errorf("core.LinkVirtualDir directory not created: %v", err)
	}

	// Verify files were linked
	linkedFile1 := filepath.Join(dirPath, "Test Movie (2024).mkv")
	linkedFile2 := filepath.Join(dirPath, "Test Movie (2024).srt")

	if _, err := os.Stat(linkedFile1); err != nil {
		t.Errorf("core.LinkVirtualDir child 1 not linked: %v", err)
	}
	if _, err := os.Stat(linkedFile2); err != nil {
		t.Errorf("core.LinkVirtualDir child 2 not linked: %v", err)
	}

	// Verify content
	content1, err := os.ReadFile(linkedFile1)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content1); got != "video content" {
		t.Errorf("core.LinkVirtualDir child 1 content = %q, want %q", got, "video content")
	}

	// Verify metadata success status
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("core.LinkVirtualDir directory status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
	if cm1.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("core.LinkVirtualDir child 1 status = %v, want %v", cm1.RenameStatus, core.RenameStatusSuccess)
	}
	if cm2.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("core.LinkVirtualDir child 2 status = %v, want %v", cm2.RenameStatus, core.RenameStatusSuccess)
	}
}

// Helper functions for creating test nodes
func createTestFileNode(name, path string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{
		FileInfo: &testFileInfo{name: name, isDir: false},
		Path:     path,
	})
}

func createTestDirNode(name, path string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{
		FileInfo: &testFileInfo{name: name, isDir: true},
		Path:     path,
	})
}

// Test implementation of os.FileInfo
type testFileInfo struct {
	name  string
	isDir bool
}

func (fi *testFileInfo) Name() string       { return fi.name }
func (fi *testFileInfo) Size() int64        { return 0 }
func (fi *testFileInfo) Mode() os.FileMode  { return 0644 }
func (fi *testFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *testFileInfo) IsDir() bool        { return fi.isDir }
func (fi *testFileInfo) Sys() any           { return nil }
