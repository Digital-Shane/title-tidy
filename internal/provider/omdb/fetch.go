package omdb

import (
	"context"
	"strconv"
	"strings"

	"github.com/Digital-Shane/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

func (p *Provider) fetchMovie(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if request.ID == "" && strings.TrimSpace(request.Name) == "" {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "movie fetch requires a title or an IMDb ID",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var result any
	var err error

	if request.ID != "" {
		result, err = p.client.SearchByImdbID(omdb.QueryData{ImdbID: request.ID})
	} else {
		query := omdb.QueryData{
			Title:      request.Name,
			Year:       request.Year,
			SearchType: "movie",
			Plot:       "full",
		}
		result, err = p.client.SearchByTitle(query)
	}

	if err != nil {
		return nil, p.mapError(err)
	}

	switch movie := result.(type) {
	case omdb.MovieResult:
		return p.movieResultToMetadata(movie), nil
	case *omdb.MovieResult:
		return p.movieResultToMetadata(*movie), nil
	default:
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  "movie not found",
		}
	}
}

func (p *Provider) fetchShow(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if request.ID == "" && strings.TrimSpace(request.Name) == "" {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "show fetch requires a title or an IMDb ID",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var result any
	var err error

	if request.ID != "" {
		result, err = p.client.SearchByImdbID(omdb.QueryData{ImdbID: request.ID})
	} else {
		query := omdb.QueryData{
			Title:      request.Name,
			Year:       request.Year,
			SearchType: "series",
			Plot:       "full",
		}
		result, err = p.client.SearchByTitle(query)
	}

	if err != nil {
		return nil, p.mapError(err)
	}

	switch series := result.(type) {
	case omdb.SeriesResult:
		return p.seriesResultToMetadata(series), nil
	case *omdb.SeriesResult:
		return p.seriesResultToMetadata(*series), nil
	default:
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  "series not found",
		}
	}
}

func (p *Provider) fetchSeason(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if request.Season <= 0 {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "season fetch requires a valid season number",
		}
	}

	title := strings.TrimSpace(request.Name)
	if request.ID == "" && title == "" {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "season fetch requires a title or an IMDb ID",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query := omdb.QueryData{
		Title:  title,
		Year:   request.Year,
		Season: strconv.Itoa(request.Season),
	}

	var result any
	var err error

	if request.ID != "" {
		query.ImdbID = strings.TrimSpace(request.ID)
		result, err = p.client.SearchByImdbID(query)
	} else {
		result, err = p.client.SearchByTitle(query)
	}

	if err != nil {
		return nil, p.mapError(err)
	}

	switch season := result.(type) {
	case omdb.SeasonResult:
		return p.seasonResultToMetadata(&season, request), nil
	case *omdb.SeasonResult:
		return p.seasonResultToMetadata(season, request), nil
	default:
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  "season not found",
		}
	}
}

func (p *Provider) fetchEpisode(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if request.Season <= 0 || request.Episode <= 0 {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "episode fetch requires valid season and episode numbers",
		}
	}

	title := strings.TrimSpace(request.Name)
	if request.ID == "" && title == "" {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_REQUEST",
			Message:  "episode fetch requires a title or an IMDb ID",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query := omdb.QueryData{
		Title:   title,
		Year:    request.Year,
		Season:  strconv.Itoa(request.Season),
		Episode: strconv.Itoa(request.Episode),
		Plot:    "full",
	}

	var result any
	var err error

	if request.ID != "" {
		query.ImdbID = strings.TrimSpace(request.ID)
		result, err = p.client.SearchByImdbID(query)
	} else {
		result, err = p.client.SearchByTitle(query)
	}

	if err != nil {
		return nil, p.mapError(err)
	}

	switch episode := result.(type) {
	case omdb.EpisodeResult:
		return p.episodeResultToMetadata(&episode, request), nil
	case *omdb.EpisodeResult:
		return p.episodeResultToMetadata(episode, request), nil
	default:
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "NOT_FOUND",
			Message:  "episode not found",
		}
	}
}

