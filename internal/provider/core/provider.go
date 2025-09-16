package core

import (
	"context"

	"github.com/Digital-Shane/title-tidy/internal/provider"
)

const (
	providerName = "core"
)

// Provider implements the provider.Provider interface for core local variables
type Provider struct{}

// New creates a new core provider instance
func New() *Provider {
	return &Provider{}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return providerName
}

// Description returns the provider description
func (p *Provider) Description() string {
	return "Core variables extracted from local filesystem"
}

// Capabilities returns what this provider can do
func (p *Provider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{
			provider.MediaTypeMovie,
			provider.MediaTypeShow,
			provider.MediaTypeSeason,
			provider.MediaTypeEpisode,
		},
		RequiresAuth: false,
		Priority:     0, // Lowest priority, but always enabled
	}
}

// SupportedVariables returns the template variables this provider supports
func (p *Provider) SupportedVariables() []provider.TemplateVariable {
	return []provider.TemplateVariable{
		{
			Name:        "title",
			DisplayName: "Title",
			Description: "Media title extracted from filesystem",
			MediaTypes: []provider.MediaType{
				provider.MediaTypeMovie,
				provider.MediaTypeShow,
				provider.MediaTypeSeason,
				provider.MediaTypeEpisode,
			},
			Example:  "Breaking Bad",
			Category: "basic",
			Provider: providerName,
		},
		{
			Name:        "year",
			DisplayName: "Year",
			Description: "Year extracted from filename",
			MediaTypes: []provider.MediaType{
				provider.MediaTypeMovie,
				provider.MediaTypeShow,
			},
			Example:  "2008",
			Category: "basic",
			Provider: providerName,
		},
		{
			Name:        "season",
			DisplayName: "Season",
			Description: "Season number (zero-padded)",
			MediaTypes: []provider.MediaType{
				provider.MediaTypeSeason,
				provider.MediaTypeEpisode,
			},
			Example:  "01",
			Category: "basic",
			Provider: providerName,
		},
		{
			Name:        "episode",
			DisplayName: "Episode",
			Description: "Episode number (zero-padded)",
			MediaTypes: []provider.MediaType{
				provider.MediaTypeEpisode,
			},
			Example:  "05",
			Category: "basic",
			Provider: providerName,
		},
	}
}

// ConfigSchema returns the configuration schema for this provider
func (p *Provider) ConfigSchema() provider.ConfigSchema {
	// Core provider has no configuration
	return provider.ConfigSchema{
		Fields: []provider.ConfigField{},
	}
}

// Configure applies configuration to the provider
func (p *Provider) Configure(config map[string]interface{}) error {
	// Core provider doesn't need configuration
	return nil
}

// Fetch retrieves metadata based on the request
func (p *Provider) Fetch(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	// Core provider doesn't fetch metadata
	// Core variables are resolved from FormatContext in the template resolver
	return nil, &provider.ProviderError{
		Provider: providerName,
		Code:     "NOT_IMPLEMENTED",
		Message:  "Core provider does not fetch metadata",
		Retry:    false,
	}
}
