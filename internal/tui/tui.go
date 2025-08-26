package tui

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/media"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Icon sets for different terminal capabilities
var (
	// High-quality emoji icons (for modern terminals)
	emojiIcons = map[string]string{
		"stats":      "📊",
		"tv":         "📺",
		"movie":      "🎬",
		"seasons":    "📁",
		"episodes":   "🎬",
		"video":      "🎥",
		"subtitles":  "📄",
		"needrename": "✓",
		"nochange":   "=",
		"delete":     "🗑",
		"success":    "✅",
		"error":      "❌",
		"arrows":     "↑↓←→",
	}

	// ASCII fallback (always works)
	asciiIcons = map[string]string{
		"stats":      "[*]",
		"tv":         "[TV]",
		"movie":      "[M]",
		"seasons":    "[D]",
		"episodes":   "[E]",
		"video":      "[V]",
		"subtitles":  "[S]",
		"needrename": "[+]",
		"nochange":   "[=]",
		"delete":     "[x]",
		"success":    "[v]",
		"error":      "[!]",
		"arrows":     "^v<>",
	}
)

// Cached base styles (applied with dynamic Width each render) to avoid
// re-allocating identical style pipelines on every View() call.
var (
	headerStyleBase = lipgloss.NewStyle().
			Bold(true).
			Background(colorPrimary).
			Foreground(colorBackground).
			Align(lipgloss.Center)

	statusStyleBase = lipgloss.NewStyle().
			Background(colorSecondary).
			Foreground(colorBackground).
			Padding(0, 1)
)

// RenameModel wraps the underlying treeview TUI model to add media rename
// functionality and real‑time statistics.
type RenameModel struct {
	*treeview.TuiTreeModel[treeview.FileInfo]
	renameInProgress bool
	renameComplete   bool
	successCount     int
	errorCount       int
	totalRenameOps   int
	completedOps     int
	progressModel    progress.Model
	progressVisible  bool
	currentOpIndex   int
	virtualDirCount  int
	deletionCount    int
	renameCount      int
	width            int
	height           int
	IsMovieMode      bool
	DeleteNFO        bool
	DeleteImages     bool
	IsLinkMode       bool
	LinkPath         string

	// Layout metrics
	treeWidth   int
	treeHeight  int
	statsWidth  int
	statsHeight int

	// Stat tracking
	statsCache Statistics
	statsDirty bool

	// Icon support
	iconSet map[string]string
}

// NewRenameModel returns an initialized RenameModel for the provided tree with
// default dimensions (later adjusted on the first WindowSize message).
func NewRenameModel(tree *treeview.Tree[treeview.FileInfo]) *RenameModel {
	m := &RenameModel{
		width:      80,
		height:     24,
		statsDirty: true,
	}

	// Detect terminal capabilities and configure icons
	m.detectTerminalCapabilities()
	runewidth.DefaultCondition.EastAsianWidth = false
	runewidth.DefaultCondition.StrictEmojiNeutral = true

	m.progressModel = progress.New(progress.WithGradient(string(colorPrimary), string(colorAccent)))
	m.progressModel.Width = 40
	// establish initial layout metrics before building underlying model
	m.CalculateLayout()
	m.TuiTreeModel = m.createSizedTuiModel(tree)
	return m
}

// detectTerminalCapabilities determines what icons to use based on terminal and environment
func (m *RenameModel) detectTerminalCapabilities() {
	// Check if we're in SSH
	if isSshSession() {
		m.iconSet = asciiIcons
	} else {
		m.iconSet = emojiIcons
	}
}

// getIcon returns the appropriate icon for the current terminal
func (m *RenameModel) getIcon(iconType string) string {
	if icon, exists := m.iconSet[iconType]; exists {
		return icon
	}
	// Fallback to ASCII if icon not found
	return asciiIcons[iconType]
}

