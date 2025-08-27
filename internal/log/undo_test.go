package log

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUndoRenameOperation(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	oldPath := filepath.Join(tempDir, "old.txt")
	newPath := filepath.Join(tempDir, "new.txt")

	// Create original file
	err := os.WriteFile(oldPath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Simulate a rename
	err = os.Rename(oldPath, newPath)
	if err != nil {
		t.Fatalf("Failed to rename test file: %v", err)
	}

	// Create operation log
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpRename,
		SourcePath: oldPath,
		DestPath:   newPath,
		Success:    true,
	}

	// Test undo
	result := UndoOperation(op)
	if !result.Success {
		t.Fatalf("UndoOperation failed: %v", result.Error)
	}

	// Verify file was renamed back
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("Original file should exist after undo")
	}

	if _, err := os.Stat(newPath); err == nil {
		t.Error("Renamed file should not exist after undo")
	}
}

func TestUndoDeleteOperation(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "delete.txt")

	// Create operation log for delete (should fail to undo)
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpDelete,
		SourcePath: filePath,
		Success:    true,
	}

	// Test undo (should fail as delete undo is not implemented)
	result := UndoOperation(op)
	if result.Success {
		t.Error("Delete undo should not be successful (not implemented)")
	}

	if result.Error == nil {
		t.Error("Delete undo should return error")
	}
}

func TestUndoCreateDirOperation(t *testing.T) {
	tempDir := t.TempDir()
	dirPath := filepath.Join(tempDir, "testdir")

	// Create directory
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create operation log
	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      OpCreateDir,
		DestPath:  dirPath,
		Success:   true,
	}

	// Test undo
	result := UndoOperation(op)
	if !result.Success {
		t.Fatalf("UndoOperation failed: %v", result.Error)
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); err == nil {
		t.Error("Directory should not exist after undo")
	}
}

func TestUndoCreateDirWithContent(t *testing.T) {
	tempDir := t.TempDir()
	dirPath := filepath.Join(tempDir, "testdir")

	// Create directory with content
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Add file to directory
	filePath := filepath.Join(dirPath, "file.txt")
	err = os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file in directory: %v", err)
	}

	// Create operation log
	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      OpCreateDir,
		DestPath:  dirPath,
		Success:   true,
	}

	// Test undo (should fail because directory is not empty)
	result := UndoOperation(op)
	if result.Success {
		t.Error("Undo should fail for non-empty directory")
	}

	if result.Error == nil {
		t.Error("Undo should return error for non-empty directory")
	}
}

func TestUndoSession(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	file1Old := filepath.Join(tempDir, "file1_old.txt")
	file1New := filepath.Join(tempDir, "file1_new.txt")
	file2Path := filepath.Join(tempDir, "file2.txt")
	testDir := filepath.Join(tempDir, "testdir")

	// Set up initial state
	err := os.WriteFile(file1Old, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(file2Path, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Simulate operations
	err = os.Rename(file1Old, file1New)
	if err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	err = os.Remove(file2Path)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	err = os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create session with operations (in execution order)
	session := &LogSession{
		Metadata: SessionMetadata{
			CommandArgs:   []string{"test"},
			WorkingDir:    tempDir,
			Timestamp:     time.Now(),
			SessionID:     "test_session",
			TotalOps:      3,
			SuccessfulOps: 3,
			FailedOps:     0,
		},
		Operations: []OperationLog{
			{
				ID:         "test_session_0",
				Type:       OpRename,
				SourcePath: file1Old,
				DestPath:   file1New,
				Success:    true,
			},
			{
				ID:         "test_session_1",
				Type:       OpDelete,
				SourcePath: file2Path,
				Success:    true,
			},
			{
				ID:       "test_session_2",
				Type:     OpCreateDir,
				DestPath: testDir,
				Success:  true,
			},
		},
	}

	// Test undo session
	successful, failed, errors := UndoSession(session)

	// Should have 2 successful (rename + createdir) and 1 failed (delete)
	if successful != 2 {
		t.Errorf("Expected 2 successful undos, got %d", successful)
	}

	if failed != 1 {
		t.Errorf("Expected 1 failed undo, got %d", failed)
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}

	// Verify file was renamed back
	if _, err := os.Stat(file1Old); os.IsNotExist(err) {
		t.Error("Original file should exist after undo")
	}

	// Verify directory was removed
	if _, err := os.Stat(testDir); err == nil {
		t.Error("Directory should not exist after undo")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "3 hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "3 days ago",
			time:     now.Add(-72 * time.Hour),
			expected: "3 days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatRelativeTime(%v) = %s, want %s", tt.time, result, tt.expected)
			}
		})
	}
}

