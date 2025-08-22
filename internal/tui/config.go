package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// scrollTickMsg is sent periodically to enable auto-scrolling
type scrollTickMsg struct{}

// Section represents a configuration section
type Section int

const (
	SectionShowFolder Section = iota
	SectionSeasonFolder
	SectionEpisode
	SectionMovie
)

// Model is the Bubble Tea model for the configuration UI
type Model struct {
	config         *config.FormatConfig
	originalConfig *config.FormatConfig // for reset functionality
	activeSection  Section
	inputs         map[Section]string
	cursorPos      map[Section]int
	width          int
	height         int
	saveStatus     string
	err            error
	variablesView  viewport.Model // viewport for scrolling variables list
	autoScroll     bool           // whether auto-scrolling is enabled
	scrollPaused   bool           // whether scrolling is temporarily paused
}

// New creates a new configuration UI model
func New() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create a copy for reset functionality
	originalCfg := &config.FormatConfig{
		ShowFolder:   cfg.ShowFolder,
		SeasonFolder: cfg.SeasonFolder,
		Episode:      cfg.Episode,
		Movie:        cfg.Movie,
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
		variablesView: viewport.New(0, 0),
		autoScroll:    true,
	}

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
		if m.autoScroll && !m.scrollPaused {
			// Auto-scroll slowly
			if m.variablesView.AtBottom() {
				// Reset to top when we reach the bottom
				m.variablesView.GotoTop()
			} else {
				// Scroll down by 1 config item
				m.variablesView.LineDown(4)
			}
		}
		return m, m.tickCmd()

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
			// Manual scroll up in variables view
			m.scrollPaused = true
			m.variablesView.LineUp(1)
			// Resume auto-scroll after a delay
			go func() {
				time.Sleep(3 * time.Second)
				m.scrollPaused = false
			}()
			return m, nil

		case tea.KeyDown:
			// Manual scroll down in variables view
			m.scrollPaused = true
			m.variablesView.LineDown(1)
			// Resume auto-scroll after a delay
			go func() {
				time.Sleep(3 * time.Second)
				m.scrollPaused = false
			}()
			return m, nil

		case tea.KeyPgUp:
			// Page up in variables view
			m.scrollPaused = true
			m.variablesView.HalfPageUp()
			go func() {
				time.Sleep(3 * time.Second)
				m.scrollPaused = false
			}()
			return m, nil

		case tea.KeyPgDown:
			// Page down in variables view
			m.scrollPaused = true
			m.variablesView.HalfPageDown()
			go func() {
				time.Sleep(3 * time.Second)
				m.scrollPaused = false
			}()
			return m, nil

		case tea.KeyTab:
			m.nextSection()
			m.updateVariablesContent() // Update content when section changes
			return m, nil

		case tea.KeyShiftTab:
			m.prevSection()
			m.updateVariablesContent() // Update content when section changes
			return m, nil

		case tea.KeyBackspace:
			m.deleteChar()
			return m, nil

		case tea.KeyLeft:
			if m.cursorPos[m.activeSection] > 0 {
				m.cursorPos[m.activeSection]--
			}
			return m, nil

		case tea.KeyRight:
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

		case tea.KeyCtrlQ, tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyDelete:
			if strings.Contains(msg.String(), "ctrl+delete") {
				// Ctrl+Delete: clear field
				m.inputs[m.activeSection] = ""
				m.cursorPos[m.activeSection] = 0
			}
			return m, nil

		case tea.KeySpace:
			if msg.Alt {
				// Alt+Space toggles auto-scroll
				m.autoScroll = !m.autoScroll
				return m, nil
			}
			// Regular space for text input
			m.insertText(" ")
			return m, nil

		case tea.KeyRunes:
			m.insertText(string(msg.Runes))
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

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7c3aed")).
		Padding(1, 0).
		Align(lipgloss.Center)

	tabStyle := lipgloss.NewStyle().
		Padding(0, 2)

	activeTabStyle := tabStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#7c3aed"))

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4a5568"))

	// Title
	title := titleStyle.Render("📺 Title-Tidy Format Configuration")

	// Tabs
	tabs := []string{}
	for i, label := range []string{"Show Folder", "Season Folder", "Episode", "Movie"} {
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
	if m.variablesView.TotalLineCount() > m.variablesView.Height {
		if m.autoScroll {
			if m.scrollPaused {
				scrollIndicator = " [Paused]"
			} else {
				scrollIndicator = " [Auto-scrolling]"
			}
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

func (m *Model) buildPreview(width, maxHeight int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)

	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af"))

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

	// Truncate if too tall
	if len(lines) > maxHeight {
		lines = lines[:maxHeight]
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

	help := []string{
		keyStyle.Render("Tab") + ": Switch",
		keyStyle.Render("Type directly"),
		keyStyle.Render("Ctrl+S") + ": Save",
		keyStyle.Render("Ctrl+R") + ": Reset",
		keyStyle.Render("Ctrl+Q") + ": Quit",
	}

	// Add scroll help if content is scrollable
	if m.variablesView.TotalLineCount() > m.variablesView.Height {
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
	switch m.activeSection {
	case SectionShowFolder:
		return []variable{
			{"{show}", "Show name", "Breaking Bad"},
			{"{year}", "Year", "2008"},
		}
	case SectionSeasonFolder:
		return []variable{
			{"{show}", "Show name", "Breaking Bad"},
			{"{season}", "Season number", "01"},
			{"{season_code}", "Season code", "S01"},
			{"{season_name}", "Full season name", "Season 01"},
		}
	case SectionEpisode:
		return []variable{
			{"{show}", "Show name", "Breaking Bad"},
			{"{year}", "Year", "2008"},
			{"{season}", "Season number", "01"},
			{"{episode}", "Episode number", "05"},
			{"{season_code}", "Season code", "S01"},
			{"{episode_code}", "Episode code", "E05"},
		}
	case SectionMovie:
		return []variable{
			{"{movie}", "Movie name", "The Matrix"},
			{"{year}", "Year", "1999"},
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
	// Use current input values to generate previews
	cfg := &config.FormatConfig{
		ShowFolder:   m.inputs[SectionShowFolder],
		SeasonFolder: m.inputs[SectionSeasonFolder],
		Episode:      m.inputs[SectionEpisode],
		Movie:        m.inputs[SectionMovie],
	}

	return []preview{
		{"📺", "Show", cfg.ApplyShowFolderTemplate("Breaking Bad", "2008")},
		{"📁", "Season", cfg.ApplySeasonFolderTemplate("Breaking Bad", "2008", 1)},
		{"🎬", "Episode", cfg.ApplyEpisodeTemplate("Breaking Bad", "2008", 1, 7) + ".mkv"},
		{"🎭", "Movie", cfg.ApplyMovieTemplate("The Matrix", "1999")},
	}
}

func (m *Model) nextSection() {
	m.activeSection = (m.activeSection + 1) % 4
}

func (m *Model) prevSection() {
	m.activeSection = (m.activeSection + 3) % 4 // +3 is same as -1 mod 4
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

func (m *Model) save() {
	// Update config from inputs
	m.config.ShowFolder = m.inputs[SectionShowFolder]
	m.config.SeasonFolder = m.inputs[SectionSeasonFolder]
	m.config.Episode = m.inputs[SectionEpisode]
	m.config.Movie = m.inputs[SectionMovie]

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

	m.saveStatus = "Reset to saved values"
	m.err = nil
}
