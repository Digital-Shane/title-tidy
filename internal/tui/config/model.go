package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Option configures the configuration TUI model.
const variablesAutoScrollInterval = 3 * time.Second

type variablesTickMsg struct{}

// Model orchestrates the configuration UI.
type Model struct {
	config   *config.FormatConfig
	original *config.FormatConfig

	state            ConfigState
	templateRegistry *config.TemplateRegistry
	theme            theme.Theme
	icons            map[string]string
	sections         []sectionModel
	providerSection  *providerSection
	activeIndex      int

	variables     *viewport.Model
	variablesAuto bool

	width, height int

	saveStatus string
	err        error
}

// New creates a new configuration UI model.
func New() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	original := cloneFormatConfig(cfg)

	m := &Model{
		config:   cfg,
		original: original,
		theme:    theme.Default(),
	}

	m.icons = m.theme.IconSet()

	m.state = buildStateFromConfig(cfg, m.theme)
	m.initSections()
	m.variables = components.NewViewport(0, 0, m.theme)
	m.variablesAuto = true
	m.refreshVariablesPanel()

	return m, nil
}

// NewWithRegistry creates a new configuration UI model with a template registry.
func NewWithRegistry(reg *config.TemplateRegistry) (*Model, error) {
	m, err := New()
	if err != nil {
		return nil, err
	}
	m.templateRegistry = reg
	m.refreshVariablesPanel()
	return m, nil
}

