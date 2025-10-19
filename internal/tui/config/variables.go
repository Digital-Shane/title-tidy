package config

import (
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

func buildVariables(section Section, state *ConfigState, registry *config.TemplateRegistry) []variable {
	switch section {
	case SectionLogging:
		return []variable{
			{"Space/Enter", "Toggle logging on/off", ""},
			{"↑/↓ arrows", "Switch between toggle and retention", ""},
			{"Retention", "Auto-cleanup old logs", "Days to keep log files"},
		}
	case SectionProviders:
		return []variable{
			{"←/→ arrows", "Switch between provider columns", ""},
			{"↑/↓ arrows", "Navigate settings in the active column", ""},
			{"Space/Enter", "Toggle highlighted setting", ""},
			{"OMDb API Key", "Create at omdbapi.com/apikey.aspx", "8+ characters"},
			{"TMDB API Key", "Generate from the TMDB web console", "32 hex characters"},
			{"TMDB Language", "Preferred metadata language code", "en-US, fr-FR, etc."},
			{"ffprobe", "Enable codec metadata extraction", "Adds audio/video codec and resolution variables"},
		}
	}

	if registry != nil {
		mediaType, ok := sectionMediaType(section)
		if ok {
			vars := registry.GetVariablesForMediaType(mediaType)
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
	}

	return fallbackVariables(section)
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

func providerEnabledForVariable(reg *config.TemplateRegistry, variableName string, state *ConfigState) bool {
	owners := reg.VariableProviders(variableName)
	if len(owners) == 0 {
		return true
	}
	for _, owner := range owners {
		switch owner {
		case "tmdb":
			if state.Providers.TMDB.Enabled {
				return true
			}
		case "omdb":
			if state.Providers.OMDB.Enabled {
				return true
			}
		case "ffprobe":
			if state.Providers.FFProbeEnabled {
				return true
			}
		default:
			return true
		}
	}
	return false
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

func fallbackVariables(section Section) []variable {
	switch section {
	case SectionShowFolder:
		return []variable{
			{"{title}", "Show title", "Breaking Bad"},
			{"{year}", "Year", "2008"},
			{"{rating}", "Average rating", "8.5"},
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
			{"{rating}", "Episode rating", "8.3"},
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