// CalculateLayout recomputes panel dimensions from current window size.
func (m *RenameModel) CalculateLayout() {
	// Set tree width to 60%
	tw := m.width * 6 / 10
	// Reserve space for header (1) + newline after header (1) + newline before status (1) + status bar (1) = 4 lines
	th := m.height - 4
	// Ensure min height
	if th < 5 {
		th = 5
	}
	m.treeWidth = tw
	m.treeHeight = th
	// Stats panel uses remaining width
	m.statsWidth = m.width - tw
	// Stats panel has same height as tree (both panels should align)
	m.statsHeight = th
	// ensure a minimal positive stats height
	if m.statsHeight < 1 {
		m.statsHeight = 1
	}
}

// createSizedTuiModel builds a tree model sized to current dimensions and
// disables treeview features (search/reset) not needed for this application.
func (m *RenameModel) createSizedTuiModel(tree *treeview.Tree[treeview.FileInfo]) *treeview.TuiTreeModel[treeview.FileInfo] {
	// Create custom key map without search and reset
	keyMap := treeview.DefaultKeyMap()
	keyMap.SearchStart = []string{} // Disable search
	keyMap.Reset = []string{}       // Disable ctrl+r reset

	return treeview.NewTuiTreeModel(tree,
		treeview.WithTuiWidth[treeview.FileInfo](m.treeWidth),
		treeview.WithTuiHeight[treeview.FileInfo](m.treeHeight),
		treeview.WithTuiAllowResize[treeview.FileInfo](true),
		treeview.WithTuiDisableNavBar[treeview.FileInfo](true),
		treeview.WithTuiKeyMap[treeview.FileInfo](keyMap),
	)
}

// Init initializes the embedded tree model and requests an initial window size.
func (m *RenameModel) Init() tea.Cmd {
	return tea.Batch(
		m.TuiTreeModel.Init(),
		tea.WindowSize(),
	)
}

// Update handles Bubble Tea messages (resize, key events, internal completion).
func (m *RenameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size changes
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Record full window size
		m.width = msg.Width
		m.height = msg.Height
		// Recalculate layout metrics once, then forward scaled size to tree model
		m.CalculateLayout()
		internalMsg := tea.WindowSizeMsg{Width: m.treeWidth, Height: m.treeHeight}
		updated, cmd := m.TuiTreeModel.Update(internalMsg)
		if tm, ok := updated.(*treeview.TuiTreeModel[treeview.FileInfo]); ok {
			m.TuiTreeModel = tm
		}
		return m, cmd

	case tea.KeyMsg:
		// Handle custom keys before passing to tree model
		switch msg.String() {
		case "delete", "d":
			if focusedNode := m.TuiTreeModel.Tree.GetFocusedNode(); focusedNode != nil {
				// Move focus up one position before deletion to maintain nearby focus
				m.TuiTreeModel.Tree.Move(context.Background(), -1)
				m.removeNodeFromTree(focusedNode)
				m.statsDirty = true
			}
			return m, nil
		case "r":
			if !m.renameInProgress {
				m.renameInProgress = true
				m.prepareRenameProgress()
				m.progressVisible = true
				return m, m.PerformRenames()
			}
		case "pgup":
			// Page up - move up by viewport height
			pageSize := max(m.treeHeight, 10)
			m.TuiTreeModel.Tree.Move(context.Background(), -pageSize)
			return m, nil
		case "pgdown":
			// Page down - move down by viewport height
			pageSize := max(m.treeHeight, 10)
			m.TuiTreeModel.Tree.Move(context.Background(), pageSize)
			return m, nil
		}

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		switch msg.Type {
		case tea.MouseWheelUp:
			// Scroll up by 1 line
			m.TuiTreeModel.Tree.Move(context.Background(), -1)
			return m, nil
		case tea.MouseWheelDown:
			// Scroll down by 1 line
			m.TuiTreeModel.Tree.Move(context.Background(), 1)
			return m, nil
		}

	case RenameCompleteMsg:
		m.renameInProgress = false
		m.renameComplete = true
		m.successCount = msg.successCount
		m.errorCount = msg.errorCount
		m.statsDirty = true
		m.progressVisible = false
		return m, nil
	case renameProgressMsg:
		// update bar percent
		var pct float64
		if m.totalRenameOps > 0 {
			pct = min(float64(m.completedOps)/float64(m.totalRenameOps), 1)
		}
		cmd := m.progressModel.SetPercent(pct)
		// schedule next step until completion
		return m, tea.Batch(cmd, m.PerformRenames())
	case progress.FrameMsg:
		// propagate animation frames for the progress bar so percent updates render
		pm, cmd := m.progressModel.Update(msg)
		m.progressModel = pm.(progress.Model)
		return m, cmd
	}

	// Pass through to embedded tree model for other messages
	updatedModel, cmd := m.TuiTreeModel.Update(msg)
	if tm, ok := updatedModel.(*treeview.TuiTreeModel[treeview.FileInfo]); ok {
		m.TuiTreeModel = tm
	}

	return m, cmd
}

