package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

type variable struct {
	name        string
	description string
	example     string
}

func buildVariables(section Section, state *ConfigState, registry *provider.Registry) []variable {
	switch section {
	case SectionRename:
		return []variable{
			{"↑/↓ arrows", "Select a rename setting", ""},
			{"Space/Enter", "Toggle tag preservation", "Enter advances between replacement fields"},
			{"Ctrl+N", "Add a replacement pair", `For example, ":" replaced by " - "`},
			{"Ctrl+D", "Remove the selected pair", ""},
			{"Replacements", "Literal, case-sensitive matches", "Longest match first; empty values delete matches"},
		}
	case SectionLogging:
		return []variable{
			{"Space/Enter", "Toggle logging on/off", ""},
			{"↑/↓ arrows", "Switch between toggle and retention", ""},
			{"Retention", "Auto-cleanup old logs", "Days to keep log files"},
		}
	case SectionProviders:
		result := []variable{
			{"←/→ arrows", "Switch between provider columns", ""},
			{"↑/↓ arrows", "Navigate settings in the active column", ""},
			{"Space/Enter", "Toggle highlighted setting", ""},
		}
		for _, p := range config.ProvidersByDisplayOrder(registry) {
			caps := p.Capabilities()
			if caps.Local {
				continue
			}

			displayName := config.ProviderDisplayName(p)
			schema := p.ConfigSchema()
			if len(schema.Fields) == 0 {
				result = append(result, variable{
					name:        displayName,
					description: p.Description(),
					example:     providerVariableExample(p),
				})
				continue
			}

			for _, field := range schema.Fields {
				result = append(result, variable{
					name:        displayName + " " + field.DisplayName,
					description: field.Description,
					example:     configFieldExample(field),
				})
			}
		}
		return result
	}

	mediaType, ok := sectionMediaType(section)
	if ok {
		vars := config.TemplateVariablesForMediaType(registry, mediaType)
		result := make([]variable, 0, len(vars))
		for _, v := range vars {
			if !providerEnabledForVariable(registry, v.Name, state) {
				continue
			}
			name := "{" + v.Name + "}"
			result = append(result, variable{name: name, description: v.Description, example: v.Example})
		}
		if len(result) > 0 {
			sort.SliceStable(result, func(i, j int) bool {
				pi := variableProviderPriority(registry, result[i].name)
				pj := variableProviderPriority(registry, result[j].name)
				if pi != pj {
					return pi < pj
				}
				return result[i].name < result[j].name
			})
			return result
		}
	}

	return nil
}

func sectionMediaType(section Section) (provider.MediaType, bool) {
	switch section {
	case SectionShowFolder:
		return provider.MediaTypeShow, true
	case SectionSeasonFolder:
		return provider.MediaTypeSeason, true
	case SectionEpisode:
		return provider.MediaTypeEpisode, true
	case SectionMovie:
		return provider.MediaTypeMovie, true
	default:
		return provider.MediaType(""), false
	}
}

func providerEnabledForVariable(reg *provider.Registry, variableName string, state *ConfigState) bool {
	owners := config.TemplateVariableProviders(reg, variableName)
	if len(owners) == 0 {
		return true
	}
	for _, p := range owners {
		if p.Capabilities().Local {
			return true
		}
		if state == nil {
			continue
		}
		providerState := state.Providers.Provider(p.Name())
		if providerState != nil && providerState.Enabled {
			return true
		}
	}
	return false
}

func variableProviderPriority(reg *provider.Registry, name string) int {
	if reg == nil {
		return 0
	}
	trimmed := strings.TrimPrefix(strings.TrimSuffix(name, "}"), "{")
	return config.TemplateVariableDisplayOrder(reg, trimmed)
}

func configFieldExample(field provider.ConfigField) string {
	if field.Validation != nil {
		if len(field.Validation.Options) > 0 {
			values := make([]string, 0, min(3, len(field.Validation.Options)))
			for idx, option := range field.Validation.Options {
				if idx >= 3 {
					break
				}
				values = append(values, option.Value)
			}
			if len(field.Validation.Options) > len(values) {
				return strings.Join(values, ", ") + ", etc."
			}
			return strings.Join(values, ", ")
		}
		if field.Validation.MinLength > 0 {
			if field.Validation.MaxLength == field.Validation.MinLength {
				return fmt.Sprintf("%d characters", field.Validation.MinLength)
			}
			return fmt.Sprintf("%d+ characters", field.Validation.MinLength)
		}
		if field.Validation.Pattern != "" {
			return field.Validation.Pattern
		}
	}
	if field.Default != nil {
		return fmt.Sprintf("%v", field.Default)
	}
	return ""
}

func providerVariableExample(p provider.Provider) string {
	vars := p.SupportedVariables()
	if len(vars) == 0 {
		return ""
	}
	names := make([]string, 0, min(3, len(vars)))
	for idx, v := range vars {
		if idx >= 3 {
			break
		}
		names = append(names, "{"+v.Name+"}")
	}
	return "Adds " + strings.Join(names, ", ")
}