func (m *Model) initSections() {
	m.sections = []sectionModel{
		newTemplateSection(&m.state.Templates.Show, m.theme),
		newTemplateSection(&m.state.Templates.Season, m.theme),
		newTemplateSection(&m.state.Templates.Episode, m.theme),
		newTemplateSection(&m.state.Templates.Movie, m.theme),
		newRenameSection(&m.state.Rename, m.theme),
		newLoggingSection(&m.state.Logging, m.theme),
	}
	provider := newProviderSection(&m.state.Providers, m.theme)
	m.sections = append(m.sections, provider)
	m.providerSection = provider
	m.activeIndex = 0
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if cmd := m.scheduleVariablesTick(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.sections[m.activeIndex].Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.sections[m.activeIndex].Section() == SectionProviders {
		if cmd := m.providerSection.Activate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.variables != nil {
		updated, cmd := m.variables.Update(msg)
		*m.variables = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)
		m.refreshVariablesPanel()
		return m, tea.Batch(cmds...)

	case variablesTickMsg:
		if m.variablesAuto {
			m.autoScrollVariables()
		}
		if cmd := m.scheduleVariablesTick(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tmdbValidateCmd, tmdbValidationMsg, tvdbValidateCmd, tvdbValidationMsg, omdbValidateCmd, omdbValidationMsg:
		if cmd := m.handleProviderMessage(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.refreshVariablesPanel()
		return m, tea.Batch(cmds...)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		case tea.KeyCtrlS:
			m.save()
			m.refreshVariablesPanel()
			return m, tea.Batch(cmds...)
		case tea.KeyCtrlR:
			m.reset()
			m.refreshVariablesPanel()
			return m, tea.Batch(cmds...)
		case tea.KeyTab:
			if cmd := m.setActiveSection((m.activeIndex + 1) % len(m.sections)); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.refreshVariablesPanel()
			return m, tea.Batch(cmds...)
		case tea.KeyShiftTab:
			next := (m.activeIndex - 1 + len(m.sections)) % len(m.sections)
			if cmd := m.setActiveSection(next); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.refreshVariablesPanel()
			return m, tea.Batch(cmds...)
		case tea.KeySpace:
			if key.Alt {
				m.variablesAuto = !m.variablesAuto
				if m.variablesAuto {
					if cmd := m.scheduleVariablesTick(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
				return m, tea.Batch(cmds...)
			}
		case tea.KeyUp:
			if m.activeSection() != SectionLogging && m.activeSection() != SectionProviders {
				m.disableVariablesAuto()
				m.scrollVariables(-1, 1)
			}
		case tea.KeyDown:
			if m.activeSection() != SectionLogging && m.activeSection() != SectionProviders {
				m.disableVariablesAuto()
				m.scrollVariables(1, 1)
			}
		case tea.KeyPgUp:
			if m.activeSection() != SectionLogging && m.activeSection() != SectionProviders && m.variables != nil {
				m.disableVariablesAuto()
				m.scrollVariables(-1, m.variables.Height/2)
			}
		case tea.KeyPgDown:
			if m.activeSection() != SectionLogging && m.activeSection() != SectionProviders && m.variables != nil {
				m.disableVariablesAuto()
				m.scrollVariables(1, m.variables.Height/2)
			}
		}
	}

	if cmd := m.dispatchToActiveSection(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.refreshVariablesPanel()
	return m, tea.Batch(cmds...)
}

func (m *Model) handleProviderMessage(msg tea.Msg) tea.Cmd {
	if m.providerSection == nil {
		return nil
	}
	switch msg := msg.(type) {
	case tmdbValidateCmd:
		_, cmd := m.providerSection.handleTMDBValidateCmd(msg)
		return cmd
	case tmdbValidationMsg:
		_, cmd := m.providerSection.handleTMDBValidationMsg(msg)
		return cmd
	case tvdbValidateCmd:
		_, cmd := m.providerSection.handleTVDBValidateCmd(msg)
		return cmd
	case tvdbValidationMsg:
		_, cmd := m.providerSection.handleTVDBValidationMsg(msg)
		return cmd
	case omdbValidateCmd:
		_, cmd := m.providerSection.handleOMDBValidateCmd(msg)
		return cmd
	case omdbValidationMsg:
		_, cmd := m.providerSection.handleOMDBValidationMsg(msg)
		return cmd
	default:
		return nil
	}
}

func (m *Model) dispatchToActiveSection(msg tea.Msg) tea.Cmd {
	_, cmd := m.sections[m.activeIndex].Update(msg)
	return cmd
}

func (m *Model) handleWindowResize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 4
	if rightWidth < 0 {
		rightWidth = 0
	}

	panelHeight := m.height - 10
	if panelHeight < 0 {
		panelHeight = 0
	}

	if m.activeSection() != SectionProviders {
		viewportWidth := leftWidth - 4
		viewportHeight := panelHeight - 4
		if viewportWidth < 0 {
			viewportWidth = 0
		}
		if viewportHeight < 0 {
			viewportHeight = 0
		}
		if m.variables != nil {
			m.variables.Width = viewportWidth
			m.variables.Height = viewportHeight
		}
	}

	for _, sec := range m.sections {
		sec.Resize(rightWidth - 2)
	}
}

func (m *Model) setActiveSection(idx int) tea.Cmd {
	if idx == m.activeIndex {
		return nil
	}
	m.sections[m.activeIndex].Blur()
	m.activeIndex = idx
	cmd := m.sections[m.activeIndex].Focus()
	if m.activeSection() == SectionProviders {
		if activate := m.providerSection.Activate(); activate != nil {
			cmd = tea.Batch(cmd, activate)
		}
	}
	return cmd
}

func (m *Model) activeSection() Section {
	return m.sections[m.activeIndex].Section()
}

// View renders the UI.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	if m.width < 30 || m.height < 10 {
		return "Terminal too small. Please resize to at least 30x10."
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Colors().Primary).
		Padding(1, 0).
		Align(lipgloss.Center).
		Width(m.width).
		Render(m.icons["title"] + " Title-Tidy Format Configuration")

	tabs := m.renderTabs()

	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 4
	if rightWidth < 0 {
		rightWidth = 0
	}
	panelHeight := m.height - 10
	if panelHeight < 0 {
		panelHeight = 0
	}

	leftPanel := m.renderLeftPanel(leftWidth, panelHeight)
	rightPanel := m.renderRightPanel(rightWidth, panelHeight)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, title, tabs, panels, status)
}

func (m *Model) renderTabs() string {
	labels := []string{"Show Folder", "Season Folder", "Episode", "Movie", "Rename", "Logging", "Providers"}
	base := lipgloss.NewStyle().Padding(0, 2)
	active := base.Bold(true).Foreground(m.theme.Colors().Primary)

	rendered := make([]string, len(labels))
	for i, label := range labels {
		style := base
		if i == m.activeIndex {
			style = active
			label = "[ " + label + " ]"
		}
		rendered[i] = style.Render(label)
	}
	joined := lipgloss.JoinHorizontal(lipgloss.Center, rendered...)
	return lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).Render(joined)
}

