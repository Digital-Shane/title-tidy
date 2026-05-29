package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type providerSection struct {
	state *ProviderState
	theme theme.Theme
	icons map[string]string
	width int

	validate func(providerName, fieldName, value string) tea.Cmd
	debounce func(providerName, fieldName, value string) tea.Cmd
}

func newProviderSection(state *ProviderState, th theme.Theme) *providerSection {
	return &providerSection{
		state:    state,
		theme:    th,
		icons:    th.IconSet(),
		validate: validateProviderAPIKey,
		debounce: debouncedProviderValidate,
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
	for i := range p.state.Providers {
		for j := range p.state.Providers[i].Fields {
			p.state.Providers[i].Fields[j].Input.Blur()
		}
	}
}

func (p *providerSection) Resize(width int) { p.width = width }

func (p *providerSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		if m.Mod.Contains(tea.ModAlt) {
			return p, nil
		}
		p.ensureActiveField()
		return p, p.handleKey(m)
	case providerValidateCmd:
		return p.handleProviderValidateCmd(m)
	case providerValidationMsg:
		return p.handleProviderValidationMsg(m)
	}
	return p, nil
}

func (p *providerSection) handleKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "left", "up":
		return p.moveFocus(-1)
	case "right", "down":
		return p.moveFocus(1)
	case "enter", "space":
		if key.String() == "space" && key.Mod.Contains(tea.ModAlt) {
			return nil
		}
		if cmd, handled := p.toggleActive(); handled {
			return cmd
		}
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

