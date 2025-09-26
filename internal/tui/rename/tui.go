package rename

import (
	"context"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// UndoCompleteMsg is emitted when undo operation completes
type UndoCompleteMsg struct{ successCount, errorCount int }

func (u UndoCompleteMsg) SuccessCount() int { return u.successCount }
func (u UndoCompleteMsg) ErrorCount() int   { return u.errorCount }

// MetadataProgressMsg updates metadata fetching progress
type MetadataProgressMsg struct {
	Total     int
	Completed int
	Errors    int
	Status    string
}

// MetadataCompleteMsg signals metadata fetching is complete
type MetadataCompleteMsg struct {
	Errors int
}

// RenameModel wraps the underlying treeview TUI model to add media rename
// functionality and real‑time statistics.
type RenameModel struct {
	*treeview.TuiTreeModel[treeview.FileInfo]
	renameInProgress bool
	renameComplete   bool
	successCount     int
	errorCount       int
	progressModel    progress.Model
	progressVisible  bool
	progress         OperationProgress
	engine           *OperationEngine
	width            int
	height           int
	IsMovieMode      bool
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

	theme theme.Theme

	// Command info for logging
	Command     string
	CommandArgs []string

	// Undo support
	undoAvailable  bool
	undoInProgress bool
	undoComplete   bool
	undoSuccess    int
	undoFailed     int

	// Stats panel scrolling
	statsViewport *viewport.Model
	statsFocused  bool // whether the stats panel is focused for scrolling

	// Metadata fetching progress
	metadataFetching  bool
	metadataTotal     int
	metadataCompleted int
	metadataErrors    int
	metadataStatus    string
}

// Option configures a RenameModel during construction.
type Option func(*RenameModel)

// WithTheme overrides the theme used by the rename TUI.
func WithTheme(th theme.Theme) Option {
	return func(m *RenameModel) {
		m.theme = th
	}
}

// NewOperationEngine constructs a fresh operation engine snapshot using the
// current rename model configuration and tree state.
func (m *RenameModel) NewOperationEngine() *OperationEngine {
	if m.TuiTreeModel == nil {
		return NewOperationEngine(OperationConfig{})
	}
	return NewOperationEngine(OperationConfig{
		Tree:        m.TuiTreeModel.Tree,
		IsLinkMode:  m.IsLinkMode,
		LinkPath:    m.LinkPath,
		Command:     m.Command,
		CommandArgs: m.CommandArgs,
	})
}

func (m *RenameModel) headerStyle() lipgloss.Style {
	return m.theme.HeaderStyle()
}

func (m *RenameModel) statusBarStyle() lipgloss.Style {
	return m.theme.StatusBarStyle()
}

func (m *RenameModel) panelStyle() lipgloss.Style {
	return m.theme.PanelStyle()
}

func (m *RenameModel) panelTitleStyle() lipgloss.Style {
	return m.theme.PanelTitleStyle()
}

// NewRenameModel returns an initialized RenameModel for the provided tree with
// default dimensions (later adjusted on the first WindowSize message).
func NewRenameModel(tree *treeview.Tree[treeview.FileInfo], opts ...Option) *RenameModel {
	m := &RenameModel{
		width:      80,
		height:     24,
		statsDirty: true,
	}

	initOpts := append([]Option{WithTheme(theme.Default())}, opts...)
	for _, opt := range initOpts {
		opt(m)
	}

	runewidth.DefaultCondition.EastAsianWidth = false
	runewidth.DefaultCondition.StrictEmojiNeutral = true

	gradient := m.theme.ProgressGradient()
	if len(gradient) < 2 {
		gradient = []string{string(m.theme.Colors().Primary), string(m.theme.Colors().Accent)}
	}
	m.progressModel = progress.New(progress.WithGradient(gradient[0], gradient[1]))
	m.progressModel.Width = 40
	// establish initial layout metrics before building underlying model
	m.CalculateLayout()

	// Initialize stats viewport
	m.statsViewport = components.NewViewport(m.statsWidth, m.statsHeight, m.theme)

	m.TuiTreeModel = m.createSizedTuiModel(tree)
	return m
}

// getIcon returns the appropriate icon for the current terminal
func (m *RenameModel) getIcon(iconType string) string {
	return m.theme.Icon(iconType)
}

func (m *RenameModel) arrowIcons() (string, string) {
	icons := []rune(m.getIcon("arrows"))
	switch {
	case len(icons) >= 4:
		return string(icons[0:2]), string(icons[2:4])
	case len(icons) >= 2:
		return string(icons[0]), string(icons[1:])
	default:
		return "↑↓", "←→"
	}
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

	// Update stats viewport dimensions if initialized
	if m.statsViewport != nil && (m.statsViewport.Width > 0 || m.statsViewport.Height > 0) {
		// Account for border and padding in viewport dimensions
		// Border (2) + padding (2) = 4 total horizontal frame size
		frameWidth := 4
		frameHeight := 4

		viewportWidth := m.statsWidth - frameWidth
		viewportHeight := m.statsHeight - frameHeight

		if viewportWidth < 1 {
			viewportWidth = 1
		}
		if viewportHeight < 1 {
			viewportHeight = 1
		}

		m.statsViewport.Width = viewportWidth
		m.statsViewport.Height = viewportHeight
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
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Toggle between tree and stats panel focus
			m.statsFocused = !m.statsFocused
			return m, nil

		case "delete", "d":
			if focusedNode := m.TuiTreeModel.Tree.GetFocusedNode(); focusedNode != nil {
				retargetFocusAfterRemoval(m.TuiTreeModel.Tree)
				m.removeNodeFromTree(focusedNode)
				m.statsDirty = true
			}
			return m, nil
		case "r":
			if !m.renameInProgress {
				m.renameInProgress = true
				m.renameComplete = false
				m.successCount = 0
				m.errorCount = 0
				m.engine = m.NewOperationEngine()
				m.progress = OperationProgress{
					OverallTotal: m.engine.TotalOperations(),
				}
				m.progressVisible = true
				cmds := []tea.Cmd{m.progressModel.SetPercent(0)}
				cmds = append(cmds, m.engine.ProcessNextCmd())
				return m, tea.Batch(cmds...)
			}
		case "u", "U":
			if m.undoAvailable && !m.undoInProgress {
				m.undoInProgress = true
				m.undoAvailable = false
				m.progressVisible = true
				return m, m.performUndo()
			}
		case "up":
			if m.statsFocused {
				// Scroll stats panel up
				m.statsViewport.ScrollUp(1)
				return m, nil
			}
		case "down":
			if m.statsFocused {
				// Scroll stats panel down
				m.statsViewport.ScrollDown(1)
				return m, nil
			}
		case "pgup":
			if m.statsFocused {
				// Page up in stats panel
				m.statsViewport.HalfPageUp()
				return m, nil
			}
			// Page up - move up by viewport height in tree
			pageSize := max(m.treeHeight, 10)
			m.TuiTreeModel.Tree.Move(context.Background(), -pageSize)
			return m, nil
		case "pgdown":
			if m.statsFocused {
				// Page down in stats panel
				m.statsViewport.HalfPageDown()
				return m, nil
			}
			// Page down - move down by viewport height in tree
			pageSize := max(m.treeHeight, 10)
			m.TuiTreeModel.Tree.Move(context.Background(), pageSize)
			return m, nil
		}

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButton(4): // Mouse wheel up
			if m.statsFocused {
				// Scroll stats panel up
				m.statsViewport.ScrollUp(1)
			} else {
				// Scroll tree up by 1 line
				m.TuiTreeModel.Tree.Move(context.Background(), -1)
			}
			return m, nil
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButton(5): // Mouse wheel down
			if m.statsFocused {
				// Scroll stats panel down
				m.statsViewport.ScrollDown(1)
			} else {
				// Scroll tree down by 1 line
				m.TuiTreeModel.Tree.Move(context.Background(), 1)
			}
			return m, nil
		}

	case MetadataProgressMsg:
		m.metadataFetching = true
		m.metadataTotal = msg.Total
		m.metadataCompleted = msg.Completed
		m.metadataErrors = msg.Errors
		m.metadataStatus = msg.Status
		return m, nil

	case MetadataCompleteMsg:
		m.metadataFetching = false
		m.metadataStatus = ""
		if msg.Errors > 0 {
			m.metadataStatus = fmt.Sprintf("Metadata fetching complete (%d errors)", msg.Errors)
		}
		return m, nil

	case RenameCompleteMsg:
		m.renameInProgress = false
		m.renameComplete = true
		m.successCount = msg.successCount
		m.errorCount = msg.errorCount
		m.statsDirty = true
		m.progressVisible = false
		m.undoAvailable = msg.successCount > 0
		m.engine = nil
		m.progress.SuccessCount = msg.successCount
		m.progress.ErrorCount = msg.errorCount
		m.progress.OverallCompleted = m.progress.OverallTotal
		cmd := m.progressModel.SetPercent(1)
		return m, cmd
	case UndoCompleteMsg:
		m.undoInProgress = false
		m.undoComplete = true
		m.undoSuccess = msg.successCount
		m.undoFailed = msg.errorCount
		m.progressVisible = false
		return m, nil
	case OperationProgressMsg:
		m.progress = msg.Progress
		m.successCount = msg.Progress.SuccessCount
		m.errorCount = msg.Progress.ErrorCount
		m.statsDirty = true
		var pct float64
		if msg.Progress.OverallTotal > 0 {
			pct = math.Min(float64(msg.Progress.OverallCompleted)/float64(msg.Progress.OverallTotal), 1)
		}
		cmds := []tea.Cmd{m.progressModel.SetPercent(pct)}
		if m.engine != nil {
			cmds = append(cmds, m.engine.ProcessNextCmd())
		}
		return m, tea.Batch(cmds...)
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
	style := m.headerStyle().Width(m.width)

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
	// Show metadata fetching progress if active
	if m.metadataFetching {
		textStyle := m.statusBarStyle()
		statusMsg := "Fetching metadata..."
		if m.metadataTotal > 0 {
			statusMsg = fmt.Sprintf("Fetching metadata... (%d/%d)", m.metadataCompleted, m.metadataTotal)
		}
		if m.metadataStatus != "" {
			statusMsg = m.metadataStatus
		}
		return m.statusBarStyle().Width(m.width).Render(textStyle.Render(statusMsg))
	}

	if m.progressVisible && m.renameInProgress {
		// show progress bar with styled text
		bar := m.progressModel.View()
		// Style the text with the same background as the right side of the gradient
		textStyle := m.statusBarStyle()
		operationText := "Renaming..."
		if m.IsLinkMode {
			operationText = "Linking..."
		}
		completed := m.progress.OverallCompleted
		total := m.progress.OverallTotal
		statusText := textStyle.Render(fmt.Sprintf("%d/%d - %s", completed, total, operationText))
		// Combine bar and styled text, then apply the full width style
		combined := fmt.Sprintf("%s  %s", bar, statusText)
		return m.statusBarStyle().Width(m.width - 1).Render(combined)
	}

	if m.progressVisible && m.undoInProgress {
		// show progress bar with undo text
		bar := m.progressModel.View()
		textStyle := m.statusBarStyle()
		statusText := textStyle.Render("Undoing operations...")
		combined := fmt.Sprintf("%s  %s", bar, statusText)
		return m.statusBarStyle().Width(m.width).Render(combined)
	}

	renameKey := "r: Rename"
	if m.IsLinkMode {
		renameKey = "r: Link"
	}

	// Add undo info if available or completed
	undoInfo := ""
	if m.undoAvailable {
		undoInfo = "u: Undo  │  "
	} else if m.undoComplete {
		if m.undoFailed > 0 {
			undoInfo = fmt.Sprintf("Undo: %d success, %d failed  │  ", m.undoSuccess, m.undoFailed)
		} else {
			undoInfo = fmt.Sprintf("Undo: %d operations reversed  │  ", m.undoSuccess)
		}
	}

	focusInfo := ""
	if m.statsFocused {
		focusInfo = "Tab: Tree Focus  │  "
	} else {
		focusInfo = "Tab: Stats Focus  │  "
	}

	upDown, leftRight := m.arrowIcons()
	statusText := fmt.Sprintf("%s%s: Navigate  PgUp/PgDn: Page  %s: Expand/Collapse  │  %s  │  %sd: Remove  │  Esc/Ctrl+C: Quit",
		focusInfo,
		upDown,
		leftRight,
		renameKey,
		undoInfo)
	return m.statusBarStyle().Width(m.width - 1).Render(statusText)
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

// renderStatsPanel builds and formats the statistics panel content using a scrollable viewport.
func (m *RenameModel) renderStatsPanel() string {
	// Update the viewport content when stats are dirty or viewport content is empty
	if m.statsDirty || m.statsViewport.View() == "" {
		m.updateStatsContent()
	}

	// Create border style
	borderStyle := m.panelStyle()

	// Create title with scroll indicator
	titleStyle := m.panelTitleStyle().MarginBottom(1)

	scrollIndicator := ""
	if m.statsViewport.TotalLineCount() > m.statsViewport.Height {
		if m.statsFocused {
			scrollIndicator = " [Use Tab+↑↓]"
		} else {
			scrollIndicator = " [Tab to scroll]"
		}
	}

	title := titleStyle.Render(fmt.Sprintf("%s Statistics%s", m.getIcon("stats"), scrollIndicator))

	// Combine title and viewport
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.statsViewport.View(),
	)

	// Apply the border and sizing
	return borderStyle.
		Width(m.statsWidth - borderStyle.GetHorizontalFrameSize()).
		Height(m.statsHeight - borderStyle.GetVerticalFrameSize()).
		Render(content)
}

