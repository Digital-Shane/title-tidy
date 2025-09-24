package components

import (
	"github.com/Digital-Shane/title-tidy/internal/core"
	"testing"

	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

// helper to build a node (dir or file) with path == name for simplicity
func testNode(name string, isDir bool) *treeview.Node[treeview.FileInfo] {
	fi := core.NewSimpleFileInfo(name, isDir)
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: fi, Path: name})
}

func TestMetaRule_Basic(t *testing.T) {
	t.Parallel()
	calls := 0
	cond := func(mm *core.MediaMeta) bool { calls++; return mm.Type == core.MediaShow }
	pred := metaRule(cond)
	n1 := testNode("no-meta", true)
	if pred(n1) {
		t.Errorf("metaRule(noMeta) = true, want false")
	}
	if calls != 0 {
		t.Errorf("metaRule(noMeta) calls = %d, want 0", calls)
	}
	mm := core.EnsureMeta(n1)
	mm.Type = core.MediaSeason
	if pred(n1) {
		t.Errorf("metaRule(nonMatch) = true, want false")
	}
	if calls != 1 {
		t.Errorf("metaRule(nonMatch) cond calls = %d, want 1", calls)
	}
	mm.Type = core.MediaShow
	if !pred(n1) {
		t.Errorf("metaRule(match) = false, want true")
	}
	if calls != 2 {
		t.Errorf("metaRule(match) cond calls = %d, want 2", calls)
	}
}

func TestStatusAndTypePredicates(t *testing.T) {
	t.Parallel()
	n := testNode("item", false)
	mm := core.EnsureMeta(n)
	mm.Type = core.MediaEpisode
	mm.RenameStatus = core.RenameStatusNone
	if !typeIs(core.MediaEpisode)(n) {
		t.Errorf("typeIs(MediaEpisode) = false, want true")
	}
	if typeIs(core.MediaShow)(n) {
		t.Errorf("typeIs(MediaShow) = true, want false")
	}
	if statusIs(core.RenameStatusSuccess)(n) {
		t.Errorf("statusIs(Success-before) = true, want false")
	}
	if !statusIs(core.RenameStatusNone)(n) {
		t.Errorf("statusIs(None-before) = false, want true")
	}
	mm.RenameStatus = core.RenameStatusSuccess
	if !statusIs(core.RenameStatusSuccess)(n) {
		t.Errorf("statusIs(Success-after) = false, want true")
	}
}

func TestStatusNoneTypePredicate(t *testing.T) {
	t.Parallel()
	n := testNode("movie", true)
	mm := core.EnsureMeta(n)
	mm.Type = core.MediaMovie
	mm.RenameStatus = core.RenameStatusNone
	if !statusNoneType(core.MediaMovie)(n) {
		t.Errorf("statusNoneType(Movie none) = false, want true")
	}
	mm.RenameStatus = core.RenameStatusSuccess
	if statusNoneType(core.MediaMovie)(n) {
		t.Errorf("statusNoneType(Movie success) = true, want false")
	}
}

func TestNeedsDirPredicate(t *testing.T) {
	t.Parallel()
	n := testNode("virt", true)
	mm := core.EnsureMeta(n)
	mm.Type = core.MediaMovie
	mm.RenameStatus = core.RenameStatusNone
	mm.NeedsDirectory = true
	if !needsDir()(n) {
		t.Errorf("needsDir(true) = false, want true")
	}
	mm.NeedsDirectory = false
	if needsDir()(n) {
		t.Errorf("needsDir(false) = true, want false")
	}
}

func TestRenameFormatter(t *testing.T) {
	t.Parallel()
	// Table cases for consistent comparisons
	cases := []struct {
		name     string
		nodeName string
		isDir    bool
		setup    func(*core.MediaMeta)
		want     string
	}{
		{"NoMeta", "orig", false, nil, "orig"},
		{"EmptyNew", "orig2", false, func(mm *core.MediaMeta) {}, "orig2"},
		{"Success", "old", false, func(mm *core.MediaMeta) { mm.NewName = "new"; mm.RenameStatus = core.RenameStatusSuccess }, "new"},
		{"Error", "origE", false, func(mm *core.MediaMeta) {
			mm.NewName = "ignored"
			mm.RenameStatus = core.RenameStatusError
			mm.RenameError = "boom"
		}, "origE: boom"},
		{"Virtual", "oldDir", true, func(mm *core.MediaMeta) { mm.NewName = "Movie Name"; mm.NeedsDirectory = true }, "[NEW] Movie Name"},
		{"Same", "same", false, func(mm *core.MediaMeta) { mm.NewName = "same" }, "same"},
		{"Mapping", "oldname", false, func(mm *core.MediaMeta) { mm.NewName = "New Name" }, "New Name ← oldname"},
	}
	for _, tc := range cases {
		n := testNode(tc.nodeName, tc.isDir)
		if tc.setup != nil {
			tc.setup(core.EnsureMeta(n))
		}
		got, _ := RenameFormatter(n)
		if got != tc.want {
			t.Errorf("renameFormatter(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCreateRenameProvider_Constructs(t *testing.T) {
	p := CreateRenameProvider()
	if p == nil {
		t.Fatalf("createRenameProvider() = nil, want non-nil")
	}
	n := testNode("file", false)
	mm := core.EnsureMeta(n)
	mm.NewName = "File Renamed"
	gotDirect, _ := RenameFormatter(n)
	if pf, ok := interface{}(p).(interface {
		Format(*treeview.Node[treeview.FileInfo], bool) string
	}); ok {
		gotProvider := pf.Format(n, false)
		if diff := cmp.Diff(gotDirect, gotProvider); diff != "" {
			t.Errorf("createRenameProvider.Format mismatch (-want +got)\n%s", diff)
		}
	}
}

func TestRenameFormatter_MarkedForDeletion(t *testing.T) {
	t.Parallel()
	n := testNode("delete.nfo", false)
	mm := core.EnsureMeta(n)
	mm.MarkedForDeletion = true

	got, _ := RenameFormatter(n)
	expected := "delete.nfo" // Formatter just shows filename, icon handles deletion status

	if got != expected {
		t.Errorf("RenameFormatter(deleted) = %q, want %q", got, expected)
	}
}

func TestRenameFormatter_MarkedForDeletionWithError(t *testing.T) {
	t.Parallel()
	n := testNode("failed.nfo", false)
	mm := core.EnsureMeta(n)
	mm.MarkedForDeletion = true
	mm.RenameStatus = core.RenameStatusError
	mm.RenameError = "Permission denied"

	got, _ := RenameFormatter(n)
	expected := "failed.nfo: Permission denied"

	if got != expected {
		t.Errorf("RenameFormatter(deletion error) = %q, want %q", got, expected)
	}
}