func TestGetCommandIcon(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
	}{
		{[]string{"shows"}, "📺"},
		{[]string{"seasons"}, "📁"},
		{[]string{"episodes"}, "🎬"},
		{[]string{"movies"}, "🎬"},
		{[]string{"unknown"}, "📝"},
		{[]string{}, "❓"},
	}

	for i, tt := range tests {
		testName := fmt.Sprintf("case_%d", i)
		if len(tt.args) > 0 {
			testName = "args_" + tt.args[0]
		} else {
			testName = "empty_args"
		}
		t.Run(testName, func(t *testing.T) {
			result := getCommandIcon(tt.args)
			if result != tt.expected {
				t.Errorf("getCommandIcon(%v) = %s, want %s", tt.args, result, tt.expected)
			}
		})
	}
}

func TestUndoLinkOperation(t *testing.T) {
	tempDir := t.TempDir()

	// Create source and link files
	sourcePath := filepath.Join(tempDir, "source.txt")
	linkPath := filepath.Join(tempDir, "link.txt")

	// Create source file
	err := os.WriteFile(sourcePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create symbolic link
	err = os.Symlink(sourcePath, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create operation log
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpLink,
		SourcePath: sourcePath,
		DestPath:   linkPath,
		Success:    true,
	}

	// Test undo
	result := UndoOperation(op)
	if !result.Success {
		t.Fatalf("UndoOperation failed: %v", result.Error)
	}

	// Verify link was removed
	if _, err := os.Lstat(linkPath); err == nil {
		t.Error("Link should not exist after undo")
	}

	// Verify source still exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("Source file should still exist after undo")
	}
}

func TestUndoLinkOperationMissingDest(t *testing.T) {
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpLink,
		SourcePath: "/tmp/source.txt",
		DestPath:   "",
		Success:    true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when destination path is missing")
	}
	if result.Error == nil || result.Error.Error() != "cannot undo link: destination path missing" {
		t.Errorf("UndoOperation error = %v, want 'cannot undo link: destination path missing'", result.Error)
	}
}

func TestUndoLinkOperationAlreadyRemoved(t *testing.T) {
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpLink,
		SourcePath: "/tmp/source.txt",
		DestPath:   "/tmp/nonexistent_link.txt",
		Success:    true,
	}

	// Test undo on non-existent link (should succeed)
	result := UndoOperation(op)
	if !result.Success {
		t.Errorf("UndoOperation should succeed when link is already removed: %v", result.Error)
	}
}

func TestUndoLinkOperationWrongTarget(t *testing.T) {
	tempDir := t.TempDir()

	// Create files
	sourcePath := filepath.Join(tempDir, "source.txt")
	wrongSource := filepath.Join(tempDir, "wrong.txt")
	linkPath := filepath.Join(tempDir, "link.txt")

	// Create source files
	err := os.WriteFile(sourcePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	err = os.WriteFile(wrongSource, []byte("wrong content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create wrong source file: %v", err)
	}

	// Create symbolic link to wrong source
	err = os.Symlink(wrongSource, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create operation log claiming different source
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpLink,
		SourcePath: sourcePath,
		DestPath:   linkPath,
		Success:    true,
	}

	// Test undo
	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when link points to wrong target")
	}
	if result.Error == nil {
		t.Error("UndoOperation should return error for link target mismatch")
	}
}

func TestUndoRenameOperationMissingDest(t *testing.T) {
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpRename,
		SourcePath: "/tmp/old.txt",
		DestPath:   "",
		Success:    true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when destination path is missing")
	}
	if result.Error == nil || result.Error.Error() != "cannot undo rename: destination path missing" {
		t.Errorf("UndoOperation error = %v, want 'cannot undo rename: destination path missing'", result.Error)
	}
}

