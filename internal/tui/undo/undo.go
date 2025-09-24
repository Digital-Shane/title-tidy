package undo

import (
	"fmt"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var undoSessionFn = log.UndoSession

// UndoCompleteMsg is emitted when undo operation completes.
type UndoCompleteMsg struct{ successCount, errorCount int }

func (u UndoCompleteMsg) SuccessCount() int { return u.successCount }

func (u UndoCompleteMsg) ErrorCount() int { return u.errorCount }

// UndoModel represents the TUI for selecting and undoing operations
type UndoModel struct {
	*treeview.TuiTreeModel[log.SessionSummary]
	confirmingUndo bool
	undoInProgress bool
	undoComplete   bool
	undoSuccess    int
	undoFailed     int
	width          int
	height         int
	splitRatio     float64 // ratio for left/right split
	theme          theme.Theme

	// Session details scrolling
	detailsViewport *viewport.Model
	detailsFocused  bool // whether the details panel is focused for scrolling
}

// Option configures an UndoModel during construction.
type Option func(*UndoModel)

// WithTheme overrides the default theme for the undo TUI.
func WithTheme(th theme.Theme) Option {
	return func(m *UndoModel) {
		m.theme = th
	}
}

func (m *UndoModel) headerStyle() lipgloss.Style {
	return m.theme.HeaderStyle()
}

func (m *UndoModel) statusBarStyle() lipgloss.Style {
	return m.theme.StatusBarStyle()
}

func (m *UndoModel) panelStyle() lipgloss.Style {
	return m.theme.PanelStyle()
}

func (m *UndoModel) colors() theme.Colors {
	return m.theme.Colors()
}

func (m *UndoModel) sizedPanel(width, height int, borderColor lipgloss.Color) lipgloss.Style {
	style := m.panelStyle()
	if borderColor != "" {
		style = style.BorderForeground(borderColor)
	}
	if width > 0 {
		contentWidth := width - style.GetHorizontalFrameSize()
		if contentWidth < 0 {
			contentWidth = 0
		}
		style = style.Width(contentWidth)
	}
	if height > 0 {
		contentHeight := height - style.GetVerticalFrameSize()
		if contentHeight < 0 {
			contentHeight = 0
		}
		style = style.Height(contentHeight)
	}
	return style.Padding(0, 1)
}

// NewUndoModel creates a new undo selection model
func NewUndoModel(tree *treeview.Tree[log.SessionSummary], opts ...Option) *UndoModel {
	m := &UndoModel{
		width:      80,
		height:     24,
		splitRatio: 0.5, // 50/50 split by default
	}

	initOpts := append([]Option{WithTheme(theme.Default())}, opts...)
	for _, opt := range initOpts {
		opt(m)
	}

	// Create underlying TUI model with same pattern as RenameModel
	keyMap := treeview.DefaultKeyMap()
	keyMap.SearchStart = []string{} // Disable search
	keyMap.Reset = []string{}       // Disable reset

	// Use half width for the tree view initially
	treeWidth := int(float64(m.width)*m.splitRatio) - 2
	m.TuiTreeModel = treeview.NewTuiTreeModel(tree,
		treeview.WithTuiWidth[log.SessionSummary](treeWidth),
		treeview.WithTuiHeight[log.SessionSummary](m.height-4),
		treeview.WithTuiAllowResize[log.SessionSummary](true),
		treeview.WithTuiDisableNavBar[log.SessionSummary](true),
		treeview.WithTuiKeyMap[log.SessionSummary](keyMap),
	)

	// Initialize details viewport
	rightWidth := m.width - treeWidth
	viewportHeight := m.height - 4 - 4 // Account for header, borders, and instructions
	m.detailsViewport = components.NewViewport(rightWidth-6, viewportHeight, m.theme)

	return m
}

func (m *UndoModel) Init() tea.Cmd {
	return nil
}

func (m *UndoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update the tree width to account for split
		treeWidth := int(float64(m.width)*m.splitRatio) - 2
		resizeMsg := tea.WindowSizeMsg{
			Width:  treeWidth,
			Height: m.height - 4,
		}
		treeModel, cmd := m.TuiTreeModel.Update(resizeMsg)
		m.TuiTreeModel = treeModel.(*treeview.TuiTreeModel[log.SessionSummary])

		// Update details viewport dimensions
		rightWidth := m.width - treeWidth
		viewportHeight := m.height - 4 - 4       // Account for header, borders, and instructions
		m.detailsViewport.Width = rightWidth - 6 // Account for border and padding
		m.detailsViewport.Height = viewportHeight

		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit

		case "tab":
			// Toggle between session list and details panel focus
			m.detailsFocused = !m.detailsFocused
			return m, nil

		case "up":
			if m.detailsFocused {
				// Scroll details panel up
				m.detailsViewport.ScrollUp(1)
				return m, nil
			}

		case "down":
			if m.detailsFocused {
				// Scroll details panel down
				m.detailsViewport.ScrollDown(1)
				return m, nil
			}

		case "pgup":
			if m.detailsFocused {
				// Page up in details panel
				m.detailsViewport.HalfPageUp()
				return m, nil
			}

		case "pgdown":
			if m.detailsFocused {
				// Page down in details panel
				m.detailsViewport.HalfPageDown()
				return m, nil
			}

		case "enter":
			if m.confirmingUndo {
				// Execute the undo
				focusedNode := m.TuiTreeModel.Tree.GetFocusedNode()
				if focusedNode != nil {
					m.undoInProgress = true
					m.confirmingUndo = false
					return m, m.performUndo(*focusedNode.Data())
				}
			} else if !m.undoInProgress {
				// Toggle confirmation for the selected session
				m.confirmingUndo = true
			}
			return m, nil

		case "n", "N":
			if m.confirmingUndo {
				m.confirmingUndo = false
			}
			return m, nil
		}

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButton(4): // Mouse wheel up
			if m.detailsFocused {
				// Scroll details panel up
				m.detailsViewport.ScrollUp(1)
			}
			// If tree is focused, let it handle the mouse wheel in the default handler below
			return m, nil
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButton(5): // Mouse wheel down
			if m.detailsFocused {
				// Scroll details panel down
				m.detailsViewport.ScrollDown(1)
			}
			// If tree is focused, let it handle the mouse wheel in the default handler below
			return m, nil
		}

	case UndoCompleteMsg:
		m.undoInProgress = false
		m.undoComplete = true
		m.undoSuccess = msg.successCount
		m.undoFailed = msg.errorCount
		return m, nil
	}

	// Pass other messages to the tree model if not in special states and tree is focused
	if !m.confirmingUndo && !m.undoInProgress && !m.detailsFocused {
		treeModel, cmd := m.TuiTreeModel.Update(msg)
		m.TuiTreeModel = treeModel.(*treeview.TuiTreeModel[log.SessionSummary])
		return m, cmd
	}

	return m, nil
}

