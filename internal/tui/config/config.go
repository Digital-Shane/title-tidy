package config

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// scrollTickMsg is sent periodically to enable auto-scrolling
type scrollTickMsg struct{}

// tmdbValidationMsg carries the result of TMDB API validation
type tmdbValidationMsg struct {
	apiKey string
	valid  bool
}

// tmdbValidateCmd is sent to trigger API validation after debounce delay
type tmdbValidateCmd struct {
	apiKey string
}

// omdbValidationMsg carries the result of OMDb API validation
type omdbValidationMsg struct {
	apiKey string
	valid  bool
}

// omdbValidateCmd triggers OMDb API validation after debounce delay
type omdbValidateCmd struct {
	apiKey string
}

const (
	providerColumnShared = iota
	providerColumnFFProbe
	providerColumnOMDB
	providerColumnTMDB
	providerColumnCount
)

// Section represents a configuration section
type Section int

const (
	SectionShowFolder Section = iota
	SectionSeasonFolder
	SectionEpisode
	SectionMovie
	SectionLogging
	SectionProviders
)

// Option configures the configuration TUI model.
type Option func(*Model)

// WithTheme overrides the default theme used by the configuration UI.
func WithTheme(th theme.Theme) Option {
	return func(m *Model) {
		m.theme = th
	}
}

func (m *Model) ensureTheme() {
	if m.icons != nil {
		return
	}
	if m.theme.Colors() == (theme.Colors{}) {
		m.theme = theme.Default()
	}
	m.icons = m.theme.IconSet()
}

func (m *Model) iconSet() map[string]string {
	m.ensureTheme()
	return m.icons
}

// Model is the Bubble Tea model for the configuration UI
type Model struct {
	config              *config.FormatConfig
	originalConfig      *config.FormatConfig // for reset functionality
	activeSection       Section
	inputs              map[Section]string
	cursorPos           map[Section]int
	loggingEnabled      bool   // current state of logging toggle
	loggingRetention    string // retention days as string for input
	loggingSubfocus     int    // 0=enabled toggle, 1=retention input
	tmdbAPIKey          string // TMDB API key (masked)
	tmdbEnabled         bool   // TMDB lookup enabled
	tmdbLanguage        string // TMDB language code
	workerCount         string // Provider worker count as string for input
	tmdbSubfocus        int    // 0=enabled, 1=api key, 2=language
	ffprobeEnabled      bool   // ffprobe provider enabled
	providerColumnFocus int    // 0=shared, 1=ffprobe, 2=OMDb, 3=TMDB
	tmdbValidation      string // API key validation status: "", "validating", "valid", "invalid"
	tmdbValidatedKey    string // Last API key that was validated
	omdbAPIKey          string
	omdbEnabled         bool
	omdbSubfocus        int
	omdbValidation      string
	omdbValidatedKey    string
	width               int
	height              int
	saveStatus          string
	err                 error
	variablesView       viewport.Model           // viewport for scrolling variables list
	autoScroll          bool                     // whether auto-scrolling is enabled
	templateRegistry    *config.TemplateRegistry // registry for template variables
	theme               theme.Theme
	icons               map[string]string
	tmdbValidate        func(string) tea.Cmd
	tmdbDebounce        func(string) tea.Cmd
	omdbValidate        func(string) tea.Cmd
	omdbDebounce        func(string) tea.Cmd
}

// New creates a new configuration UI model
func New(opts ...Option) (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create a copy for reset functionality
	originalCfg := &config.FormatConfig{
		ShowFolder:       cfg.ShowFolder,
		SeasonFolder:     cfg.SeasonFolder,
		Episode:          cfg.Episode,
		Movie:            cfg.Movie,
		LogRetentionDays: cfg.LogRetentionDays,
		EnableLogging:    cfg.EnableLogging,
		TMDBAPIKey:       cfg.TMDBAPIKey,
		EnableTMDBLookup: cfg.EnableTMDBLookup,
		TMDBLanguage:     cfg.TMDBLanguage,
		TMDBWorkerCount:  cfg.TMDBWorkerCount,
		OMDBAPIKey:       cfg.OMDBAPIKey,
		EnableOMDBLookup: cfg.EnableOMDBLookup,
		EnableFFProbe:    cfg.EnableFFProbe,
	}

	m := &Model{
		config:         cfg,
		originalConfig: originalCfg,
		activeSection:  SectionShowFolder,
		inputs: map[Section]string{
			SectionShowFolder:   cfg.ShowFolder,
			SectionSeasonFolder: cfg.SeasonFolder,
			SectionEpisode:      cfg.Episode,
			SectionMovie:        cfg.Movie,
		},
		cursorPos: map[Section]int{
			SectionShowFolder:   len(cfg.ShowFolder),
			SectionSeasonFolder: len(cfg.SeasonFolder),
			SectionEpisode:      len(cfg.Episode),
			SectionMovie:        len(cfg.Movie),
		},
		loggingEnabled:      cfg.EnableLogging,
		loggingRetention:    fmt.Sprintf("%d", cfg.LogRetentionDays),
		loggingSubfocus:     0,
		tmdbAPIKey:          cfg.TMDBAPIKey,
		tmdbEnabled:         cfg.EnableTMDBLookup,
		tmdbLanguage:        cfg.TMDBLanguage,
		workerCount:         fmt.Sprintf("%d", cfg.TMDBWorkerCount),
		tmdbSubfocus:        0,
		omdbAPIKey:          cfg.OMDBAPIKey,
		omdbEnabled:         cfg.EnableOMDBLookup,
		omdbSubfocus:        0,
		ffprobeEnabled:      cfg.EnableFFProbe,
		providerColumnFocus: providerColumnShared,
		variablesView:       viewport.New(0, 0),
		autoScroll:          true,
		tmdbValidate:        validateTMDBAPIKey,
		tmdbDebounce:        debouncedTMDBValidate,
		omdbValidate:        validateOMDBAPIKey,
		omdbDebounce:        debouncedOMDBValidate,
	}

	initOpts := append([]Option{WithTheme(theme.Default())}, opts...)
	for _, opt := range initOpts {
		opt(m)
	}

	m.ensureTheme()
	m.icons = m.theme.IconSet()

	return m, nil
}

// NewWithRegistry creates a new configuration UI model with a template registry
func NewWithRegistry(templateReg *config.TemplateRegistry, opts ...Option) (*Model, error) {
	m, err := New(opts...)
	if err != nil {
		return nil, err
	}
	m.templateRegistry = templateReg
	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return m.tickCmd()
}

