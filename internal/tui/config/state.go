package config

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

// Section represents each top-level configuration panel.
type Section int

const (
	SectionShowFolder Section = iota
	SectionSeasonFolder
	SectionEpisode
	SectionMovie
	SectionLogging
	SectionProviders
)

// TemplateSections encapsulates all template editors with dedicated state.
type TemplateSections struct {
	Show    TemplateSectionState
	Season  TemplateSectionState
	Episode TemplateSectionState
	Movie   TemplateSectionState
}

// For returns the state associated with the given section.
func (t *TemplateSections) For(section Section) *TemplateSectionState {
	switch section {
	case SectionShowFolder:
		return &t.Show
	case SectionSeasonFolder:
		return &t.Season
	case SectionEpisode:
		return &t.Episode
	case SectionMovie:
		return &t.Movie
	default:
		return nil
	}
}

// TemplateSectionState holds the edit state for a single template editor.
type TemplateSectionState struct {
	Section Section
	Title   string
	Input   textinput.Model
}

// LoggingField identifies the focusable elements within the logging section.
type LoggingField int

const (
	LoggingFieldToggle LoggingField = iota
	LoggingFieldRetention
)

// LoggingState tracks logging configuration and UI focus.
type LoggingState struct {
	Enabled   bool
	Focus     LoggingField
	Retention textinput.Model
}

// ProviderState stores metadata provider configuration and focus management.
type ProviderState struct {
	WorkerCount textinput.Model
	Active      ProviderField

	FFProbeEnabled bool
	TMDB           ProviderServiceState
	OMDB           ProviderServiceState
}

// ProviderField enumerates focusable inputs within the provider section UI.
type ProviderField int

const (
	ProviderFieldWorkers ProviderField = iota
	ProviderFieldFFProbe
	ProviderFieldOMDBToggle
	ProviderFieldOMDBKey
	ProviderFieldTMDBToggle
	ProviderFieldTMDBKey
	ProviderFieldTMDBLanguage
)

// ProviderServiceState describes the configuration for a single provider.
type ProviderServiceState struct {
	Enabled bool

	APIKey   textinput.Model
	Language textinput.Model

	Validation ProviderValidationState
}

// MaskedAPIKey returns the masked representation of the API key using the
// provided prefix/suffix visibility values.
func (p ProviderServiceState) MaskedAPIKey(prefix, suffix int) string {
	return maskAPIKeyVisible(p.APIKey.Value(), prefix, suffix)
}

// ProviderValidationState tracks validation status for API-backed providers.
type ProviderValidationState struct {
	Status        ProviderValidationStatus
	LastValidated string
}

// Reset clears validation progress and history.
func (p *ProviderValidationState) Reset() {
	p.Status = ProviderValidationUnknown
	p.LastValidated = ""
}

// ProviderValidationStatus enumerates validation phases for API keys.
type ProviderValidationStatus int

const (
	ProviderValidationUnknown ProviderValidationStatus = iota
	ProviderValidationValidating
	ProviderValidationValid
	ProviderValidationInvalid
)

// String converts the validation status into a human readable label.
func (s ProviderValidationStatus) String() string {
	switch s {
	case ProviderValidationValidating:
		return "Validating..."
	case ProviderValidationValid:
		return "Valid"
	case ProviderValidationInvalid:
		return "Invalid"
	case ProviderValidationUnknown:
		return ""
	default:
		return ""
	}
}

// ConfigState aggregates all section-specific state objects.
type ConfigState struct {
	Templates TemplateSections
	Logging   LoggingState
	Providers ProviderState
}

func maskAPIKeyVisible(key string, prefix, suffix int) string {
	key = strings.TrimSpace(key)
	if len(key) == 0 {
		return ""
	}
	if prefix < 0 {
		prefix = 0
	}
	if suffix < 0 {
		suffix = 0
	}
	if prefix+suffix >= len(key) {
		return strings.Repeat("*", len(key))
	}
	maskedLen := len(key) - prefix - suffix
	return key[:prefix] + strings.Repeat("*", maskedLen) + key[len(key)-suffix:]
}
