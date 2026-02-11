package config

import (
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type renameSection struct {
	state *RenameState
	theme theme.Theme
	icons map[string]string
}

func newRenameSection(state *RenameState, th theme.Theme) *renameSection {
	return &renameSection{state: state, theme: th, icons: th.IconSet()}
}

func (r *renameSection) Init() tea.Cmd { return nil }

func (r *renameSection) Section() Section { return SectionRename }

func (r *renameSection) Title() string { return "Rename" }

func (r *renameSection) Focus() tea.Cmd { return nil }

func (r *renameSection) Blur() {}

func (r *renameSection) Resize(width int) {}

func (r *renameSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEnter, tea.KeySpace:
			r.state.PreserveExistingTags = !r.state.PreserveExistingTags
		}
	}

	return r, nil
}

func (r *renameSection) View() string {
	colors := r.theme.Colors()

	toggleIcon := "[ ]"
	if r.state.PreserveExistingTags {
		toggleIcon = "[" + r.icons["check"] + "]"
	}

	toggleStyle := lipgloss.NewStyle().Foreground(colors.Error)
	if r.state.PreserveExistingTags {
		toggleStyle = lipgloss.NewStyle().Foreground(colors.Success)
	}

	help := lipgloss.NewStyle().Foreground(colors.Muted)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		r.theme.PanelTitleStyle().Render("Rename Behavior"),
		toggleStyle.Render(toggleIcon+" Preserve Existing Tags"),
		help.Render("Keep existing bracketed tags from source filenames (for example [Uncut]) when generating new movie names."),
		help.Render("Use Space or Enter to toggle."),
	)
}
