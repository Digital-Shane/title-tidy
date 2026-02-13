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
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type metadataEventMsg struct {
	event core.MetadataEvent
	done  bool
}

type metadataRetryFinishedMsg struct {
	provider core.MetadataProviderType
	key      string
	failure  *core.MetadataFailure
	err      error
}

const metadataErrorBaseLines = 6

func newMetadataSearchInput(th theme.Theme) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = ""
	ti.CharLimit = 256
	colors := th.Colors()
	ti.CursorStyle = lipgloss.NewStyle().Foreground(colors.Background).Background(colors.Accent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colors.Primary)
	ti.Width = 64
	ti.Blur()
	return ti
}

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
	input    textinput.Model

	failures            []core.MetadataFailure
	selectedFailure     int
	initialFailureCount int
	manualActive        bool
	manualSkipped       bool
	retrying            bool
	manualStatus        string
	engineFinished      bool

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
	tvdbEnabled := cfg.EnableTVDBLookup && cfg.TVDBAPIKey != ""
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
			TVDB: core.TVDBProviderConfig{
				Enabled: tvdbEnabled,
				APIKey:  cfg.TVDBAPIKey,
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
		engine:       engine,
		summary:      summary,
		width:        80,
		height:       12,
		progress:     prog,
		theme:        th,
		shouldRun:    len(summary.ActiveProviders) > 0,
		input:        newMetadataSearchInput(th),
		manualStatus: "",
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
		m.updateInputWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		if m.manualActive {
			return m.handleManualKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case metadataEventMsg:
		return m.handleMetadataEvent(msg)
	case metadataRetryFinishedMsg:
		return m.handleRetryFinished(msg)
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m *MetadataProgressModel) handleMetadataEvent(msg metadataEventMsg) (tea.Model, tea.Cmd) {
	if msg.done {
		m.engineFinished = true
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if m.manualActive {
			return m, nil
		}
		m.summary = m.engine.SummarySnapshot()
		m.errors = m.engine.Errors()

		failures := m.engine.ProviderFailures()
		if len(failures) > 0 {
			m.failures = failures
			m.initialFailureCount = len(failures)
			m.manualActive = true
			if m.selectedFailure >= len(m.failures) || m.selectedFailure < 0 {
				m.selectedFailure = 0
			}
			m.prepareInputForSelection()
			m.manualStatus = m.describeFailure(m.failures[m.selectedFailure])
			return m, nil
		}

		m.done = true
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
		if m.manualActive {
			return m, tea.Batch(cmd, m.waitForEvent())
		}
		failures := m.engine.ProviderFailures()
		if len(failures) > 0 {
			m.failures = failures
			m.initialFailureCount = len(failures)
			m.manualActive = true
			if m.selectedFailure >= len(m.failures) || m.selectedFailure < 0 {
				m.selectedFailure = 0
			}
			m.prepareInputForSelection()
			m.manualStatus = m.describeFailure(m.failures[m.selectedFailure])
			return m, tea.Batch(cmd, m.waitForEvent())
		}
		m.done = true
		return m, tea.Batch(cmd, tea.Quit)
	}
	return m, tea.Batch(cmd, m.waitForEvent())
}

func (m *MetadataProgressModel) handleRetryFinished(msg metadataRetryFinishedMsg) (tea.Model, tea.Cmd) {
	m.retrying = false
	if msg.err != nil {
		m.manualStatus = fmt.Sprintf("Retry failed: %v", msg.err)
		return m, nil
	}

	m.summary = m.engine.SummarySnapshot()
	m.errors = m.engine.Errors()

	if msg.failure != nil {
		m.updateFailure(*msg.failure)
		m.manualStatus = m.describeFailure(*msg.failure)
		m.prepareInputForSelection()
		return m, nil
	}

	m.failures = m.engine.ProviderFailures()
	if len(m.failures) == 0 {
		m.manualActive = false
		m.done = true
		m.manualStatus = fmt.Sprintf("Resolved metadata via %s", strings.ToUpper(string(msg.provider)))
		return m, tea.Quit
	}
	if m.selectedFailure >= len(m.failures) {
		m.selectedFailure = len(m.failures) - 1
	}
	m.manualStatus = fmt.Sprintf("Resolved via %s. %d remaining.", strings.ToUpper(string(msg.provider)), len(m.failures))
	m.prepareInputForSelection()
	return m, nil
}

func (m *MetadataProgressModel) handleManualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case tea.KeyUp:
		m.moveFailureSelection(-1)
		return m, nil
	case tea.KeyDown:
		m.moveFailureSelection(1)
		return m, nil
	case tea.KeyEnter:
		if m.retrying || len(m.failures) == 0 {
			return m, nil
		}
		failure := m.failures[m.selectedFailure]
		query := m.input.Value()
		m.retrying = true
		m.manualStatus = fmt.Sprintf("Retrying %s…", strings.ToUpper(string(failure.Provider)))
		return m, m.retryFailureCmd(failure, query)
	case tea.KeyCtrlS:
		m.manualSkipped = true
		m.manualActive = false
		m.done = true
		return m, tea.Quit
	}

	switch msg.String() {
	case "shift+tab":
		m.moveFailureSelection(-1)
		return m, nil
	case "tab":
		m.moveFailureSelection(1)
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *MetadataProgressModel) retryFailureCmd(failure core.MetadataFailure, query string) tea.Cmd {
	provider := failure.Provider
	key := failure.Item.Key
	return func() tea.Msg {
		retryCtx := m.ctx
		if retryCtx == nil || retryCtx.Err() != nil {
			retryCtx = context.Background()
		}
		updated, err := m.engine.RetryProvider(retryCtx, key, provider, query)
		return metadataRetryFinishedMsg{provider: provider, key: key, failure: updated, err: err}
	}
}

