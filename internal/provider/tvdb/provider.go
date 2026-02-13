package tvdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	tvdbapi "github.com/dashotv/tvdb"
	"github.com/dashotv/tvdb/openapi/models/operations"
	"github.com/dashotv/tvdb/openapi/models/shared"
)

const providerName = "tvdb"

// TVDBClient captures the dashotv client methods used by this provider.
type TVDBClient interface {
	GetSearchResults(request operations.GetSearchResultsRequest) (*tvdbapi.GetSearchResultsResponse, error)
	GetSeriesExtended(id float64, meta *operations.GetSeriesExtendedQueryParamMeta, short *bool) (*tvdbapi.GetSeriesExtendedResponse, error)
	GetMovieExtended(id float64, meta *operations.QueryParamMeta, short *bool) (*tvdbapi.GetMovieExtendedResponse, error)
	GetSeriesEpisodes(request operations.GetSeriesEpisodesRequest) (*tvdbapi.GetSeriesEpisodesResponse, error)
}

// Provider implements the provider.Provider interface for TVDB.
type Provider struct {
	client TVDBClient
	apiKey string
	config map[string]interface{}
}

// New creates a new TVDB provider instance.
func New() *Provider {
	return &Provider{config: make(map[string]interface{})}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return providerName
}

// Description returns a human readable description of the provider.
func (p *Provider) Description() string {
	return "TheTVDB (TVDB) provided metadata"
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
		Priority:     95,
	}
}

// SupportedVariables returns the template variables supported by TVDB.
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
				Description: "TVDB API key. Generate one from your thetvdb.com account dashboard",
				Sensitive:   true,
				Validation: &provider.ConfigFieldValidation{
					MinLength: 8,
					MaxLength: 128,
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

	client, err := tvdbapi.Login(apiKey)
	if err != nil {
		return p.mapError(err)
	}

	p.apiKey = apiKey
	p.config = config
	p.client = client

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
		return p.fetchMovie(request)
	case provider.MediaTypeShow:
		return p.fetchShow(request)
	case provider.MediaTypeSeason:
		return p.fetchSeason(request)
	case provider.MediaTypeEpisode:
		return p.fetchEpisode(request)
	default:
		return nil, fmt.Errorf("unsupported media type: %s", request.MediaType)
	}
}

func (p *Provider) fetchMovie(request provider.FetchRequest) (*provider.Metadata, error) {
	record, err := p.searchMovieRecord(request)
	if err != nil {
		return nil, err
	}

	meta := operations.QueryParamMetaTranslations
	resp, err := p.client.GetMovieExtended(float64(record.ID), &meta, nil)
	if err != nil {
		return nil, p.mapError(err)
	}
	if resp == nil || resp.Data == nil {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "movie not found", Retry: false}
	}

	movie := resp.Data
	genres := make([]string, 0, len(movie.Genres))
	for _, g := range movie.Genres {
		if g.Name != nil && strings.TrimSpace(*g.Name) != "" {
			genres = append(genres, strings.TrimSpace(*g.Name))
		}
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     firstNonEmptyString(pointerToString(movie.Name), record.Name),
			Year:      firstNonEmptyString(pointerToString(movie.Year), record.Year),
			MediaType: provider.MediaTypeMovie,
			Rating:    pointerToFloat32(movie.Score),
			Genres:    genres,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.9,
	}

	if runtime := pointerToInt64(movie.Runtime); runtime > 0 {
		metadata.Extended["runtime"] = int(runtime)
		metadata.Sources["runtime"] = providerName
	}

	if imdbID := findRemoteID(movie.RemoteIds, "imdb"); imdbID != "" {
		metadata.IDs["imdb_id"] = imdbID
		metadata.Sources["imdb_id"] = providerName
	}

	if metadata.Core.Title != "" {
		metadata.Sources["title"] = providerName
	}
	if metadata.Core.Year != "" {
		metadata.Sources["year"] = providerName
	}
	if metadata.Core.Rating > 0 {
		metadata.Sources["rating"] = providerName
	}
	if len(genres) > 0 {
		metadata.Sources["genres"] = providerName
	}

	return metadata, nil
}