// View returns the full TUI string (header, tree+stats layout, status bar).
func (m *RenameModel) View() string {
	var b strings.Builder

	// Render header
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')

	// Stats Panel
	b.WriteString(m.renderTwoPanelLayout())
	b.WriteByte('\n')

	// Render integrated status bar
	b.WriteString(m.renderStatusBar())
	return b.String()
}

// renderHeader creates the single‑line header bar with mode + working directory.
func (m *RenameModel) renderHeader() string {
	style := headerStyleBase.Width(m.width)

	path, _ := os.Getwd()
	var title string
	if m.IsLinkMode {
		if m.IsMovieMode {
			title = fmt.Sprintf("%s Movie Link - %s → %s", m.getIcon("movie"), path, m.LinkPath)
		} else {
			title = fmt.Sprintf("%s TV Show Link - %s → %s", m.getIcon("tv"), path, m.LinkPath)
		}
	} else {
		if m.IsMovieMode {
			title = fmt.Sprintf("%s Movie Rename - %s", m.getIcon("movie"), path)
		} else {
			title = fmt.Sprintf("%s TV Show Rename - %s", m.getIcon("tv"), path)
		}
	}
	return style.Render(title)
}

// renderStatusBar renders a single line of key hints and actions.
func (m *RenameModel) renderStatusBar() string {
	if m.progressVisible && m.renameInProgress {
		// show progress bar with styled text
		bar := m.progressModel.View()
		// Style the text with the same background as the right side of the gradient
		textStyle := lipgloss.NewStyle().
			Background(colorSecondary).
			Foreground(colorBackground).
			Padding(0, 1)
		operationText := "Renaming..."
		if m.IsLinkMode {
			operationText = "Linking..."
		}
		statusText := textStyle.Render(fmt.Sprintf("%d/%d - %s", m.completedOps, m.totalRenameOps, operationText))
		// Combine bar and styled text, then apply the full width style
		combined := fmt.Sprintf("%s  %s", bar, statusText)
		return statusStyleBase.Width(m.width - 1).Render(combined)
	}
	renameKey := "r: Rename"
	if m.IsLinkMode {
		renameKey = "r: Link"
	}
	statusText := fmt.Sprintf("%s: Navigate  PgUp/PgDn: Page  %s: Expand/Collapse  │  %s  │  d: Remove  │  esc: Quit",
		m.getIcon("arrows")[:2], // First two characters (up/down arrows)
		m.getIcon("arrows")[2:], // Last two characters (left/right arrows)
		renameKey)
	return statusStyleBase.Width(m.width - 1).Render(statusText)
}

// renderTwoPanelLayout joins the tree view and statistics panel horizontally.
func (m *RenameModel) renderTwoPanelLayout() string {
	statsPanel := m.renderStatsPanel()
	treeView := m.TuiTreeModel.View()

	// Force tree view to use exact allocated width to prevent stats panel from jumping
	treeContainer := lipgloss.NewStyle().
		Width(m.treeWidth).
		MaxWidth(m.treeWidth).
		Render(treeView)

	// Stats panel already handles its own width internally, don't double-wrap
	return lipgloss.JoinHorizontal(lipgloss.Top, treeContainer, statsPanel)
}

