package config

import (
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type templateSection struct {
	state *TemplateSectionState
	theme theme.Theme
	width int
}

func newTemplateSection(state *TemplateSectionState, th theme.Theme) *templateSection {
	return &templateSection{state: state, theme: th}
}

func (t *templateSection) Init() tea.Cmd { return nil }

func (t *templateSection) Section() Section { return t.state.Section }

func (t *templateSection) Title() string { return t.state.Title }

func (t *templateSection) Focus() tea.Cmd {
	return t.state.Input.Focus()
}

func (t *templateSection) Blur() {
	t.state.Input.Blur()
}

func (t *templateSection) Resize(width int) {
	t.width = width
	if width > 0 {
		t.state.Input.Width = width
	}
}

func (t *templateSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+delete" {
			t.state.Input.SetValue("")
			return t, nil
		}
		if key.Type == tea.KeySpace {
			key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
			var cmd tea.Cmd
			t.state.Input, cmd = t.state.Input.Update(key)
			return t, cmd
		}
	}

	var cmd tea.Cmd
	t.state.Input, cmd = t.state.Input.Update(msg)
	return t, cmd
}

func (t *templateSection) View() string {
	label := t.theme.PanelTitleStyle().Render("Format Template:")
	return lipgloss.JoinVertical(lipgloss.Left, label, t.state.Input.View())
}
