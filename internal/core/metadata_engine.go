package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/treeview"
	"github.com/mhmtszr/concurrent-swiss-map"
)

// MetadataEngine orchestrates metadata fetching across multiple providers while
// exposing progress snapshots for UI consumption.
type MetadataEngine struct {
	workerCount int
	localProv   *local.Provider
	tree        *treeview.Tree[treeview.FileInfo]

	tmdbProvider    provider.Provider
	omdbProvider    provider.Provider
	ffprobeProvider provider.Provider

	metadata *csmap.CsMap[string, *provider.Metadata]

	summaryMu sync.RWMutex
	summary   MetadataSummary

	errorsMu sync.Mutex
	errors   []error

	failuresMu sync.Mutex
	failures   []MetadataFailure

	activeProviders []string
}

// MetadataSummary captures the state of the metadata pipeline at a point in time.
type MetadataSummary struct {
	TotalItems      int
	ProcessedItems  int
	ActiveWorkers   int
	WorkerLimit     int
	PhaseIndex      int
	PhaseName       string
	ActiveProviders []string
	ErrorCount      int
	LastItem        string
	Done            bool
	Canceled        bool
}

// MetadataEvent represents an update emitted by the engine.
type MetadataEvent struct {
	Summary MetadataSummary
	Err     error
}

// MetadataProviderType identifies a metadata provider that can contribute
// results to the aggregation pipeline. Only providers that support manual
// search overrides are currently enumerated.
type MetadataProviderType string

const (
	MetadataProviderTMDB MetadataProviderType = "tmdb"
	MetadataProviderOMDB MetadataProviderType = "omdb"
)

// MetadataFailure captures a provider-specific failure for a metadata item so
// callers (e.g., the TUI) can offer manual search overrides before proceeding.
type MetadataFailure struct {
	Item     MetadataItem
	Provider MetadataProviderType
	Query    string
	Err      error
	Attempts int
}

// MetadataEngineConfig configures provider access for the metadata engine.
type MetadataEngineConfig struct {
	Tree          *treeview.Tree[treeview.FileInfo]
	LocalProvider *local.Provider
	WorkerCount   int
	Providers     MetadataProvidersConfig
}

// MetadataProvidersConfig contains per-provider configuration.
type MetadataProvidersConfig struct {
	TMDB    TMDBProviderConfig
	OMDB    OMDBProviderConfig
	FFProbe FFProbeProviderConfig
}

// TMDBProviderConfig describes TMDB provider configuration.
type TMDBProviderConfig struct {
	Enabled      bool
	APIKey       string
	Language     string
	CacheEnabled *bool
	Provider     provider.Provider
}

// OMDBProviderConfig describes OMDb provider configuration.
type OMDBProviderConfig struct {
	Enabled  bool
	APIKey   string
	Provider provider.Provider
}

// FFProbeProviderConfig describes ffprobe provider configuration.
type FFProbeProviderConfig struct {
	Enabled  bool
	Provider provider.Provider
}

// NewMetadataEngine constructs an engine with sane defaults applied.
func NewMetadataEngine(cfg MetadataEngineConfig) *MetadataEngine {
	localProv := cfg.LocalProvider
	if localProv == nil {
		localProv = local.New()
	}
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 20
	}

	engine := &MetadataEngine{
		workerCount: workerCount,
		localProv:   localProv,
		tree:        cfg.Tree,
		metadata:    csmap.Create[string, *provider.Metadata](),
		summary: MetadataSummary{
			WorkerLimit: workerCount,
		},
	}

	engine.initProviders(cfg.Providers)
	engine.summary.ActiveProviders = slices.Clone(engine.activeProviders)

	return engine
}

