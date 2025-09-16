package tmdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/patrickmn/go-cache"
	"github.com/ryanbradynd05/go-tmdb"
)

// Fetch retrieves metadata based on the request
func (p *Provider) Fetch(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if p.client == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	// Build cache key
	cacheKey := p.buildCacheKey(request)

	// Check cache first
	if p.cache != nil {
		if cached, found := p.cache.Get(cacheKey); found {
			if meta, ok := cached.(*provider.Metadata); ok {
				return meta, nil
			}
		}
	}

	// Fetch based on media type
	var metadata *provider.Metadata
	var err error

	switch request.MediaType {
	case provider.MediaTypeMovie:
		metadata, err = p.fetchMovie(ctx, request)
	case provider.MediaTypeShow:
		metadata, err = p.fetchShow(ctx, request)
	case provider.MediaTypeSeason:
		metadata, err = p.fetchSeason(ctx, request)
	case provider.MediaTypeEpisode:
		metadata, err = p.fetchEpisode(ctx, request)
	default:
		return nil, fmt.Errorf("unsupported media type: %s", request.MediaType)
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	if p.cache != nil && metadata != nil {
		p.cache.Set(cacheKey, metadata, cache.DefaultExpiration)
	}

	return metadata, nil
}

// fetchMovie fetches movie metadata
func (p *Provider) fetchMovie(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	options := map[string]string{
		"language": p.getLanguage(request),
	}
	if request.Year != "" {
		options["year"] = request.Year
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	// Search for the movie
	results, err := p.client.SearchMovie(request.Name, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if results == nil || len(results.Results) == 0 {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  fmt.Sprintf("no results found for movie: %s", request.Name),
			Retry:    false,
		}
	}

	// Take the first result
	movie := results.Results[0]

	// Always get full movie details for complete metadata
	var fullMovie *tmdb.Movie
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	fullMovie, err = p.client.GetMovieInfo(movie.ID, options)
	if err != nil {
		// Fall back to search result data
		return p.movieSearchResultToMetadata(&movie), nil
	}

	if fullMovie != nil {
		return p.movieToMetadata(fullMovie), nil
	}
	return p.movieSearchResultToMetadata(&movie), nil
}

// fetchShow fetches TV show metadata
func (p *Provider) fetchShow(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	options := map[string]string{
		"language":           p.getLanguage(request),
		"append_to_response": "external_ids",
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	// Search for the show
	results, err := p.client.SearchTv(request.Name, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if results == nil || len(results.Results) == 0 {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  fmt.Sprintf("no results found for show: %s", request.Name),
			Retry:    false,
		}
	}

	// Take the first result
	show := results.Results[0]

	// Always get full show details for complete metadata
	var fullShow *tmdb.TV
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	fullShow, err = p.client.GetTvInfo(show.ID, options)
	if err != nil {
		// Fall back to search result data
		return p.tvSearchResultToMetadata(&show), nil
	}

	if fullShow != nil {
		return p.tvToMetadata(fullShow), nil
	}
	return p.tvSearchResultToMetadata(&show), nil
}

// fetchSeason fetches season metadata
func (p *Provider) fetchSeason(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	// First need to get show ID
	showID, err := p.getShowID(ctx, request)
	if err != nil {
		return nil, err
	}

	options := map[string]string{
		"language": p.getLanguage(request),
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	season, err := p.client.GetTvSeasonInfo(showID, request.Season, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if season == nil {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  fmt.Sprintf("season %d not found", request.Season),
			Retry:    false,
		}
	}

	return p.seasonToMetadata(season, showID), nil
}

// fetchEpisode fetches episode metadata
func (p *Provider) fetchEpisode(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	// First need to get show ID
	showID, err := p.getShowID(ctx, request)
	if err != nil {
		return nil, err
	}

	options := map[string]string{
		"language": p.getLanguage(request),
	}

	// Apply rate limiting
	if err := p.rateLimiter.wait(); err != nil {
		return nil, p.mapError(err)
	}

	episode, err := p.client.GetTvEpisodeInfo(showID, request.Season, request.Episode, options)
	if err != nil {
		return nil, p.mapError(err)
	}

	if episode == nil {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  fmt.Sprintf("episode S%02dE%02d not found", request.Season, request.Episode),
			Retry:    false,
		}
	}

	// Also get show info for the series name (with external IDs)
	var show *tmdb.TV
	if err := p.rateLimiter.wait(); err == nil {
		optionsWithExternal := map[string]string{
			"language":           p.getLanguage(request),
			"append_to_response": "external_ids",
		}
		show, _ = p.client.GetTvInfo(showID, optionsWithExternal)
	}

	return p.episodeToMetadata(episode, show, showID), nil
}

// getShowID gets the TMDB show ID from a request
func (p *Provider) getShowID(ctx context.Context, request provider.FetchRequest) (int, error) {
	// Check if ID is provided
	if request.ID != "" {
		id, err := strconv.Atoi(request.ID)
		if err == nil {
			return id, nil
		}
	}

	// Search for the show
	options := map[string]string{
		"language": p.getLanguage(request),
	}

	if err := p.rateLimiter.wait(); err != nil {
		return 0, p.mapError(err)
	}

	results, err := p.client.SearchTv(request.Name, options)
	if err != nil {
		return 0, p.mapError(err)
	}

	if results == nil || len(results.Results) == 0 {
		return 0, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  fmt.Sprintf("show not found: %s", request.Name),
			Retry:    false,
		}
	}

	return results.Results[0].ID, nil
}

// Conversion functions

func (p *Provider) movieSearchResultToMetadata(movie *tmdb.MovieShort) *provider.Metadata {
	releaseYear := ""
	if movie.ReleaseDate != "" && len(movie.ReleaseDate) >= 4 {
		releaseYear = movie.ReleaseDate[:4]
	}

	return &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     movie.Title,
			Year:      releaseYear,
			MediaType: provider.MediaTypeMovie,
			Overview:  movie.Overview,
			Rating:    movie.VoteAverage,
		},
		Extended: map[string]interface{}{
			"popularity": movie.Popularity,
			"vote_count": movie.VoteCount,
		},
		IDs: map[string]string{
			"tmdb_id": fmt.Sprintf("%d", movie.ID),
		},
		Sources: map[string]string{
			"title":    providerName,
			"year":     providerName,
			"overview": providerName,
			"rating":   providerName,
		},
		Confidence: 0.8, // Lower confidence for search results
	}
}

