package config

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"
)

func TestRenameSectionRegexToggleAndPersistence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.FilenameReplacements = config.FilenameReplacements{{Search: "[Cut].", Replacement: "$5"}}
	state := buildRenameState(cfg, theme.Default())
	section := newRenameSection(&state, theme.Default())
	state.Focus = 1
	section.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !state.RegexReplacements || state.Replacements[0].Search.Value() != `\[Cut\]\.` || state.Replacements[0].Replacement.Value() != "$$5" {
		t.Fatal("literal row was not escaped on enabling regex")
	}
	state.Replacements = append(state.Replacements, newFilenameReplacement(`(?i)(extended)`, "[$1]", theme.Default()))
	state.Replacements[1].Regex = true
	section.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if state.RegexReplacements || state.Replacements[0].Search.Value() != "[Cut]." || state.Replacements[0].Replacement.Value() != "$5" {
		t.Fatal("literal row failed to round-trip")
	}
	cfg.FilenameReplacements, _ = state.filenameReplacements()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	state = buildRenameState(loaded, theme.Default())
	section = newRenameSection(&state, theme.Default())
	state.Focus = 1
	section.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := state.Replacements[1].Search.Value(); got != `(?i)(extended)` {
		t.Fatalf("complex regex changed after disabling, saving and re-enabling: %q", got)
	}
}

func TestRenameSectionReorderDeleteAndValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.FilenameReplacementsRegex = true
	cfg.FilenameReplacements = config.FilenameReplacements{
		{Search: "first", Replacement: "second"}, {Search: "second", Replacement: "last"},
	}
	state := buildRenameState(cfg, theme.Default())
	section := newRenameSection(&state, theme.Default())
	state.Focus = 4
	section.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	if state.Focus != 2 || state.Replacements[0].Search.Value() != "second" {
		t.Fatal("pair order/focus did not follow Ctrl+Up")
	}
	section.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	rules, err := state.filenameReplacements()
	if err != nil || rules[0].Search != "first" {
		t.Fatal("pair order not restored by Ctrl+Down")
	}
	state.Replacements[1].Search.SetValue("[")
	if _, err := state.filenameReplacements(); err == nil {
		t.Fatal("invalid Go regex accepted")
	}
	view := ansi.Strip(section.View().Content)
	if !strings.Contains(view, "missing closing ]") {
		t.Fatalf("compiler error missing from selected row: %s", view)
	}
	section.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if diff := cmp.Diff("first", state.Replacements[0].Search.Value()); diff != "" {
		t.Fatal(diff)
	}
	section.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if len(state.Replacements) != 0 || section.activeInput() != nil {
		t.Fatal("deleting final pair left invalid focus")
	}
}

func TestRenameRegexPanelFitsTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	m.state.Rename.RegexReplacements = true
	for _, search := range []string{`[Ee]xtended(.[Vv]ersion)?`, `\[.+?\]`, `@Extended@`} {
		pair := newFilenameReplacement(search, "[Extended]", m.theme)
		pair.Regex = true
		m.state.Rename.Replacements = append(m.state.Rename.Replacements, pair)
	}
	m.state.Rename.Focus = 6
	m.setActiveSection(4)
	m.handleWindowResize(tea.WindowSizeMsg{Width: 100, Height: 36})
	view := m.View().Content
	if got := lipgloss.Height(view); got > 36 {
		t.Fatalf("regex panel grew to %d lines:\n%s", got, ansi.Strip(view))
	}
	m.state.Rename.Replacements[2].Search.SetValue("[")
	view = m.View().Content
	if got := lipgloss.Height(view); got > 36 {
		t.Fatalf("invalid regex panel grew to %d lines:\n%s", got, ansi.Strip(view))
	}
	if !strings.Contains(ansi.Strip(view), "missing closing ]") {
		t.Fatal("regex error not visible")
	}
}