func (p *Provider) fetchShow(request provider.FetchRequest) (*provider.Metadata, error) {
	record, err := p.searchSeriesRecord(request)
	if err != nil {
		return nil, err
	}

	meta := operations.GetSeriesExtendedQueryParamMetaTranslations
	resp, err := p.client.GetSeriesExtended(float64(record.ID), &meta, nil)
	if err != nil {
		return nil, p.mapError(err)
	}
	if resp == nil || resp.Data == nil {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "series not found", Retry: false}
	}

	series := resp.Data
	genres := make([]string, 0, len(series.Genres))
	for _, g := range series.Genres {
		if g.Name != nil && strings.TrimSpace(*g.Name) != "" {
			genres = append(genres, strings.TrimSpace(*g.Name))
		}
	}

	networks := make([]string, 0, 2)
	if series.OriginalNetwork != nil && series.OriginalNetwork.Name != nil && strings.TrimSpace(*series.OriginalNetwork.Name) != "" {
		networks = append(networks, strings.TrimSpace(*series.OriginalNetwork.Name))
	}
	if series.LatestNetwork != nil && series.LatestNetwork.Name != nil && strings.TrimSpace(*series.LatestNetwork.Name) != "" {
		latest := strings.TrimSpace(*series.LatestNetwork.Name)
		if !stringSliceContains(networks, latest) {
			networks = append(networks, latest)
		}
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     firstNonEmptyString(pointerToString(series.Name), record.Name),
			Year:      firstNonEmptyString(pointerToString(series.Year), record.Year),
			MediaType: provider.MediaTypeShow,
			Overview:  pointerToString(series.Overview),
			Rating:    pointerToFloat32(series.Score),
			Genres:    genres,
			Country:   pointerToString(series.Country),
			Language:  pointerToString(series.OriginalLanguage),
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.9,
	}

	if len(networks) > 0 {
		metadata.Extended["networks"] = strings.Join(networks, ", ")
		metadata.Sources["networks"] = providerName
	}
	if runtime := pointerToInt64(series.AverageRuntime); runtime > 0 {
		metadata.Extended["runtime"] = int(runtime)
		metadata.Sources["runtime"] = providerName
	}
	if imdbID := findRemoteID(series.RemoteIds, "imdb"); imdbID != "" {
		metadata.IDs["imdb_id"] = imdbID
		metadata.Sources["imdb_id"] = providerName
	}

	if metadata.Core.Title != "" {
		metadata.Sources["title"] = providerName
	}
	if metadata.Core.Year != "" {
		metadata.Sources["year"] = providerName
	}
	if metadata.Core.Overview != "" {
		metadata.Sources["overview"] = providerName
	}
	if metadata.Core.Rating > 0 {
		metadata.Sources["rating"] = providerName
	}
	if len(genres) > 0 {
		metadata.Sources["genres"] = providerName
	}

	return metadata, nil
}

func (p *Provider) fetchSeason(request provider.FetchRequest) (*provider.Metadata, error) {
	if request.Season <= 0 {
		return nil, &provider.ProviderError{Provider: providerName, Code: "INVALID_REQUEST", Message: "season fetch requires a valid season number", Retry: false}
	}

	record, err := p.searchSeriesRecord(request)
	if err != nil {
		return nil, err
	}

	seasonNum := int64(request.Season)
	episodes, err := p.client.GetSeriesEpisodes(operations.GetSeriesEpisodesRequest{
		ID:         float64(record.ID),
		SeasonType: "official",
		Season:     &seasonNum,
		Page:       0,
	})
	if err != nil {
		return nil, p.mapError(err)
	}
	if episodes == nil || episodes.Data == nil {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "season not found", Retry: false}
	}

	title := record.Name
	if episodes.Data.Series != nil && episodes.Data.Series.Name != nil && strings.TrimSpace(*episodes.Data.Series.Name) != "" {
		title = strings.TrimSpace(*episodes.Data.Series.Name)
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     title,
			Year:      firstNonEmptyString(record.Year, request.Year),
			SeasonNum: request.Season,
			MediaType: provider.MediaTypeSeason,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.8,
	}

	metadata.Extended["episode_count"] = len(episodes.Data.Episodes)
	if title != "" {
		metadata.Sources["title"] = providerName
	}
	if metadata.Core.Year != "" {
		metadata.Sources["year"] = providerName
	}

	seriesExt, extErr := p.client.GetSeriesExtended(float64(record.ID), nil, nil)
	if extErr == nil && seriesExt != nil && seriesExt.Data != nil {
		if imdbID := findRemoteID(seriesExt.Data.RemoteIds, "imdb"); imdbID != "" {
			metadata.IDs["imdb_id"] = imdbID
			metadata.Sources["imdb_id"] = providerName
		}
	}

	return metadata, nil
}