func TestUndoRenameOperationDestNotFound(t *testing.T) {
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpRename,
		SourcePath: "/tmp/old.txt",
		DestPath:   "/tmp/nonexistent.txt",
		Success:    true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when destination file not found")
	}
	if result.Error == nil {
		t.Error("UndoOperation should return error when destination file not found")
	}
}

func TestUndoRenameOperationSourceExists(t *testing.T) {
	tempDir := t.TempDir()

	// Create both old and new paths
	oldPath := filepath.Join(tempDir, "old.txt")
	newPath := filepath.Join(tempDir, "new.txt")

	// Create both files
	err := os.WriteFile(oldPath, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	err = os.WriteFile(newPath, []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Create operation log
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpRename,
		SourcePath: oldPath,
		DestPath:   newPath,
		Success:    true,
	}

	// Test undo
	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when original path already exists")
	}
	if result.Error == nil {
		t.Error("UndoOperation should return error when original path already exists")
	}
}

func TestUndoCreateDirOperationSourcePath(t *testing.T) {
	tempDir := t.TempDir()
	dirPath := filepath.Join(tempDir, "testdir")

	// Create directory
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create operation log with path in SourcePath instead of DestPath
	op := OperationLog{
		ID:         "test_op",
		Timestamp:  time.Now(),
		Type:       OpCreateDir,
		SourcePath: dirPath,
		DestPath:   "",
		Success:    true,
	}

	// Test undo
	result := UndoOperation(op)
	if !result.Success {
		t.Fatalf("UndoOperation failed: %v", result.Error)
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); err == nil {
		t.Error("Directory should not exist after undo")
	}
}

func TestUndoCreateDirOperationMissingPath(t *testing.T) {
	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      OpCreateDir,
		Success:   true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when path is missing")
	}
	if result.Error == nil || result.Error.Error() != "cannot undo directory creation: path missing" {
		t.Errorf("UndoOperation error = %v, want 'cannot undo directory creation: path missing'", result.Error)
	}
}

func TestUndoCreateDirOperationAlreadyRemoved(t *testing.T) {
	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      OpCreateDir,
		DestPath:  "/tmp/nonexistent_dir",
		Success:   true,
	}

	// Test undo on non-existent directory (should succeed)
	result := UndoOperation(op)
	if !result.Success {
		t.Errorf("UndoOperation should succeed when directory is already removed: %v", result.Error)
	}
}

func TestUndoCreateDirOperationNotADir(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "notadir.txt")

	// Create a file instead of directory
	err := os.WriteFile(filePath, []byte("not a directory"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      OpCreateDir,
		DestPath:  filePath,
		Success:   true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail when path is not a directory")
	}
	if result.Error == nil {
		t.Error("UndoOperation should return error when path is not a directory")
	}
}

func TestUndoUnknownOperation(t *testing.T) {
	op := OperationLog{
		ID:        "test_op",
		Timestamp: time.Now(),
		Type:      "UnknownOpType",
		Success:   true,
	}

	result := UndoOperation(op)
	if result.Success {
		t.Error("UndoOperation should fail for unknown operation type")
	}
	if result.Error == nil {
		t.Error("UndoOperation should return error for unknown operation type")
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "s"},
		{1, ""},
		{2, "s"},
		{100, "s"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("n_%d", tt.n), func(t *testing.T) {
			result := plural(tt.n)
			if result != tt.expected {
				t.Errorf("plural(%d) = %q, want %q", tt.n, result, tt.expected)
			}
		})
	}
}

func TestFormatRelativeTimeEdgeCases(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "exactly 7 days ago",
			time:     now.Add(-7 * 24 * time.Hour),
			expected: now.Add(-7 * 24 * time.Hour).Format("Jan 2, 2006"),
		},
		{
			name:     "8 days ago",
			time:     now.Add(-8 * 24 * time.Hour),
			expected: now.Add(-8 * 24 * time.Hour).Format("Jan 2, 2006"),
		},
		{
			name:     "exactly 1 minute",
			time:     now.Add(-60 * time.Second),
			expected: "1 minute ago",
		},
		{
			name:     "exactly 1 hour",
			time:     now.Add(-60 * time.Minute),
			expected: "1 hour ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatRelativeTime(%v) = %s, want %s", tt.time, result, tt.expected)
			}
		})
	}
}
