package config

import (
	"context"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"
)

type fakeProvider struct {
	name string
	vars []provider.TemplateVariable
}

func (f fakeProvider) Name() string        { return f.name }
func (f fakeProvider) Description() string { return "fake provider" }
func (f fakeProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{}
}
func (f fakeProvider) SupportedVariables() []provider.TemplateVariable { return f.vars }
func (f fakeProvider) Configure(map[string]interface{}) error          { return nil }
func (f fakeProvider) ConfigSchema() provider.ConfigSchema             { return provider.ConfigSchema{} }
func (f fakeProvider) Fetch(context.Context, provider.FetchRequest) (*provider.Metadata, error) {
	return nil, nil
}

func TestNewWithRegistrySetsTemplateRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := config.NewTemplateRegistry()

	m, err := NewWithRegistry(reg)
	if err != nil {
		t.Fatalf("NewWithRegistry() error = %v", err)
	}
	if m.templateRegistry != reg {
		t.Fatalf("templateRegistry = %p, want %p", m.templateRegistry, reg)
	}
}

func TestBuildVariablesFiltersByEnabledProviders(t *testing.T) {
	reg := config.NewTemplateRegistry()
	if err := reg.RegisterProvider(fakeProvider{
		name: "local",
		vars: []provider.TemplateVariable{{
			Name:        "title",
			Description: "local title",
			MediaTypes:  []provider.MediaType{provider.MediaTypeShow},
		}},
	}); err != nil {
		t.Fatalf("RegisterProvider(local) error = %v", err)
	}

	if err := reg.RegisterProvider(fakeProvider{
		name: "tmdb",
		vars: []provider.TemplateVariable{{
			Name:        "rating",
			Description: "tmdb rating",
			MediaTypes:  []provider.MediaType{provider.MediaTypeShow},
		}},
	}); err != nil {
		t.Fatalf("RegisterProvider(tmdb) error = %v", err)
	}

	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())

	vars := buildVariables(SectionShowFolder, &state, reg)
	want := []variable{{name: "{title}", description: "local title"}}
	if diff := cmp.Diff(want, vars, cmp.AllowUnexported(variable{})); diff != "" {
		t.Fatalf("variables diff (-want +got):\n%s", diff)
	}

	state.Providers.TMDB.Enabled = true
	vars = buildVariables(SectionShowFolder, &state, reg)
	want = []variable{
		{name: "{title}", description: "local title"},
		{name: "{rating}", description: "tmdb rating"},
	}
	if diff := cmp.Diff(want, vars, cmp.AllowUnexported(variable{})); diff != "" {
		t.Fatalf("variables diff (-want +got):\n%s", diff)
	}
}

func TestBuildPreviewsProviders(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())
	state.Providers.FFProbeEnabled = true
	state.Providers.TMDB.Enabled = true
	state.Providers.TMDB.APIKey.SetValue("abc")
	state.Providers.TMDB.Validation.Status = ProviderValidationValid
	state.Providers.TMDB.Language.SetValue("es-ES")
	state.Providers.OMDB.Enabled = true
	state.Providers.OMDB.APIKey.SetValue("xyz")
	state.Providers.OMDB.Validation.Status = ProviderValidationValidating

	previews := buildPreviews(SectionProviders, &state, theme.Default().IconSet(), nil)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}

	if got["ffprobe"] != "Enabled" {
		t.Errorf("ffprobe preview = %q, want Enabled", got["ffprobe"])
	}
	if got["TMDB API"] != "Valid" {
		t.Errorf("TMDB API preview = %q, want Valid", got["TMDB API"])
	}
	if got["OMDb API"] != "Validating..." {
		t.Errorf("OMDb API preview = %q, want Validating...", got["OMDb API"])
	}
	if got["Language"] != "es-ES" {
		t.Errorf("Language preview = %q, want es-ES", got["Language"])
	}
}

func TestBuildPreviewsLogging(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{EnableLogging: true, LogRetentionDays: 15}, theme.Default())
	previews := buildPreviews(SectionLogging, &state, theme.Default().IconSet(), nil)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Logging"] != "Enabled" {
		t.Errorf("Logging preview = %q, want Enabled", got["Logging"])
	}
	if got["Retention"] != "15 days" {
		t.Errorf("Retention preview = %q, want 15 days", got["Retention"])
	}
}

func TestBuildPreviewsRename(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{PreserveExistingTags: true}, theme.Default())
	previews := buildPreviews(SectionRename, &state, theme.Default().IconSet(), nil)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Preserve Existing Tags"] != "Enabled" {
		t.Errorf("Preserve Existing Tags preview = %q, want Enabled", got["Preserve Existing Tags"])
	}
}

func TestBuildPreviewsTemplateRegistry(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())
	state.Templates.Show.Input.SetValue("{title}::{genres}")
	state.Templates.Season.Input.SetValue("Season {season}")
	state.Templates.Episode.Input.SetValue("{episode_title} - {audio_codec}")
	state.Templates.Movie.Input.SetValue("{title}-movie")

	reg := config.NewTemplateRegistry()

	previews := buildPreviews(SectionEpisode, &state, theme.Default().IconSet(), reg)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Episode"] != "Gray Matter - aac.mkv" {
		t.Errorf("Episode preview = %q, want Gray Matter - aac.mkv", got["Episode"])
	}

	previews = buildPreviews(SectionShowFolder, &state, theme.Default().IconSet(), reg)
	got = map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Show"] != "Breaking Bad::Drama, Crime" {
		t.Errorf("Show preview = %q, want Breaking Bad::Drama, Crime", got["Show"])
	}

	previews = buildPreviews(SectionMovie, &state, theme.Default().IconSet(), reg)
	got = map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Movie"] != "The Matrix-movie" {
		t.Errorf("Movie preview = %q, want The Matrix-movie", got["Movie"])
	}
}

func TestMaskAPIKeyVisible(t *testing.T) {
	cases := []struct {
		name           string
		key            string
		prefix, suffix int
		want           string
	}{
		{"empty", "", 2, 2, ""},
		{"negative", "abcdef", -1, -1, "******"},
		{"oversized", "abc", 2, 2, "***"},
		{"normal", "abcdefgh", 2, 2, "ab****gh"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskAPIKeyVisible(tc.key, tc.prefix, tc.suffix); got != tc.want {
				t.Fatalf("maskAPIKeyVisible(%q, %d, %d) = %q, want %q", tc.key, tc.prefix, tc.suffix, got, tc.want)
			}
		})
	}
}

// Ensure the provider section exposes validation hooks for tests.
func TestProviderSectionActivateTriggersValidation(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())
	state.Providers.TMDB.Enabled = true
	state.Providers.TMDB.APIKey.SetValue("secret")

	ps := newProviderSection(&state.Providers, theme.Default())

	var called int
	ps.tmdbValidate = func(key string) tea.Cmd {
		called++
		if key != "secret" {
			t.Fatalf("tmdbValidate called with %q, want secret", key)
		}
		return nil
	}
	ps.tmdbDebounce = func(string) tea.Cmd { return nil }

	if cmd := ps.Activate(); cmd != nil {
		cmd()
	}
	if called != 1 {
		t.Fatalf("activate validation calls = %d, want 1", called)
	}
}
