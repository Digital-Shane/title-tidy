package progress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/google/go-cmp/cmp"
)

type fakeFileInfo struct {
	name string
	dir  bool
}

func (fi fakeFileInfo) Name() string { return fi.name }
func (fi fakeFileInfo) Size() int64  { return 0 }
func (fi fakeFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (fi fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi fakeFileInfo) IsDir() bool        { return fi.dir }
func (fi fakeFileInfo) Sys() any           { return nil }

func newFakeNode(path string, dir bool) *treeview.Node[treeview.FileInfo] {
	name := filepath.Base(path)
	data := treeview.FileInfo{FileInfo: fakeFileInfo{name: name, dir: dir}, Path: path}
	return treeview.NewNode(path, name, data)
}

func finalIndexProgressModel(t *testing.T, tm *teatest.TestModel) *IndexProgressModel {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	model, ok := final.(*IndexProgressModel)
	if !ok {
		t.Fatalf("Final model type = %T, want *IndexProgressModel", final)
	}
	return model
}

func finalOutput(t *testing.T, tm *teatest.TestModel) []byte {
	t.Helper()
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second)))
	if err != nil {
		t.Fatalf("FinalOutput read error = %v", err)
	}
	return out
}

func withIndexProgressBuilder(t *testing.T, builder treeBuilderFunc) {
	t.Helper()
	original := indexProgressTreeBuilder
	indexProgressTreeBuilder = builder
	t.Cleanup(func() {
		indexProgressTreeBuilder = original
	})
}

func newIndexProgressTestModel(t *testing.T, model *IndexProgressModel, opts ...teatest.TestOption) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, model, opts...)
	t.Cleanup(func() {
		_ = tm.Quit()
	})
	return tm
}

func TestIndexProgressTUICompletesAndReportsProgress(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "shows"), 0o755); err != nil {
		t.Fatalf("os.Mkdir(shows) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "movie.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(movie) error = %v", err)
	}

	fakeTree := &treeview.Tree[treeview.FileInfo]{}
	withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
		cfg := treeview.NewMasterConfig(opts)
		cfg.ReportProgress(1, newFakeNode(path, true))
		cfg.ReportProgress(2, newFakeNode(filepath.Join(path, "shows"), true))
		cfg.ReportProgress(3, newFakeNode(filepath.Join(path, "movie.mkv"), false))
		return fakeTree, nil
	})

	model := NewIndexProgressModel(tempDir, IndexConfig{MaxDepth: 2, IncludeDirs: true}, theme.Default())
	tm := newIndexProgressTestModel(t, model, teatest.WithInitialTermSize(100, 20))

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	finalModel := finalIndexProgressModel(t, tm)
	output := finalOutput(t, tm)

	if diff := cmp.Diff(2, finalModel.totalRoots); diff != "" {
		t.Errorf("totalRoots diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(2, finalModel.processedRoots); diff != "" {
		t.Errorf("processedRoots diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, finalModel.filesIndexed); diff != "" {
		t.Errorf("filesIndexed diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1.0, finalModel.progress.Percent()); diff != "" {
		t.Errorf("progress.Percent diff (-want +got):\n%s", diff)
	}
	if finalModel.Tree() != fakeTree {
		t.Errorf("Tree() pointer = %p, want %p", finalModel.Tree(), fakeTree)
	}
	if finalModel.Err() != nil {
		t.Errorf("Err() = %v, want nil", finalModel.Err())
	}

	for _, want := range []string{"Indexing Media Library", "Roots processed: 2/2", "Files indexed: 1"} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("final output missing %q; output = %q", want, output)
		}
	}
}

func TestIndexProgressTUIQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyType
	}{
		{name: "ctrl_c", key: tea.KeyCtrlC},
		{name: "esc", key: tea.KeyEsc},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("data"), 0o644); err != nil {
				t.Fatalf("os.WriteFile(file.txt) error = %v", err)
			}

			ready := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseClose := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseClose)

			withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
				cfg := treeview.NewMasterConfig(opts)
				cfg.ReportProgress(1, newFakeNode(path, true))
				close(ready)
				<-release
				cfg.ReportProgress(2, newFakeNode(filepath.Join(path, "file.txt"), false))
				return &treeview.Tree[treeview.FileInfo]{}, nil
			})

			model := NewIndexProgressModel(tempDir, IndexConfig{MaxDepth: 1}, theme.Default())
			tm := newIndexProgressTestModel(t, model)
			<-ready
			tm.Send(tea.KeyMsg{Type: tc.key})

			tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
			finalModel := finalIndexProgressModel(t, tm)

			if finalModel.indexingDone {
				t.Error("indexingDone = true, want false when exiting via keybinding")
			}

			releaseClose()
		})
	}
}

func TestIndexProgressTUIWindowResize(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(file.txt) error = %v", err)
	}

	ready := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseClose := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseClose)

	withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
		cfg := treeview.NewMasterConfig(opts)
		cfg.ReportProgress(1, newFakeNode(path, true))
		close(ready)
		<-release
		cfg.ReportProgress(2, newFakeNode(filepath.Join(path, "file.txt"), false))
		return &treeview.Tree[treeview.FileInfo]{}, nil
	})

	model := NewIndexProgressModel(tempDir, IndexConfig{MaxDepth: 1}, theme.Default())
	tm := newIndexProgressTestModel(t, model)
	<-ready
	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 40})

	releaseClose()
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	finalModel := finalIndexProgressModel(t, tm)

	if diff := cmp.Diff(100, finalModel.width); diff != "" {
		t.Errorf("width diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(96, finalModel.progress.Width); diff != "" {
		t.Errorf("progress.Width diff (-want +got):\n%s", diff)
	}
}

func TestIndexProgressTUIErrorState(t *testing.T) {
	tempDir := t.TempDir()
	errBoom := errors.New("boom")

	withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
		cfg := treeview.NewMasterConfig(opts)
		cfg.ReportProgress(1, newFakeNode(path, true))
		return nil, errBoom
	})

	model := NewIndexProgressModel(tempDir, IndexConfig{MaxDepth: 1}, theme.Default())
	tm := newIndexProgressTestModel(t, model)

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	finalModel := finalIndexProgressModel(t, tm)

	if !errors.Is(finalModel.Err(), errBoom) {
		t.Errorf("Err() = %v, want error boom", finalModel.Err())
	}

	out := finalOutput(t, tm)
	if !bytes.Contains(out, []byte("Error: boom")) {
		t.Errorf("Final output missing error message; output = %q", out)
	}
}
