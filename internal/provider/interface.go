package provider

import (
	"context"
)

// MediaType represents the type of media content
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeShow    MediaType = "show"
	MediaTypeSeason  MediaType = "season"
	MediaTypeEpisode MediaType = "episode"
)

// Provider is the main interface that all metadata providers must implement
type Provider interface {
	// Identification
	Name() string
	Description() string

	// Capability discovery
	Capabilities() ProviderCapabilities
	SupportedVariables() []TemplateVariable

	// Configuration
	Configure(config map[string]interface{}) error
	ConfigSchema() ConfigSchema

	// Data fetching
	Fetch(ctx context.Context, request FetchRequest) (*Metadata, error)
}

// ProviderCapabilities describes what a provider can do
type ProviderCapabilities struct {
	MediaTypes   []MediaType // What media types are supported
	RequiresAuth bool        // Whether authentication is required
	Priority     int         // Default priority for this provider (higher = preferred)
}

// TemplateVariable describes a template variable that a provider can supply
type TemplateVariable struct {
	Name        string      // Variable name (without braces), e.g., "director"
	DisplayName string      // Human-readable name for UI
	Description string      // Description of what this variable contains
	MediaTypes  []MediaType // Which media types support this variable
	Example     string      // Example value
	Provider    string      // Provider that supplies this variable
	Category    string      // Category for grouping (e.g., "Basic", "Advanced", "Technical")
	Format      string      // Format hint (e.g., "date", "number", "list")
}

// ConfigSchema describes the configuration requirements for a provider
type ConfigSchema struct {
	Fields []ConfigField
}

// ConfigField describes a single configuration field
type ConfigField struct {
	Name        string                 // Field name
	DisplayName string                 // Human-readable name
	Type        ConfigFieldType        // Field type
	Required    bool                   // Whether this field is required
	Default     interface{}            // Default value
	Description string                 // Help text
	Validation  *ConfigFieldValidation // Validation rules
	Sensitive   bool                   // Whether this contains sensitive data (for masking)
	DependsOn   string                 // Field that this depends on
}

// ConfigFieldType represents the type of a configuration field
type ConfigFieldType string

const (
	ConfigFieldTypeInt      ConfigFieldType = "int"
	ConfigFieldTypeBool     ConfigFieldType = "bool"
	ConfigFieldTypeSelect   ConfigFieldType = "select"
	ConfigFieldTypePassword ConfigFieldType = "password"
)

// ConfigFieldValidation contains validation rules for a field
type ConfigFieldValidation struct {
	MinLength int                 // Minimum string length
	MaxLength int                 // Maximum string length
	Pattern   string              // Regex pattern
	MinValue  int                 // Minimum numeric value
	MaxValue  int                 // Maximum numeric value
	Options   []ConfigFieldOption // For select fields
}

// ConfigFieldOption represents an option for select fields
type ConfigFieldOption struct {
	Value       string
	Label       string
	Description string
}

// FetchRequest represents a request for metadata
type FetchRequest struct {
	MediaType MediaType
	Name      string
	Year      string
	Season    int
	Episode   int
	ID        string                 // Provider-specific ID if known
	Language  string                 // Preferred language
	Extra     map[string]interface{} // Provider-specific parameters
}

// Metadata represents the fetched metadata
type Metadata struct {
	// Core fields that are common across all providers
	Core CoreMetadata

	// Extended fields that are provider-specific
	Extended map[string]interface{}

	// Track which provider supplied which field
	Sources map[string]string

	// Provider-specific IDs
	IDs map[string]string

	// Quality/confidence score for this metadata
	Confidence float64
}

// CoreMetadata contains the essential metadata fields
type CoreMetadata struct {
	// Basic identification
	Title     string
	Year      string
	MediaType MediaType

	// TV-specific
	SeasonNum   int
	EpisodeName string
	EpisodeNum  int

	// Common fields
	Overview string
	Rating   float32
	Genres   []string
	Language string
	Country  string
}

// ProviderError represents an error from a provider
type ProviderError struct {
	Provider   string
	Code       string
	Message    string
	Retry      bool
	RetryAfter int // Seconds to wait before retry
}

func (e *ProviderError) Error() string {
	return e.Message
}
