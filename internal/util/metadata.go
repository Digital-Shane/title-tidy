package util

import (
	"context"
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
)

// GenerateMetadataKey creates a unique key for caching metadata
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

// FetchMetadataWithDependencies fetches metadata with proper dependency resolution
// For episodes/seasons, it ensures show metadata is fetched first
// Returns both metadata and error so callers can handle rate limiting properly
func FetchMetadataWithDependencies(tmdbProvider provider.Provider, name, year string, season, episode int, isMovie bool, cache map[string]*provider.Metadata) (*provider.Metadata, error) {
	if tmdbProvider == nil || name == "" {
		return nil, nil
	}

	var meta *provider.Metadata
	var err error
	ctx := context.Background()

	if isMovie {
		// Fetch movie metadata
		request := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      name,
			Year:      year,
		}
		meta, err = tmdbProvider.Fetch(ctx, request)
	} else if season > 0 && episode > 0 {
		// For episodes, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := cache[showKey]

		if showMeta == nil {
			// Fetch show metadata first
			request := provider.FetchRequest{
				MediaType: provider.MediaTypeShow,
				Name:      name,
			}
			showMeta, err = tmdbProvider.Fetch(ctx, request)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				cache[showKey] = showMeta
			}
		}

		if showMeta != nil {
			// Extract show ID from metadata
			showID := ""
			if tmdbID, ok := showMeta.IDs["tmdb_id"]; ok {
				showID = tmdbID
			}

			if showID != "" {
				// Fetch episode metadata
				request := provider.FetchRequest{
					MediaType: provider.MediaTypeEpisode,
					ID:        showID,
					Season:    season,
					Episode:   episode,
				}
				meta, err = tmdbProvider.Fetch(ctx, request)
			}
		}
	} else if season > 0 {
		// For seasons, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := cache[showKey]

		if showMeta == nil {
			// Fetch show metadata first
			request := provider.FetchRequest{
				MediaType: provider.MediaTypeShow,
				Name:      name,
			}
			showMeta, err = tmdbProvider.Fetch(ctx, request)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				cache[showKey] = showMeta
			}
		}

		if showMeta != nil {
			// Extract show ID from metadata
			showID := ""
			if tmdbID, ok := showMeta.IDs["tmdb_id"]; ok {
				showID = tmdbID
			}

			if showID != "" {
				// Fetch season metadata
				request := provider.FetchRequest{
					MediaType: provider.MediaTypeSeason,
					ID:        showID,
					Season:    season,
				}
				meta, err = tmdbProvider.Fetch(ctx, request)
			}
		}
	} else {
		// TV Show
		request := provider.FetchRequest{
			MediaType: provider.MediaTypeShow,
			Name:      name,
		}
		meta, err = tmdbProvider.Fetch(ctx, request)
	}

	return meta, err
}