func (p *providerSection) focusOrder() []ProviderFocus {
	fields := []ProviderFocus{{Kind: ProviderFocusWorkers}}
	for _, providerState := range p.state.Providers {
		providerName := providerState.Name()
		fields = append(fields, ProviderFocus{Kind: ProviderFocusToggle, ProviderName: providerName})
		if !providerState.Enabled {
			continue
		}
		for _, field := range providerState.Fields {
			fields = append(fields, ProviderFocus{
				Kind:         ProviderFocusField,
				ProviderName: providerName,
				FieldName:    field.Schema.Name,
			})
		}
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

func (p *providerSection) toggleActive() (tea.Cmd, bool) {
	switch p.state.Active.Kind {
	case ProviderFocusToggle:
		providerState := p.providerState(p.state.Active.ProviderName)
		if providerState == nil {
			return nil, true
		}
		providerState.Enabled = !providerState.Enabled
		if !providerState.Enabled {
			providerState.Validation.Reset()
			p.ensureActiveField()
			return p.applyFocus(), true
		}
		return tea.Batch(p.applyFocus(), p.queueValidation(providerState)), true
	case ProviderFocusField:
		providerState := p.providerState(p.state.Active.ProviderName)
		fieldState := p.activeField()
		if providerState == nil || fieldState == nil || !providerState.Enabled {
			return nil, false
		}
		if fieldState.Schema.Type != provider.ConfigFieldTypeBool {
			return nil, false
		}
		fieldState.Input.SetValue(toggleBoolString(fieldState.Input.Value()))
		fieldState.Input.CursorEnd()
		return nil, true
	}
	return nil, false
}

func (p *providerSection) applyFocus() tea.Cmd {
	p.state.WorkerCount.Blur()
	for i := range p.state.Providers {
		for j := range p.state.Providers[i].Fields {
			p.state.Providers[i].Fields[j].Input.Blur()
		}
	}

	if p.state.Active.Kind == ProviderFocusWorkers {
		return p.state.WorkerCount.Focus()
	}

	providerState := p.providerState(p.state.Active.ProviderName)
	fieldState := p.activeField()
	if providerState != nil && providerState.Enabled && fieldState != nil {
		return fieldState.Input.Focus()
	}
	return nil
}

func (p *providerSection) handleTextInputs(key tea.KeyPressMsg) (tea.Cmd, bool) {
	if p.state.Active.Kind == ProviderFocusWorkers {
		if key.String() == "space" {
			return nil, true
		}
		if key.Text != "" {
			digits := make([]rune, 0, len(key.Text))
			for _, r := range key.Text {
				if unicode.IsDigit(r) {
					digits = append(digits, r)
				}
			}
			if len(digits) == 0 {
				return nil, true
			}
			key = tea.KeyPressMsg{Code: digits[0], Text: string(digits)}
		}
		prev := p.state.WorkerCount.Value()
		var cmd tea.Cmd
		p.state.WorkerCount, cmd = p.state.WorkerCount.Update(key)
		if prev != p.state.WorkerCount.Value() {
			return cmd, true
		}
		return cmd, true
	}

	if p.state.Active.Kind != ProviderFocusField {
		return nil, false
	}
	providerState := p.providerState(p.state.Active.ProviderName)
	fieldState := p.activeField()
	if providerState == nil || fieldState == nil || !providerState.Enabled {
		return nil, false
	}
	if fieldState.Schema.Type == provider.ConfigFieldTypeBool {
		return nil, false
	}
	if key.String() == "space" && fieldState.Schema.Type == provider.ConfigFieldTypePassword {
		return nil, true
	}
	if key.Text != "" {
		filtered := filterProviderInput(fieldState.Schema, key.Text)
		if filtered == "" {
			return nil, true
		}
		key = tea.KeyPressMsg{Code: []rune(filtered)[0], Text: filtered}
	}

	prev := fieldState.Input.Value()
	var cmd tea.Cmd
	fieldState.Input, cmd = fieldState.Input.Update(key)
	if prev != fieldState.Input.Value() && isValidationField(fieldState.Schema) {
		if debounced := p.queueValidation(providerState); debounced != nil {
			cmd = tea.Batch(cmd, debounced)
		}
	}
	return cmd, true
}

func (p *providerSection) queueValidation(providerState *ProviderServiceState) tea.Cmd {
	if providerState == nil || !providerState.Enabled {
		return nil
	}
	fieldState := providerState.RequiredPasswordField()
	if fieldState == nil {
		return nil
	}
	value := strings.TrimSpace(fieldState.Input.Value())
	providerState.Validation.Reset()
	if value == "" {
		return nil
	}
	return p.debounce(providerState.Name(), fieldState.Schema.Name, value)
}

func (p *providerSection) handleProviderValidateCmd(cmd providerValidateCmd) (tea.Model, tea.Cmd) {
	providerState := p.providerState(cmd.providerName)
	fieldState := p.providerField(cmd.providerName, cmd.fieldName)
	if providerState == nil || fieldState == nil {
		return p, nil
	}
	value := strings.TrimSpace(fieldState.Input.Value())
	if cmd.value == "" || cmd.value != value {
		return p, nil
	}
	if cmd.value == providerState.Validation.LastValidated {
		return p, nil
	}
	providerState.Validation.Status = ProviderValidationValidating
	return p, p.validate(cmd.providerName, cmd.fieldName, cmd.value)
}

func (p *providerSection) handleProviderValidationMsg(msg providerValidationMsg) (tea.Model, tea.Cmd) {
	providerState := p.providerState(msg.providerName)
	fieldState := p.providerField(msg.providerName, msg.fieldName)
	if providerState == nil || fieldState == nil {
		return p, nil
	}
	value := strings.TrimSpace(fieldState.Input.Value())
	if msg.value != value {
		return p, nil
	}
	if msg.valid {
		providerState.Validation.Status = ProviderValidationValid
	} else {
		providerState.Validation.Status = ProviderValidationInvalid
	}
	providerState.Validation.LastValidated = msg.value
	return p, nil
}

func (p *providerSection) Activate() tea.Cmd {
	var cmds []tea.Cmd
	for i := range p.state.Providers {
		providerState := &p.state.Providers[i]
		if cmd := p.validateOnActivate(providerState); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (p *providerSection) validateOnActivate(providerState *ProviderServiceState) tea.Cmd {
	if providerState == nil || !providerState.Enabled {
		return nil
	}
	fieldState := providerState.RequiredPasswordField()
	if fieldState == nil {
		return nil
	}
	value := strings.TrimSpace(fieldState.Input.Value())
	if value == "" || value == providerState.Validation.LastValidated {
		return nil
	}
	providerState.Validation.Status = ProviderValidationValidating
	return p.validate(providerState.Name(), fieldState.Schema.Name, value)
}

func (p *providerSection) View() tea.View {
	colors := p.theme.Colors()
	title := p.theme.PanelTitleStyle().Render("Metadata Providers")

	columns := []string{p.renderSharedColumn(colors)}
	for i := range p.state.Providers {
		columns = append(columns, p.renderProviderColumn(&p.state.Providers[i], colors))
	}

	columnGap := 2
	minColumnWidth := 22
	totalGap := columnGap * max(len(columns)-1, 0)
	inline := p.width-totalGap >= minColumnWidth*len(columns)

	if inline {
		gap := lipgloss.NewStyle().Width(columnGap).Render(" ")
		rowParts := make([]string, 0, len(columns)*2-1)
		for i, column := range columns {
			if i > 0 {
				rowParts = append(rowParts, gap)
			}
			rowParts = append(rowParts, column)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, rowParts...)
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, row))
	}

	separator := lipgloss.NewStyle().Height(1).Render("")
	stackParts := make([]string, 0, len(columns)*2-1)
	for i, column := range columns {
		if i > 0 {
			stackParts = append(stackParts, separator)
		}
		stackParts = append(stackParts, column)
	}
	stacked := lipgloss.JoinVertical(lipgloss.Left, stackParts...)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, stacked))
}

