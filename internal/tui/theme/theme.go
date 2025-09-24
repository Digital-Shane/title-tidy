package theme

import (
	"os"
	"runtime"

	"github.com/charmbracelet/lipgloss"
)

// IconSet represents a collection of icons keyed by semantic usage.
type IconSet map[string]string

// clone returns a copy of the icon set to avoid shared mutation across themes.
func (s IconSet) clone() IconSet {
	if s == nil {
		return nil
	}
	clone := make(IconSet, len(s))
	for k, v := range s {
		clone[k] = v
	}
	return clone
}

// Colors holds the shared color palette used across the TUI.
type Colors struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Background lipgloss.Color
	Muted      lipgloss.Color
	Success    lipgloss.Color
	Error      lipgloss.Color
}

// Borders defines reusable border styles.
type Borders struct {
	Panel lipgloss.Border
}

// Spacing captures commonly used spacing values.
type Spacing struct {
	PanelPadding   int
	PanelGap       int
	StatusHPadding int
}

// BadgeKind enumerates supported badge style variants.
type BadgeKind int

const (
	BadgeInfo BadgeKind = iota
	BadgeSuccess
	BadgeError
	BadgeMuted
)

// Theme centralizes palette, border, spacing, and icon configuration.
type Theme struct {
	colors   Colors
	borders  Borders
	spacing  Spacing
	icons    IconSet
	fallback IconSet
}

// Option configures a Theme during construction.
type Option func(*Theme)

// WithIconSet overrides the icon set used by the theme.
func WithIconSet(set IconSet) Option {
	return func(t *Theme) {
		t.icons = set.clone()
	}
}

// WithColors overrides the base color palette.
func WithColors(colors Colors) Option {
	return func(t *Theme) {
		t.colors = colors
	}
}

// WithSpacing overrides the default spacing values.
func WithSpacing(spacing Spacing) Option {
	return func(t *Theme) {
		t.spacing = spacing
	}
}

// WithBorders overrides the border configuration.
func WithBorders(borders Borders) Option {
	return func(t *Theme) {
		t.borders = borders
	}
}

// New constructs a Theme with optional overrides applied.
func New(opts ...Option) Theme {
	defaults := []Option{
		WithColors(Colors{
			Primary:    lipgloss.Color("#3a6b4a"),
			Secondary:  lipgloss.Color("#5a8c6a"),
			Accent:     lipgloss.Color("#8fc279"),
			Background: lipgloss.Color("#f8f8f8"),
			Muted:      lipgloss.Color("#9ba8c0"),
			Success:    lipgloss.Color("#5dc796"),
			Error:      lipgloss.Color("#f04c56"),
		}),
		WithBorders(Borders{Panel: lipgloss.RoundedBorder()}),
		WithSpacing(Spacing{PanelPadding: 1, PanelGap: 2, StatusHPadding: 1}),
		WithIconSet(defaultIconSet()),
	}

	t := Theme{fallback: asciiIcons.clone()}

	for _, opt := range append(defaults, opts...) {
		opt(&t)
	}

	if t.icons == nil {
		t.icons = defaultIconSet()
	}

	return t
}

// Default returns the default Theme configuration.
func Default() Theme {
	return New()
}

// Colors exposes the theme color palette.
func (t Theme) Colors() Colors {
	return t.colors
}

// Borders exposes the theme border configuration.
func (t Theme) Borders() Borders {
	return t.borders
}

// Spacing exposes the theme spacing configuration.
func (t Theme) Spacing() Spacing {
	return t.spacing
}

// Icon returns a themed icon with ASCII fallback if unavailable.
func (t Theme) Icon(name string) string {
	if icon, ok := t.icons[name]; ok {
		return icon
	}
	if icon, ok := t.fallback[name]; ok {
		return icon
	}
	return ""
}

// IconSet returns a defensive copy of the themed icon map.
func (t Theme) IconSet() IconSet {
	return t.icons.clone()
}

