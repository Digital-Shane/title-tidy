package tui

import (
	"context"
	"fmt"
	"sync"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/util"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MetadataProgressModel displays progress while fetching metadata from TMDB
type MetadataProgressModel struct {
	// Tree to process
	tree *treeview.Tree[treeview.FileInfo]
	cfg  *config.FormatConfig

	// Progress tracking
	totalItems     int
	processedItems int
	currentItem    string

	// Results storage
	metadata     map[string]*provider.EnrichedMetadata
	metadataMu   sync.RWMutex
	tmdbProvider *provider.TMDBProvider

	// UI components
	width    int
	height   int
	progress progress.Model
	msgCh    chan tea.Msg
	done     bool
	err      error
}

// metadataProgressMsg updates progress
type metadataProgressMsg struct {
	item string
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

	return &MetadataProgressModel{
		tree:         tree,
		cfg:          cfg,
		totalItems:   items,
		width:        80,
		height:       12,
		progress:     p,
		msgCh:        make(chan tea.Msg, 64),
		metadata:     make(map[string]*provider.EnrichedMetadata),
		tmdbProvider: tmdbProvider,
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
	// Collect unique items to fetch
	items := m.collectMetadataItems()

	// Process each item
	for _, item := range items {
		// Check if already cached
		m.metadataMu.RLock()
		if _, exists := m.metadata[item.Key]; exists {
			m.metadataMu.RUnlock()
			m.processedItems++
			continue
		}
		m.metadataMu.RUnlock()

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

		// Update current item
		m.currentItem = currentDesc
		select {
		case m.msgCh <- metadataProgressMsg{item: currentDesc}:
		default:
		}

		// Fetch metadata using the helper function
		meta := util.FetchMetadataWithDependencies(m.tmdbProvider, item.Name, item.Year, item.Season, item.Episode, item.IsMovie, m.metadata)

		// Store result if successful
		if meta != nil {
			m.metadataMu.Lock()
			m.metadata[item.Key] = meta
			m.metadataMu.Unlock()
		}

		m.processedItems++
	}

	m.done = true
	m.msgCh <- metadataCompleteMsg{}
}

// collectMetadataItems collects all unique items that need metadata
func (m *MetadataProgressModel) collectMetadataItems() []MetadataItem {
	var items []MetadataItem
	seen := make(map[string]bool)

	// First pass: collect all shows/movies (depth 0)
	for ni := range m.tree.BreadthFirst(context.Background()) {
		if ni.Depth == 0 {
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
					})
				}
			}
		}
	}

	// Second pass: collect seasons and episodes (only for shows, not movies)
	for ni := range m.tree.BreadthFirst(context.Background()) {
		switch ni.Depth {
		case 1: // Seasons
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
						})
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
							items = append(items, MetadataItem{
								Name:    showName,
								Year:    year,
								Season:  season,
								Episode: episode,
								Key:     key,
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

	info := fmt.Sprintf("Items processed: %d/%d", m.processedItems, m.totalItems)

	currentStyle := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true)

	current := ""
	if m.currentItem != "" {
		current = currentStyle.Render(fmt.Sprintf("Current: %s", m.currentItem))
	}

	statsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1).
		Width(m.width - 4)

	stats := fmt.Sprintf("Total Items: %d\nProcessed: %d\nProgress: %d%%",
		m.totalItems, m.processedItems, percent)

	status := lipgloss.NewStyle().
		Background(colorSecondary).
		Foreground(colorBackground).
		Width(m.width).
		Render("Fetching metadata... please wait")

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		bar,
		info,
		current,
		statsStyle.Render(stats),
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
	return m.err
}
