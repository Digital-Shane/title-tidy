package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

func TestLiteralReplacementCompatibility(t *testing.T) {
	var cfg FormatConfig
	if err := json.Unmarshal([]byte(`{"filename_replacements":{"ab":"x", "a":"ab", "?":"", ":":" - "}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.ApplyFilenameReplacements("ab a: ?"); got != "x ab -" {
		t.Fatalf("legacy literal replacement = %q", got)
	}
}

func TestRegexReplacementPipeline(t *testing.T) {
	cfg := &FormatConfig{FilenameReplacementsRegex: true, FilenameReplacements: FilenameReplacements{
		{Search: `[Ee]xtended(.[Vv]ersion)?`, Replacement: "@Extended@"},
		{Search: `\[.+?\]`, Replacement: ""},
		{Search: `@Extended@`, Replacement: "[Extended]"},
	}}
	input := "Argo_Extended_version_(2012)_[H.264].[AAC].[DVDRip]"
	if got := cfg.FilenamePreprocessor()(input); got != "Argo_[Extended]_(2012)_.." {
		t.Fatalf("ordered pipeline = %q", got)
	}
	cfg.FilenameReplacements = FilenameReplacements{
		{Search: `(?i)(?P<cut>extended|uncut)[._]version`, Replacement: `[${cut}] $$ $1`},
	}
	if got := cfg.FilenamePreprocessor()("Extended_version Uncut.version"); got != "[Extended] $ Extended [Uncut] $ Uncut" {
		t.Fatalf("global captures = %q", got)
	}
	if got := cfg.ApplyFilenameReplacements("Extended_version"); got != "Extended_version" {
		t.Fatalf("regex pipeline applied again to generated name: %q", got)
	}
}

func TestReplacementModeConversion(t *testing.T) {
	literal := FilenameReplacement{Search: `[Extended].($)`, Replacement: "[Cut] $5 ${cut}"}
	regex := literal.AsRegex()
	cfg := &FormatConfig{FilenameReplacementsRegex: true, FilenameReplacements: FilenameReplacements{regex}}
	if got := cfg.FilenamePreprocessor()(literal.Search); got != literal.Replacement {
		t.Errorf("quoted literal = %q, want %q", got, literal.Replacement)
	}
	if diff := cmp.Diff(literal, regex.AsLiteral()); diff != "" {
		t.Fatal(diff)
	}
	for _, rule := range []FilenameReplacement{
		{Search: `(?i)extended`, Replacement: "[Extended]", Regex: true},
		{Search: `(extended)`, Replacement: "$1", Regex: true},
		{Search: `extended`, Replacement: "${cut}", Regex: true},
		{Search: `[Ee]xtended(.[Vv]ersion)?`, Replacement: "[Extended]", Regex: true},
		{Search: `[`, Replacement: "", Regex: true},
	} {
		if diff := cmp.Diff(rule, rule.AsLiteral().AsRegex()); diff != "" {
			t.Errorf("complex regex changed: %s", diff)
		}
	}
}

func TestRegexReplacementConfigRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := DefaultConfig()
	cfg.FilenameReplacementsRegex = true
	cfg.FilenameReplacements = FilenameReplacements{
		{Search: "z", Replacement: "x", Regex: true},
		{Search: "x", Replacement: "z", Regex: true},
		{Search: "z", Replacement: "last", Regex: true},
	}
	for _, enabled := range []bool{true, false, true} {
		cfg.FilenameReplacementsRegex = enabled
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(cfg, loaded); diff != "" {
			t.Fatal(diff)
		}
		if !enabled && loaded.ApplyFilenameReplacements("z") != "z" {
			t.Fatal("paused regex was treated as literal")
		}
	}
}

func TestInvalidReplacementRejectedBeforeWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("HOME"), ".title-tidy", "config.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []FilenameReplacement{
		{Search: "["}, {Search: "(?=cut)"}, {Search: ""},
		{Search: "cut", Replacement: "a/b"},
	} {
		cfg.FilenameReplacementsRegex = true
		cfg.FilenameReplacements = FilenameReplacements{rule}
		if err := cfg.Save(); err == nil {
			t.Errorf("invalid rule saved: %+v", rule)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("invalid save modified configuration: %v", err)
	}
}

func TestRegexModeRoundTripPreservesPatternSpelling(t *testing.T) {
	for _, search := range []string{
		`\x41`, `\x{41}`, `[A]`, `(?:Argo)`, `(?s)Argo`, `(?-i)Argo`,
		`\Q[Extended].\E`, `\t`, `\n`, `\r`, `\a`, `\f`, `\v`,
	} {
		t.Run(search, func(t *testing.T) {
			rule := FilenameReplacement{Search: search, Replacement: "[Cut] $$", Regex: true}
			if diff := cmp.Diff(rule, rule.AsLiteral().AsRegex()); diff != "" {
				t.Fatalf("mode cycle rewrote an existing regex (-want +got):\n%s", diff)
			}
		})
	}
}

func FuzzFilenameReplacementModeRoundTrip(f *testing.F) {
	for _, seed := range []struct{ search, replacement string }{
		{`[Cut].($)`, "$5 ${cut} $$"}, {"\t\n", ""}, {"監督版", "Director's Cut"},
		{`\x41`, "$$"}, {`(?:Argo)`, "[Extended]"}, {`(cut)`, "${1}"}, {`[`, "$"},
	} {
		f.Add(seed.search, seed.replacement)
	}
	f.Fuzz(func(t *testing.T, search, replacement string) {
		if len(search) > 1024 || len(replacement) > 1024 || !utf8.ValidString(search) || !utf8.ValidString(replacement) {
			return
		}
		literal := FilenameReplacement{Search: search, Replacement: replacement}
		if ValidateFilenameReplacement(literal, false) == nil {
			regex := literal.AsRegex()
			if got := regex.AsLiteral(); got != literal {
				t.Fatalf("literal changed after cycle: before=%+v after=%+v", literal, got)
			}
			pattern, err := regexp.Compile(regex.Search)
			if err != nil || pattern.ReplaceAllString(search, regex.Replacement) != replacement {
				t.Fatalf("converted literal changed meaning: %+v, %v", literal, err)
			}
		}
		regex := FilenameReplacement{Search: search, Replacement: replacement, Regex: true}
		if ValidateFilenameReplacement(regex, true) == nil {
			if got := regex.AsLiteral().AsRegex(); got != regex {
				t.Fatalf("regex changed after cycle: before=%+v after=%+v", regex, got)
			}
		}
	})
}

func TestPausedRegexRulesLeaveGeneratedNameUntouched(t *testing.T) {
	cfg := &FormatConfig{FilenameReplacements: FilenameReplacements{
		{Search: `(?i)extended`, Replacement: "[Extended]", Regex: true},
	}}
	name := " Argo\u00a0  (2012)\t[Extended] "
	if got := cfg.ApplyFilenameReplacements(name); got != name {
		t.Fatalf("paused regex pipeline changed generated name: got=%q want=%q", got, name)
	}
}
