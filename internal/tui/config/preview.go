package config

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
)

type preview struct {
	icon    string
	label   string
	preview string
}

func buildPreviews(section Section, state *ConfigState, icons map[string]string, registry *config.TemplateRegistry) []preview {
	switch section {
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
		tmdbStatus := "Disabled"
		tmdbAPI := "Not configured"
		tmdbLang := state.Providers.TMDB.Language.Value()
		if state.Providers.TMDB.Enabled {
			tmdbStatus = "Enabled"
			tmdbAPI = validationLabel(state.Providers.TMDB.Validation)
			if tmdbAPI == "" {
				tmdbAPI = "Configured"
			}
		}
		omdbStatus := "Disabled"
		omdbAPI := "Not configured"
		if state.Providers.OMDB.Enabled {
			omdbStatus = "Enabled"
			omdbAPI = validationLabel(state.Providers.OMDB.Validation)
			if omdbAPI == "" {
				omdbAPI = "Configured"
			}
		}
		ffprobeStatus := "Disabled"
		if state.Providers.FFProbeEnabled {
			ffprobeStatus = "Enabled"
		}
		if tmdbLang == "" {
			tmdbLang = "en-US"
		}
		return []preview{
			{icons["chip"], "ffprobe", ffprobeStatus},
			{icons["film"], "OMDb Lookup", omdbStatus},
			{icons["key"], "OMDb API", omdbAPI},
			{icons["film"], "TMDB Lookup", tmdbStatus},
			{icons["key"], "TMDB API", tmdbAPI},
			{icons["globe"], "Language", tmdbLang},
		}
	}

	cfg := &config.FormatConfig{
		ShowFolder:   state.Templates.Show.Input.Value(),
		SeasonFolder: state.Templates.Season.Input.Value(),
		Episode:      state.Templates.Episode.Input.Value(),
		Movie:        state.Templates.Movie.Input.Value(),
	}

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
			"tagline":     "All Hail the King",
			"networks":    "AMC",
			"audio_codec": "aac",
			"video_codec": "264",
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
			"tagline":     "Welcome to the Real World",
			"studios":     "Warner Bros.",
			"audio_codec": "aac",
			"video_codec": "264",
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

	var showPreview, seasonPreview, episodePreview, moviePreview string
	if registry != nil {
		showPreview, _ = registry.ResolveTemplate(cfg.ShowFolder, showCtx, showMetadata)
		showPreview = local.CleanName(showPreview)
		seasonPreview, _ = registry.ResolveTemplate(cfg.SeasonFolder, seasonCtx, showMetadata)
		seasonPreview = local.CleanName(seasonPreview)
		episodePreview, _ = registry.ResolveTemplate(cfg.Episode, episodeCtx, showMetadata)
		episodePreview = local.CleanName(episodePreview) + ".mkv"
		moviePreview, _ = registry.ResolveTemplate(cfg.Movie, movieCtx, movieMetadata)
		moviePreview = local.CleanName(moviePreview)
	} else {
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