func (e *MetadataEngine) initProviders(cfg MetadataProvidersConfig) {
	if cfg.TMDB.Enabled {
		prov := cfg.TMDB.Provider
		if prov == nil {
			prov = tmdb.New()
		}
		if cfg.TMDB.APIKey != "" {
			cacheEnabled := true
			if cfg.TMDB.CacheEnabled != nil {
				cacheEnabled = *cfg.TMDB.CacheEnabled
			}
			conf := map[string]interface{}{
				"api_key":       cfg.TMDB.APIKey,
				"language":      cfg.TMDB.Language,
				"cache_enabled": cacheEnabled,
			}
			if err := prov.Configure(conf); err == nil {
				e.tmdbProvider = prov
				e.activeProviders = append(e.activeProviders, providerNameOrDefault(prov, "TMDB"))
			}
		}
	}

	if cfg.OMDB.Enabled {
		prov := cfg.OMDB.Provider
		if prov == nil {
			prov = omdb.New()
		}
		if cfg.OMDB.APIKey != "" {
			if err := prov.Configure(map[string]interface{}{"api_key": cfg.OMDB.APIKey}); err == nil {
				e.omdbProvider = prov
				e.activeProviders = append(e.activeProviders, providerNameOrDefault(prov, "OMDb"))
			}
		}
	}

	if cfg.FFProbe.Enabled {
		prov := cfg.FFProbe.Provider
		if prov == nil {
			prov = ffprobe.New()
		}
		e.ffprobeProvider = prov
		e.activeProviders = append(e.activeProviders, providerNameOrDefault(prov, "ffprobe"))
	}
}

func providerNameOrDefault(prov provider.Provider, fallback string) string {
	if prov == nil {
		return fallback
	}
	if name := prov.Name(); name != "" {
		return name
	}
	return fallback
}

// Start begins metadata fetching and returns a stream of progress events.
func (e *MetadataEngine) Start(ctx context.Context) <-chan MetadataEvent {
	events := make(chan MetadataEvent, 128)
	go e.run(ctx, events)
	return events
}

// Metadata returns the aggregated metadata map. The map is safe to read once
// the engine has completed.
func (e *MetadataEngine) Metadata() map[string]*provider.Metadata {
	if e.metadata == nil {
		return nil
	}
	result := make(map[string]*provider.Metadata, e.metadata.Count())
	e.metadata.Range(func(key string, value *provider.Metadata) bool {
		result[key] = value
		return false
	})
	return result
}

// Errors returns a copy of the accumulated errors.
func (e *MetadataEngine) Errors() []error {
	e.errorsMu.Lock()
	defer e.errorsMu.Unlock()
	if len(e.errors) == 0 {
		return nil
	}
	cloned := make([]error, len(e.errors))
	copy(cloned, e.errors)
	return cloned
}

// SummarySnapshot returns the latest progress summary.
func (e *MetadataEngine) SummarySnapshot() MetadataSummary {
	e.summaryMu.RLock()
	defer e.summaryMu.RUnlock()
	return e.summary
}

func (e *MetadataEngine) run(ctx context.Context, events chan<- MetadataEvent) {
	defer close(events)

	if e.tree == nil {
		e.emit(ctx, events, nil)
		return
	}

	if e.tmdbProvider == nil && e.omdbProvider == nil && e.ffprobeProvider == nil {
		e.summaryMu.Lock()
		e.summary.Done = true
		e.summaryMu.Unlock()
		e.emit(ctx, events, nil)
		return
	}

	items := e.collectMetadataItems()
	phaseGroups := e.groupByPhase(items)

	e.summaryMu.Lock()
	e.summary.TotalItems = len(items)
	e.summaryMu.Unlock()

	e.emit(ctx, events, nil)

	for phase := 0; phase <= 2; phase++ {
		phaseItems := phaseGroups[phase]
		if len(phaseItems) == 0 {
			continue
		}

		e.summaryMu.Lock()
		e.summary.PhaseIndex = phase
		e.summary.PhaseName = phaseName(phase)
		e.summary.ActiveWorkers = 0
		e.summaryMu.Unlock()

		e.runPhase(ctx, events, phaseItems)
		if ctx.Err() != nil {
			return
		}
	}

	e.summaryMu.Lock()
	e.summary.Done = true
	e.summaryMu.Unlock()
	e.emit(ctx, events, nil)
}

