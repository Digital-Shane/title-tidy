package config

import (
	"strings"
	"unicode"

	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type providerSection struct {
	state *ProviderState
	theme theme.Theme
	icons map[string]string
	width int

	tmdbValidate func(string) tea.Cmd
	tmdbDebounce func(string) tea.Cmd
	tvdbValidate func(string) tea.Cmd
	tvdbDebounce func(string) tea.Cmd
	omdbValidate func(string) tea.Cmd
	omdbDebounce func(string) tea.Cmd
}

func newProviderSection(state *ProviderState, th theme.Theme) *providerSection {
	return &providerSection{
		state:        state,
		theme:        th,
		icons:        th.IconSet(),
		tmdbValidate: validateTMDBAPIKey,
		tmdbDebounce: debouncedTMDBValidate,
		tvdbValidate: validateTVDBAPIKey,
		tvdbDebounce: debouncedTVDBValidate,
		omdbValidate: validateOMDBAPIKey,
		omdbDebounce: debouncedOMDBValidate,
	}
}

func (p *providerSection) Init() tea.Cmd { return nil }

func (p *providerSection) Section() Section { return SectionProviders }

func (p *providerSection) Title() string { return "Providers" }

func (p *providerSection) Focus() tea.Cmd {
	p.ensureActiveField()
	return p.applyFocus()
}

func (p *providerSection) Blur() {
	p.state.WorkerCount.Blur()
	p.state.TMDB.APIKey.Blur()
	p.state.TMDB.Language.Blur()
	p.state.TVDB.APIKey.Blur()
	p.state.OMDB.APIKey.Blur()
}

func (p *providerSection) Resize(width int) {
	p.width = width
	if width > 0 {
		p.state.WorkerCount.Width = width
		p.state.TMDB.APIKey.Width = width
		p.state.TMDB.Language.Width = width
		p.state.TVDB.APIKey.Width = width
		p.state.OMDB.APIKey.Width = width
	}
}

func (p *providerSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if m.Alt {
			return p, nil
		}
		p.ensureActiveField()
		return p, p.handleKey(m)
	case tmdbValidateCmd:
		return p.handleTMDBValidateCmd(m)
	case tmdbValidationMsg:
		return p.handleTMDBValidationMsg(m)
	case tvdbValidateCmd:
		return p.handleTVDBValidateCmd(m)
	case tvdbValidationMsg:
		return p.handleTVDBValidationMsg(m)
	case omdbValidateCmd:
		return p.handleOMDBValidateCmd(m)
	case omdbValidationMsg:
		return p.handleOMDBValidationMsg(m)
	}
	return p, nil
}

func (p *providerSection) handleKey(key tea.KeyMsg) tea.Cmd {
	switch key.Type {
	case tea.KeyLeft, tea.KeyUp:
		return p.moveFocus(-1)
	case tea.KeyRight, tea.KeyDown:
		return p.moveFocus(1)
	case tea.KeyEnter, tea.KeySpace:
		if key.Type == tea.KeySpace && key.Alt {
			return nil
		}
		return p.toggleActive()
	}

	if cmd, handled := p.handleTextInputs(key); handled {
		return cmd
	}

	return nil
}