func (p *Provider) movieToMetadata(movie *tmdb.Movie) *provider.Metadata {
	releaseYear := ""
	if movie.ReleaseDate != "" && len(movie.ReleaseDate) >= 4 {
		releaseYear = movie.ReleaseDate[:4]
	}

	genres := make([]string, 0, len(movie.Genres))
	for _, g := range movie.Genres {
		genres = append(genres, g.Name)
	}

	extended := map[string]interface{}{
		"popularity": movie.Popularity,
		"vote_count": movie.VoteCount,
		"tagline":    movie.Tagline,
	}

	if movie.Budget > 0 {
		extended["budget"] = movie.Budget
	}
	if movie.Revenue > 0 {
		extended["revenue"] = movie.Revenue
	}
	if movie.Homepage != "" {
		extended["homepage"] = movie.Homepage
	}

	// Add production companies
	if len(movie.ProductionCompanies) > 0 {
		companies := []string{}
		for _, c := range movie.ProductionCompanies {
			companies = append(companies, c.Name)
		}
		extended["production_companies"] = strings.Join(companies, ", ")
		extended["studios"] = companies[0] // First one as primary studio
	}

	ids := map[string]string{
		"tmdb_id": fmt.Sprintf("%d", movie.ID),
	}
	if movie.ImdbID != "" {
		ids["imdb_id"] = movie.ImdbID
	}

	return &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     movie.Title,
			Year:      releaseYear,
			MediaType: provider.MediaTypeMovie,
			Overview:  movie.Overview,
			Rating:    movie.VoteAverage,
			Genres:    genres,
		},
		Extended: extended,
		IDs:      ids,
		Sources: map[string]string{
			"title":    providerName,
			"year":     providerName,
			"overview": providerName,
			"rating":   providerName,
			"genres":   providerName,
			"runtime":  providerName,
		},
		Confidence: 1.0, // Full confidence for detailed results
	}
}

