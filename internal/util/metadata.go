package util

import (
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
func FetchMetadataWithDependencies(tmdbProvider *provider.TMDBProvider, name, year string, season, episode int, isMovie bool, cache map[string]*provider.EnrichedMetadata) (*provider.EnrichedMetadata, error) {
	if tmdbProvider == nil || name == "" {
		return nil, nil
	}

	var meta *provider.EnrichedMetadata
	var err error

	if isMovie {
		meta, err = tmdbProvider.SearchMovie(name, year)
	} else if season > 0 && episode > 0 {
		// For episodes, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := cache[showKey]

		if showMeta == nil {
			// Fetch show metadata first
			showMeta, err = tmdbProvider.SearchTVShow(name)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				cache[showKey] = showMeta
			}
		}

		if showMeta != nil && showMeta.ID > 0 {
			meta, err = tmdbProvider.GetEpisodeInfo(showMeta.ID, season, episode)
			if meta != nil {
				meta.ShowName = showMeta.ShowName
				if meta.ShowName == "" {
					meta.ShowName = showMeta.Title
				}
			}
		}
	} else if season > 0 {
		// For seasons, first get show metadata
		showKey := GenerateMetadataKey("show", name, year, 0, 0)
		showMeta := cache[showKey]

		if showMeta == nil {
			// Fetch show metadata first
			showMeta, err = tmdbProvider.SearchTVShow(name)
			if err != nil {
				return nil, err // Return error (including rate limiting) immediately
			}
			if showMeta != nil {
				cache[showKey] = showMeta
			}
		}

		if showMeta != nil && showMeta.ID > 0 {
			meta, err = tmdbProvider.GetSeasonInfo(showMeta.ID, season)
			if meta != nil {
				meta.ShowName = showMeta.ShowName
				if meta.ShowName == "" {
					meta.ShowName = showMeta.Title
				}
			}
		}
	} else {
		// TV Show
		meta, err = tmdbProvider.SearchTVShow(name)
	}

	return meta, err
}
