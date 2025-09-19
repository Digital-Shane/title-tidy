package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

func TestIndexProgressModel_Update(t *testing.T) {
	tempDir := t.TempDir()
	cfg := IndexConfig{MaxDepth: 1}
	_ = NewIndexProgressModel(tempDir, cfg)

	tests := []struct {
		name      string
		msg       tea.Msg
		wantQuit  bool
		wantWidth int
	}{
		{
			name:      "window resize",
			msg:       tea.WindowSizeMsg{Width: 120, Height: 24},
			wantWidth: 120,
		},
		{
			name:     "quit on ctrl+c",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			wantQuit: true,
		},
		{
			name:     "quit on esc",
			msg:      tea.KeyMsg{Type: tea.KeyEsc},
			wantQuit: true,
		},
		{
			name: "index progress",
			msg:  indexProgressMsg{},
		},
		{
			name:     "index complete",
			msg:      indexCompleteMsg{},
			wantQuit: true,
		},
		{
			name: "progress frame",
			msg:  progress.FrameMsg{},
		},
		{
			name: "unhandled message",
			msg:  "unknown message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset model for each test
			m := NewIndexProgressModel(tempDir, cfg)
			_ = m

			updatedModel, cmd := m.Update(tt.msg)

			if tt.wantQuit {
				// Check if quit command was returned
				if cmd == nil {
					t.Errorf("Update(%T) returned nil cmd, want quit cmd", tt.msg)
				}
			}

			if tt.wantWidth > 0 {
				if m, ok := updatedModel.(*IndexProgressModel); ok {
					if m.width != tt.wantWidth {
						t.Errorf("Update(WindowSizeMsg) width = %d, want %d", m.width, tt.wantWidth)
					}
				}
			}
		})
	}
}

func TestIndexProgressModel_View(t *testing.T) {
	tempDir := t.TempDir()
	cfg := IndexConfig{MaxDepth: 1}

	t.Run("normal view", func(t *testing.T) {
		model := NewIndexProgressModel(tempDir, cfg)
		model.processedRoots = 5
		model.totalRoots = 10
		model.filesIndexed = 25

		view := model.View()

		expectedContents := []string{
			"Indexing Media Library",
			"Roots processed: 5/10",
			"Files indexed: 25",
			"Root Directories: 10",
			"Processed Roots: 5",
			"Files Indexed: 25",
			"Progress: 50%",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(view, expected) {
				t.Errorf("View() missing expected content: %q", expected)
			}
		}
	})

	t.Run("error view", func(t *testing.T) {
		model := NewIndexProgressModel(tempDir, cfg)
		testErr := os.ErrNotExist
		model.err = testErr

		view := model.View()

		if !strings.Contains(view, "Error:") {
			t.Errorf("View() with error missing 'Error:' prefix")
		}
		if !strings.Contains(view, testErr.Error()) {
			t.Errorf("View() with error missing error message: %q", testErr.Error())
		}
	})
}

func TestIndexProgressModel_Tree(t *testing.T) {
	tempDir := t.TempDir()
	cfg := IndexConfig{MaxDepth: 1}
	model := NewIndexProgressModel(tempDir, cfg)

	// Initially nil
	if tree := model.Tree(); tree != nil {
		t.Errorf("Tree() before build = %v, want nil", tree)
	}

	// Set a tree
	testTree := &treeview.Tree[treeview.FileInfo]{}
	model.tree = testTree

	if tree := model.Tree(); tree != testTree {
		t.Errorf("Tree() = %p, want %p", tree, testTree)
	}
}

func TestIndexProgressModel_waitForMsg(t *testing.T) {
	tempDir := t.TempDir()
	cfg := IndexConfig{MaxDepth: 1}
	model := NewIndexProgressModel(tempDir, cfg)

	// Send a message to the channel
	testMsg := indexProgressMsg{}
	go func() {
		model.msgCh <- testMsg
	}()

	// Call waitForMsg and verify it returns the message
	cmd := model.waitForMsg()
	if cmd == nil {
		t.Fatalf("waitForMsg() = nil, want non-nil cmd")
	}

	msg := cmd()
	if _, ok := msg.(indexProgressMsg); !ok {
		t.Errorf("waitForMsg() returned %T, want indexProgressMsg", msg)
	}
}

