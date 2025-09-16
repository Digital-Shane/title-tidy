package core

import (
	"errors"
	"testing"

	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

// helper to make a bare node with optional pre-populated Extra map
func newTestNode(name string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: &SimpleFileInfo{name: name, isDir: true}, Path: name})
}

func TestGetMeta_NilAndAbsent(t *testing.T) {
	t.Parallel()
	if got := GetMeta(nil); got != nil {
		t.Errorf("getMeta(nil) = %v, want nil", got)
	}
	n := newTestNode("show")
	if got := GetMeta(n); got != nil {
		t.Errorf("getMeta(noExtra) = %v, want nil", got)
	}
	n.Data().Extra = map[string]any{"other": 123}
	if got := GetMeta(n); got != nil {
		t.Errorf("getMeta(noMetaKey) = %v, want nil", got)
	}
	n.Data().Extra["meta"] = "not-a-meta"
	if got := GetMeta(n); got != nil {
		t.Errorf("getMeta(wrongType) = %v, want nil", got)
	}
}

func TestGetMeta_Present(t *testing.T) {
	t.Parallel()
	n := newTestNode("show")
	mmStored := &MediaMeta{Type: MediaShow, NewName: "Show Name"}
	n.Data().Extra = map[string]any{"meta": mmStored}
	if got := GetMeta(n); got != mmStored {
		t.Errorf("getMeta(present) = %v, want %v", got, mmStored)
	}
}

func TestEnsureMeta_CreationAndReuse(t *testing.T) {
	t.Parallel()
	n := newTestNode("show")
	mm1 := EnsureMeta(n)
	if mm1 == nil {
		t.Fatalf("ensureMeta(new) = nil, want non-nil")
	}
	zeroWant := MediaMeta{}
	if diff := cmp.Diff(zeroWant.RenameStatus, mm1.RenameStatus); diff != "" || mm1.Type != 0 || mm1.NewName != "" {
		// Only checking fields that should be zero; cmp used for consistency.
		t.Errorf("ensureMeta(new) zero mismatch: %+v", mm1)
	}
	n.Data().Extra["other"] = 42
	mm2 := EnsureMeta(n)
	if mm1 != mm2 {
		t.Errorf("ensureMeta(reuse) pointers differ: %p vs %p", mm1, mm2)
	}
	if v := n.Data().Extra["other"]; v != 42 {
		t.Errorf("ensureMeta(reuse) other key = %v, want 42", v)
	}
}

func TestMediaMeta_fail(t *testing.T) {
	t.Parallel()
	m := &MediaMeta{}
	sentinel := errors.New("boom")
	if got := m.Fail(sentinel); !errors.Is(got, sentinel) {
		t.Errorf("MediaMeta.fail() = %v, want %v", got, sentinel)
	}
	if m.RenameStatus != RenameStatusError {
		t.Errorf("MediaMeta.fail() status = %v, want %v", m.RenameStatus, RenameStatusError)
	}
	if m.RenameError != sentinel.Error() {
		t.Errorf("MediaMeta.fail() RenameError = %q, want %q", m.RenameError, sentinel.Error())
	}
	second := errors.New("other")
	m.Fail(second)
	if m.RenameError != "other" {
		t.Errorf("MediaMeta.fail(second) RenameError = %q, want 'other'", m.RenameError)
	}
}

func TestMediaMeta_success(t *testing.T) {
	t.Parallel()
	m := &MediaMeta{}
	m.Success()
	if m.RenameStatus != RenameStatusSuccess {
		t.Errorf("MediaMeta.success() status = %v, want %v", m.RenameStatus, RenameStatusSuccess)
	}
	m.RenameError = "previous"
	m.Success()
	if m.RenameStatus != RenameStatusSuccess || m.RenameError != "previous" {
		t.Errorf("MediaMeta.success() mutated fields = %+v", m)
	}
}
