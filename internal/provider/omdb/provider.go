package omdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

const providerName = "omdb"

// Provider implements the provider.Provider interface for OMDb.
type Provider struct {
	client     *omdb.Client
	httpClient *http.Client
	apiKey     string
	baseURL    string
	config     map[string]interface{}
}

// New creates a new OMDb provider instance.
func New() *Provider {
	return &Provider{
		baseURL: omdb.DefaultURL,
		config:  make(map[string]interface{}),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return providerName
}

// Description returns a human readable description of the provider.
func (p *Provider) Description() string {
	return "Open Movie Database (OMDB) provided metadata"
}

// Capabilities returns what this provider can handle.
func (p *Provider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{
			provider.MediaTypeMovie,
			provider.MediaTypeShow,
			provider.MediaTypeSeason,
			provider.MediaTypeEpisode,
		},
		RequiresAuth: true,
		Priority:     90,
	}
}

// SupportedVariables returns the template variables supported by OMDb.
func (p *Provider) SupportedVariables() []provider.TemplateVariable {
	return []provider.TemplateVariable{
		{
			Name:        "rating",
			DisplayName: "Rating",
			Description: "Average user rating",
			MediaTypes:  []provider.MediaType{provider.MediaTypeMovie, provider.MediaTypeShow, provider.MediaTypeEpisode},
			Example:     "8.7",
			Category:    "ratings",
			Format:      "number",
			Provider:    providerName,
		},
		{
			Name:        "genres",
			DisplayName: "Genres",
			Description: "List of genres",
			MediaTypes:  []provider.MediaType{provider.MediaTypeMovie, provider.MediaTypeShow},
			Example:     "Action, Sci-Fi",
			Category:    "basic",
			Format:      "list",
			Provider:    providerName,
		},
		{
			Name:        "networks",
			DisplayName: "Networks",
			Description: "TV networks",
			MediaTypes:  []provider.MediaType{provider.MediaTypeShow},
			Example:     "HBO",
			Category:    "production",
			Format:      "list",
			Provider:    providerName,
		},
		{
			Name:        "episode_title",
			DisplayName: "Episode Title",
			Description: "Title of the episode",
			MediaTypes:  []provider.MediaType{provider.MediaTypeEpisode},
			Example:     "Pilot",
			Category:    "basic",
			Provider:    providerName,
		},
		{
			Name:        "imdb_id",
			DisplayName: "IMDB ID",
			Description: "Internet Movie Database ID",
			MediaTypes:  []provider.MediaType{provider.MediaTypeMovie, provider.MediaTypeShow},
			Example:     "tt0133093",
			Category:    "identifiers",
			Provider:    providerName,
		},
	}
}

// ConfigSchema returns the configuration schema for this provider.
func (p *Provider) ConfigSchema() provider.ConfigSchema {
	return provider.ConfigSchema{
		Fields: []provider.ConfigField{
			{
				Name:        "api_key",
				DisplayName: "API Key",
				Type:        provider.ConfigFieldTypePassword,
				Required:    true,
				Description: "OMDb API key. Request one from https://www.omdbapi.com/apikey.aspx",
				Sensitive:   true,
				Validation: &provider.ConfigFieldValidation{
					MinLength: 8,
					MaxLength: 64,
					Pattern:   "^[A-Za-z0-9]+$",
				},
			},
		},
	}
}

// Configure applies configuration to the provider.
func (p *Provider) Configure(config map[string]interface{}) error {
	apiKeyRaw, ok := config["api_key"].(string)
	if !ok {
		return fmt.Errorf("api_key is required")
	}

	apiKey := strings.TrimSpace(apiKeyRaw)
	if apiKey == "" {
		return fmt.Errorf("api_key is required")
	}

	// Allow overriding the HTTP client before configuration (useful for tests).
	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	p.apiKey = apiKey
	p.config = config
	p.client = omdb.NewClient(p.apiKey, p.httpClient)

	return nil
}

// Fetch retrieves metadata for the given request.
func (p *Provider) Fetch(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if p.client == nil || p.apiKey == "" {
		return nil, fmt.Errorf("provider not configured")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch request.MediaType {
	case provider.MediaTypeMovie:
		return p.fetchMovie(ctx, request)
	case provider.MediaTypeShow:
		return p.fetchShow(ctx, request)
	case provider.MediaTypeSeason:
		return p.fetchSeason(ctx, request)
	case provider.MediaTypeEpisode:
		return p.fetchEpisode(ctx, request)
	default:
		return nil, fmt.Errorf("unsupported media type: %s", request.MediaType)
	}
}

func (p *Provider) mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "invalid api key"), strings.Contains(lower, "missing omdb api key"):
		return &provider.ProviderError{
			Provider: providerName,
			Code:     "AUTH_FAILED",
			Message:  "OMDb authentication failed: " + msg,
			Retry:    false,
		}
	case strings.Contains(lower, "not found"):
		return &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  msg,
			Retry:    false,
		}
	case strings.Contains(lower, "limit reached"), strings.Contains(lower, "too many requests"):
		return &provider.ProviderError{
			Provider:   providerName,
			Code:       "RATE_LIMITED",
			Message:    msg,
			Retry:      true,
			RetryAfter: 5,
		}
	default:
		return &provider.ProviderError{
			Provider: providerName,
			Code:     "UNKNOWN",
			Message:  msg,
			Retry:    false,
		}
	}
}

// buildRequest constructs an HTTP request with common parameters applied.
func (p *Provider) buildRequest(ctx context.Context, params map[string]string) (*http.Request, error) {
	if p.httpClient == nil {
		return nil, fmt.Errorf("http client not configured")
	}

	values := url.Values{}
	for k, v := range params {
		if v == "" {
			continue
		}
		values.Set(k, v)
	}
	values.Set("apikey", p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = values.Encode()
	return req, nil
}

// parseRuntime attempts to convert runtime strings (e.g., "136 min") to integer minutes.
func parseRuntime(value string) int {
	if value == "" {
		return 0
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	minutes, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0
	}
	return minutes
}