// tickCmd returns a command that sends a tick message for auto-scrolling
func (m *Model) tickCmd() tea.Cmd {
	return components.Tick(3*time.Second, func(time.Time) tea.Msg {
		return scrollTickMsg{}
	})
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scrollTickMsg:
		if m.autoScroll {
			// Only auto-scroll if content doesn't fit in viewport
			// Add some buffer to prevent unnecessary scrolling when content barely fits
			if m.variablesView.TotalLineCount() > m.variablesView.Height+1 {
				// Auto-scroll slowly
				if m.variablesView.AtBottom() {
					// Reset to top when we reach the bottom
					m.variablesView.GotoTop()
				} else {
					// Scroll down by 1 config item
					m.variablesView.ScrollDown(4)
				}
			}
		}
		return m, m.tickCmd()

	case tmdbValidationMsg:
		// Only update if this validation is for the current API key
		if msg.apiKey == m.tmdbAPIKey {
			if msg.valid {
				m.tmdbValidation = "valid"
			} else {
				m.tmdbValidation = "invalid"
			}
			m.tmdbValidatedKey = msg.apiKey
		}
		return m, nil

	case tmdbValidateCmd:
		// Only validate if the API key hasn't changed since the command was issued
		if msg.apiKey == m.tmdbAPIKey && msg.apiKey != "" && msg.apiKey != m.tmdbValidatedKey {
			m.tmdbValidation = "validating"
			return m, m.tmdbValidate(msg.apiKey)
		}
		return m, nil

	case omdbValidationMsg:
		if msg.apiKey == m.omdbAPIKey {
			if msg.valid {
				m.omdbValidation = "valid"
			} else {
				m.omdbValidation = "invalid"
			}
			m.omdbValidatedKey = msg.apiKey
		}
		return m, nil

	case omdbValidateCmd:
		if msg.apiKey == m.omdbAPIKey && msg.apiKey != "" && msg.apiKey != m.omdbValidatedKey {
			m.omdbValidation = "validating"
			return m, m.omdbValidate(msg.apiKey)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport dimensions
		panelHeight := m.height - 10
		if panelHeight < 0 {
			panelHeight = 0
		}
		leftWidth := m.width / 3
		if m.activeSection != SectionProviders {
			viewportWidth := leftWidth - 4    // Account for borders and padding
			viewportHeight := panelHeight - 4 // Account for borders and title
			if viewportWidth < 0 {
				viewportWidth = 0
			}
			if viewportHeight < 0 {
				viewportHeight = 0
			}
			m.variablesView.Width = viewportWidth
			m.variablesView.Height = viewportHeight
		}
		// Update content after resize
		m.updateVariablesContent()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.activeSection == SectionLogging {
				// Within logging section, up arrow switches between enable/retention
				m.loggingSubfocus = (m.loggingSubfocus + 1) % 2
				return m, nil
			}
			if m.activeSection == SectionProviders {
				switch m.providerColumnFocus {
				case providerColumnShared:
					// Single field, nothing to move between
				case providerColumnTMDB:
					if m.tmdbEnabled && m.tmdbSubfocus > 0 {
						m.tmdbSubfocus--
					}
				case providerColumnOMDB:
					if m.omdbEnabled && m.omdbSubfocus > 0 {
						m.omdbSubfocus--
					}
				}
				return m, nil
			}
			// Manual scroll up in variables view - disable auto-scroll
			if m.autoScroll {
				m.autoScroll = false
			}
			m.variablesView.ScrollUp(1)
			return m, nil

		case tea.KeyDown:
			if m.activeSection == SectionLogging {
				// Within logging section, down arrow switches between enable/retention
				m.loggingSubfocus = (m.loggingSubfocus + 1) % 2
				return m, nil
			}
			if m.activeSection == SectionProviders {
				switch m.providerColumnFocus {
				case providerColumnShared:
					// Single field, nothing to move between
				case providerColumnTMDB:
					if m.tmdbEnabled && m.tmdbSubfocus < 2 {
						m.tmdbSubfocus++
					}
				case providerColumnOMDB:
					if m.omdbEnabled && m.omdbSubfocus < 1 {
						m.omdbSubfocus++
					}
				}
				return m, nil
			}
			// Manual scroll down in variables view - disable auto-scroll
			if m.autoScroll {
				m.autoScroll = false
			}
			m.variablesView.ScrollDown(1)
			return m, nil

		case tea.KeyPgUp:
			// Page up in variables view - disable auto-scroll
			if m.autoScroll {
				m.autoScroll = false
			}
			m.variablesView.HalfPageUp()
			return m, nil

		case tea.KeyPgDown:
			// Page down in variables view - disable auto-scroll
			if m.autoScroll {
				m.autoScroll = false
			}
			m.variablesView.HalfPageDown()
			return m, nil

		case tea.KeyTab:
			cmd := m.nextSection()
			m.updateVariablesContent() // Update content when section changes
			return m, cmd

		case tea.KeyShiftTab:
			cmd := m.prevSection()
			m.updateVariablesContent() // Update content when section changes
			return m, cmd

		case tea.KeyBackspace:
			if m.activeSection == SectionLogging && m.loggingSubfocus == 1 && m.loggingEnabled {
				// Backspace in logging retention field when logging is enabled
				m.deleteLoggingChar()
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == providerColumnTMDB && m.tmdbSubfocus == 1 {
				// Backspace in TMDB API key field
				m.deleteTMDBChar()
				// Trigger debounced validation after deletion
				return m, m.tmdbDebounce(m.tmdbAPIKey)
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == providerColumnTMDB && m.tmdbSubfocus == 2 {
				// Backspace in TMDB language field
				m.deleteTMDBChar()
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == providerColumnOMDB && m.omdbSubfocus == 1 {
				m.deleteOMDBChar()
				return m, m.omdbDebounce(m.omdbAPIKey)
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == providerColumnShared {
				m.deleteWorkerCountChar()
			} else if m.activeSection != SectionLogging && m.activeSection != SectionProviders {
				m.deleteChar()
			}
			return m, nil

		case tea.KeyLeft:
			if m.activeSection == SectionProviders {
				if m.providerColumnFocus > providerColumnShared {
					m.providerColumnFocus--
				} else {
					m.providerColumnFocus = providerColumnTMDB
				}
				return m, nil
			}
			if m.cursorPos[m.activeSection] > 0 {
				m.cursorPos[m.activeSection]--
			}
			return m, nil

		case tea.KeyRight:
			if m.activeSection == SectionProviders {
				if m.providerColumnFocus < providerColumnTMDB {
					m.providerColumnFocus++
				} else {
					m.providerColumnFocus = providerColumnShared
				}
				return m, nil
			}
			if m.cursorPos[m.activeSection] < len(m.inputs[m.activeSection]) {
				m.cursorPos[m.activeSection]++
			}
			return m, nil

		case tea.KeyHome:
			m.cursorPos[m.activeSection] = 0
			return m, nil

		case tea.KeyEnd:
			m.cursorPos[m.activeSection] = len(m.inputs[m.activeSection])
			return m, nil

		case tea.KeyCtrlS:
			m.save()
			return m, nil

		case tea.KeyCtrlR:
			m.reset()
			return m, nil

		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyDelete:
			if strings.Contains(msg.String(), "ctrl+delete") {
				// Ctrl+Delete: clear field
				m.inputs[m.activeSection] = ""
				m.cursorPos[m.activeSection] = 0
			}
			return m, nil

		case tea.KeyEnter:
			if m.activeSection == SectionLogging && m.loggingSubfocus == 0 {
				// Enter toggles logging enabled in logging section
				m.loggingEnabled = !m.loggingEnabled
			}
			if m.activeSection == SectionProviders {
				var cmd tea.Cmd
				switch m.providerColumnFocus {
				case providerColumnTMDB:
					if m.tmdbSubfocus == 0 {
						m.tmdbEnabled = !m.tmdbEnabled
						if !m.tmdbEnabled {
							m.tmdbSubfocus = 0
							m.tmdbValidation = ""
							m.tmdbValidatedKey = ""
						}
						m.updateVariablesContent()
					}
				case providerColumnOMDB:
					if m.omdbSubfocus == 0 {
						m.omdbEnabled = !m.omdbEnabled
						if !m.omdbEnabled {
							m.omdbSubfocus = 0
							m.omdbValidation = ""
							m.omdbValidatedKey = ""
						} else {
							m.omdbValidatedKey = ""
							if len(m.omdbAPIKey) > 0 {
								m.omdbValidation = "validating"
								cmd = m.omdbDebounce(m.omdbAPIKey)
							} else {
								m.omdbValidation = ""
							}
						}
						m.updateVariablesContent()
					}
				case providerColumnFFProbe:
					m.ffprobeEnabled = !m.ffprobeEnabled
					m.updateVariablesContent()
				}
				return m, cmd
			}
			return m, nil

		case tea.KeySpace:
			if msg.Alt {
				// Alt+Space toggles auto-scroll
				m.autoScroll = !m.autoScroll
				return m, nil
			}
			if m.activeSection == SectionLogging && m.loggingSubfocus == 0 {
				// Space toggles logging enabled in logging section
				m.loggingEnabled = !m.loggingEnabled
				return m, nil
			}
			if m.activeSection == SectionProviders {
				var cmd tea.Cmd
				switch m.providerColumnFocus {
				case providerColumnTMDB:
					if m.tmdbSubfocus == 0 {
						m.tmdbEnabled = !m.tmdbEnabled
						if !m.tmdbEnabled {
							m.tmdbSubfocus = 0
							m.tmdbValidation = ""
							m.tmdbValidatedKey = ""
						}
						m.updateVariablesContent()
						return m, nil
					}
				case providerColumnOMDB:
					if m.omdbSubfocus == 0 {
						m.omdbEnabled = !m.omdbEnabled
						if !m.omdbEnabled {
							m.omdbSubfocus = 0
							m.omdbValidation = ""
							m.omdbValidatedKey = ""
						} else {
							m.omdbValidatedKey = ""
							if len(m.omdbAPIKey) > 0 {
								m.omdbValidation = "validating"
								cmd = m.omdbDebounce(m.omdbAPIKey)
							} else {
								m.omdbValidation = ""
							}
						}
						m.updateVariablesContent()
						return m, cmd
					}
				case providerColumnFFProbe:
					m.ffprobeEnabled = !m.ffprobeEnabled
					m.updateVariablesContent()
					return m, nil
				}
			}
			// Regular space for text input
			if m.activeSection == SectionLogging && m.loggingSubfocus == 1 {
				// No spaces in retention field
				return m, nil
			}
			if m.activeSection == SectionProviders {
				if (m.providerColumnFocus == providerColumnTMDB && m.tmdbSubfocus == 1) ||
					(m.providerColumnFocus == providerColumnOMDB && m.omdbSubfocus == 1) ||
					m.providerColumnFocus == providerColumnShared {
					// No spaces in API key or numeric fields
					return m, nil
				}
			}
			m.insertText(" ")
			return m, nil

		case tea.KeyRunes:
			if m.activeSection == SectionLogging && m.loggingSubfocus == 1 && m.loggingEnabled {
				// Only allow digits in retention field when logging is enabled
				runes := string(msg.Runes)
				for _, r := range runes {
					if r >= '0' && r <= '9' {
						m.insertLoggingText(string(r))
					}
				}
			} else if m.activeSection == SectionProviders {
				runes := string(msg.Runes)
				switch m.providerColumnFocus {
				case providerColumnTMDB:
					switch m.tmdbSubfocus {
					case 1:
						m.insertTMDBAPIKey(runes)
						return m, m.tmdbDebounce(m.tmdbAPIKey)
					case 2:
						for _, r := range runes {
							if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
								m.insertTMDBLanguage(string(r))
							}
						}
					}
				case providerColumnOMDB:
					if m.omdbSubfocus == 1 {
						m.insertOMDBAPIKey(runes)
						return m, m.omdbDebounce(m.omdbAPIKey)
					}
				case providerColumnShared:
					for _, r := range runes {
						if r >= '0' && r <= '9' {
							m.insertWorkerCount(string(r))
						}
					}
				}
			} else if m.activeSection != SectionLogging && m.activeSection != SectionProviders {
				m.insertText(string(msg.Runes))
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Handle very small terminal sizes
	if m.width < 30 || m.height < 10 {
		return "Terminal too small. Please resize to at least 30x10."
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7c3aed")).
		Padding(1, 0).
		Align(lipgloss.Center)

	tabStyle := lipgloss.NewStyle().
		Padding(0, 2)

	activeTabStyle := tabStyle.
		Bold(true).
		Foreground(lipgloss.Color("#7c3aed"))

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4a5568"))

	// Title
	icons := m.iconSet()
	title := titleStyle.Render(icons["title"] + " Title-Tidy Format Configuration")

	// Tabs
	tabs := []string{}
	for i, label := range []string{"Show Folder", "Season Folder", "Episode", "Movie", "Logging", "Providers"} {
		style := tabStyle
		if Section(i) == m.activeSection {
			label = "[ " + label + " ]"
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(label))
	}
	tabLine := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	tabLine = lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).Render(tabLine)

	// Calculate panel dimensions
	panelHeight := m.height - 10 // Account for title, tabs, status
	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 4 // Account for borders

	// Ensure minimum dimensions
	if panelHeight < 1 {
		panelHeight = 1
	}
	if leftWidth < 1 {
		leftWidth = 1
	}
	if rightWidth < 1 {
		rightWidth = 1
	}

	var leftContent string
	if m.activeSection == SectionProviders {
		sidebarWidth := leftWidth - 4
		if sidebarWidth < 0 {
			sidebarWidth = 0
		}
		leftContent = m.renderProvidersSidebar(sidebarWidth)
	} else {
		viewportWidth := leftWidth - 4
		if viewportWidth < 0 {
			viewportWidth = 0
		}
		viewportHeight := panelHeight - 4
		if viewportHeight < 0 {
			viewportHeight = 0
		}
		m.variablesView.Width = viewportWidth
		m.variablesView.Height = viewportHeight
		leftContent = m.renderVariablesViewport()
	}
	leftPanel := borderStyle.Width(leftWidth).Height(panelHeight).Render(leftContent)

	// Right panel: Input and preview or available components
	rightContent := m.buildRightPanel(rightWidth-2, panelHeight-2) // -2 for borders
	rightPanel := borderStyle.Width(rightWidth).Height(panelHeight).Render(rightContent)

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar
	status := m.buildStatusBar()

	// Combine everything
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		tabLine,
		panels,
		status,
	)
}

func (m *Model) updateVariablesContent() {
	variableStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60a5fa")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af"))

	exampleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6b7280")).
		Italic(true)

	lines := []string{}

	variables := m.getVariablesForSection()
	for _, v := range variables {
		lines = append(lines, variableStyle.Render(v.name))
		lines = append(lines, descStyle.Render("  "+v.description))
		if v.example != "" {
			lines = append(lines, exampleStyle.Render("  Example: "+v.example))
		}
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")
	m.variablesView.SetContent(content)
}

func (m *Model) renderVariablesViewport() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		MarginBottom(1)

	scrollIndicator := ""
	if m.variablesView.TotalLineCount() > m.variablesView.Height+1 {
		if m.autoScroll {
			scrollIndicator = " [Auto-scrolling]"
		} else {
			scrollIndicator = " [Manual]"
		}
	}

	title := titleStyle.Render("Available Components" + scrollIndicator)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.variablesView.View(),
	)
}

func (m *Model) renderProvidersSidebar(width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		MarginBottom(1)

	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af"))

	if width > 0 {
		bodyStyle = bodyStyle.Width(width)
	}

	lines := []string{
		"Shared settings apply to every metadata provider.",
		"Use Left/Right to switch columns, Up/Down to move within a column.",
		"Press Space to toggle options; type numbers to edit worker count.",
	}

	description := bodyStyle.Render(strings.Join(lines, "\n"))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Provider Controls"),
		description,
	)
}

