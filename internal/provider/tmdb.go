package provider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	tmdb "github.com/ryanbradynd05/go-tmdb"
)

var (
	ErrNoResults      = errors.New("no results found")
	ErrInvalidAPIKey  = errors.New("invalid API key")
	ErrRateLimited    = errors.New("rate limited")
	ErrAPIUnavailable = errors.New("API unavailable")

	// yearRegex matches 4-digit years between 1900-2100
	yearRegex = regexp.MustCompile(`(19[0-9]{2}|20[0-9]{2}|2100)`)
)

// tvSearchResult represents a TV show result from TMDB search (matches the inline struct in tmdb.TvSearchResults.Results)
type tvSearchResult struct {
	BackdropPath  string   `json:"backdrop_path"`
	ID            int      `json:"id"`
	OriginalName  string   `json:"original_name"`
	FirstAirDate  string   `json:"first_air_date"`
	OriginCountry []string `json:"origin_country"`
	PosterPath    string   `json:"poster_path"`
	Popularity    float32  `json:"popularity"`
	Name          string   `json:"name"`
	VoteAverage   float32  `json:"vote_average"`
	VoteCount     uint32   `json:"vote_count"`
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

// TMDBProvider implements the MetadataProvider interface using TMDB
type TMDBProvider struct {
	client    TMDBClient
	cache     *cache.Cache
	cacheFile string
	language  string

	// Rate limiting
	rateLimiter *rateLimiter
}

// NewTMDBProvider creates a new TMDB provider instance
func NewTMDBProvider(apiKey, language string) (*TMDBProvider, error) {
	if apiKey == "" {
		return nil, ErrInvalidAPIKey
	}

	if language == "" {
		language = "en-US"
	}

	config := tmdb.Config{
		APIKey:   apiKey,
		Proxies:  nil,
		UseProxy: false,
	}

	client := tmdb.Init(config)

	// Set up cache file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".title-tidy", "tmdb_cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cacheFile := filepath.Join(cacheDir, "tmdb_cache.gob")

	// Create cache with 7 day expiration, 10 minute cleanup interval
	c := cache.New(7*24*time.Hour, 10*time.Minute)

	// Try to load existing cache from disk
	if _, err := os.Stat(cacheFile); err == nil {
		_ = c.LoadFile(cacheFile)
	}

	return &TMDBProvider{
		client:      client, // tmdb.TMDb implements TMDBClient directly
		cache:       c,
		cacheFile:   cacheFile,
		language:    language,
		rateLimiter: newRateLimiter(38, 10*time.Second), // 38 requests per 10 seconds (under 40 limit)
	}, nil
}

// SearchMovie searches for a movie by name and optionally year
func (p *TMDBProvider) SearchMovie(name, year string) (*EnrichedMetadata, error) {
	if name == "" {
		return nil, errors.New("movie name is required")
	}

	cacheKey := fmt.Sprintf("movie:%s:%s:%s", name, year, p.language)

	// Check memory cache first
	if cached, found := p.cache.Get(cacheKey); found {
		if meta, ok := cached.(*EnrichedMetadata); ok {
			return meta, nil
		}
	}

	options := map[string]string{
		"language": p.language,
	}
	if year != "" {
		options["year"] = year
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	results, err := p.client.SearchMovie(name, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if results == nil || len(results.Results) == 0 {
		return nil, ErrNoResults
	}

	// Take the first result (best match)
	movie := results.Results[0]

	// Get full movie details
	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	fullMovie, err := p.client.GetMovieInfo(movie.ID, options)
	if err != nil {
		// Fall back to search result data
		meta := p.movieSearchResultToMetadata(&movie, name, year)
		// Store in cache
		p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
		return meta, nil
	}

	meta := p.movieToMetadata(fullMovie, name, year)
	// Store in cache
	p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
	return meta, nil
}

// SearchTVShow searches for a TV show by name
func (p *TMDBProvider) SearchTVShow(name string) (*EnrichedMetadata, error) {
	if name == "" {
		return nil, errors.New("show name is required")
	}

	cacheKey := fmt.Sprintf("tvshow:%s:%s", name, p.language)

	// Check memory cache first
	if cached, found := p.cache.Get(cacheKey); found {
		if meta, ok := cached.(*EnrichedMetadata); ok {
			return meta, nil
		}
	}

	options := map[string]string{
		"language": p.language,
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	results, err := p.client.SearchTv(name, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if results == nil || len(results.Results) == 0 {
		return nil, ErrNoResults
	}

	// Take the first result (best match)
	show := results.Results[0]

	// Get full show details
	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	fullShow, err := p.client.GetTvInfo(show.ID, options)
	if err != nil {
		// Fall back to search result data
		meta := p.tvSearchResultToMetadata((*tvSearchResult)(&show), name)
		// Store in cache
		p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
		return meta, nil
	}

	meta := p.tvToMetadata(fullShow, name)
	// Store in cache
	p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
	return meta, nil
}

// GetSeasonInfo gets information about a specific season
func (p *TMDBProvider) GetSeasonInfo(showID, seasonNum int) (*EnrichedMetadata, error) {
	if showID == 0 || seasonNum < 0 {
		return nil, errors.New("invalid show ID or season number")
	}

	cacheKey := fmt.Sprintf("season:%d:%d:%s", showID, seasonNum, p.language)

	// Check memory cache first
	if cached, found := p.cache.Get(cacheKey); found {
		if meta, ok := cached.(*EnrichedMetadata); ok {
			return meta, nil
		}
	}

	options := map[string]string{
		"language": p.language,
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	season, err := p.client.GetTvSeasonInfo(showID, seasonNum, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if season == nil {
		return nil, ErrNoResults
	}

	meta := p.seasonToMetadata(season, showID)
	// Store in cache
	p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
	return meta, nil
}

// GetEpisodeInfo gets information about a specific episode
func (p *TMDBProvider) GetEpisodeInfo(showID, season, episode int) (*EnrichedMetadata, error) {
	if showID == 0 || season < 0 || episode < 1 {
		return nil, errors.New("invalid show ID, season, or episode number")
	}

	cacheKey := fmt.Sprintf("episode:%d:%d:%d:%s", showID, season, episode, p.language)

	// Check memory cache first
	if cached, found := p.cache.Get(cacheKey); found {
		if meta, ok := cached.(*EnrichedMetadata); ok {
			return meta, nil
		}
	}

	options := map[string]string{
		"language": p.language,
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, err
	}

	ep, err := p.client.GetTvEpisodeInfo(showID, season, episode, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if ep == nil {
		return nil, ErrNoResults
	}

	// Also get show info for the series name
	// Apply rate limiting for the second request
	var show *tmdb.TV
	if err := p.rateLimiter.wait(); err == nil {
		show, _ = p.client.GetTvInfo(showID, options)
	}

	meta := p.episodeToMetadata(ep, show, showID)
	// Store in cache
	p.cache.Set(cacheKey, meta, cache.DefaultExpiration)
	return meta, nil
}

// Helper functions to convert TMDB types to EnrichedMetadata

func (p *TMDBProvider) movieSearchResultToMetadata(movie *tmdb.MovieShort, localName, localYear string) *EnrichedMetadata {
	releaseYear := ""
	if movie.ReleaseDate != "" && len(movie.ReleaseDate) >= 4 {
		releaseYear = movie.ReleaseDate[:4]
	}

	return &EnrichedMetadata{
		Title:     movie.Title,
		Year:      releaseYear,
		Overview:  movie.Overview,
		Rating:    movie.VoteAverage,
		ID:        movie.ID,
		LocalName: localName,
		LocalYear: localYear,
	}
}

func (p *TMDBProvider) movieToMetadata(movie *tmdb.Movie, localName, localYear string) *EnrichedMetadata {
	releaseYear := ""
	if movie.ReleaseDate != "" && len(movie.ReleaseDate) >= 4 {
		releaseYear = movie.ReleaseDate[:4]
	}

	genres := make([]string, 0, len(movie.Genres))
	for _, g := range movie.Genres {
		genres = append(genres, g.Name)
	}

	return &EnrichedMetadata{
		Title:     movie.Title,
		Year:      releaseYear,
		Overview:  movie.Overview,
		Rating:    movie.VoteAverage,
		Genres:    genres,
		Runtime:   int(movie.Runtime),
		Tagline:   movie.Tagline,
		ID:        movie.ID,
		LocalName: localName,
		LocalYear: localYear,
	}
}

func (p *TMDBProvider) tvSearchResultToMetadata(show *tvSearchResult, localName string) *EnrichedMetadata {
	firstAirYear := ""
	if show.FirstAirDate != "" && len(show.FirstAirDate) >= 4 {
		firstAirYear = show.FirstAirDate[:4]
	}

	return &EnrichedMetadata{
		ShowName:  show.Name,
		Title:     show.Name,
		Year:      firstAirYear,
		Rating:    show.VoteAverage,
		ID:        show.ID,
		LocalName: localName,
	}
}

func (p *TMDBProvider) tvToMetadata(show *tmdb.TV, localName string) *EnrichedMetadata {
	firstAirYear := ""
	if show.FirstAirDate != "" && len(show.FirstAirDate) >= 4 {
		firstAirYear = show.FirstAirDate[:4]
	}

	genres := make([]string, 0, len(show.Genres))
	for _, g := range show.Genres {
		genres = append(genres, g.Name)
	}

	runtime := 0
	if len(show.EpisodeRunTime) > 0 {
		runtime = show.EpisodeRunTime[0]
	}

	return &EnrichedMetadata{
		ShowName:    show.Name,
		Title:       show.Name,
		Year:        firstAirYear,
		Overview:    show.Overview,
		Rating:      show.VoteAverage,
		Genres:      genres,
		Runtime:     runtime,
		SeasonCount: show.NumberOfSeasons,
		ID:          show.ID,
		LocalName:   localName,
	}
}

func (p *TMDBProvider) seasonToMetadata(season *tmdb.TvSeason, showID int) *EnrichedMetadata {
	return &EnrichedMetadata{
		SeasonName:  season.Name,
		SeasonNum:   season.SeasonNumber,
		Overview:    season.Overview,
		EpisodeAir:  season.AirDate,
		ID:          showID,
		LocalSeason: season.SeasonNumber,
	}
}

func (p *TMDBProvider) episodeToMetadata(episode *tmdb.TvEpisode, show *tmdb.TV, showID int) *EnrichedMetadata {
	meta := &EnrichedMetadata{
		EpisodeName:  episode.Name,
		EpisodeAir:   episode.AirDate,
		Overview:     episode.Overview,
		Rating:       episode.VoteAverage,
		SeasonNum:    episode.SeasonNumber,
		EpisodeNum:   episode.EpisodeNumber,
		ID:           showID,
		LocalSeason:  episode.SeasonNumber,
		LocalEpisode: episode.EpisodeNumber,
	}

	if show != nil {
		meta.ShowName = show.Name
		meta.Title = show.Name
		if show.FirstAirDate != "" && len(show.FirstAirDate) >= 4 {
			meta.Year = show.FirstAirDate[:4]
		}
	}

	return meta
}

func (p *TMDBProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
		return ErrInvalidAPIKey
	}
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return ErrRateLimited
	}
	if strings.Contains(errStr, "503") || strings.Contains(errStr, "unavailable") {
		return ErrAPIUnavailable
	}

	return fmt.Errorf("TMDB API error: %w", err)
}

// SetClient sets the TMDB client (for testing)
func (p *TMDBProvider) SetClient(client TMDBClient) {
	p.client = client
}

// SaveCache persists the cache to disk
func (p *TMDBProvider) SaveCache() error {
	if p.cacheFile == "" {
		return nil // No cache file configured
	}

	return p.cache.SaveFile(p.cacheFile)
}

// GetIDFromMetadata extracts the TMDB ID from metadata or performs a search
func (p *TMDBProvider) GetIDFromMetadata(meta *EnrichedMetadata, mediaType string) (int, error) {
	if meta.ID > 0 {
		return meta.ID, nil
	}

	switch mediaType {
	case "movie":
		result, err := p.SearchMovie(meta.LocalName, meta.LocalYear)
		if err != nil {
			return 0, err
		}
		return result.ID, nil
	case "tv":
		result, err := p.SearchTVShow(meta.LocalName)
		if err != nil {
			return 0, err
		}
		return result.ID, nil
	default:
		return 0, errors.New("unknown media type")
	}
}

// ParseYear extracts a year from a string (helper for year extraction)
func ParseYear(s string) string {
	match := yearRegex.FindString(s)
	return match
}

// rateLimiter implements a simple sliding window rate limiter
type rateLimiter struct {
	mu          sync.Mutex
	requests    []time.Time
	maxRequests int
	window      time.Duration
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(maxRequests int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		maxRequests: maxRequests,
		window:      window,
		requests:    make([]time.Time, 0, maxRequests),
	}
}

// wait blocks until a request can be made within rate limits
// This method never returns an error - it always waits until a request can be made
func (r *rateLimiter) wait() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Clean up old requests outside the window
	cutoff := now.Add(-r.window)
	validRequests := make([]time.Time, 0, r.maxRequests)
	for _, req := range r.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	r.requests = validRequests

	// If we're under the limit, allow the request immediately
	if len(r.requests) < r.maxRequests {
		r.requests = append(r.requests, now)
		return nil
	}

	// We need to wait. Calculate when the oldest request will expire
	oldestRequest := r.requests[0]
	waitTime := r.window - now.Sub(oldestRequest)

	// Add a small buffer to ensure the request has actually expired
	waitTime += 10 * time.Millisecond

	// Release the lock before sleeping
	r.mu.Unlock()

	// Wait for the window to allow another request
	time.Sleep(waitTime)

	// Re-acquire lock and record the request
	r.mu.Lock()

	// Clean up again after waiting
	now = time.Now()
	cutoff = now.Add(-r.window)
	validRequests = make([]time.Time, 0, r.maxRequests)
	for _, req := range r.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	r.requests = validRequests

	// Record this request
	r.requests = append(r.requests, now)

	return nil
}