func (e *MetadataEngine) runPhase(ctx context.Context, events chan<- MetadataEvent, items []MetadataItem) {
	workerCount := min(e.workerCount, len(items))
	workCh := make(chan MetadataItem)
	resultCh := make(chan MetadataResult)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go e.worker(ctx, &wg, workCh, resultCh)
	}

	// announce initial worker pool size
	e.summaryMu.Lock()
	e.summary.ActiveWorkers = workerCount
	e.summaryMu.Unlock()
	e.emit(ctx, events, nil)

	go func() {
		defer close(workCh)
		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			if _, exists := e.metadata.Load(item.Key); exists {
				e.incrementProcessed(item)
				e.emit(ctx, events, nil)
				continue
			}
			select {
			case workCh <- item:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for {
		select {
		case <-ctx.Done():
			e.summaryMu.Lock()
			e.summary.Canceled = true
			e.summary.ActiveWorkers = 0
			e.summaryMu.Unlock()
			e.emit(ctx, events, ctx.Err())
			return
		case res, ok := <-resultCh:
			if !ok {
				e.summaryMu.Lock()
				e.summary.ActiveWorkers = 0
				e.summaryMu.Unlock()
				e.emit(ctx, events, nil)
				return
			}
			e.processResult(res)
			e.emit(ctx, events, nil)
		}
	}
}

func (e *MetadataEngine) worker(ctx context.Context, wg *sync.WaitGroup, workCh <-chan MetadataItem, resultCh chan<- MetadataResult) {
	defer wg.Done()

	for item := range workCh {
		if ctx.Err() != nil {
			return
		}

		tmdbMeta, tmdbErr := e.fetchTMDBMetadata(ctx, item)
		omdbMeta, omdbErr := e.fetchOMDBMetadata(ctx, item)
		ffprobeMeta, ffprobeErr := e.fetchFFProbeMetadata(ctx, item)

		base := tmdbMeta
		extra := make([]*provider.Metadata, 0, 2)
		if base == nil && omdbMeta != nil {
			base = omdbMeta
		} else if omdbMeta != nil {
			extra = append(extra, omdbMeta)
		}
		if ffprobeMeta != nil {
			extra = append(extra, ffprobeMeta)
		}

		combined := MergeMetadata(item, base, extra...)

		errs := make([]error, 0, 3)
		if tmdbErr != nil {
			errs = append(errs, tmdbErr)
		}
		if omdbErr != nil {
			errs = append(errs, omdbErr)
		}
		if ffprobeErr != nil {
			errs = append(errs, ffprobeErr)
		}

		select {
		case resultCh <- MetadataResult{
			Item:       item,
			Meta:       combined,
			Errs:       errs,
			TMDBErr:    tmdbErr,
			OMDBErr:    omdbErr,
			FFProbeErr: ffprobeErr,
		}:
		case <-ctx.Done():
			return
		}
	}
}

func (e *MetadataEngine) processResult(res MetadataResult) {
	if res.Meta != nil {
		e.metadata.Store(res.Item.Key, res.Meta)
	}

	failureCount := e.updateProviderFailures(res)
	e.appendErrors(res)
	e.summaryMu.Lock()
	e.summary.ProcessedItems++
	e.summary.ErrorCount = failureCount
	e.summary.LastItem = FormatMetadataProgressMessage(res.Item)
	e.summaryMu.Unlock()
}

func (e *MetadataEngine) incrementProcessed(item MetadataItem) {
	e.summaryMu.Lock()
	e.summary.ProcessedItems++
	e.summary.LastItem = FormatMetadataProgressMessage(item)
	e.summaryMu.Unlock()
}

func (e *MetadataEngine) appendErrors(res MetadataResult) int {
	filtered := make([]error, 0, len(res.Errs))
	for _, err := range res.Errs {
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		filtered = append(filtered, fmt.Errorf("%s: %w", res.Item.Name, err))
	}

	if len(filtered) == 0 {
		e.errorsMu.Lock()
		count := len(e.errors)
		e.errorsMu.Unlock()
		return count
	}

	e.errorsMu.Lock()
	e.errors = append(e.errors, filtered...)
	count := len(e.errors)
	e.errorsMu.Unlock()
	return count
}

func (e *MetadataEngine) updateProviderFailures(res MetadataResult) int {
	e.failuresMu.Lock()
	defer e.failuresMu.Unlock()

	e.updateFailureLocked(res.Item, MetadataProviderTMDB, res.Item.Name, res.TMDBErr)
	e.updateFailureLocked(res.Item, MetadataProviderOMDB, res.Item.Name, res.OMDBErr)

	return len(e.failures)
}

// ProviderFailures returns a snapshot of provider errors that require manual
// intervention before progressing. The returned slice is a copy safe for
// modification by the caller.
func (e *MetadataEngine) ProviderFailures() []MetadataFailure {
	e.failuresMu.Lock()
	defer e.failuresMu.Unlock()

	if len(e.failures) == 0 {
		return nil
	}

	cloned := make([]MetadataFailure, len(e.failures))
	copy(cloned, e.failures)
	return cloned
}

// RetryProvider re-executes a provider lookup using an optional manual search
// term. It returns nil when the failure has been resolved. If the provider
// still fails, the updated failure (including attempt count and error) is
// returned for display. A non-nil error indicates an unexpected engine issue.
func (e *MetadataEngine) RetryProvider(ctx context.Context, key string, providerType MetadataProviderType, nameOverride string) (*MetadataFailure, error) {
	e.failuresMu.Lock()
	idx := e.findFailureIndexLocked(key, providerType)
	if idx < 0 {
		e.failuresMu.Unlock()
		return nil, fmt.Errorf("metadata failure for %s/%s not found", providerType, key)
	}
	failure := e.failures[idx]
	e.failuresMu.Unlock()

	query := strings.TrimSpace(nameOverride)
	if query == "" {
		query = strings.TrimSpace(failure.Query)
	}
	if query == "" {
		query = failure.Item.Name
	}

	attemptItem := failure.Item
	attemptItem.Name = query

	var (
		meta     *provider.Metadata
		fetchErr error
	)

	switch providerType {
	case MetadataProviderTMDB:
		if e.tmdbProvider == nil {
			return nil, fmt.Errorf("tmdb provider not configured")
		}
		meta, fetchErr = FetchTMDBMetadata(ctx, e.tmdbProvider, e.metadataCache(), attemptItem)
	case MetadataProviderOMDB:
		if e.omdbProvider == nil {
			return nil, fmt.Errorf("omdb provider not configured")
		}
		meta, fetchErr = FetchOMDBMetadata(ctx, e.omdbProvider, attemptItem, e.metadataCache())
	default:
		return nil, fmt.Errorf("unsupported metadata provider %q", providerType)
	}

	if fetchErr != nil || meta == nil {
		e.failuresMu.Lock()
		defer e.failuresMu.Unlock()

		idx = e.findFailureIndexLocked(key, providerType)
		if idx < 0 {
			return nil, fmt.Errorf("metadata failure for %s/%s not found after retry", providerType, key)
		}

		updated := e.failures[idx]
		updated.Attempts++
		updated.Query = query
		updated.Item.Name = attemptItem.Name
		if meta == nil && fetchErr == nil {
			fetchErr = fmt.Errorf("no metadata returned for %q", query)
		}
		updated.Err = fetchErr
		e.failures[idx] = updated

		e.summaryMu.Lock()
		e.summary.ErrorCount = len(e.failures)
		e.summaryMu.Unlock()

		failureCopy := updated
		return &failureCopy, nil
	}

	e.applyManualMetadata(attemptItem, providerType, meta)

	e.failuresMu.Lock()
	idx = e.findFailureIndexLocked(key, providerType)
	if idx >= 0 {
		e.failures = append(e.failures[:idx], e.failures[idx+1:]...)
	}
	remaining := len(e.failures)
	e.failuresMu.Unlock()

	e.summaryMu.Lock()
	e.summary.ErrorCount = remaining
	e.summary.LastItem = FormatMetadataProgressMessage(attemptItem)
	e.summaryMu.Unlock()

	return nil, nil
}

func (e *MetadataEngine) updateFailureLocked(item MetadataItem, providerType MetadataProviderType, query string, err error) {
	idx := e.findFailureIndexLocked(item.Key, providerType)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if idx >= 0 {
			e.failures = append(e.failures[:idx], e.failures[idx+1:]...)
		}
		return
	}

	if query == "" {
		query = item.Name
	}

	var provErr *provider.ProviderError
	if errors.As(err, &provErr) {
		// Only treat search-not-found scenarios as retryable in the manual UI.
		if provErr.Code != "NOT_FOUND" {
			if idx >= 0 {
				e.failures = append(e.failures[:idx], e.failures[idx+1:]...)
			}
			return
		}
	}

	if idx >= 0 {
		existing := e.failures[idx]
		existing.Err = err
		existing.Query = query
		existing.Item = item
		if existing.Attempts == 0 {
			existing.Attempts = 1
		}
		e.failures[idx] = existing
		return
	}

	failure := MetadataFailure{
		Item:     item,
		Provider: providerType,
		Query:    query,
		Err:      err,
		Attempts: 1,
	}
	e.failures = append(e.failures, failure)
}