func (m *Model) buildRightPanel(width, height int) string {
	inputHeight := 3
	previewHeight := height - inputHeight - 1

	// Ensure minimum dimensions
	if previewHeight < 0 {
		previewHeight = 0
	}

	// Input section
	input := m.buildInputField(width)

	parts := []string{input}

	if m.activeSection != SectionProviders {
		preview := m.buildPreview(width, previewHeight)
		separatorWidth := width
		if separatorWidth < 0 {
			separatorWidth = 0
		}
		separator := lipgloss.NewStyle().Foreground(lipgloss.Color("#4a5568")).Render(strings.Repeat("─", separatorWidth))
		parts = append(parts, separator, preview)
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) buildInputField(width int) string {
	if m.activeSection == SectionLogging {
		return m.buildLoggingInputField(width)
	}
	if m.activeSection == SectionProviders {
		return m.buildProvidersInputField(width)
	}

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	label := labelStyle.Render("Format Template:")

	// Build input with cursor
	text := m.inputs[m.activeSection]
	pos := m.cursorPos[m.activeSection]

	var display string
	if pos >= len(text) {
		// Cursor at end
		display = inputStyle.Render(text) + cursorStyle.Render(" ")
	} else {
		// Cursor in middle
		before := text[:pos]
		at := string(text[pos])
		after := ""
		if pos+1 < len(text) {
			after = text[pos+1:]
		}
		display = inputStyle.Render(before) + cursorStyle.Render(at) + inputStyle.Render(after)
	}

	// Truncate if too wide
	if lipgloss.Width(display) > width {
		// Simple truncation - in production would be smarter about cursor visibility
		display = display[:width-3] + "..."
	}

	return lipgloss.JoinVertical(lipgloss.Left, label, display)
}

func (m *Model) buildLoggingInputField(width int) string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	enabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	focusedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	retentionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	label := labelStyle.Render("Logging Configuration:")

	// Build logging enabled toggle
	var enabledToggle string
	enabledText := "Enabled"
	if !m.loggingEnabled {
		enabledText = "Disabled"
	}

	icons := m.iconSet()
	if m.loggingSubfocus == 0 {
		if m.loggingEnabled {
			enabledToggle = focusedStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = focusedStyle.Render("[ ] " + enabledText)
		}
	} else {
		if m.loggingEnabled {
			enabledToggle = enabledStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = disabledStyle.Render("[ ] " + enabledText)
		}
	}

	// Build retention input
	var retentionField string
	retentionLabel := "Retention Days: "

	if m.loggingSubfocus == 1 {
		// Show cursor in retention field
		retentionField = retentionLabel + focusedStyle.Render(m.loggingRetention+" ")
	} else {
		retentionField = retentionLabel + retentionStyle.Render(m.loggingRetention)
	}

	// Disable retention field visually if logging is disabled
	if !m.loggingEnabled {
		retentionField = disabledStyle.Render(retentionLabel + m.loggingRetention + " (disabled)")
	}

	lines := []string{
		label,
		enabledToggle,
		retentionField,
	}

	return strings.Join(lines, "\n")
}

