package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhmtszr/concurrent-swiss-map"
)

// MetadataProgressModel displays progress while fetching metadata from TMDB
type MetadataProgressModel struct {
	// Tree to process
	tree          *treeview.Tree[treeview.FileInfo]
	cfg           *config.FormatConfig
	localProvider *local.Provider

	// Progress tracking
	totalItems     int
	processedItems int
	currentPhase   string
	activeWorkers  int
	workersMu      sync.RWMutex

	// Results storage
	metadata        *csmap.CsMap[string, *provider.Metadata]
	tmdbProvider    provider.Provider
	ffprobeProvider provider.Provider
	activeProviders []string

	// Worker pool
	workerCount int
	workCh      chan MetadataItem
	resultCh    chan metadataResult

	// Cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// UI components
	width    int
	height   int
	progress progress.Model
	msgCh    chan tea.Msg
	done     bool
	err      error
	errors   []error
	errorsMu sync.Mutex
}

// Get reads cached metadata using the concurrent map.
func (m *MetadataProgressModel) Get(key string) (*provider.Metadata, bool) {
	if m.metadata == nil {
		return nil, false
	}
	return m.metadata.Load(key)
}

// Set stores metadata using the concurrent map.
func (m *MetadataProgressModel) Set(key string, meta *provider.Metadata) {
	if meta == nil || m.metadata == nil {
		return
	}
	m.metadata.Store(key, meta)
}

// metadataProgressMsg updates progress
type metadataProgressMsg struct {
	phase string
	item  string
}

// metadataCompleteMsg signals completion
type metadataCompleteMsg struct{}

// MetadataItem represents an item that needs metadata
type MetadataItem struct {
	Name      string
	Year      string
	Season    int
	Episode   int
	IsMovie   bool
	Key       string // Unique key for caching
	Phase     int    // 0=show/movie, 1=season, 2=episode
	MediaType provider.MediaType
	Node      *treeview.Node[treeview.FileInfo]
}

// metadataResult represents the result of fetching metadata
type metadataResult struct {
	item MetadataItem
	meta *provider.Metadata
	errs []error
}

type localNodeInfo struct {
	mediaType provider.MediaType
	metadata  *provider.Metadata
}

