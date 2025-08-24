package core

import "github.com/Digital-Shane/treeview"

// MediaType enumerates the semantic classification of a node within the media library hierarchy.
type MediaType int

const (
	MediaShow      MediaType = iota // Top‑level TV show directory
	MediaSeason                     // Season directory inside a show
	MediaEpisode                    // Individual episode file (video or subtitle)
	MediaMovie                      // Movie directory (real or virtual)
	MediaMovieFile                  // File inside a movie directory (video or subtitle)
)

// LinkMode specifies the type of file system link to create instead of renaming.
type LinkMode int

const (
	LinkModeNone LinkMode = iota // Normal rename operation (no linking)
	LinkModeAuto                  // Try hard link first, fall back to soft link
	LinkModeHard                  // Hard links only (fail if not possible)
	LinkModeSoft                  // Soft/symbolic links only
)

// RenameStatus represents the lifecycle stage of a proposed rename operation.
// A node starts at RenameStatusNone; after execution it is marked success or
// error with an accompanying message when relevant.
type RenameStatus int

const (
	RenameStatusNone    RenameStatus = iota // Rename not yet attempted, or no change needed
	RenameStatusSuccess                     // Rename succeeded
	RenameStatusError                       // Rename failed; see RenameError for detail
)

// MediaMeta holds per-node rename intent and results.
//
// Fields:
//   - Type: Media classification used for rule selection and statistics.
//   - NewName: Proposed final name (filename or directory name). Empty implies
//     no change or unknown format.
//   - RenameStatus / RenameError: Outcome of the rename attempt. Error message
//     is only populated when status == RenameStatusError.
//   - IsVirtual: True when the node does not (yet) exist on disk; used for
//     synthesized movie directories wrapping loose video files.
//   - NeedsDirectory: Signals that a directory must be created before children
//     are renamed beneath it (typically paired with IsVirtual).
//   - MarkedForDeletion: True when the file should be deleted during rename operation.
//   - LinkMode: Type of link to create instead of renaming (None for normal rename).
//   - LinkTarget: Root directory for creating linked file structure.
//
// The zero value is meaningful: it encodes an untyped, unprocessed node with no rename proposal.
type MediaMeta struct {
	Type              MediaType
	NewName           string
	RenameStatus      RenameStatus
	RenameError       string
	IsVirtual         bool
	NeedsDirectory    bool
	MarkedForDeletion bool
	LinkMode          LinkMode
	LinkTarget        string
}

// GetMeta retrieves the existing *MediaMeta attached to n or nil when absent.
// It is safe to call with a nil node.
func GetMeta(n *treeview.Node[treeview.FileInfo]) *MediaMeta {
	if n == nil || n.Data().Extra == nil {
		return nil
	}
	if m, ok := n.Data().Extra["meta"].(*MediaMeta); ok {
		return m
	}
	return nil
}

// EnsureMeta returns the existing *MediaMeta for n, creating and attaching a
// new instance if needed. The returned pointer is always non-nil.
func EnsureMeta(n *treeview.Node[treeview.FileInfo]) *MediaMeta {
	if n.Data().Extra == nil {
		n.Data().Extra = map[string]any{}
	}
	if m, ok := n.Data().Extra["meta"].(*MediaMeta); ok {
		return m
	}
	m := &MediaMeta{}
	n.Data().Extra["meta"] = m
	return m
}

func (m *MediaMeta) Fail(err error) error {
	m.RenameStatus = RenameStatusError
	m.RenameError = err.Error()
	return err
}

func (m *MediaMeta) Success() {
	m.RenameStatus = RenameStatusSuccess
}
