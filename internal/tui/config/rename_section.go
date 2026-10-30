package config

import (
	"fmt"

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
	if r.state.Focus < 2 || r.state.Focus >= 2+2*len(r.state.Replacements) {
		return nil
	}
	pair := &r.state.Replacements[(r.state.Focus-2)/2]
	if r.state.Focus%2 == 0 {
		return &pair.Search
	}
	return &pair.Replacement
}

func (r *renameSection) moveFocus(delta int) tea.Cmd {
	r.Blur()
	count := 2 + 2*len(r.state.Replacements)
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
		case "ctrl+up", "ctrl+down":
			if r.activeInput() != nil {
				index := (r.state.Focus - 2) / 2
				delta := 1
				if key.String() == "ctrl+up" {
					delta = -1
				}
				next := index + delta
				if next >= 0 && next < len(r.state.Replacements) {
					r.Blur()
					r.state.Replacements[index], r.state.Replacements[next] = r.state.Replacements[next], r.state.Replacements[index]
					r.state.Focus += 2 * delta
					return r, r.Focus()
				}
			}
			return r, nil
		case "ctrl+n":
			r.Blur()
			r.state.Replacements = append(r.state.Replacements, newFilenameReplacement("", "", r.theme))
			r.Resize(r.width)
			r.state.Replacements[len(r.state.Replacements)-1].Regex = r.state.RegexReplacements
			r.state.Focus = 2 * len(r.state.Replacements)
			return r, r.Focus()
		case "ctrl+d":
			if r.activeInput() != nil {
				r.Blur()
				index := (r.state.Focus - 2) / 2
				r.state.Replacements = append(r.state.Replacements[:index], r.state.Replacements[index+1:]...)
				r.state.Focus = min(r.state.Focus, 1+2*len(r.state.Replacements))
				return r, r.Focus()
			}
		case "enter", "space":
			if r.state.Focus == 0 {
				r.state.PreserveExistingTags = !r.state.PreserveExistingTags
				return r, nil
			}
			if r.state.Focus == 1 {
				r.toggleRegex()
				return r, nil
			}
			if key.String() == "enter" {
				return r, r.moveFocus(1)
			}
		}
	}
	if input := r.activeInput(); input != nil {
		before := input.Value()
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		if r.state.Focus%2 == 0 && input.Value() != before {
			pair := &r.state.Replacements[(r.state.Focus-2)/2]
			pair.searchSource = input.Value()
			pair.searchDisplay = input.Value()
		}
		return r, cmd
	}

	return r, nil
}

func (r *renameSection) toggleRegex() {
	r.state.RegexReplacements = !r.state.RegexReplacements
	for i := range r.state.Replacements {
		pair := &r.state.Replacements[i]
		rule := pair.rule()
		if r.state.RegexReplacements {
			rule = rule.AsRegex()
		} else {
			rule = rule.AsLiteral()
		}
		pair.setSearchText(rule.Search)
		pair.Replacement.SetValue(rule.Replacement)
		pair.Regex = rule.Regex
	}
}