// renderStatsPanel builds and formats the statistics panel content.
func (m *RenameModel) renderStatsPanel() string {
	// Create base style with border and padding
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1)

	// Calculate the actual content width needed after accounting for frame
	// The frame includes border (2) + padding (2) = 4 horizontal chars
	frameWidth := borderStyle.GetHorizontalFrameSize()
	frameHeight := borderStyle.GetVerticalFrameSize()

	contentWidth := m.statsWidth - frameWidth
	contentHeight := m.statsHeight - frameHeight

	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Apply calculated dimensions to the style
	// Width/Height in lipgloss is for the content area only when padding is used
	style := borderStyle.
		Width(contentWidth).
		Height(contentHeight)

	stats := m.calculateStats()
	var b strings.Builder
	b.Grow(512)

	// Format stats content with appropriate icons based on terminal capabilities
	fmt.Fprintf(&b, "%s Statistics\n\n", m.getIcon("stats"))
	b.WriteString("Files Found:\n")
	if m.IsMovieMode {
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("movie"), "Movies:", stats.movieCount)
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("video"), "Video Files:", stats.movieFileCount-stats.subtitleCount)
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("subtitles"), "Subtitles:", stats.subtitleCount)
	} else {
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("tv"), "TV Shows:", stats.showCount)
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("seasons"), "Seasons:", stats.seasonCount)
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("episodes"), "Episodes:", stats.episodeCount)
		fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("subtitles"), "Subtitles:", stats.subtitleCount)
	}

	b.WriteString("\nRename Status:\n")
	renameLabel := "Need rename:"
	if m.IsLinkMode {
		renameLabel = "To link:"
	}
	fmt.Fprintf(&b, "  %s %-13s %d\n", m.getIcon("needrename"), renameLabel, stats.needRenameCount)
	fmt.Fprintf(&b, "  %s %-13s %d\n", m.getIcon("nochange"), "No change:", stats.noChangeCount)
	if stats.toDeleteCount > 0 {
		fmt.Fprintf(&b, "  %s %-13s %d\n", m.getIcon("delete"), "To delete:", stats.toDeleteCount)
	}

	if stats.successCount > 0 || stats.errorCount > 0 {
		b.WriteString("\nLast Operation:\n")
		if stats.successCount > 0 {
			fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("success"), "Success:", stats.successCount)
		}
		if stats.errorCount > 0 {
			fmt.Fprintf(&b, "  %s %-12s %d\n", m.getIcon("error"), "Errors:", stats.errorCount)
		}
	}

	if m.progressVisible && m.renameInProgress {
		percent := 0
		if m.totalRenameOps > 0 {
			percent = (m.completedOps * 100) / m.totalRenameOps
		}
		b.WriteString("\nRename Progress:\n")
		fmt.Fprintf(&b, "  %d/%d (%d%%)\n", m.completedOps, m.totalRenameOps, percent)
	}

	var totalItems int
	if m.IsMovieMode {
		totalItems = stats.movieCount + stats.movieFileCount
	} else {
		totalItems = stats.showCount + stats.seasonCount + stats.episodeCount + stats.subtitleCount
	}

	fmt.Fprintf(&b, "\nTotal items: %d\n", totalItems)
	if totalItems > 0 {
		percentNeedRename := (stats.needRenameCount * 100) / totalItems
		if m.IsLinkMode {
			fmt.Fprintf(&b, "To link: %d%%", percentNeedRename)
		} else {
			fmt.Fprintf(&b, "Need rename: %d%%", percentNeedRename)
		}
	}

	return style.Render(b.String())
}

