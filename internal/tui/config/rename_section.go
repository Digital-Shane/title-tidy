package config

import (
	"fmt"
	"sort"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type renameSection struct {
	state *RenameState
	theme theme.Theme
	icons map[string]string
	width int
}

func newRenameSection(state *RenameState, th theme.Theme) *renameSection {
	r := &renameSection{state: state, theme: th, icons: th.IconSet()}
	r.Resize(64)
	return r
}

func (r *renameSection) Init() tea.Cmd { return nil }

func (r *renameSection) Section() Section { return SectionRename }

func (r *renameSection) Title() string { return "Rename" }

func (r *renameSection) Focus() tea.Cmd {
	if input := r.activeInput(); input != nil {
		return input.Focus()
	}
	return nil
}

func (r *renameSection) Blur() {
	for i := range r.state.Replacements {
		r.state.Replacements[i].Search.Blur()
		r.state.Replacements[i].Replacement.Blur()
	}
}

func (r *renameSection) Resize(width int) {
	r.width = width
	for i := range r.state.Replacements {
		r.state.Replacements[i].Search.SetWidth(max(width-10, 1))
		r.state.Replacements[i].Replacement.SetWidth(max(width-10, 1))
	}
}

func (r *renameSection) activeInput() *textinput.Model {
	if r.state.Focus <= 0 || r.state.Focus > 2*len(r.state.Replacements) {
		return nil
	}
	pair := &r.state.Replacements[(r.state.Focus-1)/2]
	if r.state.Focus%2 == 1 {
		return &pair.Search
	}
	return &pair.Replacement
}

func (r *renameSection) moveFocus(delta int) tea.Cmd {
	r.Blur()
	count := 1 + 2*len(r.state.Replacements)
	r.state.Focus = (r.state.Focus + delta + count) % count
	return r.Focus()
}

func (r *renameSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "up":
			return r, r.moveFocus(-1)
		case "down":
			return r, r.moveFocus(1)
		case "ctrl+n":
			r.Blur()
			r.state.Replacements = append(r.state.Replacements, newFilenameReplacement("", "", r.theme))
			r.Resize(r.width)
			r.state.Focus = 2*len(r.state.Replacements) - 1
			return r, r.Focus()
		case "ctrl+d":
			if r.activeInput() != nil {
				r.Blur()
				index := (r.state.Focus - 1) / 2
				r.state.Replacements = append(r.state.Replacements[:index], r.state.Replacements[index+1:]...)
				r.state.Focus = min(r.state.Focus, 2*len(r.state.Replacements))
				return r, r.Focus()
			}
		case "enter", "space":
			if r.state.Focus == 0 {
				r.state.PreserveExistingTags = !r.state.PreserveExistingTags
				return r, nil
			}
			if key.String() == "enter" {
				return r, r.moveFocus(1)
			}
		}
	}
	if input := r.activeInput(); input != nil {
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		return r, cmd
	}

	return r, nil
}

func (r *renameSection) View() tea.View {
	colors := r.theme.Colors()

	toggleIcon := "[ ]"
	if r.state.PreserveExistingTags {
		toggleIcon = "[" + r.icons["check"] + "]"
	}

	toggleStyle := lipgloss.NewStyle().Foreground(colors.Error)
	if r.state.PreserveExistingTags {
		toggleStyle = lipgloss.NewStyle().Foreground(colors.Success)
	}

	help := lipgloss.NewStyle().Foreground(colors.Muted).Width(max(r.width, 1))

	focused := lipgloss.NewStyle().Background(colors.Accent).Foreground(colors.Background)
	toggle := toggleStyle.Render(toggleIcon + " Preserve Existing Tags")
	if r.state.Focus == 0 {
		toggle = focused.Render(toggleIcon + " Preserve Existing Tags")
	}
	rows := []string{
		r.theme.PanelTitleStyle().Render("Rename Behavior"),
		toggle,
		help.Render("Keep bracketed tags from source names (for example [Uncut])."),
		"",
		r.theme.PanelTitleStyle().Render("Filename Replacements"),
	}
	if len(r.state.Replacements) == 0 {
		rows = append(rows, help.Render("No replacements (disabled). Press Ctrl+N to add a pair."))
	} else {
		// Keep the active pair visible without growing the panel with the config.
		first := max(0, (r.state.Focus-1)/2-2)
		last := min(first+3, len(r.state.Replacements))
		rows = append(rows, help.Render(fmt.Sprintf("Pairs %d–%d of %d", first+1, last, len(r.state.Replacements))))
		for i := first; i < last; i++ {
			pair := &r.state.Replacements[i]
			for j, field := range []struct {
				label string
				input *textinput.Model
			}{{"Find:    ", &pair.Search}, {"Replace: ", &pair.Replacement}} {
				value := fmt.Sprintf("%q", field.input.Value())
				if r.state.Focus == 1+2*i+j {
					value = field.input.View()
				} else {
					value = ansi.Truncate(value, max(r.width-10, 1), "…")
				}
				rows = append(rows, field.label+value)
			}
		}
	}
	rows = append(rows, help.Render("↑/↓ select · Enter next · Ctrl+N add · Ctrl+D remove pair"),
		help.Render("Literal, case-sensitive text. An empty replacement deletes matches."))
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func newFilenameReplacement(search, replacement string, th theme.Theme) FilenameReplacementState {
	searchInput := newTemplateInput(search, th)
	searchInput.Placeholder = "Search text"
	replacementInput := newTemplateInput(replacement, th)
	replacementInput.Placeholder = "(empty = delete)"
	return FilenameReplacementState{
		Search:      searchInput,
		Replacement: replacementInput,
	}
}

func buildRenameState(cfg *config.FormatConfig, th theme.Theme) RenameState {
	state := RenameState{PreserveExistingTags: cfg.PreserveExistingTags}
	keys := make([]string, 0, len(cfg.FilenameReplacements))
	for key := range cfg.FilenameReplacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		state.Replacements = append(state.Replacements, newFilenameReplacement(key, cfg.FilenameReplacements[key], th))
	}
	return state
}

func (r *RenameState) filenameReplacements() (map[string]string, error) {
	values := make(map[string]string, len(r.Replacements))
	for _, pair := range r.Replacements {
		key := pair.Search.Value()
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("filename_replacements: duplicate search text %q", key)
		}
		values[key] = pair.Replacement.Value()
	}
	if err := config.ValidateFilenameReplacements(values); err != nil {
		return nil, err
	}
	return values, nil
}