func (r *renameSection) View() tea.View {
	colors := r.theme.Colors()
	help := lipgloss.NewStyle().Foreground(colors.Muted).Width(max(r.width, 1))
	errorStyle := lipgloss.NewStyle().Foreground(colors.Error)
	toggle := func(label string, enabled bool, focus int) string {
		icon := "[ ]"
		style := lipgloss.NewStyle().Foreground(colors.Muted)
		if enabled {
			icon = "[" + r.icons["check"] + "]"
			style = style.Foreground(colors.Success)
		}
		if r.state.Focus == focus {
			style = style.Background(colors.Accent).Foreground(colors.Background)
		}
		return style.Render(icon + " " + label)
	}
	rows := []string{
		r.theme.PanelTitleStyle().Render("Rename Behavior"),
		toggle("Preserve Existing Tags", r.state.PreserveExistingTags, 0),
		help.Render("Keep bracketed tags from source names (for example [Uncut])."),
		"",
		r.theme.PanelTitleStyle().Render("Filename Replacements"),
		toggle("Use Go Regular Expressions", r.state.RegexReplacements, 1),
	}
	if len(r.state.Replacements) == 0 {
		rows = append(rows, help.Render("No replacements (disabled). Press Ctrl+N to add a pair."))
	} else {
		first := max(0, (r.state.Focus-2)/2-1)
		last := min(first+2, len(r.state.Replacements))
		rows = append(rows, help.Render(fmt.Sprintf("Pairs %d–%d of %d", first+1, last, len(r.state.Replacements))))
		for i := first; i < last; i++ {
			pair := &r.state.Replacements[i]
			rule := pair.rule()
			err := config.ValidateFilenameReplacement(rule, r.state.RegexReplacements)
			for j, field := range []struct {
				label string
				input *textinput.Model
			}{{fmt.Sprintf("%d Find:  ", i+1), &pair.Search}, {"Replace: ", &pair.Replacement}} {
				label := field.label
				text := field.input.Value()
				if j == 0 {
					text = rule.Search
				}
				value := fmt.Sprintf("%q", text)
				if r.state.Focus == 2+2*i+j {
					input := *field.input
					if err != nil {
						styles := input.Styles()
						styles.Focused.Text = errorStyle
						input.SetStyles(styles)
					}
					value = input.View()
				} else {
					value = ansi.Truncate(value, max(r.width-10, 1), "…")
				}
				if err != nil {
					label = errorStyle.Render(label)
					value = errorStyle.Render(value)
				}
				rows = append(rows, label+value)
			}
			if r.state.Focus == 2+2*i && rule.Search != pair.Search.Value() {
				rows = append(rows, help.Render(ansi.Truncate(fmt.Sprintf("Saved search: %q (retained until edited)", rule.Search), max(r.width, 1), "…")))
			}
			if err != nil && (r.state.Focus-2)/2 == i && r.state.Focus >= 2 {
				rows = append(rows, errorStyle.Render(ansi.Truncate(err.Error(), max(r.width, 1), "…")))
			} else if pair.Regex && !r.state.RegexReplacements {
				rows = append(rows, help.Render("Regex paused in literal mode."))
			}
		}
	}
	rows = append(rows, help.Render("Ctrl+N add · Ctrl+D remove · Ctrl+↑/↓ reorder"))
	if r.state.RegexReplacements {
		rows = append(rows, help.Render("Regex source preprocessing. Captures: $1 / ${name}."))
	} else {
		rows = append(rows, help.Render("Literal final names; complex regex rules paused."))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func newFilenameReplacement(search, replacement string, th theme.Theme) FilenameReplacementState {
	searchInput := newTemplateInput(search, th)
	searchInput.Placeholder = "Search text"
	replacementInput := newTemplateInput(replacement, th)
	replacementInput.Placeholder = "(empty = delete)"
	return FilenameReplacementState{
		Search:        searchInput,
		Replacement:   replacementInput,
		searchSource:  search,
		searchDisplay: searchInput.Value(),
	}
}

// rule returns source text while its sanitized display is unedited. An actual
// edit adopts the text the user entered, including after deleting and retyping it.
func (pair *FilenameReplacementState) rule() config.FilenameReplacement {
	search := pair.Search.Value()
	if search == pair.searchDisplay {
		search = pair.searchSource
	}
	return config.FilenameReplacement{Search: search, Replacement: pair.Replacement.Value(), Regex: pair.Regex}
}

func (pair *FilenameReplacementState) setSearchText(search string) {
	pair.Search.SetValue(search)
	pair.searchSource = search
	pair.searchDisplay = pair.Search.Value()
}

func buildRenameState(cfg *config.FormatConfig, th theme.Theme) RenameState {
	state := RenameState{PreserveExistingTags: cfg.PreserveExistingTags, RegexReplacements: cfg.FilenameReplacementsRegex}
	for _, rule := range cfg.FilenameReplacements {
		pair := newFilenameReplacement(rule.Search, rule.Replacement, th)
		pair.Regex = rule.Regex || cfg.FilenameReplacementsRegex
		state.Replacements = append(state.Replacements, pair)
	}
	return state
}

func (r *RenameState) filenameReplacements() (config.FilenameReplacements, error) {
	values := make(config.FilenameReplacements, 0, len(r.Replacements))
	for _, pair := range r.Replacements {
		values = append(values, pair.rule())
	}
	if err := config.ValidateFilenameReplacements(values, r.RegexReplacements); err != nil {
		return nil, err
	}
	return values, nil
}
