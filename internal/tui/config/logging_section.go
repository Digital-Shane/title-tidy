package config

import (
	"unicode"

	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type loggingSection struct {
	state *LoggingState
	theme theme.Theme
	icons map[string]string
	width int
}

func newLoggingSection(state *LoggingState, th theme.Theme) *loggingSection {
	return &loggingSection{state: state, theme: th, icons: th.IconSet()}
}

func (l *loggingSection) Init() tea.Cmd { return nil }

func (l *loggingSection) Section() Section { return SectionLogging }

func (l *loggingSection) Title() string { return "Logging" }

func (l *loggingSection) Focus() tea.Cmd {
	if l.state.Focus == LoggingFieldRetention {
		return l.state.Retention.Focus()
	}
	l.state.Retention.Blur()
	return nil
}

func (l *loggingSection) Blur() {
	l.state.Retention.Blur()
}

func (l *loggingSection) Resize(width int) {
	l.width = width
	if width > 0 {
		l.state.Retention.Width = width
	}
}

func (l *loggingSection) moveFocus(delta int) tea.Cmd {
	next := (int(l.state.Focus) + delta + 2) % 2
	l.state.Focus = LoggingField(next)
	if l.state.Focus == LoggingFieldRetention {
		return l.state.Retention.Focus()
	}
	l.state.Retention.Blur()
	return nil
}

func (l *loggingSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyUp:
			return l, l.moveFocus(-1)
		case tea.KeyDown:
			return l, l.moveFocus(1)
		case tea.KeyEnter, tea.KeySpace:
			if l.state.Focus == LoggingFieldToggle {
				l.state.Enabled = !l.state.Enabled
				return l, nil
			}
			if key.Type == tea.KeySpace {
				return l, nil
			}
		}

		if l.state.Focus == LoggingFieldRetention {
			if !l.state.Enabled {
				return l, nil
			}
			switch key.Type {
			case tea.KeyRunes:
				filtered := make([]rune, 0, len(key.Runes))
				for _, r := range key.Runes {
					if unicode.IsDigit(r) {
						filtered = append(filtered, r)
					}
				}
				if len(filtered) == 0 {
					return l, nil
				}
				key = tea.KeyMsg{Type: tea.KeyRunes, Runes: filtered}
			case tea.KeySpace:
				return l, nil
			}
			var cmd tea.Cmd
			l.state.Retention, cmd = l.state.Retention.Update(key)
			return l, cmd
		}
	}

	return l, nil
}

func (l *loggingSection) View() string {
	colors := l.theme.Colors()

	label := l.theme.PanelTitleStyle().Render("Logging Configuration")

	toggleLabel := "Disabled"
	toggleIcon := "[ ]"
	if l.state.Enabled {
		toggleLabel = "Enabled"
		toggleIcon = "[" + l.icons["check"] + "]"
	}

	focusedStyle := lipgloss.NewStyle().
		Background(colors.Accent).
		Foreground(colors.Background)

	enabledStyle := lipgloss.NewStyle().Foreground(colors.Success)
	disabledStyle := lipgloss.NewStyle().Foreground(colors.Error)

	toggleStyle := disabledStyle
	if l.state.Enabled {
		toggleStyle = enabledStyle
	}
	toggleText := toggleIcon + " " + toggleLabel
	if l.state.Focus == LoggingFieldToggle {
		toggleText = focusedStyle.Render(toggleText)
	} else {
		toggleText = toggleStyle.Render(toggleText)
	}

	retentionLabel := "Retention Days: "
	var retentionValue string
	if l.state.Focus == LoggingFieldRetention && l.state.Enabled {
		retentionValue = focusedStyle.Render(l.state.Retention.View())
	} else if l.state.Enabled {
		retentionValue = lipgloss.NewStyle().Foreground(colors.Primary).Render(l.state.Retention.Value())
	} else {
		disabled := lipgloss.NewStyle().Foreground(colors.Muted)
		retentionValue = disabled.Render(l.state.Retention.Value() + " (disabled)")
	}

	help := lipgloss.NewStyle().Foreground(colors.Muted).Render("Auto-cleans log history when enabled.")

	rows := []string{label, toggleText, retentionLabel + retentionValue, "", help}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