func (e *MetadataEngine) findFailureIndexLocked(key string, providerType MetadataProviderType) int {
	for idx, failure := range e.failures {
		if failure.Item.Key == key && failure.Provider == providerType {
			return idx
		}
	}
	return -1
}

func (e *MetadataEngine) applyManualMetadata(item MetadataItem, providerType MetadataProviderType, meta *provider.Metadata) {
	if meta == nil {
		return
	}

	existing, _ := e.metadata.Load(item.Key)

	switch providerType {
	case MetadataProviderTMDB:
		extra := make([]*provider.Metadata, 0, 1)
		if existing != nil {
			extra = append(extra, existing)
		}
		combined := MergeMetadata(item, meta, extra...)
		if combined != nil {
			e.metadata.Store(item.Key, combined)
		}
	case MetadataProviderOMDB:
		if existing != nil {
			combined := MergeMetadata(item, existing, meta)
			if combined != nil {
				e.metadata.Store(item.Key, combined)
			}
			return
		}
		combined := MergeMetadata(item, meta)
		if combined != nil {
			e.metadata.Store(item.Key, combined)
		}
	default:
		if existing != nil {
			combined := MergeMetadata(item, existing, meta)
			if combined != nil {
				e.metadata.Store(item.Key, combined)
			}
			return
		}
		combined := MergeMetadata(item, meta)
		if combined != nil {
			e.metadata.Store(item.Key, combined)
		}
	}
}