func (m *Model) renderLeftPanel(width, height int) string {
	panel := m.theme.PanelStyle().Width(width).Height(height)

	var content string
	if m.activeSection() == SectionProviders {
		content = m.renderProvidersSidebar(width - 4)
	} else {
		content = m.renderVariablesSidebar(width - 4)
	}

	return panel.Render(content)
}

func (m *Model) renderProvidersSidebar(width int) string {
	title := m.theme.PanelTitleStyle().Render("Provider Controls")
	bodyStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted)
	if width > 0 {
		bodyStyle = bodyStyle.Width(width)
	}
	lines := []string{
		"Shared settings apply to every metadata provider.",
		"Use Up/Down to move between fields.",
		"Press Space or Enter to toggle; type numbers to edit worker count.",
	}
	body := bodyStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, title, body)
}

func (m *Model) renderVariablesSidebar(width int) string {
	if width < 0 {
		width = 0
	}

	title := m.theme.PanelTitleStyle().Render("Template Variables")

	muted := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted)
	if width > 0 {
		muted = muted.Width(width)
	}

	info := muted.Render("Showing template tokens available for the active section.")

	indicatorStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Accent)
	indicatorText := "Auto Scroll: On (Alt+Space)"
	if !m.variablesAuto {
		indicatorStyle = muted
		indicatorText = "Auto Scroll: Off (Alt+Space)"
	}
	indicator := indicatorStyle.Render(indicatorText)

	body := m.variables.View()
	if strings.TrimSpace(body) == "" {
		body = muted.Render("No template variables available.")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		indicator,
		info,
		"",
		body,
	)
}

func (m *Model) renderRightPanel(width, height int) string {
	panel := m.theme.PanelStyle().Width(width).Height(height)

	sectionView := m.sections[m.activeIndex].View()

	if m.activeSection() == SectionProviders {
		return panel.Render(sectionView)
	}

	previews := buildPreviews(m.activeSection(), &m.state, m.icons, m.templateRegistry)
	previewView := m.renderPreview(previews, width-2)
	separator := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted).Render(strings.Repeat("─", max(width-2, 0)))

	content := lipgloss.JoinVertical(lipgloss.Left, sectionView, separator, previewView)
	return panel.Render(content)
}

func (m *Model) renderPreview(previews []preview, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Success)
	lines := []string{m.theme.PanelTitleStyle().Render("Live Previews:"), ""}
	for _, p := range previews {
		line := fmt.Sprintf("%s %s %s", p.icon, labelStyle.Render(p.label+":"), valueStyle.Render(p.preview))
		if lipgloss.Width(line) > width && width > 3 {
			line = line[:width-3] + "..."
		}
		lines = append(lines, line)
	}
	if m.activeSection() != SectionProviders && m.activeSection() != SectionLogging && m.activeSection() != SectionRename {
		if !m.state.Providers.TMDB.Enabled && !m.state.Providers.TVDB.Enabled && !m.state.Providers.OMDB.Enabled {
			hintStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Accent).Italic(true)
			lines = append(lines, "", hintStyle.Render("Enable TMDB, TVDB, or OMDb for richer variables"))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderStatusBar() string {
	key := lipgloss.NewStyle().Foreground(m.theme.Colors().Accent).Bold(true)
	help := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted)
	success := m.theme.StatusBarStyle().Foreground(m.theme.Colors().Background)
	failure := lipgloss.NewStyle().Foreground(m.theme.Colors().Error).Bold(true)

	parts := []string{
		key.Render("Tab") + ": Switch",
		key.Render("Type") + ": Edit",
		key.Render("Ctrl+S") + ": Save",
		key.Render("Ctrl+R") + ": Reset",
		key.Render("Esc/Ctrl+C") + ": Quit",
	}
	if m.variablesOverflowing() {
		parts = append(parts, key.Render("↑↓")+": Scroll", key.Render("Alt+Space")+": Toggle auto")
	}

	line := help.Render(strings.Join(parts, " │ "))
	if m.saveStatus != "" {
		if m.err != nil {
			line += " │ " + failure.Render(m.saveStatus)
		} else {
			line += " │ " + success.Render(m.saveStatus)
		}
	}
	return line
}

