package components

import tea "github.com/charmbracelet/bubbletea"

// Focusable describes components that can gain or lose focus.
type Focusable interface {
	Focus() tea.Cmd
	Blur()
	Focused() bool
}

// Scrollable captures the common scrolling operations supported by viewports.
type Scrollable interface {
	ScrollUp(lines int)
	ScrollDown(lines int)
	HalfPageUp()
	HalfPageDown()
}

// Table models the minimal behaviour expected from tabular widgets used in the TUI.
type Table interface {
	Focusable
	View() string
	RowCount() int
}