func (e *MetadataEngine) emit(ctx context.Context, events chan<- MetadataEvent, err error) {
	summary := e.SummarySnapshot()
	select {
	case events <- MetadataEvent{Summary: summary, Err: err}:
	case <-ctx.Done():
	}
}

func (e *MetadataEngine) collectMetadataItems() []MetadataItem {
	return CollectMetadataItems(e.tree, e.localProv)
}

func (e *MetadataEngine) groupByPhase(items []MetadataItem) map[int][]MetadataItem {
	groups := make(map[int][]MetadataItem, 3)
	for _, item := range items {
		groups[item.Phase] = append(groups[item.Phase], item)
	}
	return groups
}

func phaseName(phase int) string {
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

func (e *MetadataEngine) fetchTMDBMetadata(ctx context.Context, item MetadataItem) (*provider.Metadata, error) {
	if e.tmdbProvider == nil {
		return nil, nil
	}
	return FetchTMDBMetadata(ctx, e.tmdbProvider, e.metadataCache(), item)
}

func (e *MetadataEngine) fetchOMDBMetadata(ctx context.Context, item MetadataItem) (*provider.Metadata, error) {
	if e.omdbProvider == nil {
		return nil, nil
	}
	return FetchOMDBMetadata(ctx, e.omdbProvider, item, e.metadataCache())
}

func (e *MetadataEngine) fetchFFProbeMetadata(ctx context.Context, item MetadataItem) (*provider.Metadata, error) {
	if e.ffprobeProvider == nil {
		return nil, nil
	}
	return FetchFFProbeMetadata(ctx, e.ffprobeProvider, item)
}

func (e *MetadataEngine) metadataCache() provider.MetadataCache {
	return metadataCacheAdapter{engine: e}
}

type metadataCacheAdapter struct {
	engine *MetadataEngine
}

func (c metadataCacheAdapter) Get(key string) (*provider.Metadata, bool) {
	if c.engine == nil || c.engine.metadata == nil {
		return nil, false
	}
	return c.engine.metadata.Load(key)
}

func (c metadataCacheAdapter) Set(key string, meta *provider.Metadata) {
	if c.engine == nil || c.engine.metadata == nil || meta == nil {
		return
	}
	c.engine.metadata.Store(key, meta)
}