func (p *Provider) movieResultToMetadata(result omdb.MovieResult) *provider.Metadata {
	genres := omdb.SplitAndTrim(result.Genre)

	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     result.Title,
			Year:      omdb.FirstYear(result.Year),
			MediaType: provider.MediaTypeMovie,
			Overview:  result.Plot,
			Rating:    omdb.ParseRating(result.ImdbRating),
			Genres:    genres,
			Language:  result.Language,
			Country:   result.Country,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.9,
	}

	if result.ImdbID != "" {
		meta.IDs["imdb_id"] = result.ImdbID
		meta.Sources["imdb_id"] = providerName
	}

	if result.Runtime != "" {
		meta.Extended["runtime"] = parseRuntime(result.Runtime)
		meta.Sources["runtime"] = providerName
	}

	if len(genres) > 0 {
		meta.Sources["genres"] = providerName
	}

	if result.Plot != "" {
		meta.Sources["overview"] = providerName
	}
	meta.Sources["rating"] = providerName
	meta.Sources["title"] = providerName
	meta.Sources["year"] = providerName

	return meta
}

func (p *Provider) seriesResultToMetadata(result omdb.SeriesResult) *provider.Metadata {
	genres := omdb.SplitAndTrim(result.Genre)

	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     result.Title,
			Year:      omdb.FirstYear(result.Year),
			MediaType: provider.MediaTypeShow,
			Overview:  result.Plot,
			Rating:    omdb.ParseRating(result.ImdbRating),
			Genres:    genres,
			Language:  result.Language,
			Country:   result.Country,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.85,
	}

	if result.ImdbID != "" {
		meta.IDs["imdb_id"] = result.ImdbID
		meta.Sources["imdb_id"] = providerName
	}

	if result.TotalSeasons != "" {
		meta.Extended["total_seasons"] = result.TotalSeasons
	}

	if result.Runtime != "" {
		meta.Extended["runtime"] = parseRuntime(result.Runtime)
		meta.Sources["runtime"] = providerName
	}

	if result.Plot != "" {
		meta.Sources["overview"] = providerName
	}

	meta.Sources["rating"] = providerName
	meta.Sources["title"] = providerName
	meta.Sources["year"] = providerName
	if len(genres) > 0 {
		meta.Sources["genres"] = providerName
	}

	return meta
}

func (p *Provider) seasonResultToMetadata(resp *omdb.SeasonResult, request provider.FetchRequest) *provider.Metadata {
	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:     request.Name,
			Year:      omdb.FirstYearFromEpisodes(resp.Episodes),
			SeasonNum: request.Season,
			MediaType: provider.MediaTypeSeason,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.7,
	}

	if resp.Title != "" {
		meta.Core.Title = resp.Title
	}

	if request.ID != "" {
		meta.IDs["imdb_id"] = request.ID
		meta.Sources["imdb_id"] = providerName
	}

	meta.Extended["episode_count"] = len(resp.Episodes)

	return meta
}

func (p *Provider) episodeResultToMetadata(resp *omdb.EpisodeResult, request provider.FetchRequest) *provider.Metadata {
	genres := omdb.SplitAndTrim(resp.Genre)

	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       request.Name,
			Year:        omdb.FirstYear(resp.Released),
			SeasonNum:   request.Season,
			EpisodeNum:  request.Episode,
			EpisodeName: resp.Title,
			MediaType:   provider.MediaTypeEpisode,
			Overview:    resp.Plot,
			Rating:      omdb.ParseRating(resp.ImdbRating),
			Genres:      genres,
			Language:    resp.Language,
			Country:     resp.Country,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 0.85,
	}

	if request.Name == "" && resp.SeriesID != "" {
		meta.Core.Title = resp.SeriesID
	}

	if resp.ImdbID != "" {
		meta.IDs["imdb_id"] = resp.ImdbID
		meta.Sources["imdb_id"] = providerName
	}
	if resp.SeriesID != "" {
		meta.IDs["series_id"] = resp.SeriesID
	}

	if resp.Runtime != "" {
		meta.Extended["runtime"] = parseRuntime(resp.Runtime)
		meta.Sources["runtime"] = providerName
	}

	meta.Sources["episode_title"] = providerName
	meta.Sources["rating"] = providerName
	if meta.Core.Overview != "" {
		meta.Sources["overview"] = providerName
	}
	if len(genres) > 0 {
		meta.Sources["genres"] = providerName
	}

	return meta
}