func (m *UndoModel) View() string {
	var b strings.Builder

	// Header
	header := m.headerStyle().Width(m.width).Render("Title-Tidy Undo Sessions")

	b.WriteString(header)
	b.WriteByte('\n')

	if m.undoComplete {
		// Show undo results
		resultText := fmt.Sprintf("Undo completed: %d operations reversed", m.undoSuccess)
		if m.undoFailed > 0 {
			resultText = fmt.Sprintf("Undo completed: %d success, %d failed", m.undoSuccess, m.undoFailed)
		}

		result := m.statusBarStyle().Width(m.width).Render(resultText)

		b.WriteString(result)
		b.WriteByte('\n')

		statusText := "Press 'Ctrl+C' or 'esc' to exit"
		status := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Foreground(m.colors().Muted).
			Render(statusText)
		b.WriteString(status)

	} else if m.undoInProgress {
		// Show undo in progress
		progressText := "Undoing operations..."
		progress := m.statusBarStyle().Width(m.width).Render(progressText)

		b.WriteString(progress)
		b.WriteByte('\n')

	} else if m.confirmingUndo {
		// Show confirmation dialog
		focusedNode := m.TuiTreeModel.Tree.GetFocusedNode()
		if focusedNode != nil {
			summary := *focusedNode.Data()
			confirmView := m.renderConfirmation(summary)
			b.WriteString(confirmView)
		}

	} else {
		// Show session list with sidebar
		b.WriteString(m.renderMainView())
	}

	return b.String()
}