func (m *Model) buildProvidersInputField(width int) string {
	if width < 0 {
		width = 0
	}
	columnGap := 2
	minColumnWidth := 20
	columnCount := 4
	totalGap := columnGap * (columnCount - 1)
	available := width - totalGap
	inlineLayout := available >= minColumnWidth*columnCount
	columnWidth := width
	if inlineLayout {
		columnWidth = available / columnCount
	}

	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	sharedColumn := columnStyle.Render(m.buildSharedProviderColumn(columnWidth))
	ffprobeColumn := columnStyle.Render(m.buildFFProbeColumn(columnWidth))
	omdbColumn := columnStyle.Render(m.buildOMDBColumn(columnWidth))
	tmdbColumn := columnStyle.Render(m.buildTMDBColumn(columnWidth))

	if inlineLayout {
		gap := lipgloss.NewStyle().Width(columnGap).Render(" ")
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			sharedColumn,
			gap,
			ffprobeColumn,
			gap,
			omdbColumn,
			gap,
			tmdbColumn,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		sharedColumn,
		"",
		ffprobeColumn,
		"",
		omdbColumn,
		"",
		tmdbColumn,
	)
}

func (m *Model) buildPreview(width, maxHeight int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")).
		Italic(true)

	lines := []string{
		titleStyle.Render("Live Previews:"),
		"",
	}

	// Generate previews
	previews := m.generatePreviews()
	for _, p := range previews {
		icon := p.icon
		label := labelStyle.Render(p.label + ":")
		preview := previewStyle.Render(p.preview)
		line := fmt.Sprintf("%s %s %s", icon, label, preview)

		// Truncate if too wide
		if lipgloss.Width(line) > width {
			line = line[:width-3] + "..."
		}
		lines = append(lines, line)
	}

	// Add hint about metadata providers if both are disabled for template sections
	showProviderHint := !m.tmdbEnabled && !m.omdbEnabled && (m.activeSection == SectionShowFolder ||
		m.activeSection == SectionSeasonFolder ||
		m.activeSection == SectionEpisode ||
		m.activeSection == SectionMovie)
	if showProviderHint {
		// Add spacing before hint
		lines = append(lines, "")
		lines = append(lines, hintStyle.Render("💡 Enable TMDB or OMDb for more template options"))
		lines = append(lines, hintStyle.Render("   (ratings, genres, taglines, IMDb data, etc.)"))
	}

	// Truncate if too tall (but keep the hint if present)
	if maxHeight > 0 && len(lines) > maxHeight {
		// Check if we have a hint (last 3 lines)
		hasHint := showProviderHint

		if hasHint && len(lines) >= 3 && maxHeight > 3 {
			// Keep the hint at the bottom
			hintLines := lines[len(lines)-3:]
			lines = lines[:maxHeight-3]
			lines = append(lines, hintLines...)
		} else {
			lines = lines[:maxHeight]
		}
	}

	return strings.Join(lines, "\n")
}

func (m *Model) buildStatusBar() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6b7280"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60a5fa")).
		Bold(true)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444")).
		Bold(true)

	quitHelp := fmt.Sprintf("%s/%s: Quit",
		keyStyle.Render("Esc"),
		keyStyle.Render("Ctrl+C"),
	)
	help := []string{
		keyStyle.Render("Tab") + ": Switch",
		keyStyle.Render("Type directly"),
		keyStyle.Render("Ctrl+S") + ": Save",
		keyStyle.Render("Ctrl+R") + ": Reset",
		quitHelp,
	}

	// Add scroll help if content is scrollable
	if m.variablesView.TotalLineCount() > m.variablesView.Height+1 {
		help = append(help, keyStyle.Render("↑↓")+": Scroll", keyStyle.Render("Alt+Space")+": Toggle auto")
	}

	helpLine := helpStyle.Render(strings.Join(help, " │ "))

	// Add save status if present
	if m.saveStatus != "" {
		if m.err != nil {
			helpLine += " │ " + errorStyle.Render(m.saveStatus)
		} else {
			helpLine += " │ " + statusStyle.Render(m.saveStatus)
		}
	}

	return helpLine
}

type variable struct {
	name        string
	description string
	example     string
}