func TestRenameTogglePreservesRulesAcrossRepeatedReopens(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.FilenameReplacements = config.FilenameReplacements{
		{Search: "[Cut].($)", Replacement: "$5 ${cut} $$"},
		{Search: "tab\tand\nnewline", Replacement: "[Cut]"},
		{Search: "監督版", Replacement: "Director's Cut"},
		{Search: `\x41`, Replacement: "$$", Regex: true},
		{Search: `[A]`, Replacement: "[Cut]", Regex: true},
		{Search: `(?:Argo)`, Replacement: "[Cut]", Regex: true},
		{Search: `(?i)(extended)`, Replacement: "[$1]", Regex: true},
	}
	want := slices.Clone(cfg.FilenameReplacements)
	for range 3 {
		state := buildRenameState(cfg, theme.Default())
		section := newRenameSection(&state, theme.Default())
		section.toggleRegex()
		cfg.FilenameReplacements, _ = state.filenameReplacements()
		cfg.FilenameReplacementsRegex = state.RegexReplacements
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		loaded, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		state = buildRenameState(loaded, theme.Default())
		section = newRenameSection(&state, theme.Default())
		section.toggleRegex()
		cfg.FilenameReplacements, err = state.filenameReplacements()
		cfg.FilenameReplacementsRegex = state.RegexReplacements
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, cfg.FilenameReplacements); diff != "" {
			t.Fatalf("rules changed after toggling and reopening (-want +got):\n%s", diff)
		}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		cfg, err = config.Load()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenameSearchEditsReplaceSanitizedSource(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.FilenameReplacements = config.FilenameReplacements{{Search: "\t", Replacement: "[Cut]"}}
	state := buildRenameState(cfg, theme.Default())
	section := newRenameSection(&state, theme.Default())
	state.Focus = 2
	section.Focus()
	if got := state.Replacements[0].rule().Search; got != "\t" {
		t.Fatalf("opening editor changed saved tab: %q", got)
	}
	// Cursor movement must not count as editing the sanitized input.
	section.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if got := state.Replacements[0].rule().Search; got != "\t" {
		t.Fatalf("cursor movement changed saved tab: %q", got)
	}
	section.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	section.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	section.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	// Retyping the same display text must adopt the edit, not resurrect the tab.
	if got := state.Replacements[0].rule().Search; got != " " {
		t.Fatalf("edited search reverted to old control character: %q", got)
	}
	section.toggleRegex()
	section.toggleRegex()
	if got := state.Replacements[0].rule().Search; got != " " {
		t.Fatalf("toggle lost edited search: %q", got)
	}
}

func TestRenameLiteralDollarEscapingDoesNotAccumulate(t *testing.T) {
	for _, replacement := range []string{"$", "$$", "$1", "${cut}", "$$1 $${cut}", ""} {
		t.Run(replacement, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.FilenameReplacements = config.FilenameReplacements{{Search: ".[](){}*+?^$|\\", Replacement: replacement}}
			state := buildRenameState(cfg, theme.Default())
			section := newRenameSection(&state, theme.Default())
			for range 10 {
				section.toggleRegex()
				rule := state.Replacements[0].rule()
				if got := rule.Replacement; got != strings.ReplaceAll(replacement, "$", "$$") {
					t.Fatalf("escaping accumulated in regex mode: %q", got)
				}
				section.toggleRegex()
				rules, err := state.filenameReplacements()
				if err != nil || cmp.Diff(cfg.FilenameReplacements, rules) != "" {
					t.Fatalf("toggle lost literal rule: %+v, %v", rules, err)
				}
			}
		})
	}
}

func TestRenamePreviewCountsPausedRules(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{FilenameReplacements: config.FilenameReplacements{
		{Search: `(?i)extended`, Replacement: "[Extended]", Regex: true},
	}}, theme.Default())
	for _, preview := range buildPreviews(SectionRename, &state, theme.Default().IconSet(), nil) {
		if preview.label == "Replacements" {
			if preview.preview != "0 active, 1 paused" {
				t.Fatalf("paused rules counted as active: %q", preview.preview)
			}
			return
		}
	}
	t.Fatal("replacements preview missing")
}
