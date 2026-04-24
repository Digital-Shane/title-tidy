package components

import (
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// NewViewport constructs a themed viewport with optional overrides.
func NewViewport(width, height int, th theme.Theme) *viewport.Model {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	baseStyle := th.PanelStyle().
		BorderStyle(lipgloss.Border{}).
		BorderForeground(lipgloss.Color(""))
	vp.Style = baseStyle
	return &vp
}