// Statistics aggregates counts derived from the current tree plus the most
// recent batch rename operation.
//
// Fields:
//   - showCount / seasonCount / episodeCount: counts of TV hierarchy nodes.
//   - movieCount / movieFileCount: counts for movie mode (directories & files).
//   - subtitleCount: number of subtitle files (subset of episode/movie files).
//   - needRenameCount: nodes where NewName differs from current name.
//   - noChangeCount: nodes with a proposed name identical to current name.
//   - successCount / errorCount: results from the last performRenames run.
//   - toDeleteCount: nodes marked for deletion.
type Statistics struct {
	showCount       int
	seasonCount     int
	episodeCount    int
	subtitleCount   int
	movieCount      int
	movieFileCount  int
	needRenameCount int
	noChangeCount   int
	successCount    int
	errorCount      int
	toDeleteCount   int
}

// calculateStats walks the tree to produce aggregate counts while preserving
// previously recorded success/error tallies from the last rename operation.
func (m *RenameModel) calculateStats() Statistics {
	// Fast path: return cached stats if still valid
	if !m.statsDirty {
		// always ensure latest success/error counts reflected even if cache reused
		m.statsCache.successCount = m.successCount
		m.statsCache.errorCount = m.errorCount
		return m.statsCache
	}

	stats := Statistics{}
	for nodeInfo := range m.Tree.All(context.Background()) {
		node := nodeInfo.Node
		mm := core.GetMeta(node)
		if mm == nil {
			continue
		}
		switch mm.Type {
		case core.MediaShow:
			stats.showCount++
		case core.MediaSeason:
			stats.seasonCount++
		case core.MediaEpisode:
			stats.episodeCount++
		case core.MediaMovie:
			stats.movieCount++
		case core.MediaMovieFile:
			stats.movieFileCount++
		}
		if !node.Data().IsDir() && media.IsSubtitle(node.Data().Name()) {
			stats.subtitleCount++
		}
		if mm.MarkedForDeletion {
			stats.toDeleteCount++
		} else if m.IsLinkMode {
			// In link mode, count based on destination paths
			if mm.DestinationPath != "" {
				stats.needRenameCount++
			} else {
				stats.noChangeCount++
			}
		} else if mm.NewName != "" {
			// In rename mode, count based on name changes
			if mm.NewName != node.Name() {
				stats.needRenameCount++
			} else {
				stats.noChangeCount++
			}
		}
	}
	stats.successCount = m.successCount
	stats.errorCount = m.errorCount
	m.statsCache = stats
	m.statsDirty = false
	return stats
}

// removeNodeFromTree removes the given node from the tree by checking if it's a root node
// (has no parent) and either removing it from the root slice or from its parent's children.
func (m *RenameModel) removeNodeFromTree(nodeToRemove *treeview.Node[treeview.FileInfo]) {
	if nodeToRemove == nil {
		return
	}

	parent := nodeToRemove.Parent()
	if parent == nil {
		m.removeRootNode(nodeToRemove)
		return
	}

	// Remove the node from its parent's children
	currentChildren := parent.Children()
	// Create a new slice to avoid modifying the original
	childrenCopy := make([]*treeview.Node[treeview.FileInfo], len(currentChildren))
	copy(childrenCopy, currentChildren)
	filteredChildren := slices.DeleteFunc(childrenCopy, func(n *treeview.Node[treeview.FileInfo]) bool {
		return n == nodeToRemove
	})
	parent.SetChildren(filteredChildren)
}

// removeRootNode removes a root node from the tree's internal nodes slice
func (m *RenameModel) removeRootNode(nodeToRemove *treeview.Node[treeview.FileInfo]) {
	// Get the current root nodes and filter out the node to remove
	currentRoots := m.TuiTreeModel.Tree.Nodes()
	// Create a new slice to avoid modifying the original
	rootsCopy := make([]*treeview.Node[treeview.FileInfo], len(currentRoots))
	copy(rootsCopy, currentRoots)
	filteredRoots := slices.DeleteFunc(rootsCopy, func(n *treeview.Node[treeview.FileInfo]) bool {
		return n == nodeToRemove
	})
	m.TuiTreeModel.Tree.SetNodes(filteredRoots)
}