// HeaderStyle returns the shared style used for primary headers.
func (t Theme) HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Background(t.colors.Primary).
		Foreground(t.colors.Background).
		Align(lipgloss.Center)
}

// StatusBarStyle returns the shared style used for footer/status bars.
func (t Theme) StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.colors.Secondary).
		Foreground(t.colors.Background).
		Padding(0, t.spacing.StatusHPadding)
}

// PanelStyle returns the shared panel container style.
func (t Theme) PanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.borders.Panel).
		BorderForeground(t.colors.Accent).
		Padding(t.spacing.PanelPadding)
}

// PanelTitleStyle returns the shared style for panel titles.
func (t Theme) PanelTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Underline(true)
}

// BadgeStyle returns the shared badge style for the requested variant.
func (t Theme) BadgeStyle(kind BadgeKind) lipgloss.Style {
	base := lipgloss.NewStyle().Padding(0, 1).Bold(true)

	switch kind {
	case BadgeSuccess:
		return base.Background(t.colors.Success).Foreground(t.colors.Background)
	case BadgeError:
		return base.Background(t.colors.Error).Foreground(t.colors.Background)
	case BadgeMuted:
		return base.Background(t.colors.Muted).Foreground(t.colors.Background)
	default:
		return base.Background(t.colors.Accent).Foreground(t.colors.Background)
	}
}

// ProgressGradient returns the gradient colors for progress bars.
func (t Theme) ProgressGradient() []string {
	return []string{string(t.colors.Primary), string(t.colors.Accent)}
}

// defaultIconSet chooses the best icon set for the current terminal.
func defaultIconSet() IconSet {
	if isLimitedTerminal() {
		return asciiIcons.clone()
	}
	return emojiIcons.clone()
}

// isLimitedTerminal detects environments where ASCII icons are preferable.
func isLimitedTerminal() bool {
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return true
	}
	return runtime.GOOS == "windows"
}

var emojiIcons = IconSet{
	"tv":         "📺",
	"show":       "📺",
	"movie":      "🎬",
	"episode":    "🎬",
	"season":     "📁",
	"seasons":    "📁",
	"episodes":   "🎬",
	"video":      "🎥",
	"film":       "🎬",
	"moviefile":  "🎥",
	"folder":     "📁",
	"document":   "📄",
	"subtitles":  "📄",
	"default":    "📄",
	"success":    "✅",
	"error":      "❌",
	"delete":     "❌",
	"check":      "✅",
	"needrename": "✓",
	"nochange":   "=",
	"virtual":    "➕",
	"link":       "🔗",
	"unknown":    "❓",
	"stats":      "📊",
	"chart":      "📊",
	"chip":       "🧠",
	"title":      "📺",
	"calendar":   "📅",
	"key":        "🔑",
	"globe":      "🌐",
	"arrows":     "↑↓←→",
}

var asciiIcons = IconSet{
	"tv":         "[TV]",
	"show":       "[TV]",
	"movie":      "[M]",
	"episode":    "[E]",
	"season":     "[S]",
	"seasons":    "[D]",
	"episodes":   "[E]",
	"video":      "[V]",
	"film":       "[F]",
	"moviefile":  "[F]",
	"folder":     "[D]",
	"document":   "[F]",
	"subtitles":  "[S]",
	"default":    "[ ]",
	"success":    "[v]",
	"error":      "[!]",
	"delete":     "[x]",
	"check":      "[✓]",
	"needrename": "[+]",
	"nochange":   "[=]",
	"virtual":    "[+]",
	"link":       "[→]",
	"unknown":    "[?]",
	"stats":      "[*]",
	"chart":      "[#]",
	"chip":       "[T]",
	"title":      "[TV]",
	"calendar":   "[C]",
	"key":        "[K]",
	"globe":      "[G]",
	"arrows":     "^v<>",
}
