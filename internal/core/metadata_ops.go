package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview"
)

// MetadataItem represents an item that needs metadata.
type MetadataItem struct {
	Name      string
	Year      string
	Season    int
	Episode   int
	IsMovie   bool
	Key       string
	Phase     int
	MediaType provider.MediaType
	Node      *treeview.Node[treeview.FileInfo]
}

// MetadataResult represents the result of fetching metadata for an item.
type MetadataResult struct {
	Item       MetadataItem
	Meta       *provider.Metadata
	Errs       []error
	TMDBErr    error
	TVDBErr    error
	OMDBErr    error
	FFProbeErr error
}

type localNodeInfo struct {
	mediaType provider.MediaType
	metadata  *provider.Metadata
}

// CollectMetadataItems walks the tree collecting unique metadata requests.
func CollectMetadataItems(tree *treeview.Tree[treeview.FileInfo], localProv *local.Provider) []MetadataItem {
	items := make([]MetadataItem, 0)
	itemsByKey := make(map[string]int)
	cache := make(map[*treeview.Node[treeview.FileInfo]]*localNodeInfo)
	failures := make(map[*treeview.Node[treeview.FileInfo]]struct{})

	upsertItem := func(key string, item MetadataItem) {
		if idx, exists := itemsByKey[key]; exists {
			if ShouldReplaceItemNode(items[idx], item) {
				items[idx].Node = item.Node
			}
			if item.MediaType != "" {
				items[idx].MediaType = item.MediaType
			}
			if item.IsMovie {
				items[idx].IsMovie = true
			}
			if items[idx].Year == "" && item.Year != "" {
				items[idx].Year = item.Year
			}
			if items[idx].Season == 0 && item.Season > 0 {
				items[idx].Season = item.Season
			}
			if items[idx].Episode == 0 && item.Episode > 0 {
				items[idx].Episode = item.Episode
			}
			if items[idx].Name == "" && item.Name != "" {
				items[idx].Name = item.Name
			}
			return
		}

		item.Key = key
		itemsByKey[key] = len(items)
		items = append(items, item)
	}

	for ni := range tree.BreadthFirst(context.Background()) {
		info := analyzeNodeCached(localProv, cache, failures, ni.Node)
		if info == nil || info.metadata == nil {
			continue
		}

		meta := info.metadata
		switch info.mediaType {
		case provider.MediaTypeMovie:
			if meta.Core.Title == "" {
				continue
			}
			key := provider.GenerateMetadataKey("movie", meta.Core.Title, meta.Core.Year, 0, 0)
			upsertItem(key, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				IsMovie:   true,
				Phase:     0,
				MediaType: provider.MediaTypeMovie,
				Node:      ni.Node,
			})
		case provider.MediaTypeShow:
			if meta.Core.Title == "" {
				continue
			}
			key := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			upsertItem(key, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				Phase:     0,
				MediaType: provider.MediaTypeShow,
				Node:      ni.Node,
			})
		case provider.MediaTypeSeason:
			if meta.Core.Title == "" {
				continue
			}
			showKey := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			upsertItem(showKey, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				Phase:     0,
				MediaType: provider.MediaTypeShow,
				Node:      ni.Node,
			})
			seasonKey := provider.GenerateMetadataKey("season", meta.Core.Title, meta.Core.Year, meta.Core.SeasonNum, 0)
			upsertItem(seasonKey, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				Season:    meta.Core.SeasonNum,
				Phase:     1,
				MediaType: provider.MediaTypeSeason,
				Node:      ni.Node,
			})
		case provider.MediaTypeEpisode:
			if meta.Core.Title == "" {
				continue
			}
			showKey := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			upsertItem(showKey, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				Phase:     0,
				MediaType: provider.MediaTypeShow,
				Node:      ni.Node,
			})
			episodeKey := provider.GenerateMetadataKey("episode", meta.Core.Title, meta.Core.Year, meta.Core.SeasonNum, meta.Core.EpisodeNum)
			upsertItem(episodeKey, MetadataItem{
				Name:      meta.Core.Title,
				Year:      meta.Core.Year,
				Season:    meta.Core.SeasonNum,
				Episode:   meta.Core.EpisodeNum,
				Phase:     2,
				MediaType: provider.MediaTypeEpisode,
				Node:      ni.Node,
			})
		}
	}

	return items
}