func (p *providerSection) renderSharedColumn(colors theme.Colors) string {
	focused := p.state.Active.Kind == ProviderFocusWorkers
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

func (p *providerSection) renderProviderColumn(providerState *ProviderServiceState, colors theme.Colors) string {
	if providerState == nil {
		return ""
	}
	providerName := providerState.Name()
	displayName := providerState.DisplayName()
	toggleFocused := p.state.Active == ProviderFocus{Kind: ProviderFocusToggle, ProviderName: providerName}
	toggle := p.renderToggle(displayName, providerState.Enabled, toggleFocused, colors)

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(displayName),
		toggle,
	}
	for i := range providerState.Fields {
		lines = append(lines, p.renderProviderField(providerState, &providerState.Fields[i], colors))
	}
	if providerState.RequiredPasswordField() != nil {
		lines = append(lines, p.renderValidation("Status", providerState.Validation.Status))
	}
	if description := providerState.Description(); description != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(colors.Muted).Render(description))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.NewStyle().Width(22).Render(content)
}

func (p *providerSection) renderProviderField(providerState *ProviderServiceState, fieldState *ProviderFieldState, colors theme.Colors) string {
	focused := p.state.Active == ProviderFocus{
		Kind:         ProviderFocusField,
		ProviderName: providerState.Name(),
		FieldName:    fieldState.Schema.Name,
	} && providerState.Enabled

	value := fieldState.Input.Value()
	if focused {
		value = fieldState.Input.View()
	} else if fieldState.Schema.Sensitive {
		value = maskAPIKeyVisible(value, 3, 3)
	}
	if value == "" {
		value = "-"
	}

	switch {
	case !providerState.Enabled:
		value = lipgloss.NewStyle().Foreground(colors.Muted).Render(value + " (disabled)")
	case focused:
		value = lipgloss.NewStyle().
			Background(colors.Accent).
			Foreground(colors.Background).
			Render(value)
	default:
		value = lipgloss.NewStyle().Foreground(colors.Primary).Render(value)
	}

	return fmt.Sprintf("%s: %s", fieldLabel(fieldState.Schema), value)
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

func (p *providerSection) providerState(name string) *ProviderServiceState {
	for i := range p.state.Providers {
		if p.state.Providers[i].Name() == name {
			return &p.state.Providers[i]
		}
	}
	return nil
}

func (p *providerSection) providerField(providerName, fieldName string) *ProviderFieldState {
	providerState := p.providerState(providerName)
	if providerState == nil {
		return nil
	}
	return providerState.Field(fieldName)
}

func (p *providerSection) activeField() *ProviderFieldState {
	if p.state.Active.Kind != ProviderFocusField {
		return nil
	}
	return p.providerField(p.state.Active.ProviderName, p.state.Active.FieldName)
}

func isValidationField(field provider.ConfigField) bool {
	return field.Required && field.Type == provider.ConfigFieldTypePassword
}

func filterProviderInput(field provider.ConfigField, text string) string {
	filtered := make([]rune, 0, len(text))
	for _, r := range text {
		switch field.Type {
		case provider.ConfigFieldTypeInt:
			if unicode.IsDigit(r) {
				filtered = append(filtered, r)
			}
		case provider.ConfigFieldTypeSelect:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
				filtered = append(filtered, r)
			}
		default:
			if r != ' ' && !unicode.IsControl(r) {
				filtered = append(filtered, r)
			}
		}
	}
	return string(filtered)
}

func fieldLabel(field provider.ConfigField) string {
	if strings.TrimSpace(field.DisplayName) != "" {
		return field.DisplayName
	}
	return strings.ReplaceAll(field.Name, "_", " ")
}

func toggleBoolString(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "true") {
		return "false"
	}
	return "true"
}
