package local

import (
	"context"
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

const (
	providerName = "local"
)

// Provider implements the provider.Provider interface for local filesystem metadata
type Provider struct {
	parserEngine *ParserEngine
}

// New creates a new local provider instance
func New() *Provider {
	return &Provider{
		parserEngine: NewParserEngine(),
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return providerName
}

// Description returns the provider description
func (p *Provider) Description() string {
	return "Local variables extracted from the filesystem"
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
	// Local provider has no configuration
	return provider.ConfigSchema{
		Fields: []provider.ConfigField{},
	}
}

// Configure applies configuration to the provider
func (p *Provider) Configure(config map[string]interface{}) error {
	// Local provider doesn't need configuration
	return nil
}

// Fetch retrieves metadata based on the request by parsing the provided name
func (p *Provider) Fetch(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	// Validate request
	if request.Name == "" {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "Name is required for parsing",
			Retry:    false,
		}
	}

	// Extract node from Extra if available
	var node *treeview.Node[treeview.FileInfo]
	if request.Extra != nil {
		if n, ok := request.Extra["node"].(*treeview.Node[treeview.FileInfo]); ok {
			node = n
		}
	}

	// Parse using the parser engine
	metadata, err := p.parserEngine.Parse(request.MediaType, request.Name, node)
	if err != nil {
		// Wrap error if it's not already a ProviderError
		if _, isProviderError := err.(*provider.ProviderError); !isProviderError {
			return nil, &provider.ProviderError{
				Provider: providerName,
				Code:     "PARSE_ERROR",
				Message:  fmt.Sprintf("Failed to parse %s: %v", request.Name, err),
				Retry:    false,
			}
		}
		return nil, err
	}

	// Add additional request data to metadata if provided
	if request.Year != "" && metadata.Core.Year == "" {
		metadata.Core.Year = request.Year
	}
	if request.Season > 0 && metadata.Core.SeasonNum == 0 {
		metadata.Core.SeasonNum = request.Season
	}
	if request.Episode > 0 && metadata.Core.EpisodeNum == 0 {
		metadata.Core.EpisodeNum = request.Episode
	}

	return metadata, nil
}

// Detect analyses a tree node and returns parsed metadata alongside the detected media type.
func (p *Provider) Detect(node *treeview.Node[treeview.FileInfo]) (provider.MediaType, *provider.Metadata, error) {
	return p.parserEngine.DetectNode(node)
}
