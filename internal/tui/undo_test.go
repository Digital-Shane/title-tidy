package tui

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/google/go-cmp/cmp"
)

func newTestUndoModel() *UndoModel {
	return &UndoModel{
		iconSet: map[string]string{
			"check":  "✓",
			"error":  "✗",
			"link":   "→",
			"delete": "×",
			"folder": "📁",
		},
	}
}

func TestUndoModelGetOperationIcon(t *testing.T) {
	m := newTestUndoModel()

	tests := []struct {
		name string
		op   log.OperationLog
		want string
	}{
		{
			name: "RenameSuccess",
			op:   log.OperationLog{Type: log.OpRename, Success: true},
			want: "✓",
		},
		{
			name: "RenameFailure",
			op:   log.OperationLog{Type: log.OpRename, Success: false},
			want: "✗",
		},
		{
			name: "Link",
			op:   log.OperationLog{Type: log.OpLink},
			want: "→",
		},
		{
			name: "Delete",
			op:   log.OperationLog{Type: log.OpDelete},
			want: "×",
		},
		{
			name: "CreateDir",
			op:   log.OperationLog{Type: log.OpCreateDir},
			want: "📁",
		},
		{
			name: "Unknown",
			op:   log.OperationLog{Type: log.OperationType("other")},
			want: ASCIIIcons["unknown"],
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := m.getOperationIcon(tc.op)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getOperationIcon(%s) diff (-want +got):\n%s", tc.name, diff)
			}
		})
	}
}

func TestUndoModelGetIconFallback(t *testing.T) {
	m := newTestUndoModel()

	if got := m.getIcon("check"); got != "✓" {
		t.Errorf("getIcon(check) = %q, want %q", got, "✓")
	}

	if got := m.getIcon("unknown"); got != ASCIIIcons["unknown"] {
		t.Errorf("getIcon(unknown) = %q, want %q", got, ASCIIIcons["unknown"])
	}
}

func TestUndoModelFormatOperation(t *testing.T) {
	m := newTestUndoModel()

	tests := []struct {
		name     string
		op       log.OperationLog
		maxWidth int
		want     string
	}{
		{
			name: "RenameSuccess",
			op: log.OperationLog{
				Type:       log.OpRename,
				SourcePath: "/media/old-name.mkv",
				DestPath:   "/media/new-name.mkv",
				Success:    true,
			},
			maxWidth: 80,
			want:     "old-name.mkv → new-name.mkv",
		},
		{
			name: "RenameFailureAddsSuffix",
			op: log.OperationLog{
				Type:       log.OpRename,
				SourcePath: "broken.mkv",
				DestPath:   "missing.mkv",
				Success:    false,
				Error:      "no dest",
			},
			maxWidth: 80,
			want:     "broken.mkv → missing.mkv (failed)",
		},
		{
			name: "Link",
			op: log.OperationLog{
				Type:       log.OpLink,
				SourcePath: "/tmp/symlink.srt",
			},
			maxWidth: 80,
			want:     "Link: symlink.srt",
		},
		{
			name: "Delete",
			op: log.OperationLog{
				Type:       log.OpDelete,
				SourcePath: "/tmp/remove.mkv",
			},
			maxWidth: 80,
			want:     "Delete: remove.mkv",
		},
		{
			name: "CreateDir",
			op: log.OperationLog{
				Type:     log.OpCreateDir,
				DestPath: "/data/new-folder",
			},
			maxWidth: 80,
			want:     "Create: new-folder/",
		},
		{
			name: "DefaultType",
			op: log.OperationLog{
				Type: log.OperationType("custom"),
			},
			maxWidth: 80,
			want:     "custom",
		},
		{
			name: "TruncatesLongText",
			op: log.OperationLog{
				Type: log.OperationType("averylongoperationtype"),
			},
			maxWidth: 12,
			want:     "averylong...",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := m.formatOperation(tc.op, tc.maxWidth)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("formatOperation(%s) diff (-want +got):\n%s", tc.name, diff)
			}
		})
	}
}