// NewMetadataProgressModel creates a new metadata progress model
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig) *MetadataProgressModel {
	localProvider := local.New()

	// Count items that need metadata
	items := countMetadataItems(tree, localProvider)

	p := progress.New(progress.WithGradient(string(colorPrimary), string(colorAccent)))
	p.Width = 50

	// Initialize TMDB provider if enabled
	var tmdbProvider provider.Provider
	if cfg.EnableTMDBLookup && cfg.TMDBAPIKey != "" {
		// Create new TMDB provider
		tmdbProvider = tmdb.New()

		// Configure it
		config := map[string]interface{}{
			"api_key":       cfg.TMDBAPIKey,
			"language":      cfg.TMDBLanguage,
			"cache_enabled": true,
		}

		if err := tmdbProvider.Configure(config); err != nil {
			// If provider configuration fails, continue without TMDB
			tmdbProvider = nil
		}
	}

	// Initialize ffprobe provider if enabled
	var ffprobeProvider provider.Provider
	if cfg.EnableFFProbe {
		ffprobeProvider = ffprobe.New()
	}

	activeProviders := make([]string, 0, 2)
	if tmdbProvider != nil {
		activeProviders = append(activeProviders, "TMDB")
	}
	if ffprobeProvider != nil {
		activeProviders = append(activeProviders, "ffprobe")
	}

	// Use configured worker count, with a sensible minimum
	workerCount := cfg.TMDBWorkerCount
	if workerCount <= 0 {
		workerCount = 20 // Fallback to default if not configured or invalid
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &MetadataProgressModel{
		tree:            tree,
		cfg:             cfg,
		localProvider:   localProvider,
		totalItems:      items,
		width:           80,
		height:          12,
		progress:        p,
		msgCh:           make(chan tea.Msg, 256),
		metadata:        csmap.Create[string, *provider.Metadata](),
		tmdbProvider:    tmdbProvider,
		ffprobeProvider: ffprobeProvider,
		activeProviders: activeProviders,
		workerCount:     workerCount,
		workCh:          make(chan MetadataItem, 100),
		resultCh:        make(chan metadataResult, 100),
		errors:          make([]error, 0),
		ctx:             ctx,
		cancel:          cancel,
	}
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

func addKey(seen map[string]bool, key string) bool {
	if key == "" || seen[key] {
		return false
	}
	seen[key] = true
	return true
}

// countMetadataItems counts unique items that need metadata fetching
func countMetadataItems(tree *treeview.Tree[treeview.FileInfo], localProv *local.Provider) int {
	seen := make(map[string]bool)
	cache := make(map[*treeview.Node[treeview.FileInfo]]*localNodeInfo)
	failures := make(map[*treeview.Node[treeview.FileInfo]]struct{})
	count := 0

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
			if addKey(seen, key) {
				count++
			}
		case provider.MediaTypeShow:
			if meta.Core.Title == "" {
				continue
			}
			key := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			if addKey(seen, key) {
				count++
			}
		case provider.MediaTypeSeason:
			if meta.Core.Title == "" {
				continue
			}
			showKey := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			if addKey(seen, showKey) {
				count++
			}
			seasonKey := provider.GenerateMetadataKey("season", meta.Core.Title, meta.Core.Year, meta.Core.SeasonNum, 0)
			if addKey(seen, seasonKey) {
				count++
			}
		case provider.MediaTypeEpisode:
			if meta.Core.Title == "" {
				continue
			}
			showKey := provider.GenerateMetadataKey("show", meta.Core.Title, meta.Core.Year, 0, 0)
			if addKey(seen, showKey) {
				count++
			}
			episodeKey := provider.GenerateMetadataKey("episode", meta.Core.Title, meta.Core.Year, meta.Core.SeasonNum, meta.Core.EpisodeNum)
			if addKey(seen, episodeKey) {
				count++
			}
		}
	}

	return count
}

// Init starts the async metadata fetching
func (m *MetadataProgressModel) Init() tea.Cmd {
	if m.tmdbProvider == nil && m.ffprobeProvider == nil {
		// No metadata providers are enabled, skip fetching
		m.done = true
		return tea.Quit
	}

	go m.fetchMetadataAsync()
	return m.waitForMsg()
}

func (m *MetadataProgressModel) waitForMsg() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgCh
	}
}

// fetchMetadataAsync fetches metadata for all items in the tree
func (m *MetadataProgressModel) fetchMetadataAsync() {
	// Collect and organize items by phase
	phases := m.organizeItemsByPhase()

	// Process each phase sequentially
	for phaseNum, phaseItems := range phases {
		// Check for cancellation at the start of each phase
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		if len(phaseItems) == 0 {
			continue
		}

		// Set current phase
		phaseName := m.getPhaseName(phaseNum)
		m.currentPhase = phaseName

		// Reset active workers count for this phase
		m.workersMu.Lock()
		m.activeWorkers = 0
		m.workersMu.Unlock()

		// Start workers for this phase
		var wg sync.WaitGroup

		// Create fresh channels for this phase to avoid reusing closed channels
		workCh := make(chan MetadataItem, len(phaseItems))
		resultCh := make(chan metadataResult, len(phaseItems))

		// Start worker goroutines with the new channels
		for i := 0; i < m.workerCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				m.metadataWorker(workCh, resultCh, id)
			}(i)
		}

		// Start result processor
		resultDone := make(chan bool)
		go m.processResults(resultCh, resultDone)

		// Send work items and close channel when done
		// This is now synchronous to avoid race conditions
		func() {
			defer close(workCh)
			for _, item := range phaseItems {
				// Check for cancellation
				select {
				case <-m.ctx.Done():
					return
				default:
				}

				// Skip if already cached
				if _, exists := m.Get(item.Key); exists {
					m.processedItems++
					continue
				}

				// Try to send item, but respect cancellation
				select {
				case workCh <- item:
				case <-m.ctx.Done():
					return
				}
			}
		}()

		// Wait for all workers to complete
		wg.Wait()
		close(resultCh)

		// Wait for result processor to finish
		<-resultDone

		// Reset channels for next phase
		if phaseNum < 2 {
			m.workCh = make(chan MetadataItem, 100)
			m.resultCh = make(chan metadataResult, 100)
		}
	}

	m.done = true
	m.msgCh <- metadataCompleteMsg{}
}