func (m *Model) getVariablesForSection() []variable {
	// Special handling for logging and TMDB sections
	switch m.activeSection {
	case SectionLogging:
		return []variable{
			{"Space/Enter", "Toggle logging on/off", ""},
			{"↑/↓ arrows", "Switch to fields", ""},
			{"Retention", "Auto-cleanup old logs", "Days to keep log files"},
		}
	case SectionProviders:
		return []variable{
			{"←/→ arrows", "Switch between provider columns", ""},
			{"↑/↓ arrows", "Navigate settings in the active column", ""},
			{"Space/Enter", "Toggle highlighted setting", ""},
			{"OMDb API Key", "OMDb API key (8+ characters)", "Get from omdbapi.com/apikey.aspx"},
			{"TMDB API Key", "TMDB API key (32 hex chars)", "Get from themoviedb.org (NOT Read Token)"},
			{"TMDB Language", "Preferred metadata language", "en-US, fr-FR, etc."},
			{"ffprobe", "Enable technical metadata extraction", "Adds video and audio codec options"},
		}
	}

	// Use template registry if available for template sections
	if m.templateRegistry != nil {
		var mediaType provider.MediaType
		switch m.activeSection {
		case SectionShowFolder:
			mediaType = provider.MediaTypeShow
		case SectionSeasonFolder:
			mediaType = provider.MediaTypeSeason
		case SectionEpisode:
			mediaType = provider.MediaTypeEpisode
		case SectionMovie:
			mediaType = provider.MediaTypeMovie
		default:
			return nil
		}

		// Get variables from registry
		vars := m.templateRegistry.GetVariablesForMediaType(mediaType)
		result := make([]variable, 0, len(vars))

		// Convert provider variables to UI variables, filtering by enabled providers
		for _, v := range vars {
			owners := m.templateRegistry.VariableProviders(v.Name)
			if len(owners) > 0 {
				include := false
				for _, owner := range owners {
					switch owner {
					case "tmdb":
						if m.tmdbEnabled {
							include = true
						}
					case "omdb":
						if m.omdbEnabled {
							include = true
						}
					case "ffprobe":
						if m.ffprobeEnabled {
							include = true
						}
					default:
						// Providers like "local" are always available
						include = true
					}
				}
				if !include {
					continue
				}
			}

			// Format the variable name with braces
			varName := "{" + v.Name + "}"
			result = append(result, variable{
				name:        varName,
				description: v.Description,
				example:     v.Example,
			})
		}

		// If we got variables from registry, ensure a stable order and return them
		if len(result) > 0 {
			sort.SliceStable(result, func(i, j int) bool {
				pi := variableProviderPriority(m.templateRegistry, result[i].name)
				pj := variableProviderPriority(m.templateRegistry, result[j].name)
				if pi != pj {
					return pi < pj
				}
				return result[i].name < result[j].name
			})
			return result
		}
	}

	// Fallback to hardcoded variables if registry unavailable or empty
	switch m.activeSection {
	case SectionShowFolder:
		return []variable{
			{"{title}", "Show title", "Breaking Bad"},
			{"{year}", "Year", "2008"},
			{"{rating}", "Rating", "8.5"},
			{"{genres}", "Genres", "Drama, Crime"},
			{"{tagline}", "Tagline", "All Hail the King"},
		}
	case SectionSeasonFolder:
		return []variable{
			{"{title}", "Show title", "Breaking Bad"},
			{"{season}", "Season number", "01"},
		}
	case SectionEpisode:
		return []variable{
			{"{title}", "Show title", "Breaking Bad"},
			{"{year}", "Year", "2008"},
			{"{season}", "Season number", "01"},
			{"{episode}", "Episode number", "05"},
			{"{episode_title}", "Episode title", "Gray Matter"},
			{"{air_date}", "Air date", "2008-02-24"},
			{"{rating}", "Rating", "8.3"},
			{"{runtime}", "Runtime in minutes", "48"},
		}
	case SectionMovie:
		return []variable{
			{"{title}", "Movie title", "The Matrix"},
			{"{year}", "Year", "1999"},
			{"{rating}", "Rating", "8.7"},
			{"{genres}", "Genres", "Action, Sci-Fi"},
			{"{runtime}", "Runtime in minutes", "136"},
			{"{tagline}", "Tagline", "Welcome to the Real World"},
		}
	}
	return nil
}

func variableProviderPriority(reg *config.TemplateRegistry, name string) int {
	if reg == nil {
		return 0
	}
	trimmed := strings.TrimPrefix(strings.TrimSuffix(name, "}"), "{")
	owners := reg.VariableProviders(trimmed)
	priority := 4
	for _, owner := range owners {
		switch owner {
		case "local":
			return 0
		case "tmdb":
			if priority > 1 {
				priority = 1
			}
		case "omdb":
			if priority > 2 {
				priority = 2
			}
		case "ffprobe":
			if priority > 3 {
				priority = 3
			}
		default:
			if priority > 4 {
				priority = 4
			}
		}
	}
	return priority
}

type preview struct {
	icon    string
	label   string
	preview string
}

func (m *Model) generatePreviews() []preview {
	if m.activeSection == SectionLogging {
		// Show logging configuration status
		enabledStatus := "Disabled"
		if m.loggingEnabled {
			enabledStatus = "Enabled"
		}

		icons := m.iconSet()
		return []preview{
			{icons["check"], "Logging", enabledStatus},
			{icons["calendar"], "Retention", m.loggingRetention + " days"},
			{icons["folder"], "Log Location", "~/.title-tidy/logs/"},
			{icons["document"], "Log Format", "JSON session files"},
		}
	}

	if m.activeSection == SectionProviders {
		// Show provider configuration status
		tmdbStatus := "Disabled"
		if m.tmdbEnabled {
			tmdbStatus = "Enabled"
		}

		tmdbAPIStatus := "Not configured"
		if len(m.tmdbAPIKey) > 0 {
			switch m.tmdbValidation {
			case "validating":
				tmdbAPIStatus = "Validating..."
			case "valid":
				tmdbAPIStatus = "Valid"
			case "invalid":
				tmdbAPIStatus = "Invalid"
			default:
				tmdbAPIStatus = "Configured"
			}
		}

		omdbStatus := "Disabled"
		if m.omdbEnabled {
			omdbStatus = "Enabled"
		}

		omdbAPIStatus := "Not configured"
		if len(m.omdbAPIKey) > 0 {
			switch m.omdbValidation {
			case "validating":
				omdbAPIStatus = "Validating..."
			case "valid":
				omdbAPIStatus = "Valid"
			case "invalid":
				omdbAPIStatus = "Invalid"
			default:
				omdbAPIStatus = "Configured"
			}
		}

		ffprobeStatus := "Disabled"
		if m.ffprobeEnabled {
			ffprobeStatus = "Enabled"
		}

		icons := m.iconSet()
		return []preview{
			{icons["chip"], "ffprobe", ffprobeStatus},
			{icons["film"], "OMDb Lookup", omdbStatus},
			{icons["key"], "OMDb API", omdbAPIStatus},
			{icons["film"], "TMDB Lookup", tmdbStatus},
			{icons["key"], "TMDB API", tmdbAPIStatus},
			{icons["globe"], "Language", m.tmdbLanguage},
		}
	}

	// Use current input values to generate previews
	cfg := &config.FormatConfig{
		ShowFolder:   m.inputs[SectionShowFolder],
		SeasonFolder: m.inputs[SectionSeasonFolder],
		Episode:      m.inputs[SectionEpisode],
		Movie:        m.inputs[SectionMovie],
	}

	// Create demo metadata for shows (Breaking Bad)
	showMetadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       "Breaking Bad",
			Year:        "2008",
			Rating:      8.5,
			Genres:      []string{"Drama", "Crime"},
			EpisodeName: "Gray Matter",
		},
		IDs: map[string]string{
			"imdb_id": "tt0903747",
		},
		Extended: map[string]interface{}{
			"tagline":     "All Hail the King",
			"networks":    "AMC",
			"audio_codec": "aac",
			"video_codec": "264",
		},
	}

	// Create demo metadata for movies (The Matrix)
	movieMetadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:  "The Matrix",
			Year:   "1999",
			Rating: 8.7,
			Genres: []string{"Action", "Sci-Fi"},
		},
		IDs: map[string]string{
			"imdb_id": "tt0133093",
		},
		Extended: map[string]interface{}{
			"tagline":     "Welcome to the Real World",
			"studios":     "Warner Bros.",
			"audio_codec": "aac",
			"video_codec": "264",
		},
	}

	icons := m.iconSet()

	// Create contexts for preview
	showCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Year:     "2008",
		Metadata: showMetadata,
	}
	seasonCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Year:     "2008",
		Season:   1,
		Metadata: showMetadata,
	}
	episodeCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Year:     "2008",
		Season:   1,
		Episode:  7,
		Metadata: showMetadata,
	}
	movieCtx := &config.FormatContext{
		MovieName: "The Matrix",
		Year:      "1999",
		Metadata:  movieMetadata,
	}

	// Use template registry if available for better variable resolution
	var showPreview, seasonPreview, episodePreview, moviePreview string

	if m.templateRegistry != nil {
		// Use template registry for dynamic variable resolution
		showPreview, _ = m.templateRegistry.ResolveTemplate(cfg.ShowFolder, showCtx, showMetadata)
		showPreview = local.CleanName(showPreview)

		seasonPreview, _ = m.templateRegistry.ResolveTemplate(cfg.SeasonFolder, seasonCtx, showMetadata)
		seasonPreview = local.CleanName(seasonPreview)

		episodePreview, _ = m.templateRegistry.ResolveTemplate(cfg.Episode, episodeCtx, showMetadata)
		episodePreview = local.CleanName(episodePreview) + ".mkv"

		moviePreview, _ = m.templateRegistry.ResolveTemplate(cfg.Movie, movieCtx, movieMetadata)
		moviePreview = local.CleanName(moviePreview)
	} else {
		// Fallback to old method
		showPreview = cfg.ApplyShowFolderTemplate(showCtx)
		seasonPreview = cfg.ApplySeasonFolderTemplate(seasonCtx)
		episodePreview = cfg.ApplyEpisodeTemplate(episodeCtx) + ".mkv"
		moviePreview = cfg.ApplyMovieTemplate(movieCtx)
	}

	return []preview{
		{icons["title"], "Show", showPreview},
		{icons["folder"], "Season", seasonPreview},
		{icons["episode"], "Episode", episodePreview},
		{icons["movie"], "Movie", moviePreview},
	}
}