func (p *Provider) fetchEpisode(request provider.FetchRequest) (*provider.Metadata, error) {
	if request.Season <= 0 || request.Episode <= 0 {
		return nil, &provider.ProviderError{Provider: providerName, Code: "INVALID_REQUEST", Message: "episode fetch requires valid season and episode numbers", Retry: false}
	}

	record, err := p.searchSeriesRecord(request)
	if err != nil {
		return nil, err
	}

	seasonNum := int64(request.Season)
	episodeNum := int64(request.Episode)
	episodes, err := p.client.GetSeriesEpisodes(operations.GetSeriesEpisodesRequest{
		ID:            float64(record.ID),
		SeasonType:    "official",
		Season:        &seasonNum,
		EpisodeNumber: &episodeNum,
		Page:          0,
	})
	if err != nil {
		return nil, p.mapError(err)
	}
	if episodes == nil || episodes.Data == nil || len(episodes.Data.Episodes) == 0 {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "episode not found", Retry: false}
	}

	var episode *shared.EpisodeBaseRecord
	for i := range episodes.Data.Episodes {
		e := episodes.Data.Episodes[i]
		if e.Number != nil && int(*e.Number) == request.Episode {
			episode = &e
			break
		}
	}
	if episode == nil {
		episode = &episodes.Data.Episodes[0]
	}

	title := record.Name
	if episodes.Data.Series != nil && episodes.Data.Series.Name != nil && strings.TrimSpace(*episodes.Data.Series.Name) != "" {
		title = strings.TrimSpace(*episodes.Data.Series.Name)
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       title,
			Year:        firstNonEmptyString(pointerToString(episode.Year), record.Year),
			SeasonNum:   request.Season,
			EpisodeNum:  request.Episode,
			EpisodeName: pointerToString(episode.Name),
			MediaType:   provider.MediaTypeEpisode,
			Overview:    pointerToString(episode.Overview),
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.85,
	}

	if runtime := pointerToInt64(episode.Runtime); runtime > 0 {
		metadata.Extended["runtime"] = int(runtime)
		metadata.Sources["runtime"] = providerName
	}
	if metadata.Core.EpisodeName != "" {
		metadata.Sources["episode_title"] = providerName
	}
	if metadata.Core.Overview != "" {
		metadata.Sources["overview"] = providerName
	}
	if metadata.Core.Title != "" {
		metadata.Sources["title"] = providerName
	}
	if metadata.Core.Year != "" {
		metadata.Sources["year"] = providerName
	}

	seriesExt, extErr := p.client.GetSeriesExtended(float64(record.ID), nil, nil)
	if extErr == nil && seriesExt != nil && seriesExt.Data != nil {
		metadata.Core.Rating = pointerToFloat32(seriesExt.Data.Score)
		if metadata.Core.Rating > 0 {
			metadata.Sources["rating"] = providerName
		}
		if imdbID := findRemoteID(seriesExt.Data.RemoteIds, "imdb"); imdbID != "" {
			metadata.IDs["imdb_id"] = imdbID
			metadata.Sources["imdb_id"] = providerName
		}
	}

	return metadata, nil
}

type searchRecord struct {
	ID   int64
	Name string
	Year string
}

