package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Tick schedules a message after the specified duration.
func Tick(duration time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
	return tea.Tick(duration, fn)
}

// Debounce triggers a command after the duration, collapsing rapid re-invocations.
func Debounce(duration time.Duration, fn func() tea.Msg) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg { return fn() })
}

// DebounceMsg returns a tea.Cmd that emits the provided message after the delay.
func DebounceMsg(duration time.Duration, msg tea.Msg) tea.Cmd {
	return Debounce(duration, func() tea.Msg { return msg })
}
