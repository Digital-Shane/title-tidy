package theme

import (
	"runtime"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"
)

func TestIconSetCloneCreatesIndependentCopy(t *testing.T) {
	source := IconSet{"movie": "🎬"}
	clone := source.clone()

	source["movie"] = "mutated"

	if got, want := clone["movie"], "🎬"; got != want {
		t.Errorf("IconSet.clone(%v)[%q] = %q, want %q", source, "movie", got, want)
	}
}

func TestThemeIconSetDefensiveCopy(t *testing.T) {
	icons := IconSet{"movie": "🎬"}
	theme := New(WithIconSet(icons))

	icons["movie"] = "mutated"

	if got, want := theme.Icon("movie"), "🎬"; got != want {
		t.Errorf("WithIconSet(%v) Icon(%q) = %q, want %q", icons, "movie", got, want)
	}

	exposed := theme.IconSet()
	exposed["movie"] = "changed"

	if got, want := theme.Icon("movie"), "🎬"; got != want {
		t.Errorf("IconSet() mutation impacted Icon(%q) = %q, want %q", "movie", got, want)
	}
}

func TestThemeIconLookupOrder(t *testing.T) {
	theme := Theme{
		icons:    IconSet{"primary": "icon"},
		fallback: IconSet{"fallback": "fallback-icon"},
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "primary", key: "primary", want: "icon"},
		{name: "fallback", key: "fallback", want: "fallback-icon"},
		{name: "missing", key: "missing", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := theme.Icon(tc.key); got != tc.want {
				t.Errorf("Theme.Icon(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestNewAppliesCustomOptions(t *testing.T) {
	customColors := Colors{
		Primary:    lipgloss.Color("#111111"),
		Secondary:  lipgloss.Color("#222222"),
		Accent:     lipgloss.Color("#333333"),
		Background: lipgloss.Color("#444444"),
		Muted:      lipgloss.Color("#555555"),
		Success:    lipgloss.Color("#666666"),
		Error:      lipgloss.Color("#777777"),
	}
	customSpacing := Spacing{PanelPadding: 4, PanelGap: 3, StatusHPadding: 2}
	customBorder := Borders{Panel: lipgloss.ThickBorder()}
	customIcons := IconSet{"custom": "icon"}

	theme := New(
		WithColors(customColors),
		WithSpacing(customSpacing),
		WithBorders(customBorder),
		WithIconSet(customIcons),
	)

	if diff := cmp.Diff(customColors, theme.Colors()); diff != "" {
		t.Errorf("New(...) Colors() mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(customSpacing, theme.Spacing()); diff != "" {
		t.Errorf("New(...) Spacing() mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(customBorder, theme.Borders()); diff != "" {
		t.Errorf("New(...) Borders() mismatch (-want +got):\n%s", diff)
	}

	if got, want := theme.Icon("custom"), "icon"; got != want {
		t.Errorf("New(...) Icon(%q) = %q, want %q", "custom", got, want)
	}
}

func TestNewRestoresNilIconSet(t *testing.T) {
	theme := New(WithIconSet(nil))
	want := defaultIconSet()["tv"]

	if got := theme.Icon("tv"); got != want {
		t.Errorf("New(WithIconSet(nil)) Icon(%q) = %q, want %q", "tv", got, want)
	}
}

func TestProgressGradientUsesPrimaryAndAccent(t *testing.T) {
	theme := New()
	colors := theme.Colors()

	got := theme.ProgressGradient()
	want := []string{string(colors.Primary), string(colors.Accent)}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ProgressGradient() mismatch (-want +got):\n%s", diff)
	}
}

func TestDefaultIconSetLimitedTerminal(t *testing.T) {
	t.Setenv("SSH_CLIENT", "1")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")

	got := defaultIconSet()

	if diff := cmp.Diff(asciiIcons, got); diff != "" {
		t.Errorf("defaultIconSet() in limited terminal mismatch (-want +got):\n%s", diff)
	}
}

func TestDefaultIconSetEmojiWhenNotLimited(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("defaultIconSet prefers ASCII on Windows")
	}

	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")

	got := defaultIconSet()

	if diff := cmp.Diff(emojiIcons, got); diff != "" {
		t.Errorf("defaultIconSet() without limitations mismatch (-want +got):\n%s", diff)
	}
}

func TestBadgeStyleVariants(t *testing.T) {
	theme := New()
	colors := theme.Colors()

	tests := []struct {
		name string
		kind BadgeKind
		want lipgloss.Color
	}{
		{name: "info", kind: BadgeInfo, want: colors.Accent},
		{name: "success", kind: BadgeSuccess, want: colors.Success},
		{name: "error", kind: BadgeError, want: colors.Error},
		{name: "muted", kind: BadgeMuted, want: colors.Muted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			style := theme.BadgeStyle(tc.kind)

			bg, ok := style.GetBackground().(lipgloss.Color)
			if !ok {
				t.Fatalf("BadgeStyle(%v) background = %T, want lipgloss.Color", tc.kind, style.GetBackground())
			}

			if bg != tc.want {
				t.Errorf("BadgeStyle(%v) background = %v, want %v", tc.kind, bg, tc.want)
			}

			fg, ok := style.GetForeground().(lipgloss.Color)
			if !ok {
				t.Fatalf("BadgeStyle(%v) foreground = %T, want lipgloss.Color", tc.kind, style.GetForeground())
			}

			if fg != colors.Background {
				t.Errorf("BadgeStyle(%v) foreground = %v, want %v", tc.kind, fg, colors.Background)
			}
		})
	}
}

func TestHeaderStyleProperties(t *testing.T) {
	theme := New()
	colors := theme.Colors()

	style := theme.HeaderStyle()

	if !style.GetBold() {
		t.Errorf("HeaderStyle() bold = %v, want %v", style.GetBold(), true)
	}

	if bg, ok := style.GetBackground().(lipgloss.Color); !ok || bg != colors.Primary {
		t.Errorf("HeaderStyle() background = %v, want %v", style.GetBackground(), colors.Primary)
	}

	if fg, ok := style.GetForeground().(lipgloss.Color); !ok || fg != colors.Background {
		t.Errorf("HeaderStyle() foreground = %v, want %v", style.GetForeground(), colors.Background)
	}

	if got, want := style.GetAlignHorizontal(), lipgloss.Center; got != want {
		t.Errorf("HeaderStyle() alignment = %v, want %v", got, want)
	}
}

func TestStatusBarStylePadding(t *testing.T) {
	theme := New()
	colors := theme.Colors()
	spacing := theme.Spacing()

	style := theme.StatusBarStyle()

	if bg, ok := style.GetBackground().(lipgloss.Color); !ok || bg != colors.Secondary {
		t.Errorf("StatusBarStyle() background = %v, want %v", style.GetBackground(), colors.Secondary)
	}

	if fg, ok := style.GetForeground().(lipgloss.Color); !ok || fg != colors.Background {
		t.Errorf("StatusBarStyle() foreground = %v, want %v", style.GetForeground(), colors.Background)
	}

	top, right, bottom, left := style.GetPadding()
	if top != 0 || bottom != 0 || right != spacing.StatusHPadding || left != spacing.StatusHPadding {
		t.Errorf(
			"StatusBarStyle() padding = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
			top,
			right,
			bottom,
			left,
			0,
			spacing.StatusHPadding,
			0,
			spacing.StatusHPadding,
		)
	}
}

func TestPanelStyleProperties(t *testing.T) {
	theme := New()
	colors := theme.Colors()
	spacing := theme.Spacing()
	borders := theme.Borders()

	style := theme.PanelStyle()

	if border := style.GetBorderStyle(); border != borders.Panel {
		t.Errorf("PanelStyle() border = %v, want %v", border, borders.Panel)
	}

	if top, right, bottom, left := style.GetPadding(); top != spacing.PanelPadding || right != spacing.PanelPadding || bottom != spacing.PanelPadding || left != spacing.PanelPadding {
		t.Errorf(
			"PanelStyle() padding = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
			top,
			right,
			bottom,
			left,
			spacing.PanelPadding,
			spacing.PanelPadding,
			spacing.PanelPadding,
			spacing.PanelPadding,
		)
	}

	if fg, ok := style.GetBorderTopForeground().(lipgloss.Color); !ok || fg != colors.Accent {
		t.Errorf("PanelStyle() border color = %v, want %v", style.GetBorderTopForeground(), colors.Accent)
	}
}
