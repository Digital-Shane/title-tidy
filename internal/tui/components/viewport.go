package components

import (
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// NewViewport constructs a themed viewport with optional overrides.
func NewViewport(width, height int, th theme.Theme) *viewport.Model {
	vp := viewport.New(width, height)
	baseStyle := th.PanelStyle().
		BorderStyle(lipgloss.Border{}).
		BorderForeground(lipgloss.Color(""))
	vp.Style = baseStyle
	return &vp
}