func (p *providerSection) moveFocus(delta int) tea.Cmd {
	fields := p.focusOrder()
	if len(fields) == 0 {
		return nil
	}
	idx := 0
	for i, field := range fields {
		if field == p.state.Active {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(fields)) % len(fields)
	p.state.Active = fields[idx]
	return p.applyFocus()
}

func (p *providerSection) focusOrder() []ProviderField {
	fields := []ProviderField{
		ProviderFieldWorkers,
		ProviderFieldFFProbe,
		ProviderFieldOMDBToggle,
	}
	if p.state.OMDB.Enabled {
		fields = append(fields, ProviderFieldOMDBKey)
	}
	fields = append(fields, ProviderFieldTMDBToggle)
	if p.state.TMDB.Enabled {
		fields = append(fields, ProviderFieldTMDBKey, ProviderFieldTMDBLanguage)
	}
	fields = append(fields, ProviderFieldTVDBToggle)
	if p.state.TVDB.Enabled {
		fields = append(fields, ProviderFieldTVDBKey)
	}
	return fields
}

func (p *providerSection) ensureActiveField() {
	fields := p.focusOrder()
	if len(fields) == 0 {
		return
	}
	for _, field := range fields {
		if field == p.state.Active {
			return
		}
	}
	p.state.Active = fields[0]
}

func (p *providerSection) toggleActive() tea.Cmd {
	switch p.state.Active {
	case ProviderFieldFFProbe:
		p.state.FFProbeEnabled = !p.state.FFProbeEnabled
		return nil
	case ProviderFieldOMDBToggle:
		p.state.OMDB.Enabled = !p.state.OMDB.Enabled
		if !p.state.OMDB.Enabled {
			p.state.OMDB.Validation.Reset()
			p.ensureActiveField()
			return p.applyFocus()
		}
		if cmd := p.queueOMDBValidation(); cmd != nil {
			return tea.Batch(p.applyFocus(), cmd)
		}
		return p.applyFocus()
	case ProviderFieldTMDBToggle:
		p.state.TMDB.Enabled = !p.state.TMDB.Enabled
		if !p.state.TMDB.Enabled {
			p.state.TMDB.Validation.Reset()
			p.ensureActiveField()
			return p.applyFocus()
		}
		if cmd := p.queueTMDBValidation(); cmd != nil {
			return tea.Batch(p.applyFocus(), cmd)
		}
		return p.applyFocus()
	case ProviderFieldTVDBToggle:
		p.state.TVDB.Enabled = !p.state.TVDB.Enabled
		if !p.state.TVDB.Enabled {
			p.state.TVDB.Validation.Reset()
			p.ensureActiveField()
			return p.applyFocus()
		}
		if cmd := p.queueTVDBValidation(); cmd != nil {
			return tea.Batch(p.applyFocus(), cmd)
		}
		return p.applyFocus()
	}
	return nil
}

func (p *providerSection) applyFocus() tea.Cmd {
	p.state.WorkerCount.Blur()
	p.state.TMDB.APIKey.Blur()
	p.state.TMDB.Language.Blur()
	p.state.TVDB.APIKey.Blur()
	p.state.OMDB.APIKey.Blur()

	switch p.state.Active {
	case ProviderFieldWorkers:
		return p.state.WorkerCount.Focus()
	case ProviderFieldOMDBKey:
		if p.state.OMDB.Enabled {
			return p.state.OMDB.APIKey.Focus()
		}
	case ProviderFieldTMDBKey:
		if p.state.TMDB.Enabled {
			return p.state.TMDB.APIKey.Focus()
		}
	case ProviderFieldTMDBLanguage:
		if p.state.TMDB.Enabled {
			return p.state.TMDB.Language.Focus()
		}
	case ProviderFieldTVDBKey:
		if p.state.TVDB.Enabled {
			return p.state.TVDB.APIKey.Focus()
		}
	}
	return nil
}

func (p *providerSection) handleTextInputs(key tea.KeyMsg) (tea.Cmd, bool) {
	switch p.state.Active {
	case ProviderFieldWorkers:
		if key.Type == tea.KeySpace {
			return nil, true
		}
		if key.Type == tea.KeyRunes {
			digits := make([]rune, 0, len(key.Runes))
			for _, r := range key.Runes {
				if unicode.IsDigit(r) {
					digits = append(digits, r)
				}
			}
			if len(digits) == 0 {
				return nil, true
			}
			key = tea.KeyMsg{Type: tea.KeyRunes, Runes: digits}
		}
		prev := p.state.WorkerCount.Value()
		var cmd tea.Cmd
		p.state.WorkerCount, cmd = p.state.WorkerCount.Update(key)
		if prev != p.state.WorkerCount.Value() {
			return cmd, true
		}
		return cmd, true

	case ProviderFieldOMDBKey:
		if !p.state.OMDB.Enabled {
			return nil, false
		}
		if key.Type == tea.KeySpace {
			return nil, true
		}
		prev := p.state.OMDB.APIKey.Value()
		var cmd tea.Cmd
		p.state.OMDB.APIKey, cmd = p.state.OMDB.APIKey.Update(key)
		if prev != p.state.OMDB.APIKey.Value() {
			if debounced := p.queueOMDBValidation(); debounced != nil {
				cmd = tea.Batch(cmd, debounced)
			}
		}
		return cmd, true

	case ProviderFieldTMDBKey:
		if !p.state.TMDB.Enabled {
			return nil, false
		}
		if key.Type == tea.KeySpace {
			return nil, true
		}
		prev := p.state.TMDB.APIKey.Value()
		var cmd tea.Cmd
		p.state.TMDB.APIKey, cmd = p.state.TMDB.APIKey.Update(key)
		if prev != p.state.TMDB.APIKey.Value() {
			if debounced := p.queueTMDBValidation(); debounced != nil {
				cmd = tea.Batch(cmd, debounced)
			}
		}
		return cmd, true

	case ProviderFieldTMDBLanguage:
		if !p.state.TMDB.Enabled {
			return nil, false
		}
		if key.Type == tea.KeyRunes {
			filtered := make([]rune, 0, len(key.Runes))
			for _, r := range key.Runes {
				if unicode.IsLetter(r) || r == '-' {
					filtered = append(filtered, r)
				}
			}
			if len(filtered) == 0 {
				return nil, true
			}
			key = tea.KeyMsg{Type: tea.KeyRunes, Runes: filtered}
		}
		prev := p.state.TMDB.Language.Value()
		var cmd tea.Cmd
		p.state.TMDB.Language, cmd = p.state.TMDB.Language.Update(key)
		if prev != p.state.TMDB.Language.Value() {
			// Language changes don't trigger validation directly.
		}
		return cmd, true

	case ProviderFieldTVDBKey:
		if !p.state.TVDB.Enabled {
			return nil, false
		}
		if key.Type == tea.KeySpace {
			return nil, true
		}
		prev := p.state.TVDB.APIKey.Value()
		var cmd tea.Cmd
		p.state.TVDB.APIKey, cmd = p.state.TVDB.APIKey.Update(key)
		if prev != p.state.TVDB.APIKey.Value() {
			if debounced := p.queueTVDBValidation(); debounced != nil {
				cmd = tea.Batch(cmd, debounced)
			}
		}
		return cmd, true
	}

	return nil, false
}

func (p *providerSection) queueTMDBValidation() tea.Cmd {
	if !p.state.TMDB.Enabled {
		return nil
	}
	key := strings.TrimSpace(p.state.TMDB.APIKey.Value())
	p.state.TMDB.Validation.Reset()
	if key == "" {
		return nil
	}
	return p.tmdbDebounce(key)
}

func (p *providerSection) queueOMDBValidation() tea.Cmd {
	if !p.state.OMDB.Enabled {
		return nil
	}
	key := strings.TrimSpace(p.state.OMDB.APIKey.Value())
	p.state.OMDB.Validation.Reset()
	if key == "" {
		return nil
	}
	return p.omdbDebounce(key)
}

func (p *providerSection) queueTVDBValidation() tea.Cmd {
	if !p.state.TVDB.Enabled {
		return nil
	}
	key := strings.TrimSpace(p.state.TVDB.APIKey.Value())
	p.state.TVDB.Validation.Reset()
	if key == "" {
		return nil
	}
	return p.tvdbDebounce(key)
}

func (p *providerSection) handleTMDBValidateCmd(cmd tmdbValidateCmd) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.TMDB.APIKey.Value())
	if cmd.apiKey == "" || cmd.apiKey != key {
		return p, nil
	}
	if cmd.apiKey == p.state.TMDB.Validation.LastValidated {
		return p, nil
	}
	p.state.TMDB.Validation.Status = ProviderValidationValidating
	return p, p.tmdbValidate(cmd.apiKey)
}

