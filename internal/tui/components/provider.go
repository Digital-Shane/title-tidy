package components

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/lipgloss"
)

// ---- predicate helpers ----
// metaRule adapts a metadata predicate to a node predicate. If a node lacks
// metadata the predicate returns false.
func metaRule(cond func(*core.MediaMeta) bool) func(*treeview.Node[treeview.FileInfo]) bool {
	return func(n *treeview.Node[treeview.FileInfo]) bool {
		if mm := core.GetMeta(n); mm != nil {
			return cond(mm)
		}
		return false
	}
}

// statusIs returns a predicate matching nodes whose rename status equals s
func statusIs(s core.RenameStatus) func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool { return mm.RenameStatus == s })
}

// typeIs returns a predicate matching nodes of media type t
func typeIs(t core.MediaType) func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool { return mm.Type == t })
}

// statusNoneType matches nodes with no status yet and a specific media type
func statusNoneType(t core.MediaType) func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool { return mm.RenameStatus == core.RenameStatusNone && mm.Type == t })
}

// needsDir matches virtual nodes that require a directory to be created
func needsDir() func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool {
		return mm.RenameStatus == core.RenameStatusNone && mm.NeedsDirectory
	})
}

// markedForDeletion matches nodes marked for deletion
func markedForDeletion() func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool {
		return mm.MarkedForDeletion && mm.RenameStatus == core.RenameStatusNone
	})
}

// deletionSuccess matches successfully deleted nodes
func deletionSuccess() func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool {
		return mm.MarkedForDeletion && mm.RenameStatus == core.RenameStatusSuccess
	})
}

// deletionError matches nodes that failed to delete
func deletionError() func(*treeview.Node[treeview.FileInfo]) bool {
	return metaRule(func(mm *core.MediaMeta) bool {
		return mm.MarkedForDeletion && mm.RenameStatus == core.RenameStatusError
	})
}