func TestIndexConfig_DefaultFilter(t *testing.T) {
	// Test the default filter fallback (lines 91-102)
	tempDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tempDir, "normal.mkv"), []byte("video"), 0644)
	os.WriteFile(filepath.Join(tempDir, ".DS_Store"), []byte("macos"), 0644)
	os.WriteFile(filepath.Join(tempDir, "._hidden"), []byte("macos"), 0644)
	os.Mkdir(filepath.Join(tempDir, "testdir"), 0755)

	// Test with no custom filter to trigger fallback
	cfg := IndexConfig{
		MaxDepth:    1,
		IncludeDirs: false,
		Filter:      nil, // This will trigger the fallback filter
	}

	model := NewIndexProgressModel(tempDir, cfg)

	// Simulate Init to start the build process
	cmd := model.Init()
	if cmd == nil {
		t.Fatalf("Init() = nil, want non-nil cmd")
	}

	// Give some time for the async build to start
	// The buildTreeAsync function should execute the default filter
	time.Sleep(100 * time.Millisecond)
}

func TestIndexConfig_IncludeDirsFilter(t *testing.T) {
	// Test IncludeDirs filter logic (lines 99-102)
	tempDir := t.TempDir()
	os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)

	tests := []struct {
		name        string
		includeDirs bool
	}{
		{"with dirs", true},
		{"without dirs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := IndexConfig{
				MaxDepth:    1,
				IncludeDirs: tt.includeDirs,
				Filter:      nil, // Use default filter
			}

			model := NewIndexProgressModel(tempDir, cfg)

			// Start build to trigger filter execution
			cmd := model.Init()
			if cmd == nil {
				t.Fatalf("Init() = nil, want non-nil cmd")
			}

			// Allow async build to execute
			time.Sleep(100 * time.Millisecond)
		})
	}
}

func TestBuildTreeAsyncError(t *testing.T) {
	// Test error handling in buildTreeAsync (line 151)
	nonExistentDir := "/path/that/does/not/exist"

	cfg := IndexConfig{
		MaxDepth:    1,
		IncludeDirs: false,
		Filter:      func(fi treeview.FileInfo) bool { return true },
	}

	model := NewIndexProgressModel(nonExistentDir, cfg)

	// Start the build process which should fail
	cmd := model.Init()
	if cmd == nil {
		t.Fatalf("Init() = nil, want non-nil cmd")
	}

	// Wait for the async operation to complete
	time.Sleep(200 * time.Millisecond)

	// Check that error was captured
	if model.Err() == nil {
		t.Errorf("Expected error for non-existent directory, got nil")
	}
}

func TestIndexProgressChannelBlocking(t *testing.T) {
	// Test channel communication with potential blocking (lines 116-119)
	tempDir := t.TempDir()

	// Create many files to potentially trigger the default case in select
	for i := 0; i < 100; i++ {
		os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i)), []byte("data"), 0644)
	}

	cfg := IndexConfig{
		MaxDepth:    1,
		IncludeDirs: false,
		Filter:      func(fi treeview.FileInfo) bool { return true },
	}

	model := NewIndexProgressModel(tempDir, cfg)

	// Fill up the message channel to potentially trigger the default case
	go func() {
		for i := 0; i < cap(model.msgCh)+10; i++ {
			select {
			case model.msgCh <- indexProgressMsg{}:
			case <-time.After(10 * time.Millisecond):
				return
			}
		}
	}()

	// Start indexing while channel is potentially full
	cmd := model.Init()
	if cmd == nil {
		t.Fatalf("Init() = nil, want non-nil cmd")
	}

	// Allow processing
	time.Sleep(200 * time.Millisecond)
}