// updateStatsContent generates the stats content and sets it in the viewport
func (m *RenameModel) updateStatsContent() {
	stats := m.calculateStats()
	var b strings.Builder
	b.Grow(512)

	// Format stats content with appropriate icons based on terminal capabilities
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
	fmt.Fprintf(&b, "  %s %-14s %d\n", m.getIcon("needrename"), " "+renameLabel, stats.needRenameCount)
	fmt.Fprintf(&b, "  %s %-14s %d\n", m.getIcon("nochange"), " No change:", stats.noChangeCount)
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
		completed := m.progress.OverallCompleted
		total := m.progress.OverallTotal
		if total > 0 {
			percent = (completed * 100) / total
		}
		b.WriteString("\nRename Progress:\n")
		fmt.Fprintf(&b, "  %d/%d (%d%%)\n", completed, total, percent)
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

	m.statsViewport.SetContent(b.String())
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
		if !node.Data().IsDir() && local.IsSubtitle(node.Data().Name()) {
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

func retargetFocusAfterRemoval(tree *treeview.Tree[treeview.FileInfo]) {
	if tree == nil {
		return
	}
	ctx := context.Background()
	if moved, err := tree.Move(ctx, -1); err == nil && moved {
		return
	}
	_, _ = tree.Move(ctx, 1)
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

// performUndo undoes the most recent operation session
func (m *RenameModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		// Get the latest session and undo it
		latestSession, _, err := log.FindLatestSession()
		if err != nil {
			return UndoCompleteMsg{successCount: 0, errorCount: 1}
		}

		successful, failed, _ := log.UndoSession(latestSession)
		return UndoCompleteMsg{successCount: successful, errorCount: failed}
	}
}