func (m *Model) save() {
	m.config.ShowFolder = stripNullChars(m.state.Templates.Show.Input.Value())
	m.config.SeasonFolder = stripNullChars(m.state.Templates.Season.Input.Value())
	m.config.Episode = stripNullChars(m.state.Templates.Episode.Input.Value())
	m.config.Movie = stripNullChars(m.state.Templates.Movie.Input.Value())
	m.config.PreserveExistingTags = m.state.Rename.PreserveExistingTags

	m.config.EnableLogging = m.state.Logging.Enabled
	retention := stripNullChars(m.state.Logging.Retention.Value())
	if retention == "" {
		m.config.LogRetentionDays = 30
	} else if days, err := strconv.Atoi(retention); err == nil && days > 0 {
		m.config.LogRetentionDays = days
	}

	m.config.EnableTMDBLookup = m.state.Providers.TMDB.Enabled
	m.config.TMDBAPIKey = stripNullChars(m.state.Providers.TMDB.APIKey.Value())
	m.config.TMDBLanguage = stripNullChars(m.state.Providers.TMDB.Language.Value())
	m.config.EnableTVDBLookup = m.state.Providers.TVDB.Enabled
	m.config.TVDBAPIKey = stripNullChars(m.state.Providers.TVDB.APIKey.Value())
	m.config.EnableOMDBLookup = m.state.Providers.OMDB.Enabled
	m.config.OMDBAPIKey = stripNullChars(m.state.Providers.OMDB.APIKey.Value())
	m.config.EnableFFProbe = m.state.Providers.FFProbeEnabled

	workerCount := stripNullChars(m.state.Providers.WorkerCount.Value())
	if workerCount == "" {
		m.config.TMDBWorkerCount = 10
	} else if count, err := strconv.Atoi(workerCount); err == nil && count > 0 {
		m.config.TMDBWorkerCount = count
	} else {
		m.config.TMDBWorkerCount = 10
	}

	if err := m.config.Save(); err != nil {
		m.err = err
		m.saveStatus = "Failed to save: " + err.Error()
		return
	}

	m.err = nil
	m.saveStatus = "Configuration saved!"
	m.original = cloneFormatConfig(m.config)
}

