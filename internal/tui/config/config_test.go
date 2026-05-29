package config

import (
	"context"
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/google/go-cmp/cmp"
)

type fakeProvider struct {
	name  string
	order int
	vars  []provider.TemplateVariable
}

func (f fakeProvider) Name() string        { return f.name }
func (f fakeProvider) Description() string { return "fake provider" }
func (f fakeProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes:   []provider.MediaType{provider.MediaTypeShow},
		DisplayOrder: f.order,
		Local:        f.name == "local",
	}
}
func (f fakeProvider) SupportedVariables() []provider.TemplateVariable { return f.vars }
func (f fakeProvider) Configure(map[string]interface{}) error          { return nil }
func (f fakeProvider) ConfigSchema() provider.ConfigSchema             { return provider.ConfigSchema{} }
func (f fakeProvider) Fetch(context.Context, provider.FetchRequest) (*provider.Metadata, error) {
	return nil, nil
}

func testProviderState(t *testing.T, providers *ProviderState, name string) *ProviderServiceState {
	t.Helper()
	state := providers.Provider(name)
	if state == nil {
		t.Fatalf("provider state %q not found", name)
	}
	return state
}

func testProviderField(t *testing.T, providerState *ProviderServiceState, name string) *ProviderFieldState {
	t.Helper()
	field := providerState.Field(name)
	if field == nil {
		t.Fatalf("provider field %q not found on %s", name, providerState.Name())
	}
	return field
}

func TestNewWithRegistrySetsProviderRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := provider.NewRegistry()

	m, err := NewWithRegistry(reg)
	if err != nil {
		t.Fatalf("NewWithRegistry() error = %v", err)
	}
	if m.providerRegistry != reg {
		t.Fatalf("providerRegistry = %p, want %p", m.providerRegistry, reg)
	}
}

func TestVariablesViewportKeepsHorizontalOffsetPinned(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.variables.SetWidth(8)
	m.variables.SetContent("abcdefghijklmnopqrstuvwxyz")
	m.variables.SetXOffset(6)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	model := updated.(*Model)

	if got := model.variables.XOffset(); got != 0 {
		t.Fatalf("variables XOffset = %d, want 0", got)
	}
}

func TestSaveStatusClearsAfterLatestNotification(t *testing.T) {
	if saveStatusDuration != 3*time.Second {
		t.Fatalf("saveStatusDuration = %s, want 3s", saveStatusDuration)
	}

	m := &Model{}
	if cmd := m.setSaveStatus("Configuration saved!", nil); cmd == nil {
		t.Fatal("setSaveStatus() returned nil command")
	}
	firstID := m.saveID

	m.setSaveStatus("Configuration saved!", nil)
	secondID := m.saveID

	updated, _ := m.Update(saveStatusClearMsg{id: firstID})
	model := updated.(*Model)
	if got := model.saveStatus; got != "Configuration saved!" {
		t.Fatalf("saveStatus after stale clear = %q, want Configuration saved!", got)
	}

	updated, _ = model.Update(saveStatusClearMsg{id: secondID})
	model = updated.(*Model)
	if got := model.saveStatus; got != "" {
		t.Fatalf("saveStatus after latest clear = %q, want empty", got)
	}
}