// ShouldReplaceItemNode determines if the new candidate should replace the existing node reference.
func ShouldReplaceItemNode(existing MetadataItem, candidate MetadataItem) bool {
	if existing.Node == nil {
		return true
	}
	if candidate.Node == nil {
		return false
	}
	// Prefer nodes that are closer to actual media (files over directories)
	existingData := existing.Node.Data()
	candidateData := candidate.Node.Data()
	if existingData.IsDir() && !candidateData.IsDir() {
		return true
	}
	if !existingData.IsDir() && candidateData.IsDir() {
		return false
	}

	existingName := existing.Node.Name()
	candidateName := candidate.Node.Name()

	existingIsVideo := local.IsVideo(existingName)
	candidateIsVideo := local.IsVideo(candidateName)

	if candidateIsVideo && !existingIsVideo {
		return true
	}
	if existingIsVideo && !candidateIsVideo {
		return false
	}
	// Prefer candidate when existing metadata is missing key details
	if existing.Name == "" && candidate.Name != "" {
		return true
	}
	if existing.Year == "" && candidate.Year != "" {
		return true
	}
	return false
}

// FetchTMDBMetadata retrieves metadata from TMDB with retry/backoff handling.
func FetchTMDBMetadata(ctx context.Context, prov provider.Provider, cache provider.MetadataCache, item MetadataItem) (*provider.Metadata, error) {
	if prov == nil {
		return nil, nil
	}

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		meta, err := provider.FetchMetadataWithDependencies(
			ctx,
			prov,
			item.Name,
			item.Year,
			item.Season,
			item.Episode,
			item.IsMovie,
			cache,
		)

		if meta != nil {
			return meta, nil
		}

		if err == nil {
			return nil, nil
		}

		var provErr *provider.ProviderError
		if errors.As(err, &provErr) && provErr.Code == "RATE_LIMITED" {
			waitTime := time.Duration(2<<uint(min(retryCount, 4))) * time.Second
			timer := time.NewTimer(waitTime)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
			retryCount++
			continue
		}

		return nil, err
	}
}

