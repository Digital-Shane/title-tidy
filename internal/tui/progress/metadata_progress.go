package progress

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhmtszr/concurrent-swiss-map"
)

type MetadataItem = core.MetadataItem
type metadataResult = core.MetadataResult

func (m *MetadataProgressModel) fetchTMDBMetadata(item MetadataItem) (*provider.Metadata, error) {
	return core.FetchTMDBMetadata(m.ctx, m.tmdbProvider, m, item)
}

func (m *MetadataProgressModel) fetchOMDBMetadata(item MetadataItem) (*provider.Metadata, error) {
	return core.FetchOMDBMetadata(m.ctx, m.omdbProvider, item, m)
}

func (m *MetadataProgressModel) fetchFFProbeMetadata(item MetadataItem) (*provider.Metadata, error) {
	return core.FetchFFProbeMetadata(m.ctx, m.ffprobeProvider, item)
}

func (m *MetadataProgressModel) shouldRunFFProbe(item MetadataItem) bool {
	if m.ffprobeProvider == nil {
		return false
	}
	return core.ShouldRunFFProbe(item)
}

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
	omdbProvider    provider.Provider
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

// NewMetadataProgressModel creates a new metadata progress model
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig) *MetadataProgressModel {
	localProvider := local.New()

	// Count items that need metadata
	items := core.CountMetadataItems(tree, localProvider)

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

	// Initialize OMDb provider if enabled
	var omdbProvider provider.Provider
	if cfg.EnableOMDBLookup && cfg.OMDBAPIKey != "" {
		omdbProvider = omdb.New()
		if err := omdbProvider.Configure(map[string]interface{}{
			"api_key": cfg.OMDBAPIKey,
		}); err != nil {
			omdbProvider = nil
		}
	}

	// Initialize ffprobe provider if enabled
	var ffprobeProvider provider.Provider
	if cfg.EnableFFProbe {
		ffprobeProvider = ffprobe.New()
	}

	activeProviders := make([]string, 0, 3)
	if tmdbProvider != nil {
		activeProviders = append(activeProviders, "TMDB")
	}
	if omdbProvider != nil {
		activeProviders = append(activeProviders, "OMDb")
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
		omdbProvider:    omdbProvider,
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

// Init starts the async metadata fetching
func (m *MetadataProgressModel) Init() tea.Cmd {
	if m.tmdbProvider == nil && m.omdbProvider == nil && m.ffprobeProvider == nil {
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
		currentDesc := core.FormatMetadataProgressMessage(item)

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

		tmdbMeta, tmdbErr := core.FetchTMDBMetadata(m.ctx, m.tmdbProvider, m, item)
		omdbMeta, omdbErr := core.FetchOMDBMetadata(m.ctx, m.omdbProvider, item, m)
		ffprobeMeta, ffprobeErr := core.FetchFFProbeMetadata(m.ctx, m.ffprobeProvider, item)

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

		combined := core.MergeMetadata(item, base, extra...)

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

		// Send result with cancellation check
		select {
		case resultCh <- metadataResult{
			Item: item,
			Meta: combined,
			Errs: errs,
		}:
		case <-m.ctx.Done():
			return
		}
	}
}

// processResults handles results from the provided result channel
func (m *MetadataProgressModel) processResults(resultCh <-chan metadataResult, done chan bool) {
	for result := range resultCh {
		if result.Meta != nil {
			m.Set(result.Item.Key, result.Meta)
		}

		if len(result.Errs) > 0 {
			for _, err := range result.Errs {
				if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				var provErr *provider.ProviderError
				if errors.As(err, &provErr) && provErr.Code == "NOT_FOUND" {
					continue
				}
				m.errorsMu.Lock()
				m.errors = append(m.errors, fmt.Errorf("%s: %w", result.Item.Name, err))
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
	if m.localProvider == nil {
		m.localProvider = local.New()
	}
	return core.CollectMetadataItems(m.tree, m.localProvider)
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
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
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