func (m *Model) nextSection() tea.Cmd {
	oldSection := m.activeSection
	m.activeSection = (m.activeSection + 1) % 6
	m.loggingSubfocus = 0 // Reset subfocus when changing sections
	m.tmdbSubfocus = 0    // Reset TMDB subfocus when changing sections
	m.omdbSubfocus = 0
	m.providerColumnFocus = providerColumnShared

	if m.activeSection == SectionProviders && oldSection != SectionProviders {
		return tea.Batch(
			m.validateTMDBOnSectionSwitch(),
			m.validateOMDBOnSectionSwitch(),
		)
	}
	return nil
}

func (m *Model) prevSection() tea.Cmd {
	oldSection := m.activeSection
	m.activeSection = (m.activeSection + 5) % 6 // +5 is same as -1 mod 6
	m.loggingSubfocus = 0                       // Reset subfocus when changing sections
	m.tmdbSubfocus = 0
	m.omdbSubfocus = 0
	m.providerColumnFocus = providerColumnShared

	if m.activeSection == SectionProviders && oldSection != SectionProviders {
		return tea.Batch(
			m.validateTMDBOnSectionSwitch(),
			m.validateOMDBOnSectionSwitch(),
		)
	}
	return nil
}

func (m *Model) insertText(text string) {
	current := m.inputs[m.activeSection]
	pos := m.cursorPos[m.activeSection]

	// Insert text at cursor position
	newText := current[:pos] + text + current[pos:]
	m.inputs[m.activeSection] = newText
	m.cursorPos[m.activeSection] = pos + len(text)
}

func (m *Model) deleteChar() {
	if m.cursorPos[m.activeSection] == 0 {
		return
	}

	current := m.inputs[m.activeSection]
	pos := m.cursorPos[m.activeSection]

	// Delete character before cursor
	newText := current[:pos-1] + current[pos:]
	m.inputs[m.activeSection] = newText
	m.cursorPos[m.activeSection] = pos - 1
}

func (m *Model) insertLoggingText(text string) {
	current := m.loggingRetention
	m.loggingRetention = current + text
}

func (m *Model) deleteLoggingChar() {
	if len(m.loggingRetention) == 0 {
		return
	}
	m.loggingRetention = m.loggingRetention[:len(m.loggingRetention)-1]
}

func maskAPIKeyVisible(key string, prefix, suffix int) string {
	key = strings.TrimSpace(key)
	if len(key) == 0 {
		return ""
	}
	if prefix < 0 {
		prefix = 0
	}
	if suffix < 0 {
		suffix = 0
	}
	if prefix+suffix >= len(key) {
		return strings.Repeat("*", len(key))
	}
	maskedLen := len(key) - prefix - suffix
	return key[:prefix] + strings.Repeat("*", maskedLen) + key[len(key)-suffix:]
}

func maskAPIKey(key string) string {
	return maskAPIKeyVisible(key, 4, 4)
}

func (m *Model) buildTMDBColumn(width int) string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	enabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	focusedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	label := labelStyle.Render("TMDB Configuration:")

	// Build enabled toggle
	var enabledToggle string
	enabledText := "Enabled"
	if !m.tmdbEnabled {
		enabledText = "Disabled"
	}

	icons := m.iconSet()
	isActive := m.providerColumnFocus == providerColumnTMDB

	if isActive && m.tmdbSubfocus == 0 {
		if m.tmdbEnabled {
			enabledToggle = focusedStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = focusedStyle.Render("[ ] " + enabledText)
		}
	} else {
		if m.tmdbEnabled {
			enabledToggle = enabledStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = disabledStyle.Render("[ ] " + enabledText)
		}
	}

	// Build API key field (masked)
	var apiKeyField string
	apiKeyLabel := "API Key: "
	maskedKey := maskAPIKey(m.tmdbAPIKey)

	if !m.tmdbEnabled {
		// When TMDB is disabled, render with disabled style but no suffix
		apiKeyField = disabledStyle.Render(apiKeyLabel + maskedKey)
	} else if isActive && m.tmdbSubfocus == 1 {
		// Only show focus when TMDB is enabled - show cursor after the text
		apiKeyField = apiKeyLabel + normalStyle.Render(maskedKey) + focusedStyle.Render(" ")
	} else {
		apiKeyField = apiKeyLabel + normalStyle.Render(maskedKey)
	}

	// Build language field
	var languageField string
	languageLabel := "Language: "

	if !m.tmdbEnabled {
		// When TMDB is disabled, render with disabled style but no suffix
		languageField = disabledStyle.Render(languageLabel + m.tmdbLanguage)
	} else if isActive && m.tmdbSubfocus == 2 {
		// Only show focus when TMDB is enabled - show cursor after the text
		languageField = languageLabel + normalStyle.Render(m.tmdbLanguage) + focusedStyle.Render(" ")
	} else {
		languageField = languageLabel + normalStyle.Render(m.tmdbLanguage)
	}

	// Build validation status field
	statusLabel := "Status: "
	statusValue := "Not configured"

	switch {
	case !m.tmdbEnabled:
		statusValue = "Disabled"
	case m.tmdbValidation == "validating":
		statusValue = "Validating..."
	case m.tmdbValidation == "valid":
		statusValue = "Valid"
	case m.tmdbValidation == "invalid":
		statusValue = "Invalid"
	case len(m.tmdbAPIKey) > 0 && m.tmdbValidatedKey != "" && m.tmdbValidatedKey == m.tmdbAPIKey:
		statusValue = "Valid"
	case len(m.tmdbAPIKey) > 0:
		statusValue = "Configured"
	}

	statusStyle := normalStyle
	if statusValue == "Valid" {
		statusStyle = enabledStyle
	} else if statusValue == "Invalid" {
		statusStyle = disabledStyle
	} else if statusValue == "Disabled" {
		statusStyle = disabledStyle
	}

	statusField := statusStyle.Render(statusLabel + statusValue)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")).
		MarginTop(1)

	description := tmdb.New().Description()
	info := descStyle.Render(description)

	lines := []string{
		label,
		enabledToggle,
		apiKeyField,
		languageField,
		"",
		statusField,
		info,
	}

	return strings.Join(lines, "\n")
}