// FetchOMDBMetadata retrieves metadata from OMDb.
func FetchOMDBMetadata(ctx context.Context, prov provider.Provider, item MetadataItem, cache provider.MetadataCache) (*provider.Metadata, error) {
	if prov == nil {
		return nil, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	meta, err := provider.FetchMetadataWithDependencies(
		ctx,
		prov,
		item.Name,
		item.Year,
		item.Season,
		item.Episode,
		item.IsMovie,
		cache,
	)

	return meta, err
}

func FetchTVDBMetadata(ctx context.Context, prov provider.Provider, item MetadataItem, cache provider.MetadataCache) (*provider.Metadata, error) {
	if prov == nil {
		return nil, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	meta, err := provider.FetchMetadataWithDependencies(
		ctx,
		prov,
		item.Name,
		item.Year,
		item.Season,
		item.Episode,
		item.IsMovie,
		cache,
	)

	return meta, err
}

// FetchFFProbeMetadata retrieves technical metadata using ffprobe when applicable.
func FetchFFProbeMetadata(ctx context.Context, prov provider.Provider, item MetadataItem) (*provider.Metadata, error) {
	if prov == nil {
		return nil, nil
	}

	if !ShouldRunFFProbe(item) {
		return nil, nil
	}

	path := ""
	if item.Node != nil {
		path = item.Node.Data().Path
	}
	if path == "" {
		return nil, &provider.ProviderError{
			Provider: "ffprobe",
			Code:     "MISSING_PATH",
			Message:  "ffprobe requires a valid file path",
			Retry:    false,
		}
	}

	mediaType := item.MediaType
	if mediaType == "" {
		if item.IsMovie {
			mediaType = provider.MediaTypeMovie
		} else if item.Episode > 0 {
			mediaType = provider.MediaTypeEpisode
		}
	}

	req := provider.FetchRequest{
		MediaType: mediaType,
		Name:      item.Name,
		Year:      item.Year,
		Season:    item.Season,
		Episode:   item.Episode,
		Extra: map[string]interface{}{
			"path": path,
		},
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return prov.Fetch(ctx, req)
}

// ShouldRunFFProbe determines whether ffprobe should run for the item.
func ShouldRunFFProbe(item MetadataItem) bool {
	if item.Node == nil {
		return false
	}
	if item.MediaType != provider.MediaTypeMovie && item.MediaType != provider.MediaTypeEpisode {
		return false
	}
	data := item.Node.Data()
	if data.IsDir() {
		return false
	}
	return local.IsVideo(item.Node.Name())
}

// MergeMetadata combines base metadata with additional provider results.
func MergeMetadata(item MetadataItem, base *provider.Metadata, extras ...*provider.Metadata) *provider.Metadata {
	hasAdditional := false
	for _, extra := range extras {
		if HasMetadataValues(extra) {
			hasAdditional = true
			break
		}
	}

	if base == nil && !hasAdditional {
		return nil
	}

	var merged *provider.Metadata
	if base != nil {
		merged = cloneMetadata(base)
	} else {
		merged = &provider.Metadata{
			Core: provider.CoreMetadata{
				MediaType:  item.MediaType,
				Title:      item.Name,
				Year:       item.Year,
				SeasonNum:  item.Season,
				EpisodeNum: item.Episode,
			},
			Extended:   make(map[string]interface{}),
			Sources:    make(map[string]string),
			IDs:        make(map[string]string),
			Confidence: 0,
		}
	}

	ensureMetadataMaps(merged)

	if merged.Core.MediaType == "" {
		merged.Core.MediaType = item.MediaType
	}
	if merged.Core.MediaType == "" {
		if item.IsMovie {
			merged.Core.MediaType = provider.MediaTypeMovie
		} else if item.Episode > 0 {
			merged.Core.MediaType = provider.MediaTypeEpisode
		}
	}
	if merged.Core.Title == "" {
		merged.Core.Title = item.Name
	}
	if merged.Core.Year == "" {
		merged.Core.Year = item.Year
	}
	if merged.Core.SeasonNum == 0 && item.Season > 0 {
		merged.Core.SeasonNum = item.Season
	}
	if merged.Core.EpisodeNum == 0 && item.Episode > 0 {
		merged.Core.EpisodeNum = item.Episode
	}

	for _, extra := range extras {
		if extra == nil {
			continue
		}
		ensureMetadataMaps(extra)
		for k, v := range extra.Extended {
			merged.Extended[k] = v
		}
		for k, v := range extra.Sources {
			merged.Sources[k] = v
		}
		for k, v := range extra.IDs {
			merged.IDs[k] = v
		}
		if merged.Core.Title == "" && extra.Core.Title != "" {
			merged.Core.Title = extra.Core.Title
		}
		if merged.Core.Year == "" && extra.Core.Year != "" {
			merged.Core.Year = extra.Core.Year
		}
		if merged.Core.SeasonNum == 0 && extra.Core.SeasonNum > 0 {
			merged.Core.SeasonNum = extra.Core.SeasonNum
		}
		if merged.Core.EpisodeNum == 0 && extra.Core.EpisodeNum > 0 {
			merged.Core.EpisodeNum = extra.Core.EpisodeNum
		}
		if extra.Confidence > merged.Confidence {
			merged.Confidence = extra.Confidence
		}
	}

	return merged
}

func ensureMetadataMaps(meta *provider.Metadata) {
	if meta == nil {
		return
	}
	if meta.Extended == nil {
		meta.Extended = make(map[string]interface{})
	}
	if meta.Sources == nil {
		meta.Sources = make(map[string]string)
	}
	if meta.IDs == nil {
		meta.IDs = make(map[string]string)
	}
}

func HasMetadataValues(meta *provider.Metadata) bool {
	if meta == nil {
		return false
	}
	if meta.Core.Title != "" || meta.Core.Year != "" {
		return true
	}
	return len(meta.Extended) > 0 || len(meta.Sources) > 0 || len(meta.IDs) > 0
}

func cloneMetadata(meta *provider.Metadata) *provider.Metadata {
	if meta == nil {
		return nil
	}
	copy := *meta
	if meta.Extended != nil {
		copy.Extended = make(map[string]interface{}, len(meta.Extended))
		for k, v := range meta.Extended {
			copy.Extended[k] = v
		}
	}
	if meta.Sources != nil {
		copy.Sources = make(map[string]string, len(meta.Sources))
		for k, v := range meta.Sources {
			copy.Sources[k] = v
		}
	}
	if meta.IDs != nil {
		copy.IDs = make(map[string]string, len(meta.IDs))
		for k, v := range meta.IDs {
			copy.IDs[k] = v
		}
	}
	return &copy
}

func analyzeNodeCached(provider *local.Provider, cache map[*treeview.Node[treeview.FileInfo]]*localNodeInfo, failures map[*treeview.Node[treeview.FileInfo]]struct{}, node *treeview.Node[treeview.FileInfo]) *localNodeInfo {
	if node == nil {
		return nil
	}

	if info, ok := cache[node]; ok {
		return info
	}

	if _, failed := failures[node]; failed {
		return nil
	}

	mediaType, meta, err := provider.Detect(node)
	if err != nil || meta == nil {
		failures[node] = struct{}{}
		return nil
	}

	info := &localNodeInfo{
		mediaType: mediaType,
		metadata:  meta,
	}
	meta.Core.MediaType = mediaType
	cache[node] = info
	return info
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatMetadataProgressMessage builds a human readable description for progress updates.
func FormatMetadataProgressMessage(item MetadataItem) string {
	if item.IsMovie {
		return item.Name
	}
	if item.Season > 0 && item.Episode > 0 {
		return fmt.Sprintf("%s - S%02dE%02d", item.Name, item.Season, item.Episode)
	}
	if item.Season > 0 {
		return fmt.Sprintf("%s - Season %d", item.Name, item.Season)
	}
	return item.Name
}
