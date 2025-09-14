package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MetadataProgressModel displays progress while fetching metadata from TMDB
type MetadataProgressModel struct {
	// Tree to process
	tree *treeview.Tree[treeview.FileInfo]
	cfg  *config.FormatConfig

	// Progress tracking
	totalItems     int
	processedItems int
	currentPhase   string
	activeWorkers  int
	workersMu      sync.RWMutex

	// Results storage
	metadata     map[string]*provider.EnrichedMetadata
	metadataMu   sync.RWMutex
	tmdbProvider *provider.TMDBProvider

	// Worker pool
	workerCount int
	workCh      chan MetadataItem
	resultCh    chan metadataResult

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

// metadataProgressMsg updates progress
type metadataProgressMsg struct {
	phase string
	item  string
}

// metadataCompleteMsg signals completion
type metadataCompleteMsg struct{}

// MetadataItem represents an item that needs metadata
type MetadataItem struct {
	Name    string
	Year    string
	Season  int
	Episode int
	IsMovie bool
	Key     string // Unique key for caching
	Phase   int    // 0=show/movie, 1=season, 2=episode
}

// metadataResult represents the result of fetching metadata
type metadataResult struct {
	item MetadataItem
	meta *provider.EnrichedMetadata
	err  error
}

// NewMetadataProgressModel creates a new metadata progress model
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig) *MetadataProgressModel {
	// Count items that need metadata
	items := countMetadataItems(tree)

	p := progress.New(progress.WithGradient(string(colorPrimary), string(colorAccent)))
	p.Width = 50

	// Initialize TMDB provider if enabled
	var tmdbProvider *provider.TMDBProvider
	if cfg.EnableTMDBLookup && cfg.TMDBAPIKey != "" {
		var err error
		tmdbProvider, err = provider.NewTMDBProvider(cfg.TMDBAPIKey, cfg.TMDBLanguage)
		if err != nil {
			// If provider creation fails, continue without TMDB
			tmdbProvider = nil
		}
	}

	// Use configured worker count, with a sensible minimum
	workerCount := cfg.TMDBWorkerCount
	if workerCount <= 0 {
		workerCount = 20 // Fallback to default if not configured or invalid
	}

	return &MetadataProgressModel{
		tree:         tree,
		cfg:          cfg,
		totalItems:   items,
		width:        80,
		height:       12,
		progress:     p,
		msgCh:        make(chan tea.Msg, 256),
		metadata:     make(map[string]*provider.EnrichedMetadata),
		tmdbProvider: tmdbProvider,
		workerCount:  workerCount,
		workCh:       make(chan MetadataItem, 100),
		resultCh:     make(chan metadataResult, 100),
		errors:       make([]error, 0),
	}
}

// countMetadataItems counts unique items that need metadata fetching
func countMetadataItems(tree *treeview.Tree[treeview.FileInfo]) int {
	seen := make(map[string]bool)
	count := 0

	for ni := range tree.BreadthFirst(context.Background()) {
		switch ni.Depth {
		case 0: // Shows/Movies
			name, year := config.ExtractNameAndYear(ni.Node.Name())
			if name != "" {
				// Check if it's a movie or show based on content
				isMovie := true
				for _, child := range ni.Node.Children() {
					if child.Data().IsDir() {
						// If has season folders, it's a show
						if _, found := media.ExtractSeasonNumber(child.Name()); found {
							isMovie = false
							break
						}
					}
				}

				var key string
				if isMovie {
					key = util.GenerateMetadataKey("movie", name, year, 0, 0)
				} else {
					key = util.GenerateMetadataKey("show", name, year, 0, 0)
				}
				if !seen[key] {
					seen[key] = true
					count++
				}
			}
		case 1: // Seasons
			if parent := ni.Node.Parent(); parent != nil {
				showName, year := config.ExtractNameAndYear(parent.Name())
				if season, found := media.ExtractSeasonNumber(ni.Node.Name()); found && showName != "" {
					key := util.GenerateMetadataKey("season", showName, year, season, 0)
					if !seen[key] {
						seen[key] = true
						count++
					}
				}
			}
		case 2: // Episodes
			if parent := ni.Node.Parent(); parent != nil {
				if grandparent := parent.Parent(); grandparent != nil {
					showName, year := config.ExtractNameAndYear(grandparent.Name())
					if season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node); found && showName != "" {
						key := util.GenerateMetadataKey("episode", showName, year, season, episode)
						if !seen[key] {
							seen[key] = true
							count++
						}
					}
				}
			}
		}
	}

	return count
}

// Init starts the async metadata fetching
func (m *MetadataProgressModel) Init() tea.Cmd {
	if m.tmdbProvider == nil {
		// No TMDB provider, skip metadata fetching
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

		// Capture current channels by value to avoid race conditions
		workCh := m.workCh
		resultCh := m.resultCh

		// Start worker goroutines with captured channel
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
			for _, item := range phaseItems {
				// Skip if already cached
				m.metadataMu.RLock()
				if _, exists := m.metadata[item.Key]; exists {
					m.metadataMu.RUnlock()
					m.processedItems++
					continue
				}
				m.metadataMu.RUnlock()

				workCh <- item
			}
			close(workCh)
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
		default:
		}

		// Fetch metadata with retry logic for rate limiting
		var meta *provider.EnrichedMetadata
		var err error
		retryCount := 0

		for {
			meta, err = util.FetchMetadataWithDependencies(m.tmdbProvider, item.Name, item.Year, item.Season, item.Episode, item.IsMovie, m.metadata)

			// If we got metadata successfully, break out
			if meta != nil {
				err = nil // Clear any error since we succeeded
				break
			}

			// Handle different error types
			if err == provider.ErrRateLimited {
				// For rate limiting, NEVER give up - just wait and retry
				// Use exponential backoff starting at 2 seconds, capping at 32 seconds
				waitTime := time.Duration(2<<uint(min(retryCount, 4))) * time.Second
				time.Sleep(waitTime)
				retryCount++
				// Continue the loop - never break on rate limiting
				continue
			} else if err != nil {
				// For non-retryable errors (API key invalid, etc.), break immediately
				break
			} else {
				// If err is nil but meta is also nil, it means no results found
				// This is not an error we should retry
				break
			}
		}

		// Send result
		resultCh <- metadataResult{
			item: item,
			meta: meta,
			err:  err,
		}
	}
}