func (m *Model) buildOMDBColumn(width int) string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	enabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	focusedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	label := labelStyle.Render("OMDB Configuration:")
	isActive := m.providerColumnFocus == providerColumnOMDB
	icons := m.iconSet()

	enabledText := "Enabled"
	if !m.omdbEnabled {
		enabledText = "Disabled"
	}

	var enabledToggle string
	if isActive && m.omdbSubfocus == 0 {
		if m.omdbEnabled {
			enabledToggle = focusedStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = focusedStyle.Render("[ ] " + enabledText)
		}
	} else {
		if m.omdbEnabled {
			enabledToggle = enabledStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = disabledStyle.Render("[ ] " + enabledText)
		}
	}

	// Build API key field (masked)
	var apiKeyField string
	apiKeyLabel := "API Key: "
	maskedKey := maskAPIKeyVisible(m.omdbAPIKey, 2, 2)

	if !m.omdbEnabled {
		apiKeyField = disabledStyle.Render(apiKeyLabel + maskedKey)
	} else if isActive && m.omdbSubfocus == 1 {
		apiKeyField = apiKeyLabel + normalStyle.Render(maskedKey) + focusedStyle.Render(" ")
	} else {
		apiKeyField = apiKeyLabel + normalStyle.Render(maskedKey)
	}

	// Validation status line
	statusLabel := "Status: "
	statusValue := "Not validated"
	if !m.omdbEnabled {
		statusValue = "Disabled"
	} else {
		switch m.omdbValidation {
		case "validating":
			statusValue = "Validating..."
		case "valid":
			statusValue = "Valid"
		case "invalid":
			statusValue = "Invalid"
		default:
			if m.omdbValidatedKey != "" && m.omdbValidatedKey == m.omdbAPIKey {
				statusValue = "Valid"
			}
		}
	}

	var statusStyle lipgloss.Style
	statusStyle = normalStyle
	if statusValue == "Valid" {
		statusStyle = enabledStyle
	} else if statusValue == "Invalid" || statusValue == "Disabled" {
		statusStyle = disabledStyle
	}
	statusField := statusStyle.Render(statusLabel + statusValue)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")).
		MarginTop(1)

	description := omdb.New().Description()
	info := descStyle.Render(description)

	lines := []string{
		label,
		enabledToggle,
		apiKeyField,
		"",
		statusField,
		info,
	}

	return strings.Join(lines, "\n")
}

func (m *Model) buildSharedProviderColumn(width int) string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	focusedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")).
		MarginTop(1)

	label := labelStyle.Render("Provider Settings:")
	workerLabel := "Worker Count: "
	workerValue := m.workerCount

	var workerField string
	if m.providerColumnFocus == providerColumnShared {
		workerField = workerLabel + focusedStyle.Render(workerValue+" ")
	} else {
		workerField = workerLabel + normalStyle.Render(workerValue)
	}

	info := descStyle.Render("Controls how many provider requests run concurrently.")

	lines := []string{
		label,
		workerField,
		info,
	}

	return strings.Join(lines, "\n")
}

func (m *Model) buildFFProbeColumn(width int) string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	enabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	focusedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#ffffff"))

	label := labelStyle.Render("ffprobe Configuration:")

	icons := m.iconSet()
	enabledText := "Enabled"
	if !m.ffprobeEnabled {
		enabledText = "Disabled"
	}

	var enabledToggle string
	if m.providerColumnFocus == providerColumnFFProbe {
		if m.ffprobeEnabled {
			enabledToggle = focusedStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = focusedStyle.Render("[ ] " + enabledText)
		}
	} else {
		if m.ffprobeEnabled {
			enabledToggle = enabledStyle.Render("[" + icons["check"] + "] " + enabledText)
		} else {
			enabledToggle = disabledStyle.Render("[ ] " + enabledText)
		}
	}

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")).
		MarginTop(1)

	description := ffprobe.New().Description()
	info := descStyle.Render(description)
	if description == "" {
		info = descStyle.Render("Adds video and audio codec options.")
	}

	lines := []string{
		label,
		enabledToggle,
		info,
	}

	return strings.Join(lines, "\n")
}

func (m *Model) insertTMDBAPIKey(text string) {
	// Accept whatever the user enters, validation will occur once typing stops
	m.tmdbAPIKey += text
	m.tmdbValidation = ""
	m.tmdbValidatedKey = ""
}

func (m *Model) insertTMDBLanguage(text string) {
	if len(m.tmdbLanguage) < 5 { // Language codes are max 5 chars (e.g., "en-US")
		m.tmdbLanguage += text
	}
}

func (m *Model) insertWorkerCount(text string) {
	if len(m.workerCount) < 3 { // Max 3 digits for worker count (up to 999)
		m.workerCount += text
	}
}

func (m *Model) deleteTMDBChar() {
	if m.tmdbSubfocus == 1 && len(m.tmdbAPIKey) > 0 {
		m.tmdbAPIKey = m.tmdbAPIKey[:len(m.tmdbAPIKey)-1]
		// Reset validation state when key changes
		m.tmdbValidation = ""
		m.tmdbValidatedKey = ""
	} else if m.tmdbSubfocus == 2 && len(m.tmdbLanguage) > 0 {
		m.tmdbLanguage = m.tmdbLanguage[:len(m.tmdbLanguage)-1]
	}
}

func (m *Model) deleteWorkerCountChar() {
	if len(m.workerCount) > 0 {
		m.workerCount = m.workerCount[:len(m.workerCount)-1]
	}
}

func (m *Model) insertOMDBAPIKey(text string) {
	m.omdbAPIKey += text
	m.omdbValidation = ""
	m.omdbValidatedKey = ""
}

func (m *Model) deleteOMDBChar() {
	if m.omdbSubfocus == 1 && len(m.omdbAPIKey) > 0 {
		m.omdbAPIKey = m.omdbAPIKey[:len(m.omdbAPIKey)-1]
		m.omdbValidation = ""
		m.omdbValidatedKey = ""
	}
}

