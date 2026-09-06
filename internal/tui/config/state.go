package config

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

// Section represents each top-level configuration panel.
type Section int

const (
	SectionShowFolder Section = iota
	SectionSeasonFolder
	SectionEpisode
	SectionMovie
	SectionRename
	SectionLogging
	SectionProviders
)

type RenameState struct {
	PreserveExistingTags bool
}

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
	WorkerCount        textinput.Model
	EnableManualSearch bool
	Active             ProviderFocus
	Providers          []ProviderServiceState
}

// Provider returns a provider state by stable provider name.
func (p *ProviderState) Provider(name string) *ProviderServiceState {
	if p == nil {
		return nil
	}
	for i := range p.Providers {
		if p.Providers[i].Name() == name {
			return &p.Providers[i]
		}
	}
	return nil
}

// EnabledProviderCount returns the number of enabled metadata providers.
func (p *ProviderState) EnabledProviderCount() int {
	if p == nil {
		return 0
	}
	count := 0
	for _, providerState := range p.Providers {
		if providerState.Enabled {
			count++
		}
	}
	return count
}

// ProviderFocusKind identifies the kind of focused provider control.
type ProviderFocusKind int

const (
	ProviderFocusWorkers ProviderFocusKind = iota
	ProviderFocusManualSearch
	ProviderFocusToggle
	ProviderFocusField
)

// ProviderFocus identifies a focusable provider control.
type ProviderFocus struct {
	Kind         ProviderFocusKind
	ProviderName string
	FieldName    string
}

// ProviderServiceState describes the UI state for a single provider.
type ProviderServiceState struct {
	Provider   provider.Provider
	Enabled    bool
	Fields     []ProviderFieldState
	Validation ProviderValidationState
}

func (p *ProviderServiceState) Name() string {
	if p == nil || p.Provider == nil {
		return ""
	}
	return p.Provider.Name()
}

func (p *ProviderServiceState) DisplayName() string {
	if p == nil || p.Provider == nil {
		return ""
	}
	if displayName := strings.TrimSpace(p.Provider.Capabilities().DisplayName); displayName != "" {
		return displayName
	}
	return p.Provider.Name()
}

func (p *ProviderServiceState) Description() string {
	if p == nil || p.Provider == nil {
		return ""
	}
	return p.Provider.Description()
}

func (p *ProviderServiceState) Icon() string {
	if p == nil || p.Provider == nil {
		return ""
	}
	return p.Provider.Capabilities().Icon
}

// Field returns one configured provider field by schema name.
func (p *ProviderServiceState) Field(name string) *ProviderFieldState {
	if p == nil {
		return nil
	}
	for i := range p.Fields {
		if p.Fields[i].Schema.Name == name {
			return &p.Fields[i]
		}
	}
	return nil
}

// RequiredPasswordField returns the first required password field, when present.
func (p *ProviderServiceState) RequiredPasswordField() *ProviderFieldState {
	if p == nil {
		return nil
	}
	for i := range p.Fields {
		field := p.Fields[i].Schema
		if field.Required && field.Type == provider.ConfigFieldTypePassword {
			return &p.Fields[i]
		}
	}
	return nil
}

// ProviderFieldState stores UI state for one provider config field.
type ProviderFieldState struct {
	Schema provider.ConfigField
	Input  textinput.Model
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
	Rename    RenameState
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
