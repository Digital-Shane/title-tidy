package config

import "github.com/charmbracelet/bubbletea"

// sectionModel represents a focusable child model rendered inside the config UI.
type sectionModel interface {
	tea.Model

	Section() Section
	Title() string
	Focus() tea.Cmd
	Blur()
	Resize(width int)
}