func (m *Model) reset() {
	m.state.Templates.Show.Input.SetValue(m.original.ShowFolder)
	m.state.Templates.Show.Input.CursorEnd()
	m.state.Templates.Season.Input.SetValue(m.original.SeasonFolder)
	m.state.Templates.Season.Input.CursorEnd()
	m.state.Templates.Episode.Input.SetValue(m.original.Episode)
	m.state.Templates.Episode.Input.CursorEnd()
	m.state.Templates.Movie.Input.SetValue(m.original.Movie)
	m.state.Templates.Movie.Input.CursorEnd()
	m.state.Rename.PreserveExistingTags = m.original.PreserveExistingTags

	m.state.Logging.Enabled = m.original.EnableLogging
	m.state.Logging.Retention.SetValue(fmt.Sprintf("%d", m.original.LogRetentionDays))
	m.state.Logging.Retention.CursorEnd()

	m.state.Providers.TMDB.Enabled = m.original.EnableTMDBLookup
	m.state.Providers.TMDB.APIKey.SetValue(m.original.TMDBAPIKey)
	m.state.Providers.TMDB.APIKey.CursorEnd()
	m.state.Providers.TMDB.Language.SetValue(m.original.TMDBLanguage)
	m.state.Providers.TMDB.Language.CursorEnd()
	m.state.Providers.TMDB.Validation.Reset()

	m.state.Providers.TVDB.Enabled = m.original.EnableTVDBLookup
	m.state.Providers.TVDB.APIKey.SetValue(m.original.TVDBAPIKey)
	m.state.Providers.TVDB.APIKey.CursorEnd()
	m.state.Providers.TVDB.Validation.Reset()

	m.state.Providers.OMDB.Enabled = m.original.EnableOMDBLookup
	m.state.Providers.OMDB.APIKey.SetValue(m.original.OMDBAPIKey)
	m.state.Providers.OMDB.APIKey.CursorEnd()
	m.state.Providers.OMDB.Validation.Reset()

	m.state.Providers.FFProbeEnabled = m.original.EnableFFProbe
	m.state.Providers.WorkerCount.SetValue(fmt.Sprintf("%d", m.original.TMDBWorkerCount))
	m.state.Providers.WorkerCount.CursorEnd()

	m.saveStatus = "Reset to saved values"
	m.err = nil
}

func (m *Model) disableVariablesAuto() {
	if m.variablesAuto {
		m.variablesAuto = false
	}
}

func (m *Model) scrollVariables(direction int, lines int) {
	if m.variables == nil {
		return
	}
	if lines < 1 {
		lines = 1
	}
	switch {
	case direction < 0:
		m.variables.ScrollUp(lines)
	case direction > 0:
		m.variables.ScrollDown(lines)
	}
}

func (m *Model) autoScrollVariables() {
	if m.variables == nil {
		return
	}
	if !m.variablesOverflowing() {
		return
	}
	if m.variables.AtBottom() {
		m.variables.GotoTop()
		return
	}
	m.variables.ScrollDown(4)
}

func (m *Model) scheduleVariablesTick() tea.Cmd {
	if !m.variablesAuto || m.variables == nil || variablesAutoScrollInterval <= 0 {
		return nil
	}
	return tea.Tick(variablesAutoScrollInterval, func(time.Time) tea.Msg { return variablesTickMsg{} })
}

func (m *Model) variablesOverflowing() bool {
	if m.variables == nil {
		return false
	}
	return m.variables.TotalLineCount() > m.variables.Height+1
}

func (m *Model) refreshVariablesPanel() {
	vars := buildVariables(m.activeSection(), &m.state, m.templateRegistry)
	if len(vars) == 0 {
		m.variables.SetContent("")
		return
	}

	nameStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Accent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Muted)
	exampleStyle := lipgloss.NewStyle().Foreground(m.theme.Colors().Primary).Italic(true)

	lines := make([]string, 0, len(vars)*3)
	for _, v := range vars {
		lines = append(lines, nameStyle.Render(v.name))
		lines = append(lines, descStyle.Render("  "+v.description))
		if v.example != "" {
			lines = append(lines, exampleStyle.Render("  Example: "+v.example))
		}
		lines = append(lines, "")
	}
	m.variables.SetContent(strings.Join(lines, "\n"))
}

func cloneFormatConfig(cfg *config.FormatConfig) *config.FormatConfig {
	return &config.FormatConfig{
		ShowFolder:           cfg.ShowFolder,
		SeasonFolder:         cfg.SeasonFolder,
		Episode:              cfg.Episode,
		Movie:                cfg.Movie,
		PreserveExistingTags: cfg.PreserveExistingTags,
		LogRetentionDays:     cfg.LogRetentionDays,
		EnableLogging:        cfg.EnableLogging,
		TMDBAPIKey:           cfg.TMDBAPIKey,
		EnableTMDBLookup:     cfg.EnableTMDBLookup,
		TMDBLanguage:         cfg.TMDBLanguage,
		TMDBWorkerCount:      cfg.TMDBWorkerCount,
		TVDBAPIKey:           cfg.TVDBAPIKey,
		EnableTVDBLookup:     cfg.EnableTVDBLookup,
		OMDBAPIKey:           cfg.OMDBAPIKey,
		EnableOMDBLookup:     cfg.EnableOMDBLookup,
		EnableFFProbe:        cfg.EnableFFProbe,
	}
}