func (p *Provider) tvSearchResultToMetadata(show interface{}) *provider.Metadata {
	// Handle both tmdb.TvShort and the inline struct type from search results
	var id int
	var name string
	var firstAirDate string
	var voteAverage float32
	var popularity float32
	var voteCount uint32
	var originCountry []string

	// Use type assertion to handle different types
	switch s := show.(type) {
	case *tmdb.TvShort:
		id = s.ID
		name = s.Name
		firstAirDate = s.FirstAirDate
		voteAverage = s.VoteAverage
		popularity = s.Popularity
		voteCount = s.VoteCount
		originCountry = s.OriginCountry
	case *struct {
		BackdropPath  string `json:"backdrop_path"`
		ID            int
		OriginalName  string   `json:"original_name"`
		FirstAirDate  string   `json:"first_air_date"`
		OriginCountry []string `json:"origin_country"`
		PosterPath    string   `json:"poster_path"`
		Popularity    float32
		Name          string
		VoteAverage   float32 `json:"vote_average"`
		VoteCount     uint32  `json:"vote_count"`
	}:
		id = s.ID
		name = s.Name
		firstAirDate = s.FirstAirDate
		voteAverage = s.VoteAverage
		popularity = s.Popularity
		voteCount = s.VoteCount
		originCountry = s.OriginCountry
	default:
		// If we can't determine the type, return empty metadata
		return &provider.Metadata{
			Core:       provider.CoreMetadata{MediaType: provider.MediaTypeShow},
			Confidence: 0,
		}
	}

	firstAirYear := ""
	if firstAirDate != "" && len(firstAirDate) >= 4 {
		firstAirYear = firstAirDate[:4]
	}

	return &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     name,
			Year:      firstAirYear,
			MediaType: provider.MediaTypeShow,
			Rating:    voteAverage,
		},
		Extended: map[string]interface{}{
			"popularity":     popularity,
			"vote_count":     voteCount,
			"origin_country": strings.Join(originCountry, ", "),
		},
		IDs: map[string]string{
			"tmdb_id": fmt.Sprintf("%d", id),
		},
		Sources: map[string]string{
			"title":  providerName,
			"year":   providerName,
			"rating": providerName,
		},
		Confidence: 0.8,
	}
}

func (p *Provider) tvToMetadata(show *tmdb.TV) *provider.Metadata {
	firstAirYear := ""
	if show.FirstAirDate != "" && len(show.FirstAirDate) >= 4 {
		firstAirYear = show.FirstAirDate[:4]
	}

	genres := make([]string, 0, len(show.Genres))
	for _, g := range show.Genres {
		genres = append(genres, g.Name)
	}

	extended := map[string]interface{}{
		"popularity":    show.Popularity,
		"vote_count":    show.VoteCount,
		"season_count":  show.NumberOfSeasons,
		"episode_count": show.NumberOfEpisodes,
		"in_production": show.InProduction,
		"type":          show.Type,
	}

	if show.Homepage != "" {
		extended["homepage"] = show.Homepage
	}

	// Add networks
	if len(show.Networks) > 0 {
		networks := []string{}
		for _, n := range show.Networks {
			networks = append(networks, n.Name)
		}
		extended["networks"] = strings.Join(networks, ", ")
	}

	// Add production companies
	if len(show.ProductionCompanies) > 0 {
		companies := []string{}
		for _, c := range show.ProductionCompanies {
			companies = append(companies, c.Name)
		}
		extended["production_companies"] = strings.Join(companies, ", ")
	}

	ids := map[string]string{
		"tmdb_id": fmt.Sprintf("%d", show.ID),
	}

	// Add IMDB ID if available from external IDs
	if show.ExternalIDs != nil && show.ExternalIDs.ImdbID != "" {
		ids["imdb_id"] = show.ExternalIDs.ImdbID
	}

	return &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     show.Name,
			Year:      firstAirYear,
			MediaType: provider.MediaTypeShow,
			Overview:  show.Overview,
			Rating:    show.VoteAverage,
			Genres:    genres,
		},
		Extended: extended,
		IDs:      ids,
		Sources: map[string]string{
			"title":    providerName,
			"year":     providerName,
			"overview": providerName,
			"rating":   providerName,
			"genres":   providerName,
			"runtime":  providerName,
		},
		Confidence: 1.0,
	}
}

