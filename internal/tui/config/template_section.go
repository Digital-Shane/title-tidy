package config

import (
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		t.state.Input.SetWidth(width)
	}
}

func (t *templateSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "ctrl+delete" {
			t.state.Input.SetValue("")
			return t, nil
		}
		if key.String() == "space" {
			key = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		}
		if key.Text != "" {
			filtered := filterInvalidFilenameRunes([]rune(key.Text))
			if len(filtered) == 0 {
				return t, nil
			}
			key = tea.KeyPressMsg{Code: filtered[0], Text: string(filtered)}
			var cmd tea.Cmd
			t.state.Input, cmd = t.state.Input.Update(key)
			t.state.Input.SetValue(sanitizeTemplateValue(t.state.Input.Value()))
			return t, cmd
		}
	}

	var cmd tea.Cmd
	t.state.Input, cmd = t.state.Input.Update(msg)
	t.state.Input.SetValue(sanitizeTemplateValue(t.state.Input.Value()))
	return t, cmd
}

func (t *templateSection) View() tea.View {
	label := t.theme.PanelTitleStyle().Render("Format Template:")
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, label, t.state.Input.View()))
}

func filterInvalidFilenameRunes(runes []rune) []rune {
	if len(runes) == 0 {
		return runes
	}

	filtered := runes[:0]
	for _, r := range runes {
		if r < 32 || r == 127 {
			continue
		}
		if strings.ContainsRune(invalidFilenameChars, r) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func sanitizeTemplateValue(value string) string {
	runes := []rune(value)
	result := make([]rune, 0, len(runes))

	for i, r := range runes {
		if r != '\\' {
			result = append(result, r)
			continue
		}

		if i == len(runes)-1 {
			result = append(result, r)
			continue
		}

		next := runes[i+1]
		if next == '{' || next == '}' {
			result = append(result, r)
		}
	}

	return string(result)
}

const invalidFilenameChars = "<>:\"/|?*"