func buildStateFromConfig(cfg *config.FormatConfig, th theme.Theme) ConfigState {
	tmpl := TemplateSections{
		Show: TemplateSectionState{
			Section: SectionShowFolder,
			Title:   "Show Folder",
			Input:   newTemplateInput(cfg.ShowFolder, th),
		},
		Season: TemplateSectionState{
			Section: SectionSeasonFolder,
			Title:   "Season Folder",
			Input:   newTemplateInput(cfg.SeasonFolder, th),
		},
		Episode: TemplateSectionState{
			Section: SectionEpisode,
			Title:   "Episode",
			Input:   newTemplateInput(cfg.Episode, th),
		},
		Movie: TemplateSectionState{
			Section: SectionMovie,
			Title:   "Movie",
			Input:   newTemplateInput(cfg.Movie, th),
		},
	}

	retention := textinput.New()
	configureInput(&retention, th)
	retention.SetValue(fmt.Sprintf("%d", cfg.LogRetentionDays))
	retention.CursorEnd()
	retention.CharLimit = 3

	worker := textinput.New()
	configureInput(&worker, th)
	worker.SetValue(fmt.Sprintf("%d", cfg.TMDBWorkerCount))
	worker.CursorEnd()
	worker.CharLimit = 3

	tmdbKey := textinput.New()
	configureInput(&tmdbKey, th)
	tmdbKey.SetValue(cfg.TMDBAPIKey)
	tmdbKey.CursorEnd()

	tmdbLang := textinput.New()
	configureInput(&tmdbLang, th)
	tmdbLang.SetValue(cfg.TMDBLanguage)
	tmdbLang.CursorEnd()
	tmdbLang.CharLimit = 5

	tvdbKey := textinput.New()
	configureInput(&tvdbKey, th)
	tvdbKey.SetValue(cfg.TVDBAPIKey)
	tvdbKey.CursorEnd()

	omdbKey := textinput.New()
	configureInput(&omdbKey, th)
	omdbKey.SetValue(cfg.OMDBAPIKey)
	omdbKey.CursorEnd()

	return ConfigState{
		Templates: tmpl,
		Rename: RenameState{
			PreserveExistingTags: cfg.PreserveExistingTags,
		},
		Logging: LoggingState{
			Enabled:   cfg.EnableLogging,
			Focus:     LoggingFieldToggle,
			Retention: retention,
		},
		Providers: ProviderState{
			WorkerCount:    worker,
			Active:         ProviderFieldWorkers,
			FFProbeEnabled: cfg.EnableFFProbe,
			TMDB: ProviderServiceState{
				Enabled:  cfg.EnableTMDBLookup,
				APIKey:   tmdbKey,
				Language: tmdbLang,
			},
			TVDB: ProviderServiceState{
				Enabled: cfg.EnableTVDBLookup,
				APIKey:  tvdbKey,
			},
			OMDB: ProviderServiceState{
				Enabled: cfg.EnableOMDBLookup,
				APIKey:  omdbKey,
			},
		},
	}
}

func newTemplateInput(value string, th theme.Theme) textinput.Model {
	ti := textinput.New()
	configureInput(&ti, th)
	ti.SetValue(value)
	ti.CursorEnd()
	ti.Width = 64
	return ti
}

func configureInput(ti *textinput.Model, th theme.Theme) {
	ti.Prompt = ""
	ti.Placeholder = ""
	ti.CursorStyle = lipgloss.NewStyle().Background(th.Colors().Accent).Foreground(th.Colors().Background)
	ti.TextStyle = lipgloss.NewStyle().Foreground(th.Colors().Primary)
	ti.Focus()
	ti.Blur()
}

func stripNullChars(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