// CreateRenameProvider constructs the [treeview.DefaultNodeProvider] used by
// the TUI and instant execution paths. It wires together:
//   - icon rules (status precedes type so success/error override type icons)
//   - style rules (normal & focused variants) with precedence similar to icons
//   - the custom [RenameFormatter] for inline original→new labeling.
func CreateRenameProvider() *treeview.DefaultNodeProvider[treeview.FileInfo] {
	th := theme.Default()
	colors := th.Colors()
	iconSet := th.IconSet()

	// Icon rules (order matters: status first)
	// Deletion status icons (highest priority)
	deletionSuccessIconRule := treeview.WithIconRule(deletionSuccess(), iconSet["success"])
	deletionErrorIconRule := treeview.WithIconRule(deletionError(), iconSet["delete"])
	markedForDeletionIconRule := treeview.WithIconRule(markedForDeletion(), iconSet["delete"])
	// Regular status icons
	successIconRule := treeview.WithIconRule(statusIs(core.RenameStatusSuccess), iconSet["success"])
	errorIconRule := treeview.WithIconRule(statusIs(core.RenameStatusError), iconSet["error"])
	virtualDirIconRule := treeview.WithIconRule(needsDir(), iconSet["virtual"])
	showIconRule := treeview.WithIconRule(statusNoneType(core.MediaShow), iconSet["show"])
	seasonIconRule := treeview.WithIconRule(statusNoneType(core.MediaSeason), iconSet["season"])
	episodeIconRule := treeview.WithIconRule(statusNoneType(core.MediaEpisode), iconSet["episode"])
	movieIconRule := treeview.WithIconRule(statusNoneType(core.MediaMovie), iconSet["movie"])
	movieFileIconRule := treeview.WithIconRule(func(n *treeview.Node[treeview.FileInfo]) bool {
		if local.IsSubtitle(n.Name()) {
			return false
		}
		return statusNoneType(core.MediaMovieFile)(n)
	}, iconSet["moviefile"])
	defaultIconRule := treeview.WithDefaultIcon[treeview.FileInfo](iconSet["default"])

	// Style rules (most specific first)
	showStyleRule := treeview.WithStyleRule(
		typeIs(core.MediaShow),
		lipgloss.NewStyle().Foreground(colors.Primary).Bold(true),
		lipgloss.NewStyle().Foreground(colors.Background).Bold(true).Background(colors.Secondary).PaddingRight(1),
	)
	seasonStyleRule := treeview.WithStyleRule(
		typeIs(core.MediaSeason),
		lipgloss.NewStyle().Foreground(colors.Secondary).Bold(true),
		lipgloss.NewStyle().Foreground(colors.Background).Bold(true).Background(colors.Primary),
	)
	episodeStyleRule := treeview.WithStyleRule(
		typeIs(core.MediaEpisode),
		lipgloss.NewStyle().Foreground(colors.Muted),
		lipgloss.NewStyle().Foreground(colors.Background).Background(colors.Primary),
	)
	movieStyleRule := treeview.WithStyleRule(
		typeIs(core.MediaMovie),
		lipgloss.NewStyle().Foreground(colors.Primary).Bold(true),
		lipgloss.NewStyle().Foreground(colors.Background).Bold(true).Background(colors.Secondary).PaddingRight(1),
	)
	movieFileStyleRule := treeview.WithStyleRule(
		typeIs(core.MediaMovieFile),
		lipgloss.NewStyle().Foreground(colors.Muted),
		lipgloss.NewStyle().Foreground(colors.Background).Background(colors.Primary),
	)
	successStyleRule := treeview.WithStyleRule(
		statusIs(core.RenameStatusSuccess),
		lipgloss.NewStyle().Foreground(colors.Success),
		lipgloss.NewStyle().Foreground(colors.Success).Background(colors.Background),
	)
	errorStyleRule := treeview.WithStyleRule(
		statusIs(core.RenameStatusError),
		lipgloss.NewStyle().Foreground(colors.Error),
		lipgloss.NewStyle().Foreground(colors.Error).Background(colors.Background),
	)
	// Deletion style rules
	markedForDeletionStyleRule := treeview.WithStyleRule(
		markedForDeletion(),
		lipgloss.NewStyle().Foreground(colors.Error).Strikethrough(true),
		lipgloss.NewStyle().Foreground(colors.Error).Background(colors.Background).Strikethrough(true),
	)
	deletionSuccessStyleRule := treeview.WithStyleRule(
		deletionSuccess(),
		lipgloss.NewStyle().Foreground(colors.Muted).Strikethrough(true),
		lipgloss.NewStyle().Foreground(colors.Background).Background(colors.Muted).Strikethrough(true),
	)
	defaultStyleRule := treeview.WithStyleRule(
		func(*treeview.Node[treeview.FileInfo]) bool { return true },
		lipgloss.NewStyle().Foreground(colors.Primary),
		lipgloss.NewStyle().Foreground(colors.Background).Background(colors.Primary),
	)

	formatterRule := treeview.WithFormatter(RenameFormatter)

	return treeview.NewDefaultNodeProvider(
		// Icon rules (order matters - most specific first)
		deletionSuccessIconRule, deletionErrorIconRule, markedForDeletionIconRule,
		successIconRule, errorIconRule, virtualDirIconRule, showIconRule, seasonIconRule, episodeIconRule, movieIconRule, movieFileIconRule, defaultIconRule,
		// Style rules (order matters - most specific first)
		deletionSuccessStyleRule, markedForDeletionStyleRule, successStyleRule, errorStyleRule, showStyleRule, seasonStyleRule, episodeStyleRule, movieStyleRule, movieFileStyleRule, defaultStyleRule,
		// Formatter
		formatterRule,
	)
}

// RenameFormatter produces the display label for a node during visualization.
//
//   - If no metadata or no proposed NewName exists, the original name is returned unchanged.
//   - On success, only the new name is shown (keeps the tree clean post-apply).
//   - On error, the original name plus the error message are shown.
//   - For virtual directory creation, a [NEW] prefix is prepended to the proposed name.
//   - If the new name equals the original, the original is shown.
//   - Otherwise: "<new> ← <old>" conveys the pending rename mapping.
func RenameFormatter(node *treeview.Node[treeview.FileInfo]) (string, bool) {
	mm := core.GetMeta(node)
	if mm == nil {
		return node.Name(), true
	}

	// File marked for deletion - show just the filename (icon handles the status)
	if mm.MarkedForDeletion {
		if mm.RenameStatus == core.RenameStatusError {
			return fmt.Sprintf("%s: %s", node.Name(), mm.RenameError), true
		}
		return node.Name(), true
	}

	if mm.NewName == "" {
		// no proposed rename
		return node.Name(), true
	}
	// Status specific
	switch mm.RenameStatus {
	case core.RenameStatusSuccess:
		return mm.NewName, true
	case core.RenameStatusError:
		return fmt.Sprintf("%s: %s", node.Name(), mm.RenameError), true
	}
	// Virtual / directory creation
	if mm.NeedsDirectory {
		return "[NEW] " + mm.NewName, true
	}
	// Unchanged name, keep original
	if mm.NewName == node.Name() {
		return node.Name(), true
	}
	return fmt.Sprintf("%s ← %s", mm.NewName, node.Name()), true
}