func (p *Provider) seasonToMetadata(season *tmdb.TvSeason, showID int) *provider.Metadata {
	return &provider.Metadata{
		Core: provider.CoreMetadata{
			SeasonNum: season.SeasonNumber,
			MediaType: provider.MediaTypeSeason,
			Overview:  season.Overview,
		},
		Extended: map[string]interface{}{
			"episode_count": len(season.Episodes),
		},
		IDs: map[string]string{
			"tmdb_show_id":   fmt.Sprintf("%d", showID),
			"tmdb_season_id": fmt.Sprintf("%d", season.ID),
		},
		Sources: map[string]string{
			"season_name": providerName,
			"overview":    providerName,
		},
		Confidence: 1.0,
	}
}

func (p *Provider) episodeToMetadata(episode *tmdb.TvEpisode, show *tmdb.TV, showID int) *provider.Metadata {
	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			EpisodeName: episode.Name,
			SeasonNum:   episode.SeasonNumber,
			EpisodeNum:  episode.EpisodeNumber,
			MediaType:   provider.MediaTypeEpisode,
			Overview:    episode.Overview,
			Rating:      episode.VoteAverage,
		},
		Extended: map[string]interface{}{
			"vote_count":      episode.VoteCount,
			"production_code": episode.ProductionCode,
			"still_path":      episode.StillPath,
		},
		IDs: map[string]string{
			"tmdb_show_id":    fmt.Sprintf("%d", showID),
			"tmdb_episode_id": fmt.Sprintf("%d", episode.ID),
		},
		Sources: map[string]string{
			"episode_name": providerName,
			"overview":     providerName,
			"rating":       providerName,
		},
		Confidence: 1.0,
	}

	if show != nil {
		meta.Core.Title = show.Name
		if show.FirstAirDate != "" && len(show.FirstAirDate) >= 4 {
			meta.Core.Year = show.FirstAirDate[:4]
		}
	}

	// Add guest stars if available
	if len(episode.GuestStars) > 0 {
		stars := []string{}
		for _, s := range episode.GuestStars {
			stars = append(stars, s.Name)
		}
		meta.Extended["guest_stars"] = strings.Join(stars, ", ")
	}

	// Add crew if available
	if len(episode.Crew) > 0 {
		directors := []string{}
		writers := []string{}
		for _, c := range episode.Crew {
			switch c.Job {
			case "Director":
				directors = append(directors, c.Name)
			case "Writer", "Screenplay":
				writers = append(writers, c.Name)
			}
		}
		if len(directors) > 0 {
			meta.Extended["directors"] = strings.Join(directors, ", ")
		}
		if len(writers) > 0 {
			meta.Extended["writers"] = strings.Join(writers, ", ")
		}
	}

	return meta
}

// Helper functions

func (p *Provider) buildCacheKey(request provider.FetchRequest) string {
	parts := []string{
		string(request.MediaType),
		request.Name,
		request.Year,
		fmt.Sprintf("%d", request.Season),
		fmt.Sprintf("%d", request.Episode),
		request.Language,
	}
	return strings.Join(parts, ":")
}

func (p *Provider) getLanguage(request provider.FetchRequest) string {
	if request.Language != "" {
		return request.Language
	}
	return p.language
}
