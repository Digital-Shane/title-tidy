package progress

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type metadataEventMsg struct {
	event core.MetadataEvent
	done  bool
}

const metadataErrorBaseLines = 6

// MetadataProgressModel displays progress while fetching metadata from providers.
type MetadataProgressModel struct {
	engine    *core.MetadataEngine
	events    <-chan core.MetadataEvent
	summary   core.MetadataSummary
	errors    []error
	fatalErr  error
	shouldRun bool

	width  int
	height int

	progress progress.Model
	theme    theme.Theme

	ctx    context.Context
	cancel context.CancelFunc

	done bool
}

// NewMetadataProgressModel creates a new metadata progress model.
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, th theme.Theme) *MetadataProgressModel {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	gradient := th.ProgressGradient()
	if len(gradient) < 2 {
		colors := th.Colors()
		gradient = []string{string(colors.Primary), string(colors.Accent)}
	}
	prog := progress.New(progress.WithGradient(gradient[0], gradient[1]))
	prog.Width = 50

	tmdbEnabled := cfg.EnableTMDBLookup && cfg.TMDBAPIKey != ""
	omdbEnabled := cfg.EnableOMDBLookup && cfg.OMDBAPIKey != ""
	ffprobeEnabled := cfg.EnableFFProbe

	engineCfg := core.MetadataEngineConfig{
		Tree:        tree,
		WorkerCount: cfg.TMDBWorkerCount,
		Providers: core.MetadataProvidersConfig{
			TMDB: core.TMDBProviderConfig{
				Enabled:  tmdbEnabled,
				APIKey:   cfg.TMDBAPIKey,
				Language: cfg.TMDBLanguage,
			},
			OMDB: core.OMDBProviderConfig{
				Enabled: omdbEnabled,
				APIKey:  cfg.OMDBAPIKey,
			},
			FFProbe: core.FFProbeProviderConfig{Enabled: ffprobeEnabled},
		},
	}

	if engineCfg.Providers.TMDB.Enabled {
		if engineCfg.Providers.TMDB.Language == "" {
			engineCfg.Providers.TMDB.Language = "en-US"
		}
		cacheEnabled := true
		engineCfg.Providers.TMDB.CacheEnabled = &cacheEnabled
	}

	engine := core.NewMetadataEngine(engineCfg)
	summary := engine.SummarySnapshot()

	return &MetadataProgressModel{
		engine:    engine,
		summary:   summary,
		width:     80,
		height:    12,
		progress:  prog,
		theme:     th,
		shouldRun: len(summary.ActiveProviders) > 0,
	}
}

// Init starts the metadata engine if providers are configured.
func (m *MetadataProgressModel) Init() tea.Cmd {
	if !m.shouldRun {
		m.done = true
		return tea.Quit
	}

	if m.engine == nil {
		m.done = true
		return tea.Quit
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.events = m.engine.Start(m.ctx)
	return m.waitForEvent()
}

func (m *MetadataProgressModel) waitForEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-m.events
		if !ok {
			return metadataEventMsg{done: true}
		}
		return metadataEventMsg{event: evt}
	}
}