func (m *MetadataProgressModel) updateFailure(updated core.MetadataFailure) {
	for i := range m.failures {
		if m.failures[i].Item.Key == updated.Item.Key && m.failures[i].Provider == updated.Provider {
			m.failures[i] = updated
			m.selectedFailure = i
			return
		}
	}
	m.failures = append(m.failures, updated)
	m.selectedFailure = len(m.failures) - 1
}

func (m *MetadataProgressModel) moveFailureSelection(delta int) {
	if len(m.failures) == 0 {
		return
	}
	m.selectedFailure += delta
	if m.selectedFailure < 0 {
		m.selectedFailure = 0
	}
	if m.selectedFailure >= len(m.failures) {
		m.selectedFailure = len(m.failures) - 1
	}
	m.prepareInputForSelection()
	m.manualStatus = m.describeFailure(m.failures[m.selectedFailure])
}

func (m *MetadataProgressModel) prepareInputForSelection() {
	if len(m.failures) == 0 {
		m.input.SetValue("")
		return
	}
	m.updateInputWidth(m.width)
	failure := m.failures[m.selectedFailure]
	query := strings.TrimSpace(failure.Query)
	if query == "" {
		query = failure.Item.Name
	}
	m.input.SetValue(query)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *MetadataProgressModel) updateInputWidth(width int) {
	if width <= 0 {
		return
	}
	inputWidth := width - 8
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.Width = inputWidth
}

func (m *MetadataProgressModel) describeFailure(f core.MetadataFailure) string {
	providerLabel := strings.ToUpper(string(f.Provider))
	target := core.FormatMetadataProgressMessage(f.Item)
	query := strings.TrimSpace(f.Query)
	if query == "" {
		query = f.Item.Name
	}
	attemptInfo := ""
	if f.Attempts > 1 {
		attemptInfo = fmt.Sprintf(" (attempt %d)", f.Attempts)
	}
	errText := "unknown error"
	if f.Err != nil {
		errText = f.Err.Error()
	}
	return fmt.Sprintf("[%s] %s%s — query %q: %s", providerLabel, target, attemptInfo, query, errText)
}

// View renders the progress UI.
func (m *MetadataProgressModel) View() string {
	if m.fatalErr != nil && !errors.Is(m.fatalErr, context.Canceled) {
		return fmt.Sprintf("Error: %v\n", m.fatalErr)
	}

	if m.manualActive && len(m.failures) > 0 {
		return m.renderManualResolutionView()
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

func (m *MetadataProgressModel) renderManualResolutionView() string {
	colors := m.theme.Colors()
	remaining := len(m.failures)
	resolved := m.initialFailureCount - remaining
	if resolved < 0 {
		resolved = 0
	}

	header := m.theme.HeaderStyle().Width(m.width).Render("Resolve Metadata Search")
	infoStyle := lipgloss.NewStyle().Foreground(colors.Muted)
	summaryLine := infoStyle.Width(m.width).Render(fmt.Sprintf("Failures remaining: %d (resolved: %d)", remaining, resolved))
	instructions := infoStyle.Width(m.width).Render("Use ↑/↓ to choose an item, edit the search text, press Enter to retry, ctrl+s to skip.")

	list := m.renderFailureList()
	inputSection := m.renderSearchInput()

	statusText := m.manualStatus
	if m.retrying && remaining > 0 {
		statusText = fmt.Sprintf("Retrying %s…", strings.ToUpper(string(m.failures[m.selectedFailure].Provider)))
	}
	if statusText == "" {
		statusText = "Adjust the search term and press Enter to retry."
	}
	status := m.theme.StatusBarStyle().Width(m.width).Render(statusText)

	sections := []string{
		header,
		summaryLine,
		instructions,
		list,
		inputSection,
		status,
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *MetadataProgressModel) renderFailureList() string {
	panel := m.theme.PanelStyle()
	panelWidth := m.width - panel.GetHorizontalFrameSize()
	if panelWidth < 20 {
		panelWidth = 20
	}

	if len(m.failures) == 0 {
		return panel.Width(panelWidth).Render("No failures remaining.")
	}

	colors := m.theme.Colors()
	normalStyle := lipgloss.NewStyle().Foreground(colors.Error)
	selectedStyle := lipgloss.NewStyle().Foreground(colors.Accent).Bold(true)

	entries := make([]string, 0, len(m.failures))
	for i, failure := range m.failures {
		indicator := "•"
		style := normalStyle
		if i == m.selectedFailure {
			indicator = "➜"
			style = selectedStyle
		}
		attemptInfo := ""
		if failure.Attempts > 1 {
			attemptInfo = fmt.Sprintf(" (attempt %d)", failure.Attempts)
		}
		query := strings.TrimSpace(failure.Query)
		if query == "" {
			query = failure.Item.Name
		}
		errText := "unknown error"
		if failure.Err != nil {
			errText = failure.Err.Error()
		}
		block := fmt.Sprintf(
			"%s [%s] %s%s\n  query: %q\n  error: %s",
			indicator,
			strings.ToUpper(string(failure.Provider)),
			core.FormatMetadataProgressMessage(failure.Item),
			attemptInfo,
			query,
			errText,
		)
		entries = append(entries, style.Width(panelWidth).Render(block))
	}

	return panel.Width(panelWidth).Render(strings.Join(entries, "\n\n"))
}

func (m *MetadataProgressModel) renderSearchInput() string {
	if len(m.failures) == 0 {
		return ""
	}
	providerLabel := strings.ToUpper(string(m.failures[m.selectedFailure].Provider))
	title := fmt.Sprintf("Search term (%s): ", providerLabel)
	text := m.input.View()
	return lipgloss.NewStyle().Width(m.width).Render(title + text)
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
