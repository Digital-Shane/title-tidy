package config

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

type preview struct {
	icon    string
	label   string
	preview string
}

func buildPreviews(section Section, state *ConfigState, icons map[string]string, registry *provider.Registry) []preview {
	switch section {
	case SectionRename:
		status := "Disabled"
		if state.Rename.PreserveExistingTags {
			status = "Enabled"
		}
		previews := []preview{
			{icons["check"], "Preserve Existing Tags", status},
		}
		replacements, err := state.Rename.filenameReplacements()
		if err != nil {
			return append(previews, preview{icons["document"], "Replacements", err.Error()})
		}
		status = "Disabled"
		if len(replacements) > 0 {
			status = fmt.Sprintf("%d pairs", len(replacements))
		}
		cfg := &config.FormatConfig{FilenameReplacements: replacements}
		example := cfg.ApplyFilenameReplacements("Diary of a Wimpy Kid: Dog Days (2012) [544p]")
		if sanitized, err := core.SanitizeFilename(example); err == nil {
			example = sanitized
		}
		return append(previews,
			preview{icons["check"], "Replacements", status},
			preview{icons["movie"], "Example", example},
		)

	case SectionLogging:
		status := "Disabled"
		if state.Logging.Enabled {
			status = "Enabled"
		}
		retention := state.Logging.Retention.Value()
		if retention == "" {
			retention = "Default"
		}
		return []preview{
			{icons["check"], "Logging", status},
			{icons["calendar"], "Retention", fmt.Sprintf("%s days", retention)},
			{icons["folder"], "Log Location", "~/.title-tidy/logs/"},
			{icons["document"], "Log Format", "JSON session files"},
		}

	case SectionProviders:
		return buildProviderPreviews(state, icons)
	}

	cfg := &config.FormatConfig{
		ShowFolder:   state.Templates.Show.Input.Value(),
		SeasonFolder: state.Templates.Season.Input.Value(),
		Episode:      state.Templates.Episode.Input.Value(),
		Movie:        state.Templates.Movie.Input.Value(),
	}
	cfg.FilenameReplacements, _ = state.Rename.filenameReplacements()

	showMetadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       "Breaking Bad",
			Year:        "2008",
			Rating:      8.5,
			Genres:      []string{"Drama", "Crime"},
			EpisodeName: "Gray Matter",
		},
		IDs: map[string]string{"imdb_id": "tt0903747"},
		Extended: map[string]interface{}{
			"tagline":          "All Hail the King",
			"networks":         "AMC",
			"audio_codec":      "aac",
			"video_codec":      "264",
			"video_resolution": "1080p",
		},
	}

	movieMetadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:  "The Matrix",
			Year:   "1999",
			Rating: 8.7,
			Genres: []string{"Action", "Sci-Fi"},
		},
		IDs: map[string]string{"imdb_id": "tt0133093"},
		Extended: map[string]interface{}{
			"tagline":          "Welcome to the Real World",
			"studios":          "Warner Bros.",
			"audio_codec":      "aac",
			"video_codec":      "264",
			"video_resolution": "2160p",
		},
	}

	showCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Year:     "2008",
		Metadata: showMetadata,
		Config:   cfg,
	}

	seasonCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Season:   1,
		Metadata: showMetadata,
		Config:   cfg,
	}

	episodeCtx := &config.FormatContext{
		ShowName: "Breaking Bad",
		Season:   1,
		Episode:  5,
		Metadata: showMetadata,
		Config:   cfg,
	}

	movieCtx := &config.FormatContext{
		MovieName: "The Matrix",
		Year:      "1999",
		Metadata:  movieMetadata,
		Config:    cfg,
	}

	showPreview, _ := config.ResolveTemplate(cfg.ShowFolder, showCtx, showMetadata, registry)
	seasonPreview, _ := config.ResolveTemplate(cfg.SeasonFolder, seasonCtx, showMetadata, registry)
	episodePreview, _ := config.ResolveTemplate(cfg.Episode, episodeCtx, showMetadata, registry)
	moviePreview, _ := config.ResolveTemplate(cfg.Movie, movieCtx, movieMetadata, registry)
	showPreview = cfg.ApplyFilenameReplacements(showPreview)
	seasonPreview = cfg.ApplyFilenameReplacements(seasonPreview)
	episodePreview = cfg.ApplyFilenameReplacements(episodePreview)
	moviePreview = cfg.ApplyFilenameReplacements(moviePreview)
	episodePreview += ".mkv"

	return []preview{
		{icons["title"], "Show", showPreview},
		{icons["folder"], "Season", seasonPreview},
		{icons["episode"], "Episode", episodePreview},
		{icons["movie"], "Movie", moviePreview},
	}
}

func buildProviderPreviews(state *ConfigState, icons map[string]string) []preview {
	if state == nil {
		return nil
	}
	previews := make([]preview, 0)
	for _, providerState := range state.Providers.Providers {
		status := "Disabled"
		if providerState.Enabled {
			status = "Enabled"
		}
		previews = append(previews, preview{providerIcon(providerState, icons), providerState.DisplayName(), status})
		for _, fieldState := range providerState.Fields {
			previews = append(previews, providerFieldPreview(providerState, fieldState, icons))
		}
	}
	return previews
}

func providerFieldPreview(providerState ProviderServiceState, fieldState ProviderFieldState, icons map[string]string) preview {
	label := fieldPreviewLabel(providerState, fieldState.Schema)
	icon := providerFieldIcon(fieldState.Schema, icons)
	value := fieldState.Input.Value()
	if fieldState.Schema.Sensitive {
		value = "Not configured"
		if providerState.Enabled {
			value = validationLabel(providerState.Validation)
			if value == "" {
				value = "Configured"
			}
		}
	} else if fieldState.Schema.Default != nil {
		if value == "" {
			value = fmt.Sprint(fieldState.Schema.Default)
		}
	}
	return preview{icon, label, value}
}

func fieldPreviewLabel(providerState ProviderServiceState, field provider.ConfigField) string {
	if field.PreviewLabel != "" {
		return field.PreviewLabel
	}
	if field.Sensitive {
		return providerState.DisplayName() + " " + fieldLabel(field)
	}
	return fieldLabel(field)
}

func providerFieldIcon(field provider.ConfigField, icons map[string]string) string {
	if field.Icon != "" {
		if icon := icons[field.Icon]; icon != "" {
			return icon
		}
	}
	if field.Sensitive {
		return icons["key"]
	}
	return icons["document"]
}

func providerIcon(providerState ProviderServiceState, icons map[string]string) string {
	if providerState.Icon() != "" {
		if icon := icons[providerState.Icon()]; icon != "" {
			return icon
		}
	}
	return icons["film"]
}

func validationLabel(v ProviderValidationState) string {
	switch v.Status {
	case ProviderValidationValidating:
		return "Validating..."
	case ProviderValidationValid:
		return "Valid"
	case ProviderValidationInvalid:
		return "Invalid"
	default:
		if v.LastValidated != "" {
			return "Valid"
		}
		return ""
	}
}
