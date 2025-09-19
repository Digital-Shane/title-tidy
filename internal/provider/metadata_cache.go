package provider

import (
	"context"
	"fmt"
)

// MetadataCache provides safe access to cached provider metadata.
type MetadataCache interface {
	Get(key string) (*Metadata, bool)
	Set(key string, meta *Metadata)
}

// GenerateMetadataKey creates a unique key for caching metadata.
func GenerateMetadataKey(mediaType string, name, year string, season, episode int) string {
	switch mediaType {
	case "movie":
		return fmt.Sprintf("movie:%s:%s", name, year)
	case "show":
		return fmt.Sprintf("show:%s:%s", name, year)
	case "season":
		return fmt.Sprintf("season:%s:%s:%d", name, year, season)
	case "episode":
		return fmt.Sprintf("episode:%s:%s:%d:%d", name, year, season, episode)
	default:
		return ""
	}
}

// FetchMetadataWithDependencies fetches metadata with proper dependency resolution.
// For episodes/seasons, it ensures show metadata is fetched first.
// Returns both metadata and error so callers can handle rate limiting properly.
func FetchMetadataWithDependencies(tmdbProvider Provider, name, year string, season, episode int, isMovie bool, cache MetadataCache) (*Metadata, error) {
	if tmdbProvider == nil || name == "" {
		return nil, nil
	}

	var meta *Metadata
	var err error
	ctx := context.Background()

	getFromCache := func(key string) *Metadata {
		if cache == nil {
			return nil
		}
		meta, _ := cache.Get(key)
		return meta
	}

	setInCache := func(key string, meta *Metadata) {
		if cache == nil || meta == nil {
			return
		}
		cache.Set(key, meta)
	}

	if isMovie {
		// Fetch movie metadata
		request := FetchRequest{
			MediaType: MediaTypeMovie,
			Name:      name,
			Year:      year,
		}
		meta, err = tmdbProvider.Fetch(ctx, request)
	} else if season > 0 && episode > 0 {
		// For episodes, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := getFromCache(showKey)

		if showMeta == nil {
			// Fetch show metadata first
			request := FetchRequest{
				MediaType: MediaTypeShow,
				Name:      name,
			}
			showMeta, err = tmdbProvider.Fetch(ctx, request)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				setInCache(showKey, showMeta)
			}
		}

		if showMeta != nil {
			// Extract show identifiers and metadata to enrich episode request
			showID := extractShowID(showMeta)
			showName := name
			if showMeta.Core.Title != "" {
				showName = showMeta.Core.Title
			}
			showYear := year
			if showMeta.Core.Year != "" {
				showYear = showMeta.Core.Year
			}

			request := FetchRequest{
				MediaType: MediaTypeEpisode,
				ID:        showID,
				Name:      showName,
				Year:      showYear,
				Season:    season,
				Episode:   episode,
			}
			meta, err = tmdbProvider.Fetch(ctx, request)
		}
	} else if season > 0 {
		// For seasons, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := getFromCache(showKey)

		if showMeta == nil {
			// Fetch show metadata first
			request := FetchRequest{
				MediaType: MediaTypeShow,
				Name:      name,
			}
			showMeta, err = tmdbProvider.Fetch(ctx, request)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				setInCache(showKey, showMeta)
			}
		}

		if showMeta != nil {
			showID := extractShowID(showMeta)
			showName := name
			if showMeta.Core.Title != "" {
				showName = showMeta.Core.Title
			}
			showYear := year
			if showMeta.Core.Year != "" {
				showYear = showMeta.Core.Year
			}

			request := FetchRequest{
				MediaType: MediaTypeSeason,
				ID:        showID,
				Name:      showName,
				Year:      showYear,
				Season:    season,
			}
			meta, err = tmdbProvider.Fetch(ctx, request)
		}
	} else {
		// TV Show
		request := FetchRequest{
			MediaType: MediaTypeShow,
			Name:      name,
		}
		meta, err = tmdbProvider.Fetch(ctx, request)
	}

	return meta, err
}

func extractShowID(meta *Metadata) string {
	if meta == nil {
		return ""
	}
	if id, ok := meta.IDs["tmdb_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta.IDs["imdb_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta.IDs["omdb_id"]; ok && id != "" {
		return id
	}
	if id, ok := meta.IDs["series_id"]; ok && id != "" {
		return id
	}
	return ""
}
