package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/google/go-cmp/cmp"
)

func TestIndexProgressModel_CustomFilterDelegation(t *testing.T) {
	tempDir := t.TempDir()
	var called []string
	results := make(map[string]bool)

	withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
		cfg := treeview.NewMasterConfig(opts)
		files := []treeview.FileInfo{
			{FileInfo: fakeFileInfo{name: "keep.me"}, Path: filepath.Join(path, "keep.me")},
			{FileInfo: fakeFileInfo{name: "drop.me"}, Path: filepath.Join(path, "drop.me")},
		}
		for _, fi := range files {
			results[fi.Name()] = !cfg.ShouldFilter(fi)
		}
		return &treeview.Tree[treeview.FileInfo]{}, nil
	})

	model := NewIndexProgressModel(tempDir, IndexConfig{
		Filter: func(fi treeview.FileInfo) bool {
			called = append(called, fi.Name())
			return fi.Name() == "keep.me"
		},
	})
	model.buildTreeAsync()

	if diff := cmp.Diff([]string{"keep.me", "drop.me"}, called); diff != "" {
		t.Errorf("custom filter invocation order diff (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(map[string]bool{"keep.me": true, "drop.me": false}, results); diff != "" {
		t.Errorf("custom filter results diff (-want +got):\n%s", diff)
	}
}

func TestIndexProgressModel_DefaultFilterFallback(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		includeDirs bool
		want        map[string]bool
	}{
		{
			name:        "exclude directories and metadata by default",
			includeDirs: false,
			want: map[string]bool{
				".DS_Store": false,
				"._hidden":  false,
				"regular":   true,
				"subdir":    false,
			},
		},
		{
			name:        "include directories when configured",
			includeDirs: true,
			want: map[string]bool{
				".DS_Store": false,
				"._hidden":  false,
				"regular":   true,
				"subdir":    true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := make(map[string]bool)
			withIndexProgressBuilder(t, func(_ context.Context, path string, _ bool, opts ...treeview.Option[treeview.FileInfo]) (*treeview.Tree[treeview.FileInfo], error) {
				cfg := treeview.NewMasterConfig(opts)
				files := []treeview.FileInfo{
					{FileInfo: fakeFileInfo{name: ".DS_Store"}, Path: filepath.Join(path, ".DS_Store")},
					{FileInfo: fakeFileInfo{name: "._hidden"}, Path: filepath.Join(path, "._hidden")},
					{FileInfo: fakeFileInfo{name: "regular"}, Path: filepath.Join(path, "regular")},
					{FileInfo: fakeFileInfo{name: "subdir", dir: true}, Path: filepath.Join(path, "subdir")},
				}
				for _, fi := range files {
					got[fi.Name()] = !cfg.ShouldFilter(fi)
				}
				return &treeview.Tree[treeview.FileInfo]{}, nil
			})

			model := NewIndexProgressModel(tempDir, IndexConfig{IncludeDirs: tc.includeDirs})
			model.buildTreeAsync()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("default filter results diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIndexProgressModel_UpdateProgressFrameMsg(t *testing.T) {
	model := NewIndexProgressModel(t.TempDir(), IndexConfig{})
	frameCmd := model.progress.SetPercent(1)
	msg := frameCmd()
	frameMsg, ok := msg.(progress.FrameMsg)
	if !ok {
		t.Fatalf("frameCmd() returned %T, want progress.FrameMsg", msg)
	}

	updated, cmd := model.Update(frameMsg)
	if updated != model {
		t.Fatalf("Update(progress frame) returned model %T, want same pointer", updated)
	}
	if cmd == nil {
		t.Error("Update(progress frame) returned nil cmd, want follow-up animation cmd")
	}
}

func TestIndexProgressModel_UpdateUnhandledMessage(t *testing.T) {
	model := NewIndexProgressModel(t.TempDir(), IndexConfig{})
	type unknownMsg struct{}

	updated, cmd := model.Update(unknownMsg{})
	if updated != model {
		t.Fatalf("Update(unknown message) returned model %T, want same pointer", updated)
	}
	if cmd != nil {
		t.Errorf("Update(unknown message) cmd = %v, want nil", cmd)
	}
}