// Update processes Bubble Tea messages.
func (m *MetadataProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.Width = msg.Width - 4
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case metadataEventMsg:
		if msg.done {
			m.done = true
			m.errors = m.engine.Errors()
			return m, tea.Quit
		}

		m.summary = msg.event.Summary
		if msg.event.Err != nil && !errors.Is(msg.event.Err, context.Canceled) {
			m.fatalErr = msg.event.Err
		}
		m.errors = m.engine.Errors()

		ratio := 0.0
		if m.summary.TotalItems > 0 {
			ratio = float64(m.summary.ProcessedItems) / float64(m.summary.TotalItems)
		}
		cmd := m.progress.SetPercent(ratio)
		if m.summary.Done {
			m.done = true
			return m, tea.Batch(cmd, tea.Quit)
		}
		return m, tea.Batch(cmd, m.waitForEvent())
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

// View renders the progress UI.
func (m *MetadataProgressModel) View() string {
	if m.fatalErr != nil && !errors.Is(m.fatalErr, context.Canceled) {
		return fmt.Sprintf("Error: %v\n", m.fatalErr)
	}

	if m.summary.TotalItems == 0 {
		return "No items require metadata fetching.\n"
	}

	percent := 0
	if m.summary.TotalItems > 0 {
		percent = 100 * m.summary.ProcessedItems / m.summary.TotalItems
	}

	headerText := "Fetching Metadata"
	if len(m.summary.ActiveProviders) > 0 {
		headerText = fmt.Sprintf("Fetching Metadata (%s)", strings.Join(m.summary.ActiveProviders, ", "))
	}

	infoLines := []string{fmt.Sprintf("Items processed: %d/%d", m.summary.ProcessedItems, m.summary.TotalItems)}

	statsLines := []string{
		fmt.Sprintf("Total Items: %d", m.summary.TotalItems),
		fmt.Sprintf("Processed: %d", m.summary.ProcessedItems),
		fmt.Sprintf("Progress: %d%%", percent),
		fmt.Sprintf("Max Worker Pool: %d workers", m.summary.WorkerLimit),
	}

	errors := make([]string, 0, len(m.errors))
	for _, err := range m.errors {
		errors = append(errors, err.Error())
	}

	statusText := "Fetching metadata in parallel... please wait"
	if m.summary.LastItem != "" {
		statusText = m.summary.LastItem
	}

	sections := []string{
		m.theme.HeaderStyle().Width(m.width).Render(headerText),
	}

	if m.summary.PhaseName != "" {
		colors := m.theme.Colors()
		phase := lipgloss.NewStyle().
			Foreground(colors.Accent).
			Bold(true).
			Render(fmt.Sprintf("Phase: %s | Active Workers: %d", m.summary.PhaseName, m.summary.ActiveWorkers))
		sections = append(sections, phase)
	}

	sections = append(sections, m.progress.View())

	if len(infoLines) > 0 {
		sections = append(sections, strings.Join(infoLines, "\n"))
	}

	if stats := m.renderMetadataStatsPanel(statsLines, errors); stats != "" {
		sections = append(sections, stats)
	}

	status := m.theme.StatusBarStyle().Width(m.width).Render(statusText)
	sections = append(sections, status)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *MetadataProgressModel) renderMetadataStatsPanel(statsLines, errors []string) string {
	panel := m.theme.PanelStyle()
	panelWidth := m.width - panel.GetHorizontalFrameSize()
	if panelWidth < 0 {
		panelWidth = 0
	}

	blocks := make([]string, 0, 2)
	if len(statsLines) > 0 {
		blocks = append(blocks, strings.Join(statsLines, "\n"))
	}
	if errBlock := m.renderMetadataErrorBlock(errors); errBlock != "" {
		blocks = append(blocks, errBlock)
	}

	if len(blocks) == 0 {
		return panel.Width(panelWidth).Render("")
	}

	return panel.Width(panelWidth).Render(strings.Join(blocks, "\n"))
}

func (m *MetadataProgressModel) renderMetadataErrorBlock(errors []string) string {
	if len(errors) == 0 {
		return ""
	}

	colors := m.theme.Colors()
	errorStyle := lipgloss.NewStyle().Foreground(colors.Error)

	availableHeight := m.height - metadataErrorBaseLines
	maxErrorLines := availableHeight - 1
	if maxErrorLines < 1 {
		maxErrorLines = 1
	}

	errorsToShow := len(errors)
	if errorsToShow > maxErrorLines {
		errorsToShow = maxErrorLines
	}

	startIdx := len(errors) - errorsToShow
	if startIdx < 0 {
		startIdx = 0
	}

	availableWidth := m.width - 2
	if availableWidth < 10 {
		availableWidth = 10
	}

	lines := make([]string, 0, errorsToShow+2)
	lines = append(lines, fmt.Sprintf("Errors: %d", len(errors)))
	for i := startIdx; i < len(errors); i++ {
		msg := errors[i]
		if len(msg) > availableWidth {
			msg = msg[:availableWidth-3] + "..."
		}
		lines = append(lines, fmt.Sprintf("• %s", msg))
	}

	if len(errors) > errorsToShow {
		lines = append(lines, fmt.Sprintf("... and %d more", len(errors)-errorsToShow))
	}

	return errorStyle.Render(strings.Join(lines, "\n"))
}

// Metadata returns the fetched metadata.
func (m *MetadataProgressModel) Metadata() map[string]*provider.Metadata {
	if m.engine == nil {
		return nil
	}
	return m.engine.Metadata()
}

// Err returns any fatal error encountered during metadata fetching.
func (m *MetadataProgressModel) Err() error {
	if m.fatalErr != nil && !errors.Is(m.fatalErr, context.Canceled) {
		return m.fatalErr
	}

	for _, err := range m.errors {
		var provErr *provider.ProviderError
		if errors.As(err, &provErr) {
			if provErr.Code == "AUTH_FAILED" || provErr.Code == "UNAVAILABLE" {
				return err
			}
		}
	}
	return nil
}
