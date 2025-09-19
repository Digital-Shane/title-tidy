package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// selectConfigIcons chooses the best icon set for the config UI
func selectConfigIcons() map[string]string {
	return SelectIcons()
}

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
	tmdbPreferLocal     bool   // Prefer local metadata
	tmdbWorkerCount     string // TMDB worker count as string for input
	tmdbSubfocus        int    // 0=enabled, 1=api key, 2=language, 3=prefer local, 4=worker count
	ffprobeEnabled      bool   // ffprobe provider enabled
	providerColumnFocus int    // 0=TMDB column, 1=ffprobe column
	tmdbValidation      string // API key validation status: "", "validating", "valid", "invalid"
	tmdbValidatedKey    string // Last API key that was validated
	width               int
	height              int
	saveStatus          string
	err                 error
	variablesView       viewport.Model           // viewport for scrolling variables list
	autoScroll          bool                     // whether auto-scrolling is enabled
	templateRegistry    *config.TemplateRegistry // registry for template variables
}

// New creates a new configuration UI model
func New() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create a copy for reset functionality
	originalCfg := &config.FormatConfig{
		ShowFolder:          cfg.ShowFolder,
		SeasonFolder:        cfg.SeasonFolder,
		Episode:             cfg.Episode,
		Movie:               cfg.Movie,
		LogRetentionDays:    cfg.LogRetentionDays,
		EnableLogging:       cfg.EnableLogging,
		TMDBAPIKey:          cfg.TMDBAPIKey,
		EnableTMDBLookup:    cfg.EnableTMDBLookup,
		TMDBLanguage:        cfg.TMDBLanguage,
		PreferLocalMetadata: cfg.PreferLocalMetadata,
		TMDBWorkerCount:     cfg.TMDBWorkerCount,
		EnableFFProbe:       cfg.EnableFFProbe,
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
		tmdbPreferLocal:     cfg.PreferLocalMetadata,
		tmdbWorkerCount:     fmt.Sprintf("%d", cfg.TMDBWorkerCount),
		tmdbSubfocus:        0,
		ffprobeEnabled:      cfg.EnableFFProbe,
		providerColumnFocus: 0,
		variablesView:       viewport.New(0, 0),
		autoScroll:          true,
	}

	return m, nil
}