func (p *Provider) searchSeriesRecord(request provider.FetchRequest) (*searchRecord, error) {
	query := strings.TrimSpace(request.Name)
	if request.ID != "" {
		query = strings.TrimSpace(request.ID)
	}
	if query == "" {
		return nil, &provider.ProviderError{Provider: providerName, Code: "INVALID_REQUEST", Message: "series fetch requires a title", Retry: false}
	}

	req := operations.GetSearchResultsRequest{Query: &query}
	typeSeries := "series"
	req.Type = &typeSeries
	if yr, err := strconv.Atoi(strings.TrimSpace(request.Year)); err == nil {
		yf := float64(yr)
		req.Year = &yf
	}

	resp, err := p.client.GetSearchResults(req)
	if err != nil {
		return nil, p.mapError(err)
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: fmt.Sprintf("no results found for show: %s", request.Name), Retry: false}
	}

	for _, candidate := range resp.Data {
		r := toSearchRecord(candidate)
		if r.ID == 0 {
			continue
		}
		if strings.EqualFold(pointerToString(candidate.Type), "series") {
			return r, nil
		}
	}

	return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "series not found", Retry: false}
}

func (p *Provider) searchMovieRecord(request provider.FetchRequest) (*searchRecord, error) {
	query := strings.TrimSpace(request.Name)
	if query == "" {
		return nil, &provider.ProviderError{Provider: providerName, Code: "INVALID_REQUEST", Message: "movie fetch requires a title", Retry: false}
	}

	req := operations.GetSearchResultsRequest{Query: &query}
	typeMovie := "movie"
	req.Type = &typeMovie
	if yr, err := strconv.Atoi(strings.TrimSpace(request.Year)); err == nil {
		yf := float64(yr)
		req.Year = &yf
	}

	resp, err := p.client.GetSearchResults(req)
	if err != nil {
		return nil, p.mapError(err)
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: fmt.Sprintf("no results found for movie: %s", request.Name), Retry: false}
	}

	for _, candidate := range resp.Data {
		r := toSearchRecord(candidate)
		if r.ID == 0 {
			continue
		}
		if strings.EqualFold(pointerToString(candidate.Type), "movie") {
			return r, nil
		}
	}

	return nil, &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: "movie not found", Retry: false}
}

func toSearchRecord(result shared.SearchResult) *searchRecord {
	id := parseInt64(pointerToString(result.TvdbID))
	if id == 0 {
		id = parseInt64(pointerToString(result.ID))
	}

	name := firstNonEmptyString(pointerToString(result.Name), pointerToString(result.NameTranslated), pointerToString(result.Title))
	year := pointerToString(result.Year)

	return &searchRecord{ID: id, Name: name, Year: year}
}

func findRemoteID(ids []shared.RemoteID, source string) string {
	needle := strings.ToLower(strings.TrimSpace(source))
	for _, remote := range ids {
		sourceName := strings.ToLower(strings.TrimSpace(pointerToString(remote.SourceName)))
		if strings.Contains(sourceName, needle) {
			return strings.TrimSpace(pointerToString(remote.ID))
		}
	}
	return ""
}

func pointerToString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func pointerToFloat32(value *float64) float32 {
	if value == nil {
		return 0
	}
	return float32(*value)
}

func pointerToInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringSliceContains(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}

func (p *Provider) mapError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "401"), strings.Contains(lower, "unauthorized"), strings.Contains(lower, "apikey"):
		return &provider.ProviderError{Provider: providerName, Code: "AUTH_FAILED", Message: "TVDB authentication failed: " + msg, Retry: false}
	case strings.Contains(lower, "429"), strings.Contains(lower, "too many"):
		return &provider.ProviderError{Provider: providerName, Code: "RATE_LIMITED", Message: msg, Retry: true, RetryAfter: 5}
	case strings.Contains(lower, "404"), strings.Contains(lower, "not found"):
		return &provider.ProviderError{Provider: providerName, Code: "NOT_FOUND", Message: msg, Retry: false}
	case strings.Contains(lower, "503"), strings.Contains(lower, "unavailable"):
		return &provider.ProviderError{Provider: providerName, Code: "UNAVAILABLE", Message: msg, Retry: true, RetryAfter: 30}
	default:
		return &provider.ProviderError{Provider: providerName, Code: "UNKNOWN", Message: msg, Retry: false}
	}
}