// processResults handles results from the provided result channel
func (m *MetadataProgressModel) processResults(resultCh <-chan metadataResult, done chan bool) {
	for result := range resultCh {
		if result.meta != nil {
			m.metadataMu.Lock()
			m.metadata[result.item.Key] = result.meta
			m.metadataMu.Unlock()
		}

		if result.err != nil && result.err != provider.ErrNoResults {
			m.errorsMu.Lock()
			m.errors = append(m.errors, fmt.Errorf("%s: %w", result.item.Name, result.err))
			m.errorsMu.Unlock()
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
	var items []MetadataItem
	seen := make(map[string]bool)

	// First pass: collect all shows/movies (depth 0) - Phase 0
	for ni := range m.tree.BreadthFirst(context.Background()) {
		if ni.Depth == 0 {
			// Check if this is an episode file (contains season/episode pattern)
			if !ni.Node.Data().IsDir() {
				if season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node); found {
					// This is an episode file at depth 0 (episodes command)
					// Extract show name from the part before the season/episode pattern
					showName, year := media.ExtractShowInfo(ni.Node, true)
					if showName != "" {
						// Add the show metadata request
						showKey := util.GenerateMetadataKey("show", showName, year, 0, 0)
						if !seen[showKey] {
							seen[showKey] = true
							items = append(items, MetadataItem{
								Name:    showName,
								Year:    year,
								IsMovie: false,
								Key:     showKey,
								Phase:   0,
							})
						}

						// Add the episode metadata request
						episodeKey := util.GenerateMetadataKey("episode", showName, year, season, episode)
						if !seen[episodeKey] {
							seen[episodeKey] = true
							items = append(items, MetadataItem{
								Name:    showName,
								Year:    year,
								Season:  season,
								Episode: episode,
								Key:     episodeKey,
								Phase:   2, // Episodes are phase 2
							})
						}
					}
					continue // Skip the regular movie/show detection
				}
			}

			name, year := config.ExtractNameAndYear(ni.Node.Name())
			if name != "" {
				// Check if it's a movie or show based on content
				isMovie := true
				for _, child := range ni.Node.Children() {
					if child.Data().IsDir() {
						// If has season folders, it's a show
						if _, found := media.ExtractSeasonNumber(child.Name()); found {
							isMovie = false
							break
						}
					}
				}

				var key string
				if isMovie {
					key = util.GenerateMetadataKey("movie", name, year, 0, 0)
				} else {
					key = util.GenerateMetadataKey("show", name, year, 0, 0)
				}
				if !seen[key] {
					seen[key] = true
					items = append(items, MetadataItem{
						Name:    name,
						Year:    year,
						IsMovie: isMovie,
						Key:     key,
						Phase:   0,
					})
				}
			}
		}
	}

	// Second pass: collect seasons and episodes (only for shows, not movies)
	for ni := range m.tree.BreadthFirst(context.Background()) {
		switch ni.Depth {
		case 1: // Seasons - Phase 1
			if parent := ni.Node.Parent(); parent != nil {
				showName, year := config.ExtractNameAndYear(parent.Name())
				if season, found := media.ExtractSeasonNumber(ni.Node.Name()); found && showName != "" {
					key := util.GenerateMetadataKey("season", showName, year, season, 0)
					if !seen[key] {
						seen[key] = true
						items = append(items, MetadataItem{
							Name:   showName,
							Year:   year,
							Season: season,
							Key:    key,
							Phase:  1,
						})
					}
				}
			}
		case 2: // Episodes - Phase 2
			if parent := ni.Node.Parent(); parent != nil {
				if grandparent := parent.Parent(); grandparent != nil {
					showName, year := config.ExtractNameAndYear(grandparent.Name())
					if season, episode, found := media.ParseSeasonEpisode(ni.Node.Name(), ni.Node); found && showName != "" {
						key := util.GenerateMetadataKey("episode", showName, year, season, episode)
						if !seen[key] {
							seen[key] = true
							items = append(items, MetadataItem{
								Name:    showName,
								Year:    year,
								Season:  season,
								Episode: episode,
								Key:     key,
								Phase:   2,
							})
						}
					}
				}
			}
		}
	}

	return items
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

	header := lipgloss.NewStyle().
		Bold(true).
		Background(colorPrimary).
		Foreground(colorBackground).
		Width(m.width).
		Render("Fetching Metadata from TMDB")

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
func (m *MetadataProgressModel) Metadata() map[string]*provider.EnrichedMetadata {
	return m.metadata
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
			if errors.Is(err, provider.ErrInvalidAPIKey) || errors.Is(err, provider.ErrAPIUnavailable) {
				return err
			}
		}
	}
	return nil
}