// metadataWorker processes items from the provided work channel
func (m *MetadataProgressModel) metadataWorker(workCh <-chan MetadataItem, resultCh chan<- metadataResult, workerID int) {
	// Update active workers count
	m.workersMu.Lock()
	m.activeWorkers++
	m.workersMu.Unlock()

	defer func() {
		m.workersMu.Lock()
		m.activeWorkers--
		m.workersMu.Unlock()
	}()

	for item := range workCh {
		// Check for cancellation before processing
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// Create descriptive progress message
		var currentDesc string
		if item.IsMovie {
			currentDesc = item.Name
		} else if item.Season > 0 && item.Episode > 0 {
			currentDesc = fmt.Sprintf("%s - S%02dE%02d", item.Name, item.Season, item.Episode)
		} else if item.Season > 0 {
			currentDesc = fmt.Sprintf("%s - Season %d", item.Name, item.Season)
		} else {
			currentDesc = item.Name
		}

		// Send progress update
		select {
		case m.msgCh <- metadataProgressMsg{
			phase: m.currentPhase,
			item:  currentDesc,
		}:
		case <-m.ctx.Done():
			return
		default:
		}

		tmdbMeta, tmdbErr := m.fetchTMDBMetadata(item)
		ffprobeMeta, ffprobeErr := m.fetchFFProbeMetadata(item)

		combined := mergeMetadata(item, tmdbMeta, ffprobeMeta)

		errs := make([]error, 0, 2)
		if tmdbErr != nil {
			errs = append(errs, tmdbErr)
		}
		if ffprobeErr != nil {
			errs = append(errs, ffprobeErr)
		}

		// Send result with cancellation check
		select {
		case resultCh <- metadataResult{
			item: item,
			meta: combined,
			errs: errs,
		}:
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *MetadataProgressModel) fetchTMDBMetadata(item MetadataItem) (*provider.Metadata, error) {
	if m.tmdbProvider == nil {
		return nil, nil
	}

	retryCount := 0

	for {
		select {
		case <-m.ctx.Done():
			return nil, m.ctx.Err()
		default:
		}

		meta, err := provider.FetchMetadataWithDependencies(
			m.tmdbProvider,
			item.Name,
			item.Year,
			item.Season,
			item.Episode,
			item.IsMovie,
			m,
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
			case <-m.ctx.Done():
				timer.Stop()
				return nil, m.ctx.Err()
			}
			retryCount++
			continue
		}

		return nil, err
	}
}

func (m *MetadataProgressModel) fetchFFProbeMetadata(item MetadataItem) (*provider.Metadata, error) {
	if m.ffprobeProvider == nil {
		return nil, nil
	}

	if !m.shouldRunFFProbe(item) {
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

	request := provider.FetchRequest{
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
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	default:
	}

	return m.ffprobeProvider.Fetch(m.ctx, request)
}

func (m *MetadataProgressModel) shouldRunFFProbe(item MetadataItem) bool {
	if m.ffprobeProvider == nil {
		return false
	}
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

func mergeMetadata(item MetadataItem, base *provider.Metadata, extras ...*provider.Metadata) *provider.Metadata {
	hasAdditional := false
	for _, extra := range extras {
		if hasMetadataValues(extra) {
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

func hasMetadataValues(meta *provider.Metadata) bool {
	if meta == nil {
		return false
	}
	if len(meta.Extended) > 0 || len(meta.Sources) > 0 || len(meta.IDs) > 0 {
		return true
	}
	if meta.Core.Title != "" || meta.Core.Year != "" || meta.Core.SeasonNum > 0 || meta.Core.EpisodeNum > 0 {
		return true
	}
	return false
}

func cloneMetadata(meta *provider.Metadata) *provider.Metadata {
	if meta == nil {
		return nil
	}
	clone := &provider.Metadata{
		Core:       meta.Core,
		Confidence: meta.Confidence,
	}

	if len(meta.Extended) > 0 {
		clone.Extended = make(map[string]interface{}, len(meta.Extended))
		for k, v := range meta.Extended {
			clone.Extended[k] = v
		}
	} else {
		clone.Extended = make(map[string]interface{})
	}

	if len(meta.Sources) > 0 {
		clone.Sources = make(map[string]string, len(meta.Sources))
		for k, v := range meta.Sources {
			clone.Sources[k] = v
		}
	} else {
		clone.Sources = make(map[string]string)
	}

	if len(meta.IDs) > 0 {
		clone.IDs = make(map[string]string, len(meta.IDs))
		for k, v := range meta.IDs {
			clone.IDs[k] = v
		}
	} else {
		clone.IDs = make(map[string]string)
	}

	return clone
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

// processResults handles results from the provided result channel
func (m *MetadataProgressModel) processResults(resultCh <-chan metadataResult, done chan bool) {
	for result := range resultCh {
		if result.meta != nil {
			m.Set(result.item.Key, result.meta)
		}

		if len(result.errs) > 0 {
			for _, err := range result.errs {
				if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				var provErr *provider.ProviderError
				if errors.As(err, &provErr) && provErr.Code == "NOT_FOUND" {
					continue
				}
				m.errorsMu.Lock()
				m.errors = append(m.errors, fmt.Errorf("%s: %w", result.item.Name, err))
				m.errorsMu.Unlock()
			}
		}

		m.processedItems++
	}
	done <- true
}

// organizeItemsByPhase groups items by processing phase
func (m *MetadataProgressModel) organizeItemsByPhase() map[int][]MetadataItem {
	items := m.collectMetadataItems()
	phases := make(map[int][]MetadataItem)

	for _, item := range items {
		phases[item.Phase] = append(phases[item.Phase], item)
	}

	return phases
}

// getPhaseName returns a human-readable phase name
func (m *MetadataProgressModel) getPhaseName(phase int) string {
	switch phase {
	case 0:
		return "Shows/Movies"
	case 1:
		return "Seasons"
	case 2:
		return "Episodes"
	default:
		return "Unknown"
	}
}

// collectMetadataItems collects all unique items that need metadata

func (m *MetadataProgressModel) collectMetadataItems() []MetadataItem {
	items := make([]MetadataItem, 0)
	itemsByKey := make(map[string]int)
	cache := make(map[*treeview.Node[treeview.FileInfo]]*localNodeInfo)
	failures := make(map[*treeview.Node[treeview.FileInfo]]struct{})
	localProv := m.localProvider
	if localProv == nil {
		localProv = local.New()
		m.localProvider = localProv
	}

	upsertItem := func(key string, item MetadataItem) {
		if idx, exists := itemsByKey[key]; exists {
			if shouldReplaceItemNode(items[idx], item) {
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

	for ni := range m.tree.BreadthFirst(context.Background()) {
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

func shouldReplaceItemNode(existing MetadataItem, candidate MetadataItem) bool {
	if candidate.Node == nil {
		return false
	}
	if existing.Node == nil {
		return true
	}

	candidateData := candidate.Node.Data()
	existingData := existing.Node.Data()

	candidateVideo := local.IsVideo(candidate.Node.Name())
	existingVideo := local.IsVideo(existing.Node.Name())

	if candidateVideo && !existingVideo {
		return true
	}

	if existingData != nil && existingData.IsDir() {
		if candidateData != nil && !candidateData.IsDir() {
			return true
		}
	}

	return false
}

// Update processes messages
func (m *MetadataProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.Width = msg.Width - 4
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			// Cancel the context to stop all workers
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case metadataProgressMsg:
		m.currentPhase = msg.phase
		// Don't overwrite activeWorkers - it's maintained by workers themselves
		ratio := float64(m.processedItems) / float64(max(m.totalItems, 1))
		cmd := m.progress.SetPercent(ratio)
		return m, tea.Batch(cmd, m.waitForMsg())
	case metadataCompleteMsg:
		return m, tea.Quit
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}
	return m, nil
}

// View renders the progress UI
func (m *MetadataProgressModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if m.totalItems == 0 {
		return "No items require metadata fetching.\n"
	}

	percent := 100 * m.processedItems / m.totalItems
	bar := m.progress.View()

	headerText := "Fetching Metadata"
	if len(m.activeProviders) > 0 {
		headerText = fmt.Sprintf("Fetching Metadata (%s)", strings.Join(m.activeProviders, ", "))
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Background(colorPrimary).
		Foreground(colorBackground).
		Width(m.width).
		Render(headerText)

	// Phase and worker info
	phaseStyle := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true)

	phaseInfo := ""
	if m.currentPhase != "" {
		m.workersMu.RLock()
		workers := m.activeWorkers
		m.workersMu.RUnlock()
		phaseInfo = phaseStyle.Render(fmt.Sprintf("Phase: %s | Active Workers: %d", m.currentPhase, workers))
	}

	info := fmt.Sprintf("Items processed: %d/%d", m.processedItems, m.totalItems)

	statsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1).
		Width(m.width - 4)

	stats := fmt.Sprintf("Total Items: %d\nProcessed: %d\nProgress: %d%%\nMax Worker Pool: %d workers",
		m.totalItems, m.processedItems, percent, m.workerCount)

	// Show errors if any
	errorInfo := ""
	if len(m.errors) > 0 {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

		// Show error count and error messages
		errorLines := []string{fmt.Sprintf("Errors: %d", len(m.errors))}

		// Calculate available space for errors
		// Account for: header, phase info, progress bar, stats, status bar, and some padding
		usedLines := 6 // rough estimate of other UI elements
		availableHeight := m.height - usedLines
		maxErrorLines := availableHeight - 1 // reserve one line for error count

		if maxErrorLines < 1 {
			maxErrorLines = 1 // always show at least one error
		}

		// Calculate how many errors we can show
		errorsToShow := len(m.errors)
		if errorsToShow > maxErrorLines {
			errorsToShow = maxErrorLines
		}

		// Show most recent errors
		startIdx := len(m.errors) - errorsToShow
		if startIdx < 0 {
			startIdx = 0
		}

		// Calculate available width for error messages (account for "• " prefix)
		availableWidth := m.width - 2
		if availableWidth < 10 {
			availableWidth = 10 // minimum readable width
		}

		for i := startIdx; i < len(m.errors); i++ {
			errorMsg := m.errors[i].Error()
			// Only truncate if message is longer than available width
			if len(errorMsg) > availableWidth {
				errorMsg = errorMsg[:availableWidth-3] + "..."
			}
			errorLines = append(errorLines, fmt.Sprintf("• %s", errorMsg))
		}

		// Only show "and X more" if we couldn't fit all errors
		if len(m.errors) > errorsToShow {
			errorLines = append(errorLines, fmt.Sprintf("... and %d more", len(m.errors)-errorsToShow))
		}

		errorInfo = "\n" + errorStyle.Render(strings.Join(errorLines, "\n"))
	}

	status := lipgloss.NewStyle().
		Background(colorSecondary).
		Foreground(colorBackground).
		Width(m.width).
		Render("Fetching metadata in parallel... please wait")

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		phaseInfo,
		bar,
		info,
		statsStyle.Render(stats+errorInfo),
		status,
	)

	return body
}

// Metadata returns the fetched metadata
func (m *MetadataProgressModel) Metadata() map[string]*provider.Metadata {
	if m.metadata == nil {
		return nil
	}
	result := make(map[string]*provider.Metadata, m.metadata.Count())
	m.metadata.Range(func(key string, value *provider.Metadata) bool {
		result[key] = value
		return false
	})
	return result
}

// Err returns any error
func (m *MetadataProgressModel) Err() error {
	if m.err != nil {
		return m.err
	}
	// Return aggregated errors if any critical ones exist
	if len(m.errors) > 0 {
		// Check for critical errors that should stop processing
		for _, err := range m.errors {
			var provErr *provider.ProviderError
			if errors.As(err, &provErr) {
				if provErr.Code == "AUTH_FAILED" || provErr.Code == "UNAVAILABLE" {
					return err
				}
			}
		}
	}
	return nil
}