// stripNullChars removes null characters from a string
// This is needed on Windows where terminal input can include \x00 bytes
func stripNullChars(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

func (m *Model) save() {
	// Update config from inputs, stripping null characters that Windows terminals may add
	m.config.ShowFolder = stripNullChars(m.inputs[SectionShowFolder])
	m.config.SeasonFolder = stripNullChars(m.inputs[SectionSeasonFolder])
	m.config.Episode = stripNullChars(m.inputs[SectionEpisode])
	m.config.Movie = stripNullChars(m.inputs[SectionMovie])

	// Update logging config
	m.config.EnableLogging = m.loggingEnabled
	sanitizedRetention := stripNullChars(m.loggingRetention)
	if retentionDays, err := strconv.Atoi(sanitizedRetention); err == nil {
		if retentionDays > 0 {
			m.config.LogRetentionDays = retentionDays
		}
	}

	// Update TMDB/OMDb config, stripping null characters from string fields
	m.config.TMDBAPIKey = stripNullChars(m.tmdbAPIKey)
	m.config.EnableTMDBLookup = m.tmdbEnabled
	m.config.TMDBLanguage = stripNullChars(m.tmdbLanguage)
	m.config.OMDBAPIKey = stripNullChars(m.omdbAPIKey)
	m.config.EnableOMDBLookup = m.omdbEnabled
	m.config.EnableFFProbe = m.ffprobeEnabled

	// Update worker count
	sanitizedWorkerCount := stripNullChars(m.workerCount)
	if sanitizedWorkerCount == "" {
		m.config.TMDBWorkerCount = 10 // Use default if empty
	} else if workerCount, err := strconv.Atoi(sanitizedWorkerCount); err == nil {
		if workerCount > 0 {
			m.config.TMDBWorkerCount = workerCount
		} else {
			m.config.TMDBWorkerCount = 10 // Default if invalid
		}
	} else {
		m.config.TMDBWorkerCount = 10 // Default if parsing fails
	}

	// Skip TMDB validation - trust what the user enters

	// Save to disk
	if err := m.config.Save(); err != nil {
		m.err = err
		m.saveStatus = "Failed to save: " + err.Error()
	} else {
		m.err = nil
		m.saveStatus = "Configuration saved!"
		// Update original config after successful save
		m.originalConfig.ShowFolder = m.config.ShowFolder
		m.originalConfig.SeasonFolder = m.config.SeasonFolder
		m.originalConfig.Episode = m.config.Episode
		m.originalConfig.Movie = m.config.Movie
		m.originalConfig.EnableLogging = m.config.EnableLogging
		m.originalConfig.LogRetentionDays = m.config.LogRetentionDays
		m.originalConfig.TMDBAPIKey = m.config.TMDBAPIKey
		m.originalConfig.EnableTMDBLookup = m.config.EnableTMDBLookup
		m.originalConfig.TMDBLanguage = m.config.TMDBLanguage
		m.originalConfig.TMDBWorkerCount = m.config.TMDBWorkerCount
		m.originalConfig.OMDBAPIKey = m.config.OMDBAPIKey
		m.originalConfig.EnableOMDBLookup = m.config.EnableOMDBLookup
		m.originalConfig.EnableFFProbe = m.config.EnableFFProbe
	}
}

func (m *Model) reset() {
	// Reset to last saved values
	m.inputs[SectionShowFolder] = m.originalConfig.ShowFolder
	m.inputs[SectionSeasonFolder] = m.originalConfig.SeasonFolder
	m.inputs[SectionEpisode] = m.originalConfig.Episode
	m.inputs[SectionMovie] = m.originalConfig.Movie

	// Reset cursor positions
	m.cursorPos[SectionShowFolder] = len(m.originalConfig.ShowFolder)
	m.cursorPos[SectionSeasonFolder] = len(m.originalConfig.SeasonFolder)
	m.cursorPos[SectionEpisode] = len(m.originalConfig.Episode)
	m.cursorPos[SectionMovie] = len(m.originalConfig.Movie)

	// Reset logging fields
	m.loggingEnabled = m.originalConfig.EnableLogging
	m.loggingRetention = fmt.Sprintf("%d", m.originalConfig.LogRetentionDays)
	m.loggingSubfocus = 0

	// Reset TMDB fields
	m.tmdbAPIKey = m.originalConfig.TMDBAPIKey
	m.tmdbEnabled = m.originalConfig.EnableTMDBLookup
	m.tmdbLanguage = m.originalConfig.TMDBLanguage
	m.workerCount = fmt.Sprintf("%d", m.originalConfig.TMDBWorkerCount)
	m.tmdbSubfocus = 0
	m.tmdbValidation = ""
	m.tmdbValidatedKey = ""

	// Reset OMDb fields
	m.omdbAPIKey = m.originalConfig.OMDBAPIKey
	m.omdbEnabled = m.originalConfig.EnableOMDBLookup
	m.omdbSubfocus = 0
	m.omdbValidation = ""
	m.omdbValidatedKey = ""
	m.ffprobeEnabled = m.originalConfig.EnableFFProbe
	m.providerColumnFocus = providerColumnShared

	m.saveStatus = "Reset to saved values"
	m.err = nil
}

// validateTMDBAPIKey validates an API key against the TMDB API
func validateTMDBAPIKey(apiKey string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if apiKey == "" {
			return tmdbValidationMsg{apiKey: "", valid: false}
		}

		// Try to create a TMDB provider with the API key
		tmdbProvider := tmdb.New()
		config := map[string]interface{}{
			"api_key":       apiKey,
			"language":      "en-US",
			"cache_enabled": false, // Disable cache for validation
		}

		if err := tmdbProvider.Configure(config); err != nil {
			// If provider configuration fails, it's invalid
			return tmdbValidationMsg{apiKey: apiKey, valid: false}
		}

		// Try to search for a well-known movie to validate the API key
		// Using "The Matrix" as it's a popular movie that should always return results
		request := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      "The Matrix",
			Year:      "1999",
		}
		_, err := tmdbProvider.Fetch(context.Background(), request)
		if err != nil {
			// Check if the error is specifically an invalid API key error
			var provErr *provider.ProviderError
			if errors.As(err, &provErr) && provErr.Code == "AUTH_FAILED" {
				return tmdbValidationMsg{apiKey: apiKey, valid: false}
			}
			// For other errors (network, etc.), we'll consider it invalid
			// to be safe, but you could handle this differently if needed
			return tmdbValidationMsg{apiKey: apiKey, valid: false}
		}

		// If we successfully searched for a movie, the API key is valid
		return tmdbValidationMsg{apiKey: apiKey, valid: true}
	})
}

// debouncedTMDBValidate creates a command that validates after a delay
func debouncedTMDBValidate(apiKey string) tea.Cmd {
	return components.DebounceMsg(1*time.Second, tmdbValidateCmd{apiKey: apiKey})
}

// validateTMDBOnSectionSwitch validates the API key immediately when switching to TMDB section
func (m *Model) validateTMDBOnSectionSwitch() tea.Cmd {
	// Only validate if there's an API key and it hasn't been validated recently
	if len(m.tmdbAPIKey) > 0 && m.tmdbAPIKey != m.tmdbValidatedKey {
		m.tmdbValidation = "validating"
		return m.tmdbValidate(m.tmdbAPIKey)
	}
	return nil
}

// validateOMDBAPIKey validates an API key against the OMDb API
func validateOMDBAPIKey(apiKey string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if apiKey == "" {
			return omdbValidationMsg{apiKey: "", valid: false}
		}

		prov := omdb.New()
		if err := prov.Configure(map[string]interface{}{"api_key": apiKey}); err != nil {
			return omdbValidationMsg{apiKey: apiKey, valid: false}
		}

		request := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      "The Matrix",
			Year:      "1999",
		}
		meta, err := prov.Fetch(context.Background(), request)
		if err != nil || meta == nil {
			return omdbValidationMsg{apiKey: apiKey, valid: false}
		}

		return omdbValidationMsg{apiKey: apiKey, valid: true}
	})
}

// debouncedOMDBValidate creates a command that validates after a short delay
func debouncedOMDBValidate(apiKey string) tea.Cmd {
	return components.DebounceMsg(1*time.Second, omdbValidateCmd{apiKey: apiKey})
}

// validateOMDBOnSectionSwitch validates the OMDb API key when entering provider settings
func (m *Model) validateOMDBOnSectionSwitch() tea.Cmd {
	if len(m.omdbAPIKey) > 0 && m.omdbAPIKey != m.omdbValidatedKey {
		m.omdbValidation = "validating"
		return m.omdbValidate(m.omdbAPIKey)
	}
	return nil
}
