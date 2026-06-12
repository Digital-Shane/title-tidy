package provider

import (
	"fmt"
	"strings"
)

// TemplateVariableValueSource identifies where a template variable value is read from.
type TemplateVariableValueSource string

const (
	TemplateVariableValueSourceContext  TemplateVariableValueSource = "context"
	TemplateVariableValueSourceCore     TemplateVariableValueSource = "core"
	TemplateVariableValueSourceExtended TemplateVariableValueSource = "extended"
	TemplateVariableValueSourceID       TemplateVariableValueSource = "id"
)

const (
	TemplateValueKeyTitle        = "title"
	TemplateValueKeyYear         = "year"
	TemplateValueKeySeason       = "season"
	TemplateValueKeyEpisode      = "episode"
	TemplateValueKeyEpisodeTitle = "episode_title"
	TemplateValueKeyOverview     = "overview"
	TemplateValueKeyRating       = "rating"
	TemplateValueKeyGenres       = "genres"
	TemplateValueKeyLanguage     = "language"
	TemplateValueKeyCountry      = "country"
)

// ResolveMetadataVariable resolves a provider-owned template variable from metadata.
func ResolveMetadataVariable(variable TemplateVariable, metadata *Metadata) (string, bool) {
	if metadata == nil {
		return "", false
	}
	key := variable.ValueKey
	if key == "" {
		key = variable.Name
	}

	switch variable.ValueSource {
	case TemplateVariableValueSourceCore:
		return coreMetadataValue(metadata.Core, key)
	case TemplateVariableValueSourceExtended:
		return mapValue(metadata.Extended, key)
	case TemplateVariableValueSourceID:
		value, ok := metadata.IDs[key]
		return value, ok && value != ""
	default:
		return "", false
	}
}

func coreMetadataValue(core CoreMetadata, key string) (string, bool) {
	switch key {
	case TemplateValueKeyTitle:
		return core.Title, core.Title != ""
	case TemplateValueKeyYear:
		return core.Year, core.Year != ""
	case TemplateValueKeySeason:
		if core.SeasonNum > 0 {
			return fmt.Sprintf("%02d", core.SeasonNum), true
		}
	case TemplateValueKeyEpisode:
		if core.EpisodeNum > 0 {
			return fmt.Sprintf("%02d", core.EpisodeNum), true
		}
	case TemplateValueKeyEpisodeTitle:
		return core.EpisodeName, core.EpisodeName != ""
	case TemplateValueKeyOverview:
		return core.Overview, core.Overview != ""
	case TemplateValueKeyRating:
		if core.Rating > 0 {
			return fmt.Sprintf("%.1f", core.Rating), true
		}
	case TemplateValueKeyGenres:
		if len(core.Genres) > 0 {
			return strings.Join(core.Genres, ", "), true
		}
	case TemplateValueKeyLanguage:
		return core.Language, core.Language != ""
	case TemplateValueKeyCountry:
		return core.Country, core.Country != ""
	}
	return "", false
}

func mapValue(values map[string]interface{}, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	value, ok := values[key]
	if !ok {
		return "", false
	}
	formatted := fmt.Sprint(value)
	return formatted, formatted != ""
}