func (p *providerSection) handleTMDBValidationMsg(msg tmdbValidationMsg) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.TMDB.APIKey.Value())
	if msg.apiKey != key {
		return p, nil
	}
	if msg.valid {
		p.state.TMDB.Validation.Status = ProviderValidationValid
	} else {
		p.state.TMDB.Validation.Status = ProviderValidationInvalid
	}
	p.state.TMDB.Validation.LastValidated = msg.apiKey
	return p, nil
}

func (p *providerSection) handleOMDBValidateCmd(cmd omdbValidateCmd) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.OMDB.APIKey.Value())
	if cmd.apiKey == "" || cmd.apiKey != key {
		return p, nil
	}
	if cmd.apiKey == p.state.OMDB.Validation.LastValidated {
		return p, nil
	}
	p.state.OMDB.Validation.Status = ProviderValidationValidating
	return p, p.omdbValidate(cmd.apiKey)
}

func (p *providerSection) handleOMDBValidationMsg(msg omdbValidationMsg) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.OMDB.APIKey.Value())
	if msg.apiKey != key {
		return p, nil
	}
	if msg.valid {
		p.state.OMDB.Validation.Status = ProviderValidationValid
	} else {
		p.state.OMDB.Validation.Status = ProviderValidationInvalid
	}
	p.state.OMDB.Validation.LastValidated = msg.apiKey
	return p, nil
}

func (p *providerSection) handleTVDBValidateCmd(cmd tvdbValidateCmd) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.TVDB.APIKey.Value())
	if cmd.apiKey == "" || cmd.apiKey != key {
		return p, nil
	}
	if cmd.apiKey == p.state.TVDB.Validation.LastValidated {
		return p, nil
	}
	p.state.TVDB.Validation.Status = ProviderValidationValidating
	return p, p.tvdbValidate(cmd.apiKey)
}

