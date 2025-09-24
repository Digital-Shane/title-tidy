package config

import (
	"context"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
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

func newBareModel() *Model {
	m := &Model{
		inputs: map[Section]string{
			SectionShowFolder:   "{title}",
			SectionSeasonFolder: "Season {season}",
			SectionEpisode:      "S{season}E{episode}",
			SectionMovie:        "{title} ({year})",
		},
		cursorPos:        map[Section]int{},
		loggingRetention: "30",
		tmdbLanguage:     "en-US",
	}
	m.theme = theme.Default()
	m.icons = m.theme.IconSet()
	m.tmdbValidate = validateTMDBAPIKey
	m.tmdbDebounce = debouncedTMDBValidate
	m.omdbValidate = validateOMDBAPIKey
	m.omdbDebounce = debouncedOMDBValidate
	return m
}

func TestNewWithRegistrySetsTemplateRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := config.NewTemplateRegistry()

	m, err := NewWithRegistry(reg)
	if err != nil {
		t.Fatalf("NewWithRegistry() error = %v", err)
	}

	if m.templateRegistry == nil {
		t.Fatal("templateRegistry is nil, want provided registry")
	}
	if want, got := reg, m.templateRegistry; want != got {
		t.Errorf("templateRegistry mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestGetVariablesForSectionFiltersByProviders(t *testing.T) {
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

	m := newBareModel()
	m.templateRegistry = reg
	m.activeSection = SectionShowFolder

	vars := m.getVariablesForSection()
	if diff := cmp.Diff([]variable{{name: "{title}", description: "local title", example: ""}}, vars, cmp.AllowUnexported(variable{})); diff != "" {
		t.Fatalf("variables diff (-want +got):\n%s", diff)
	}

	m.tmdbEnabled = true
	vars = m.getVariablesForSection()
	want := []variable{
		{name: "{title}", description: "local title"},
		{name: "{rating}", description: "tmdb rating"},
	}
	if diff := cmp.Diff(want, vars, cmp.AllowUnexported(variable{})); diff != "" {
		t.Errorf("variables with TMDB diff (-want +got):\n%s", diff)
	}
}

func previewsToMap(previews []preview) map[string]string {
	result := make(map[string]string, len(previews))
	for _, p := range previews {
		result[p.label] = p.preview
	}
	return result
}

func TestGeneratePreviewsLoggingSection(t *testing.T) {
	m := newBareModel()
	m.activeSection = SectionLogging
	m.loggingEnabled = true
	m.loggingRetention = "15"

	got := previewsToMap(m.generatePreviews())

	if got["Logging"] != "Enabled" {
		t.Errorf("Logging preview = %q, want Enabled", got["Logging"])
	}
	if got["Retention"] != "15 days" {
		t.Errorf("Retention preview = %q, want 15 days", got["Retention"])
	}
}

func TestGeneratePreviewsProvidersSection(t *testing.T) {
	cases := []struct {
		name           string
		tmdbEnabled    bool
		tmdbValidation string
		tmdbAPIKey     string
		omdbEnabled    bool
		omdbValidation string
		omdbAPIKey     string
		expectTMDB     string
		expectOMDB     string
	}{
		{
			name:           "configured",
			tmdbEnabled:    true,
			tmdbValidation: "",
			tmdbAPIKey:     "abc",
			omdbEnabled:    true,
			omdbValidation: "",
			omdbAPIKey:     "xyz",
			expectTMDB:     "Configured",
			expectOMDB:     "Configured",
		},
		{
			name:           "validating",
			tmdbEnabled:    true,
			tmdbValidation: "validating",
			tmdbAPIKey:     "abc",
			omdbEnabled:    true,
			omdbValidation: "validating",
			omdbAPIKey:     "xyz",
			expectTMDB:     "Validating...",
			expectOMDB:     "Validating...",
		},
		{
			name:           "invalid",
			tmdbEnabled:    true,
			tmdbValidation: "invalid",
			tmdbAPIKey:     "abc",
			omdbEnabled:    true,
			omdbValidation: "valid",
			omdbAPIKey:     "xyz",
			expectTMDB:     "Invalid",
			expectOMDB:     "Valid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newBareModel()
			m.activeSection = SectionProviders
			m.ffprobeEnabled = true
			m.tmdbEnabled = tc.tmdbEnabled
			m.tmdbAPIKey = tc.tmdbAPIKey
			m.tmdbValidation = tc.tmdbValidation
			m.tmdbLanguage = "es-ES"
			m.omdbEnabled = tc.omdbEnabled
			m.omdbAPIKey = tc.omdbAPIKey
			m.omdbValidation = tc.omdbValidation

			previewMap := previewsToMap(m.generatePreviews())

			if got := previewMap["TMDB API"]; got != tc.expectTMDB {
				t.Errorf("TMDB API preview = %q, want %q", got, tc.expectTMDB)
			}
			if got := previewMap["OMDb API"]; got != tc.expectOMDB {
				t.Errorf("OMDb API preview = %q, want %q", got, tc.expectOMDB)
			}
			if previewMap["ffprobe"] != "Enabled" {
				t.Errorf("ffprobe preview = %q, want Enabled", previewMap["ffprobe"])
			}
			if previewMap["Language"] != "es-ES" {
				t.Errorf("Language preview = %q, want es-ES", previewMap["Language"])
			}
		})
	}
}

func TestGeneratePreviewsTemplateRegistry(t *testing.T) {
	m := newBareModel()
	m.activeSection = SectionEpisode
	m.templateRegistry = config.NewTemplateRegistry()
	m.inputs[SectionShowFolder] = "{title}::{genres}"
	m.inputs[SectionSeasonFolder] = "Season {season}"
	m.inputs[SectionEpisode] = "{episode_title} - {audio_codec}"
	m.inputs[SectionMovie] = "{title}-movie"

	previews := previewsToMap(m.generatePreviews())

	if got := previews["Episode"]; got != "Gray Matter - aac.mkv" {
		t.Errorf("episode preview = %q, want Gray Matter - aac.mkv", got)
	}
	if got := previews["Show"]; got != "Breaking Bad::Drama, Crime" {
		t.Errorf("show preview = %q, want Breaking Bad::Drama, Crime", got)
	}
	if got := previews["Movie"]; got != "The Matrix-movie" {
		t.Errorf("movie preview = %q, want The Matrix-movie", got)
	}
}

func TestValidateTMDBOnSectionSwitch(t *testing.T) {
	m := newBareModel()
	m.tmdbAPIKey = "valid-key"

	orig := m.tmdbValidate
	var called int
	m.tmdbValidate = func(apiKey string) tea.Cmd {
		called++
		if apiKey != "valid-key" {
			t.Fatalf("validate called with %q, want valid-key", apiKey)
		}
		return func() tea.Msg { return tmdbValidationMsg{apiKey: apiKey, valid: true} }
	}
	defer func() { m.tmdbValidate = orig }()

	cmd := m.validateTMDBOnSectionSwitch()
	if called != 1 {
		t.Fatalf("validate called %d times, want 1", called)
	}
	if cmd == nil {
		t.Fatal("validateTMDBOnSectionSwitch returned nil command")
	}
	if msg := cmd(); msg.(tmdbValidationMsg).apiKey != "valid-key" {
		t.Errorf("returned message %+v, want apiKey valid-key", msg)
	}

	m.tmdbValidatedKey = "valid-key"
	called = 0
	if cmd := m.validateTMDBOnSectionSwitch(); cmd != nil {
		t.Error("expected nil cmd when key already validated")
	}
	if called != 0 {
		t.Errorf("validation called %d times, want 0", called)
	}

	m.tmdbAPIKey = ""
	if cmd := m.validateTMDBOnSectionSwitch(); cmd != nil {
		t.Error("expected nil cmd when key empty")
	}
}

func TestValidateOMDBOnSectionSwitch(t *testing.T) {
	m := newBareModel()
	m.omdbAPIKey = "abcd"

	orig := m.omdbValidate
	var called int
	m.omdbValidate = func(apiKey string) tea.Cmd {
		called++
		if apiKey != "abcd" {
			t.Fatalf("validate called with %q, want abcd", apiKey)
		}
		return func() tea.Msg { return omdbValidationMsg{apiKey: apiKey, valid: false} }
	}
	defer func() { m.omdbValidate = orig }()

	cmd := m.validateOMDBOnSectionSwitch()
	if called != 1 {
		t.Fatalf("validate called %d times, want 1", called)
	}
	if cmd == nil {
		t.Fatal("validateOMDBOnSectionSwitch returned nil command")
	}
	if msg := cmd(); msg.(omdbValidationMsg).apiKey != "abcd" {
		t.Errorf("returned message %+v, want apiKey abcd", msg)
	}

	m.omdbValidatedKey = "abcd"
	called = 0
	if cmd := m.validateOMDBOnSectionSwitch(); cmd != nil {
		t.Error("expected nil cmd when key already validated")
	}
	if called != 0 {
		t.Errorf("validation called %d times, want 0", called)
	}

	m.omdbAPIKey = ""
	if cmd := m.validateOMDBOnSectionSwitch(); cmd != nil {
		t.Error("expected nil cmd when key empty")
	}
}

func TestDeleteLoggingChar(t *testing.T) {
	m := newBareModel()
	m.loggingRetention = "15"
	m.deleteLoggingChar()

	if m.loggingRetention != "1" {
		t.Errorf("loggingRetention = %q, want 1", m.loggingRetention)
	}

	m.deleteLoggingChar()
	if m.loggingRetention != "" {
		t.Errorf("loggingRetention = %q, want empty", m.loggingRetention)
	}

	// No panic on empty string
	m.deleteLoggingChar()
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
				t.Errorf("maskAPIKeyVisible(%q, %d, %d) = %q, want %q", tc.key, tc.prefix, tc.suffix, got, tc.want)
			}
		})
	}
}
