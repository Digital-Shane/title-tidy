package tmdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/patrickmn/go-cache"
	"github.com/ryanbradynd05/go-tmdb"
)

const (
	providerName = "tmdb"
)

// Provider implements the provider.Provider interface for TMDB
type Provider struct {
	client      TMDBClient
	cache       *cache.Cache
	cacheFile   string
	language    string
	apiKey      string
	rateLimiter *rateLimiter
	config      map[string]interface{}
}

// TMDBClient interface for testing (matches *tmdb.TMDb exactly)
type TMDBClient interface {
	SearchMovie(name string, options map[string]string) (*tmdb.MovieSearchResults, error)
	SearchTv(name string, options map[string]string) (*tmdb.TvSearchResults, error)
	GetMovieInfo(id int, options map[string]string) (*tmdb.Movie, error)
	GetTvInfo(id int, options map[string]string) (*tmdb.TV, error)
	GetTvSeasonInfo(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error)
	GetTvEpisodeInfo(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error)
}

// New creates a new TMDB provider instance
func New() *Provider {
	return &Provider{
		language: "en-US",
		config:   make(map[string]interface{}),
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return providerName
}

// Description returns the provider description
func (p *Provider) Description() string {
	return "The Movie Database (TMDB) provided metadata"
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
		RequiresAuth: true,
		Priority:     100, // High priority as a comprehensive provider
	}
}

// SupportedVariables returns the template variables this provider supports
func (p *Provider) SupportedVariables() []provider.TemplateVariable {
	return []provider.TemplateVariable{
		// Note: We don't provide title or year as those come from the local provider
		// We only provide TMDB-specific metadata

		// Ratings and Reviews
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

		// Genres and Categories
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

		// Production Information
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

		// TV-Specific
		{
			Name:        "episode_title",
			DisplayName: "Episode Title",
			Description: "Title of the episode",
			MediaTypes:  []provider.MediaType{provider.MediaTypeEpisode},
			Example:     "Pilot",
			Category:    "basic",
			Provider:    providerName,
		},

		// Marketing
		{
			Name:        "tagline",
			DisplayName: "Tagline",
			Description: "Marketing tagline",
			MediaTypes:  []provider.MediaType{provider.MediaTypeMovie},
			Example:     "Welcome to the Real World",
			Category:    "basic",
			Provider:    providerName,
		},

		// Identifiers
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

// ConfigSchema returns the configuration schema for this provider
func (p *Provider) ConfigSchema() provider.ConfigSchema {
	return provider.ConfigSchema{
		Fields: []provider.ConfigField{
			{
				Name:        "api_key",
				DisplayName: "API Key",
				Type:        provider.ConfigFieldTypePassword,
				Required:    true,
				Description: "TMDB API key (not the Read Access Token). Get it from themoviedb.org/settings/api",
				Sensitive:   true,
				Validation: &provider.ConfigFieldValidation{
					MinLength: 32,
					MaxLength: 32,
					Pattern:   "^[a-f0-9]{32}$",
				},
			},
			{
				Name:        "language",
				DisplayName: "Language",
				Type:        provider.ConfigFieldTypeSelect,
				Required:    false,
				Default:     "en-US",
				Description: "Preferred language for metadata",
				Validation: &provider.ConfigFieldValidation{
					Options: []provider.ConfigFieldOption{
						{Value: "en-US", Label: "English (US)", Description: ""},
						{Value: "en-GB", Label: "English (UK)", Description: ""},
						{Value: "fr-FR", Label: "French", Description: ""},
						{Value: "de-DE", Label: "German", Description: ""},
						{Value: "es-ES", Label: "Spanish", Description: ""},
						{Value: "it-IT", Label: "Italian", Description: ""},
						{Value: "ja-JP", Label: "Japanese", Description: ""},
						{Value: "ko-KR", Label: "Korean", Description: ""},
						{Value: "zh-CN", Label: "Chinese", Description: ""},
						{Value: "pt-BR", Label: "Portuguese (Brazil)", Description: ""},
					},
				},
			},
			{
				Name:        "cache_enabled",
				DisplayName: "Enable Cache",
				Type:        provider.ConfigFieldTypeBool,
				Required:    false,
				Default:     true,
				Description: "Cache API responses to reduce requests",
			},
			{
				Name:        "cache_duration",
				DisplayName: "Cache Duration (hours)",
				Type:        provider.ConfigFieldTypeInt,
				Required:    false,
				Default:     168, // 7 days
				Description: "How long to cache metadata",
				DependsOn:   "cache_enabled",
				Validation: &provider.ConfigFieldValidation{
					MinValue: 1,
					MaxValue: 8760, // 1 year
				},
			},
		},
	}
}

// Configure applies configuration to the provider
func (p *Provider) Configure(config map[string]interface{}) error {
	p.config = config

	// Extract API key
	if apiKey, ok := config["api_key"].(string); ok {
		p.apiKey = apiKey
	} else {
		return fmt.Errorf("api_key is required")
	}

	// Extract language
	if language, ok := config["language"].(string); ok {
		p.language = language
	} else {
		p.language = "en-US"
	}

	// Initialize TMDB client
	tmdbConfig := tmdb.Config{
		APIKey:   p.apiKey,
		Proxies:  nil,
		UseProxy: false,
	}
	p.client = tmdb.Init(tmdbConfig)

	// Set up cache if enabled
	cacheEnabled := true
	if enabled, ok := config["cache_enabled"].(bool); ok {
		cacheEnabled = enabled
	}

	if cacheEnabled {
		cacheDuration := 168 // Default 7 days
		if duration, ok := config["cache_duration"].(int); ok {
			cacheDuration = duration
		}

		// Set up cache file path
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cacheDir := filepath.Join(homeDir, ".title-tidy", "tmdb_cache")
			os.MkdirAll(cacheDir, 0755)
			p.cacheFile = filepath.Join(cacheDir, "tmdb_cache.gob")

			// Create cache with configured expiration
			p.cache = cache.New(time.Duration(cacheDuration)*time.Hour, 10*time.Minute)

			// Try to load existing cache from disk
			if _, err := os.Stat(p.cacheFile); err == nil {
				_ = p.cache.LoadFile(p.cacheFile)
			}
		}
	}

	// Initialize rate limiter
	p.rateLimiter = newRateLimiter(38, 10*time.Second) // 38 requests per 10 seconds

	return nil
}

// SaveCache persists the cache to disk
func (p *Provider) SaveCache() error {
	if p.cache != nil && p.cacheFile != "" {
		return p.cache.SaveFile(p.cacheFile)
	}
	return nil
}

// mapError maps TMDB errors to provider errors
func (p *Provider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
		return &provider.ProviderError{
			Provider: providerName,
			Code:     "AUTH_FAILED",
			Message:  "TMDB authentication failed: " + err.Error(),
			Retry:    false,
		}
	}
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return &provider.ProviderError{
			Provider:   providerName,
			Code:       "RATE_LIMITED",
			Message:    "TMDB rate limit exceeded",
			Retry:      true,
			RetryAfter: 10,
		}
	}
	if strings.Contains(errStr, "503") || strings.Contains(errStr, "unavailable") {
		return &provider.ProviderError{
			Provider:   providerName,
			Code:       "UNAVAILABLE",
			Message:    "TMDB service unavailable",
			Retry:      true,
			RetryAfter: 30,
		}
	}

	return &provider.ProviderError{
		Provider: providerName,
		Code:     "UNKNOWN",
		Message:  "TMDB error: " + err.Error(),
		Retry:    false,
	}
}