// renderMainView renders the split view with session list and preview
func (m *UndoModel) renderMainView() string {
	// Calculate widths
	leftWidth := int(float64(m.width) * m.splitRatio)
	rightWidth := m.width - leftWidth

	// Left panel - session list
	leftPanel := m.renderSessionList(leftWidth, m.height-3)

	// Right panel - session preview
	rightPanel := m.renderSessionPreview(rightWidth, m.height-3)

	// Combine panels side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Add instructions at the bottom
	focusInfo := ""
	if m.detailsFocused {
		focusInfo = "Tab: List Focus | "
	} else {
		focusInfo = "Tab: Details Focus | "
	}

	instruction := focusInfo + "↑↓ Navigate | PgUp/PgDn: Page | Enter: Select | Esc/Ctrl+C: Quit"
	instructionStyle := lipgloss.NewStyle().
		Italic(true).
		Width(m.width).
		Align(lipgloss.Center).
		Foreground(m.colors().Muted).
		Render(instruction)

	return content + "\n" + instructionStyle
}

// renderSessionList renders the left panel with the session tree
func (m *UndoModel) renderSessionList(width, height int) string {
	colors := m.colors()
	borderStyle := m.sizedPanel(width, height, colors.Primary)
	titleWidth := width - 4
	if titleWidth < 0 {
		titleWidth = width
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colors.Primary).
		Width(titleWidth).
		Align(lipgloss.Center).
		Render("Sessions")

	// Get tree view
	treeView := m.TuiTreeModel.View()

	// Combine title and tree
	content := title + "\n" + treeView

	return borderStyle.Render(content)
}

// renderSessionPreview renders the right panel with session details using a scrollable viewport
func (m *UndoModel) renderSessionPreview(width, height int) string {
	// Update viewport content when selection changes
	focusedNode := m.TuiTreeModel.Tree.GetFocusedNode()
	if focusedNode != nil {
		summary := *focusedNode.Data()
		content := m.formatSessionDetails(summary, m.detailsViewport.Width)
		m.detailsViewport.SetContent(content)
	} else {
		emptyContent := lipgloss.NewStyle().
			Italic(true).
			Foreground(m.colors().Muted).
			Render("Select a session to view details")
		m.detailsViewport.SetContent(emptyContent)
	}

	colors := m.colors()
	borderStyle := m.sizedPanel(width, height, colors.Secondary)

	titleWidth := width - 4
	if titleWidth < 0 {
		titleWidth = width
	}
	// Create title with scroll indicator
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colors.Secondary).
		Width(titleWidth).
		Align(lipgloss.Center)

	scrollIndicator := ""
	if m.detailsViewport.TotalLineCount() > m.detailsViewport.Height {
		if m.detailsFocused {
			scrollIndicator = " [Use Tab+↑↓]"
		} else {
			scrollIndicator = " [Tab to scroll]"
		}
	}

	title := titleStyle.Render("Session Details" + scrollIndicator)

	// Combine title and viewport
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"", // Empty line separator
		m.detailsViewport.View(),
	)

	return borderStyle.Render(fullContent)
}

// formatSessionDetails formats detailed information about a session
func (m *UndoModel) formatSessionDetails(summary log.SessionSummary, width int) string {
	var b strings.Builder
	session := summary.Session
	colors := m.colors()

	// Style for labels
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colors.Accent)

	// Style for values
	valueStyle := lipgloss.NewStyle().
		Foreground(colors.Primary)

	// Command
	b.WriteString(labelStyle.Render("Command: "))
	b.WriteString(valueStyle.Render(strings.Join(session.Metadata.CommandArgs, " ")))
	b.WriteString("\n\n")

	// Timestamp
	b.WriteString(labelStyle.Render("Time: "))
	b.WriteString(valueStyle.Render(summary.RelativeTime))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Date: "))
	b.WriteString(valueStyle.Render(session.Metadata.Timestamp.Format("2006-01-02 15:04:05")))
	b.WriteString("\n\n")

	// Working directory
	b.WriteString(labelStyle.Render("Directory: "))
	workDir := session.Metadata.WorkingDir
	if len(workDir) > width-12 {
		workDir = "..." + workDir[len(workDir)-(width-15):]
	}
	b.WriteString(valueStyle.Render(workDir))
	b.WriteString("\n\n")

	// Operation statistics
	b.WriteString(labelStyle.Render("Operations:"))
	b.WriteString("\n")

	statsStyle := lipgloss.NewStyle().
		MarginLeft(2)

	stats := fmt.Sprintf("Total: %d\nSuccessful: %d\nFailed: %d",
		session.Metadata.TotalOps,
		session.Metadata.SuccessfulOps,
		session.Metadata.FailedOps)
	b.WriteString(statsStyle.Render(valueStyle.Render(stats)))
	b.WriteString("\n\n")

	// Recent operations preview
	if len(session.Operations) > 0 {
		b.WriteString(labelStyle.Render("Recent Operations:"))
		b.WriteString("\n")

		// Show up to 5 recent operations
		opCount := len(session.Operations)
		startIdx := 0
		if opCount > 5 {
			startIdx = opCount - 5
		}

		for i := startIdx; i < opCount; i++ {
			op := session.Operations[i]
			opIcon := m.getOperationIcon(op)
			opText := m.formatOperation(op, width-6)

			b.WriteString(statsStyle.Render(fmt.Sprintf("%s %s", opIcon, opText)))
			b.WriteString("\n")
		}
	}

	// Session ID
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Session ID: "))
	b.WriteString(lipgloss.NewStyle().
		Foreground(colors.Muted).
		Italic(true).
		Render(session.Metadata.SessionID))

	return b.String()
}