// NewWithRegistry creates a new configuration UI model with a template registry
func NewWithRegistry(templateReg *config.TemplateRegistry) (*Model, error) {
	m, err := New()
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
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
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
			return m, validateTMDBAPIKey(msg.apiKey)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport dimensions
		panelHeight := m.height - 10
		leftWidth := m.width / 3
		m.variablesView.Width = leftWidth - 4    // Account for borders and padding
		m.variablesView.Height = panelHeight - 4 // Account for borders and title
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
				if m.providerColumnFocus == 0 {
					// Within TMDB column, up arrow switches between fields
					if m.tmdbEnabled && m.tmdbSubfocus > 0 {
						m.tmdbSubfocus--
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
				if m.providerColumnFocus == 0 {
					// Within TMDB column, down arrow switches between fields
					if m.tmdbEnabled && m.tmdbSubfocus < 4 {
						m.tmdbSubfocus++
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
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == 0 && m.tmdbSubfocus == 1 {
				// Backspace in TMDB API key field
				m.deleteTMDBChar()
				// Trigger debounced validation after deletion
				return m, debouncedTMDBValidate(m.tmdbAPIKey)
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == 0 && m.tmdbSubfocus == 2 {
				// Backspace in TMDB language field
				m.deleteTMDBChar()
			} else if m.activeSection == SectionProviders && m.providerColumnFocus == 0 && m.tmdbSubfocus == 4 {
				// Backspace in TMDB worker count field
				m.deleteTMDBChar()
			} else if m.activeSection != SectionLogging && m.activeSection != SectionProviders {
				m.deleteChar()
			}
			return m, nil

		case tea.KeyLeft:
			if m.activeSection == SectionProviders {
				if m.providerColumnFocus > 0 {
					m.providerColumnFocus--
				}
				return m, nil
			}
			if m.cursorPos[m.activeSection] > 0 {
				m.cursorPos[m.activeSection]--
			}
			return m, nil

		case tea.KeyRight:
			if m.activeSection == SectionProviders {
				if m.providerColumnFocus < 1 {
					m.providerColumnFocus++
				} else {
					m.providerColumnFocus = 0
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
				if m.providerColumnFocus == 0 {
					if m.tmdbSubfocus == 0 {
						// Enter toggles TMDB enabled
						m.tmdbEnabled = !m.tmdbEnabled
						if !m.tmdbEnabled {
							m.tmdbSubfocus = 0
						}
						m.updateVariablesContent()
					} else if m.tmdbSubfocus == 3 && m.tmdbEnabled {
						// Enter toggles prefer local metadata (only when TMDB is enabled)
						m.tmdbPreferLocal = !m.tmdbPreferLocal
					}
				} else {
					m.ffprobeEnabled = !m.ffprobeEnabled
					m.updateVariablesContent()
				}
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
				if m.providerColumnFocus == 0 {
					if m.tmdbSubfocus == 0 {
						// Space toggles TMDB enabled
						m.tmdbEnabled = !m.tmdbEnabled
						if !m.tmdbEnabled {
							m.tmdbSubfocus = 0
						}
						m.updateVariablesContent()
						return m, nil
					} else if m.tmdbSubfocus == 3 && m.tmdbEnabled {
						// Space toggles prefer local metadata (only when TMDB is enabled)
						m.tmdbPreferLocal = !m.tmdbPreferLocal
						return m, nil
					}
				} else {
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
			if m.activeSection == SectionProviders && m.providerColumnFocus == 0 && m.tmdbSubfocus == 1 {
				// No spaces in API key field
				return m, nil
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
				if m.providerColumnFocus == 0 {
					// Handle TMDB input based on subfocus
					runes := string(msg.Runes)
					switch m.tmdbSubfocus {
					case 1:
						// API key field - accept any characters the user enters
						m.insertTMDBAPIKey(runes)
						// Trigger debounced validation
						return m, debouncedTMDBValidate(m.tmdbAPIKey)
					case 2:
						// Language field - allow letters, hyphen for codes like "en-US"
						for _, r := range runes {
							if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
								m.insertTMDBLanguage(string(r))
							}
						}
					case 4:
						// Worker count field - only allow digits
						for _, r := range runes {
							if r >= '0' && r <= '9' {
								m.insertTMDBWorkerCount(string(r))
							}
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
	icons := selectConfigIcons()
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

	// Left panel: Available components with viewport
	leftContent := m.renderVariablesViewport()
	leftPanel := borderStyle.Width(leftWidth).Height(panelHeight).Render(leftContent)

	// Right panel: Input and preview
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

func (m *Model) buildRightPanel(width, height int) string {
	inputHeight := 3
	previewHeight := height - inputHeight - 1

	// Ensure minimum dimensions
	if previewHeight < 0 {
		previewHeight = 0
	}

	// Input section
	input := m.buildInputField(width)

	// Preview section
	preview := m.buildPreview(width, previewHeight)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		input,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#4a5568")).Render(strings.Repeat("─", width)),
		preview,
	)
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

	icons := selectConfigIcons()
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
	// Determine if we have enough space for side-by-side columns
	twoColumns := width >= 48
	columnGap := 2
	columnWidth := width
	if twoColumns {
		columnWidth = (width - columnGap) / 2
		if columnWidth < 20 {
			// Fall back to stacked layout if columns would be too narrow
			twoColumns = false
			columnWidth = width
		}
	}

	columnStyle := lipgloss.NewStyle().Width(columnWidth)

	tmdbColumn := columnStyle.Render(m.buildTMDBColumn(columnWidth))
	ffprobeColumn := columnStyle.Render(m.buildFFProbeColumn(columnWidth))

	if twoColumns {
		gap := lipgloss.NewStyle().Width(columnGap).Render(" ")
		return lipgloss.JoinHorizontal(lipgloss.Top, tmdbColumn, gap, ffprobeColumn)
	}

	return lipgloss.JoinVertical(lipgloss.Left, tmdbColumn, "", ffprobeColumn)
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

	// Add hint about TMDB if it's not enabled and we're in a template section
	if !m.tmdbEnabled && (m.activeSection == SectionShowFolder ||
		m.activeSection == SectionSeasonFolder ||
		m.activeSection == SectionEpisode ||
		m.activeSection == SectionMovie) {
		// Add spacing before hint
		lines = append(lines, "")
		lines = append(lines, hintStyle.Render("💡 Enable TMDB for more template options"))
		lines = append(lines, hintStyle.Render("   (rating, genres, tagline, etc.)"))
	}

	// Truncate if too tall (but keep the hint if present)
	if maxHeight > 0 && len(lines) > maxHeight {
		// Check if we have a hint (last 3 lines)
		hasHint := !m.tmdbEnabled && (m.activeSection == SectionShowFolder ||
			m.activeSection == SectionSeasonFolder ||
			m.activeSection == SectionEpisode ||
			m.activeSection == SectionMovie)

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
			{"↑/↓ arrows", "Navigate TMDB fields", ""},
			{"Space/Enter", "Toggle highlighted setting", ""},
			{"TMDB API Key", "TMDB API key (32 hex chars)", "Get from themoviedb.org (NOT Read Token)"},
			{"TMDB Language", "Preferred metadata language", "en-US, fr-FR, etc."},
			{"Prefer Local", "Use local metadata before TMDB", "Falls back to TMDB when needed"},
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
			// Skip provider variables if the provider is not enabled
			if v.Provider == "tmdb" && !m.tmdbEnabled {
				continue
			}
			if v.Provider == "ffprobe" && !m.ffprobeEnabled {
				continue
			}

			// Format the variable name with braces
			varName := "{" + v.Name + "}"
			result = append(result, variable{
				name:        varName,
				description: v.Description,
				example:     v.Example,
			})
		}

		// If we got variables from registry, return them
		if len(result) > 0 {
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

		icons := selectConfigIcons()
		return []preview{
			{icons["check"], "Logging", enabledStatus},
			{icons["calendar"], "Retention", m.loggingRetention + " days"},
			{icons["folder"], "Log Location", "~/.title-tidy/logs/"},
			{icons["document"], "Log Format", "JSON session files"},
		}
	}

	if m.activeSection == SectionProviders {
		// Show TMDB configuration status
		enabledStatus := "Disabled"
		if m.tmdbEnabled {
			enabledStatus = "Enabled"
		}

		apiStatus := "Not configured"
		if len(m.tmdbAPIKey) > 0 {
			switch m.tmdbValidation {
			case "validating":
				apiStatus = "Validating..."
			case "valid":
				apiStatus = "Valid"
			case "invalid":
				apiStatus = "Invalid"
			default:
				apiStatus = "Configured"
			}
		}

		preferStatus := "TMDB first"
		if m.tmdbPreferLocal {
			preferStatus = "Local first"
		}

		ffprobeStatus := "Disabled"
		if m.ffprobeEnabled {
			ffprobeStatus = "Enabled"
		}

		icons := selectConfigIcons()
		return []preview{
			{icons["film"], "TMDB Lookup", enabledStatus},
			{icons["key"], "API Key", apiStatus},
			{icons["globe"], "Language", m.tmdbLanguage},
			{icons["chart"], "Priority", preferStatus},
			{icons["chip"], "ffprobe", ffprobeStatus},
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

	icons := selectConfigIcons()

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
	m.providerColumnFocus = 0

	// Validate API key when switching to TMDB section
	if m.activeSection == SectionProviders && oldSection != SectionProviders {
		return m.validateTMDBOnSectionSwitch()
	}
	return nil
}

func (m *Model) prevSection() tea.Cmd {
	oldSection := m.activeSection
	m.activeSection = (m.activeSection + 5) % 6 // +5 is same as -1 mod 6
	m.loggingSubfocus = 0                       // Reset subfocus when changing sections
	m.tmdbSubfocus = 0                          // Reset TMDB subfocus when changing sections
	m.providerColumnFocus = 0

	// Validate API key when switching to TMDB section
	if m.activeSection == SectionProviders && oldSection != SectionProviders {
		return m.validateTMDBOnSectionSwitch()
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

	icons := selectConfigIcons()
	isActive := m.providerColumnFocus == 0

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
	maskedKey := ""
	if len(m.tmdbAPIKey) > 0 {
		// Show first 4 and last 4 characters, mask the middle
		if len(m.tmdbAPIKey) <= 8 {
			maskedKey = strings.Repeat("*", len(m.tmdbAPIKey))
		} else {
			maskedKey = m.tmdbAPIKey[:4] + strings.Repeat("*", len(m.tmdbAPIKey)-8) + m.tmdbAPIKey[len(m.tmdbAPIKey)-4:]
		}
	}

	if !m.tmdbEnabled {
		// When TMDB is disabled, always show as disabled regardless of focus
		apiKeyField = disabledStyle.Render(apiKeyLabel + maskedKey + " (disabled)")
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
		// When TMDB is disabled, always show as disabled regardless of focus
		languageField = disabledStyle.Render(languageLabel + m.tmdbLanguage + " (disabled)")
	} else if isActive && m.tmdbSubfocus == 2 {
		// Only show focus when TMDB is enabled - show cursor after the text
		languageField = languageLabel + normalStyle.Render(m.tmdbLanguage) + focusedStyle.Render(" ")
	} else {
		languageField = languageLabel + normalStyle.Render(m.tmdbLanguage)
	}

	// Build prefer local metadata toggle
	var preferLocalToggle string
	preferLocalText := "Prefer Local Metadata"

	if !m.tmdbEnabled {
		// When TMDB is disabled, always show as disabled regardless of focus
		preferLocalToggle = disabledStyle.Render("[ ] " + preferLocalText + " (disabled)")
	} else if isActive && m.tmdbSubfocus == 3 {
		// Only show focus when TMDB is enabled
		if m.tmdbPreferLocal {
			preferLocalToggle = focusedStyle.Render("[" + icons["check"] + "] " + preferLocalText)
		} else {
			preferLocalToggle = focusedStyle.Render("[ ] " + preferLocalText)
		}
	} else {
		if m.tmdbPreferLocal {
			preferLocalToggle = enabledStyle.Render("[" + icons["check"] + "] " + preferLocalText)
		} else {
			preferLocalToggle = disabledStyle.Render("[ ] " + preferLocalText)
		}
	}

	// Build worker count field
	var workerCountField string
	workerCountLabel := "Worker Count: "
	workerCountValue := m.tmdbWorkerCount

	if !m.tmdbEnabled {
		// When TMDB is disabled, always show as disabled regardless of focus
		workerCountField = disabledStyle.Render(workerCountLabel + workerCountValue + " (disabled)")
	} else if isActive && m.tmdbSubfocus == 4 {
		// Only show focus when TMDB is enabled - show cursor after the text
		workerCountField = workerCountLabel + normalStyle.Render(workerCountValue) + focusedStyle.Render(" ")
	} else {
		workerCountField = workerCountLabel + normalStyle.Render(workerCountValue)
	}

	lines := []string{
		label,
		enabledToggle,
		apiKeyField,
		languageField,
		preferLocalToggle,
		workerCountField,
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

	icons := selectConfigIcons()
	enabledText := "Enabled"
	if !m.ffprobeEnabled {
		enabledText = "Disabled"
	}

	var enabledToggle string
	if m.providerColumnFocus == 1 {
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

func (m *Model) insertTMDBWorkerCount(text string) {
	if len(m.tmdbWorkerCount) < 3 { // Max 3 digits for worker count (up to 999)
		m.tmdbWorkerCount += text
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
	} else if m.tmdbSubfocus == 4 && len(m.tmdbWorkerCount) > 0 {
		m.tmdbWorkerCount = m.tmdbWorkerCount[:len(m.tmdbWorkerCount)-1]
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

	// Update TMDB config, stripping null characters from string fields
	m.config.TMDBAPIKey = stripNullChars(m.tmdbAPIKey)
	m.config.EnableTMDBLookup = m.tmdbEnabled
	m.config.TMDBLanguage = stripNullChars(m.tmdbLanguage)
	m.config.PreferLocalMetadata = m.tmdbPreferLocal
	m.config.EnableFFProbe = m.ffprobeEnabled

	// Update worker count
	sanitizedWorkerCount := stripNullChars(m.tmdbWorkerCount)
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
		m.originalConfig.PreferLocalMetadata = m.config.PreferLocalMetadata
		m.originalConfig.TMDBWorkerCount = m.config.TMDBWorkerCount
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
	m.tmdbPreferLocal = m.originalConfig.PreferLocalMetadata
	m.tmdbWorkerCount = fmt.Sprintf("%d", m.originalConfig.TMDBWorkerCount)
	m.tmdbSubfocus = 0
	m.ffprobeEnabled = m.originalConfig.EnableFFProbe
	m.providerColumnFocus = 0

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
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tmdbValidateCmd{apiKey: apiKey}
	})
}

// validateTMDBOnSectionSwitch validates the API key immediately when switching to TMDB section
func (m *Model) validateTMDBOnSectionSwitch() tea.Cmd {
	// Only validate if there's an API key and it hasn't been validated recently
	if len(m.tmdbAPIKey) > 0 && m.tmdbAPIKey != m.tmdbValidatedKey {
		m.tmdbValidation = "validating"
		return validateTMDBAPIKey(m.tmdbAPIKey)
	}
	return nil
}