func (p *providerSection) handleTVDBValidationMsg(msg tvdbValidationMsg) (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(p.state.TVDB.APIKey.Value())
	if msg.apiKey != key {
		return p, nil
	}
	if msg.valid {
		p.state.TVDB.Validation.Status = ProviderValidationValid
	} else {
		p.state.TVDB.Validation.Status = ProviderValidationInvalid
	}
	p.state.TVDB.Validation.LastValidated = msg.apiKey
	return p, nil
}

func (p *providerSection) Activate() tea.Cmd {
	var cmds []tea.Cmd
	if p.state.TMDB.Enabled {
		if cmd := p.tmdbValidateOnActivate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if p.state.OMDB.Enabled {
		if cmd := p.omdbValidateOnActivate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if p.state.TVDB.Enabled {
		if cmd := p.tvdbValidateOnActivate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (p *providerSection) tmdbValidateOnActivate() tea.Cmd {
	key := strings.TrimSpace(p.state.TMDB.APIKey.Value())
	if key == "" || key == p.state.TMDB.Validation.LastValidated {
		return nil
	}
	p.state.TMDB.Validation.Status = ProviderValidationValidating
	return p.tmdbValidate(key)
}

func (p *providerSection) omdbValidateOnActivate() tea.Cmd {
	key := strings.TrimSpace(p.state.OMDB.APIKey.Value())
	if key == "" || key == p.state.OMDB.Validation.LastValidated {
		return nil
	}
	p.state.OMDB.Validation.Status = ProviderValidationValidating
	return p.omdbValidate(key)
}

func (p *providerSection) tvdbValidateOnActivate() tea.Cmd {
	key := strings.TrimSpace(p.state.TVDB.APIKey.Value())
	if key == "" || key == p.state.TVDB.Validation.LastValidated {
		return nil
	}
	p.state.TVDB.Validation.Status = ProviderValidationValidating
	return p.tvdbValidate(key)
}

func (p *providerSection) View() string {
	colors := p.theme.Colors()
	title := p.theme.PanelTitleStyle().Render("Metadata Providers")

	shared := p.renderSharedColumn(colors)
	ffprobe := p.renderFFProbeColumn(colors)
	omdb := p.renderOMDBColumn(colors)
	tvdb := p.renderTVDBColumn(colors)
	tmdb := p.renderTMDBColumn(colors)

	columnGap := 2
	minColumnWidth := 22
	columnCount := 5
	totalGap := columnGap * (columnCount - 1)
	inline := p.width-totalGap >= minColumnWidth*columnCount

	if inline {
		gap := lipgloss.NewStyle().Width(columnGap).Render(" ")
		row := lipgloss.JoinHorizontal(lipgloss.Top, shared, gap, ffprobe, gap, omdb, gap, tmdb, gap, tvdb)
		return lipgloss.JoinVertical(lipgloss.Left, title, row)
	}

	separator := lipgloss.NewStyle().Height(1).Render("")
	stacked := lipgloss.JoinVertical(
		lipgloss.Left,
		shared,
		separator,
		ffprobe,
		separator,
		omdb,
		separator,
		tmdb,
		separator,
		tvdb,
	)
	return lipgloss.JoinVertical(lipgloss.Left, title, stacked)
}

func (p *providerSection) renderSharedColumn(colors theme.Colors) string {
	focused := p.state.Active == ProviderFieldWorkers
	field := p.state.WorkerCount.View()
	if focused {
		field = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(field)
	} else {
		field = lipgloss.NewStyle().Foreground(colors.Primary).Render(field)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("Shared"),
		"Worker Count: "+field,
		lipgloss.NewStyle().Foreground(colors.Muted).Render("Concurrent metadata fetch workers."),
	)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderFFProbeColumn(colors theme.Colors) string {
	focused := p.state.Active == ProviderFieldFFProbe
	toggle := p.renderToggle("ffprobe", p.state.FFProbeEnabled, focused, colors)
	description := lipgloss.NewStyle().Foreground(colors.Muted).Render("Enable codec metadata via ffprobe.")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("ffprobe"),
		toggle,
		description,
	)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderOMDBColumn(colors theme.Colors) string {
	toggleFocused := p.state.Active == ProviderFieldOMDBToggle
	keyFocused := p.state.Active == ProviderFieldOMDBKey && p.state.OMDB.Enabled

	toggle := p.renderToggle("OMDb", p.state.OMDB.Enabled, toggleFocused, colors)

	apiKey := p.state.OMDB.APIKey.Value()
	if keyFocused {
		apiKey = p.state.OMDB.APIKey.View()
	} else {
		apiKey = p.state.OMDB.MaskedAPIKey(2, 2)
	}

	switch {
	case !p.state.OMDB.Enabled:
		apiKey = lipgloss.NewStyle().Foreground(colors.Muted).Render(apiKey + " (disabled)")
	case keyFocused:
		apiKey = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(apiKey)
	default:
		apiKey = lipgloss.NewStyle().Foreground(colors.Primary).Render(apiKey)
	}

	status := p.renderValidation("Status", p.state.OMDB.Validation.Status)
	description := lipgloss.NewStyle().Foreground(colors.Muted).Render("Film/series metadata from OMDb.")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("OMDb"),
		toggle,
		"API Key: "+apiKey,
		status,
		description,
	)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderTMDBColumn(colors theme.Colors) string {
	toggleFocused := p.state.Active == ProviderFieldTMDBToggle
	keyFocused := p.state.Active == ProviderFieldTMDBKey && p.state.TMDB.Enabled
	langFocused := p.state.Active == ProviderFieldTMDBLanguage && p.state.TMDB.Enabled

	toggle := p.renderToggle("TMDB", p.state.TMDB.Enabled, toggleFocused, colors)

	apiKey := p.state.TMDB.APIKey.Value()
	if keyFocused {
		apiKey = p.state.TMDB.APIKey.View()
	} else {
		apiKey = p.state.TMDB.MaskedAPIKey(4, 4)
	}

	switch {
	case !p.state.TMDB.Enabled:
		apiKey = lipgloss.NewStyle().Foreground(colors.Muted).Render(apiKey + " (disabled)")
	case keyFocused:
		apiKey = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(apiKey)
	default:
		apiKey = lipgloss.NewStyle().Foreground(colors.Primary).Render(apiKey)
	}

	language := p.state.TMDB.Language.Value()
	if langFocused {
		language = p.state.TMDB.Language.View()
	}

	switch {
	case !p.state.TMDB.Enabled:
		language = lipgloss.NewStyle().Foreground(colors.Muted).Render(language + " (disabled)")
	case langFocused:
		language = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(language)
	default:
		language = lipgloss.NewStyle().Foreground(colors.Primary).Render(language)
	}

	status := p.renderValidation("Status", p.state.TMDB.Validation.Status)
	description := lipgloss.NewStyle().Foreground(colors.Muted).Render("Comprehensive metadata from TMDB.")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("TMDB"),
		toggle,
		"API Key: "+apiKey,
		"Language: "+language,
		status,
		description,
	)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderTVDBColumn(colors theme.Colors) string {
	toggleFocused := p.state.Active == ProviderFieldTVDBToggle
	keyFocused := p.state.Active == ProviderFieldTVDBKey && p.state.TVDB.Enabled

	toggle := p.renderToggle("TVDB", p.state.TVDB.Enabled, toggleFocused, colors)

	apiKey := p.state.TVDB.APIKey.Value()
	if keyFocused {
		apiKey = p.state.TVDB.APIKey.View()
	} else {
		apiKey = p.state.TVDB.MaskedAPIKey(3, 3)
	}

	switch {
	case !p.state.TVDB.Enabled:
		apiKey = lipgloss.NewStyle().Foreground(colors.Muted).Render(apiKey + " (disabled)")
	case keyFocused:
		apiKey = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(apiKey)
	default:
		apiKey = lipgloss.NewStyle().Foreground(colors.Primary).Render(apiKey)
	}

	status := p.renderValidation("Status", p.state.TVDB.Validation.Status)
	description := lipgloss.NewStyle().Foreground(colors.Muted).Render("TV and movie metadata from TVDB.")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("TVDB"),
		toggle,
		"API Key: "+apiKey,
		status,
		description,
	)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderToggle(label string, enabled, focused bool, colors theme.Colors) string {
	icon := "[ ]"
	if enabled {
		icon = "[" + p.icons["check"] + "]"
	}
	text := icon + " " + label
	if focused {
		return lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(text)
	}
	style := lipgloss.NewStyle().Foreground(colors.Error)
	if enabled {
		style = lipgloss.NewStyle().Foreground(colors.Success)
	}
	return style.Render(text)
}

func (p *providerSection) renderValidation(label string, status ProviderValidationStatus) string {
	if status == ProviderValidationUnknown {
		return label + ": Not configured"
	}
	return label + ": " + status.String()
}