// getOperationIcon returns an icon for the operation type
func (m *UndoModel) getOperationIcon(op log.OperationLog) string {
	switch op.Type {
	case log.OpRename:
		if op.Success {
			return m.getIcon("check")
		}
		return m.getIcon("error")
	case log.OpLink:
		return m.getIcon("link")
	case log.OpDelete:
		return m.getIcon("delete")
	case log.OpCreateDir:
		return m.getIcon("folder")
	default:
		return m.getIcon("unknown")
	}
}

func (m *UndoModel) getIcon(iconType string) string {
	return m.theme.Icon(iconType)
}

// formatOperation formats a single operation for display
func (m *UndoModel) formatOperation(op log.OperationLog, maxWidth int) string {
	var text string

	switch op.Type {
	case log.OpRename:
		// Show just the filename, not full path
		source := op.SourcePath
		if idx := strings.LastIndex(source, "/"); idx >= 0 {
			source = source[idx+1:]
		}
		dest := op.DestPath
		if idx := strings.LastIndex(dest, "/"); idx >= 0 {
			dest = dest[idx+1:]
		}
		text = fmt.Sprintf("%s → %s", source, dest)

	case log.OpLink:
		source := op.SourcePath
		if idx := strings.LastIndex(source, "/"); idx >= 0 {
			source = source[idx+1:]
		}
		text = fmt.Sprintf("Link: %s", source)

	case log.OpDelete:
		source := op.SourcePath
		if idx := strings.LastIndex(source, "/"); idx >= 0 {
			source = source[idx+1:]
		}
		text = fmt.Sprintf("Delete: %s", source)

	case log.OpCreateDir:
		dest := op.DestPath
		if idx := strings.LastIndex(dest, "/"); idx >= 0 {
			dest = dest[idx+1:]
		}
		text = fmt.Sprintf("Create: %s/", dest)

	default:
		text = string(op.Type)
	}

	// Truncate if too long
	if len(text) > maxWidth {
		text = text[:maxWidth-3] + "..."
	}

	// Add error indicator if failed
	if !op.Success && op.Error != "" {
		text += " (failed)"
	}

	return text
}

// renderConfirmation renders the confirmation dialog
func (m *UndoModel) renderConfirmation(summary log.SessionSummary) string {
	session := summary.Session

	// Create confirmation box
	colors := m.colors()
	confirmStyle := m.panelStyle().
		BorderForeground(colors.Accent).
		Padding(1, 2).
		Width(60).
		Align(lipgloss.Center).
		Background(colors.Background)

	confirmText := fmt.Sprintf(
		"Confirm Undo Operation\n\n"+
			"Session: %s\n"+
			"Time: %s\n"+
			"Operations: %d (Success: %d, Failed: %d)\n"+
			"Directory: %s\n\n"+
			"This will reverse all successful operations.\n\n"+
			"Press ENTER to confirm or 'n' to cancel",
		session.Metadata.CommandArgs[0],
		summary.RelativeTime,
		session.Metadata.TotalOps,
		session.Metadata.SuccessfulOps,
		session.Metadata.FailedOps,
		session.Metadata.WorkingDir)

	// Center the confirmation box
	centerStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-2).
		Align(lipgloss.Center, lipgloss.Center)

	return centerStyle.Render(confirmStyle.Render(confirmText))
}

func (m *UndoModel) performUndo(summary log.SessionSummary) tea.Cmd {
	return func() tea.Msg {
		successful, failed, _ := undoSessionFn(summary.Session)
		return UndoCompleteMsg{successCount: successful, errorCount: failed}
	}
}