func TestBuildVariablesFiltersByEnabledProviders(t *testing.T) {
	reg := provider.NewRegistry()
	if err := reg.RegisterProvider(fakeProvider{
		name:  "local",
		order: 0,
		vars: []provider.TemplateVariable{{
			Name:        "title",
			Description: "local title",
			MediaTypes:  []provider.MediaType{provider.MediaTypeShow},
		}},
	}); err != nil {
		t.Fatalf("RegisterProvider(local) error = %v", err)
	}

	if err := reg.RegisterProvider(fakeProvider{
		name:  "tmdb",
		order: 10,
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

	testProviderState(t, &state.Providers, "tmdb").Enabled = true
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
	ffprobe := testProviderState(t, &state.Providers, "ffprobe")
	ffprobe.Enabled = true
	tmdb := testProviderState(t, &state.Providers, "tmdb")
	tmdb.Enabled = true
	testProviderField(t, tmdb, "api_key").Input.SetValue("abc")
	tmdb.Validation.Status = ProviderValidationValid
	testProviderField(t, tmdb, "language").Input.SetValue("es-ES")
	omdb := testProviderState(t, &state.Providers, "omdb")
	omdb.Enabled = true
	testProviderField(t, omdb, "api_key").Input.SetValue("xyz")
	omdb.Validation.Status = ProviderValidationValidating

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

func TestBuildPreviewsProviderVariables(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())
	state.Templates.Show.Input.SetValue("{title}::{genres}")
	state.Templates.Season.Input.SetValue("Season {season}")
	state.Templates.Episode.Input.SetValue("{episode_title} - {audio_codec}")
	state.Templates.Movie.Input.SetValue("{title}-movie")

	previews := buildPreviews(SectionEpisode, &state, theme.Default().IconSet(), nil)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Episode"] != "Gray Matter - aac.mkv" {
		t.Errorf("Episode preview = %q, want Gray Matter - aac.mkv", got["Episode"])
	}

	previews = buildPreviews(SectionShowFolder, &state, theme.Default().IconSet(), nil)
	got = map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Show"] != "Breaking Bad::Drama, Crime" {
		t.Errorf("Show preview = %q, want Breaking Bad::Drama, Crime", got["Show"])
	}

	previews = buildPreviews(SectionMovie, &state, theme.Default().IconSet(), nil)
	got = map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}
	if got["Movie"] != "The Matrix-movie" {
		t.Errorf("Movie preview = %q, want The Matrix-movie", got["Movie"])
	}
}

func TestBuildPreviewsProviderVariablesWithEscapedBraces(t *testing.T) {
	state := buildStateFromConfig(&config.FormatConfig{}, theme.Default())
	state.Templates.Show.Input.SetValue(`{title} \{imdb-{imdb_id}\}`)

	previews := buildPreviews(SectionShowFolder, &state, theme.Default().IconSet(), nil)
	got := map[string]string{}
	for _, p := range previews {
		got[p.label] = p.preview
	}

	if got["Show"] != "Breaking Bad {imdb-tt0903747}" {
		t.Errorf("Show preview = %q, want %q", got["Show"], "Breaking Bad {imdb-tt0903747}")
	}
}

func TestFilterInvalidFilenameRunesAllowsBackslash(t *testing.T) {
	got := string(filterInvalidFilenameRunes([]rune(`\{imdb-\}`)))
	if got != `\{imdb-\}` {
		t.Fatalf("filterInvalidFilenameRunes() = %q, want %q", got, `\{imdb-\}`)
	}
}

func TestSanitizeTemplateValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "keeps_brace_escapes", input: `{title} \{imdb-{imdb_id}\}`, want: `{title} \{imdb-{imdb_id}\}`},
		{name: "drops_stray_backslashes", input: `foo\bar\{baz\}\\x`, want: `foobar\{baz\}x`},
		{name: "keeps_trailing_backslash_while_typing", input: `{title}\`, want: `{title}\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeTemplateValue(tt.input); got != tt.want {
				t.Fatalf("sanitizeTemplateValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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
	tmdb := testProviderState(t, &state.Providers, "tmdb")
	tmdb.Enabled = true
	testProviderField(t, tmdb, "api_key").Input.SetValue("secret")

	ps := newProviderSection(&state.Providers, theme.Default())

	var called int
	ps.validate = func(providerName, fieldName, value string) tea.Cmd {
		called++
		if providerName != "tmdb" || fieldName != "api_key" || value != "secret" {
			t.Fatalf("validate called with %q/%q/%q, want tmdb/api_key/secret", providerName, fieldName, value)
		}
		return nil
	}
	ps.debounce = func(string, string, string) tea.Cmd { return nil }

	if cmd := ps.Activate(); cmd != nil {
		cmd()
	}
	if called != 1 {
		t.Fatalf("activate validation calls = %d, want 1", called)
	}
}
