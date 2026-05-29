package provider_test

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tvdb"
)

func TestProviderVariablesMatchReadmeContract(t *testing.T) {
	tests := []struct {
		name      string
		provider  provider.Provider
		variables []string
	}{
		{
			name:     "tmdb",
			provider: tmdb.New(),
			variables: []string{
				"title",
				"episode_title",
				"air_date",
				"rating",
				"genres",
				"runtime",
				"tagline",
				"imdb_id",
				"networks",
			},
		},
		{
			name:     "omdb",
			provider: omdb.New(),
			variables: []string{
				"title",
				"episode_title",
				"rating",
				"genres",
				"imdb_id",
				"networks",
			},
		},
		{
			name:     "tvdb",
			provider: tvdb.New(),
			variables: []string{
				"title",
				"episode_title",
				"rating",
				"genres",
				"imdb_id",
				"networks",
			},
		},
		{
			name:     "ffprobe",
			provider: ffprobe.New(),
			variables: []string{
				"video_codec",
				"video_resolution",
				"audio_codec",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := variableSet(tt.provider.SupportedVariables())
			for _, want := range tt.variables {
				if !got[want] {
					t.Fatalf("%s SupportedVariables() missing %q", tt.name, want)
				}
			}
		})
	}
}

func TestLocalProviderVariablesExposeUnifiedTitleOnly(t *testing.T) {
	got := variableSet(local.New().SupportedVariables())
	for _, want := range []string{"title", "year", "season", "episode"} {
		if !got[want] {
			t.Fatalf("local SupportedVariables() missing %q", want)
		}
	}
	for _, legacy := range []string{"show", "movie"} {
		if got[legacy] {
			t.Fatalf("local SupportedVariables() includes legacy alias %q", legacy)
		}
	}
	if len(got) != 4 {
		t.Fatalf("local SupportedVariables() count = %d, want 4", len(got))
	}
}

func TestProviderVariablesDeclareValueSource(t *testing.T) {
	providers := []provider.Provider{
		local.New(),
		tmdb.New(),
		tvdb.New(),
		omdb.New(),
		ffprobe.New(),
	}

	for _, p := range providers {
		t.Run(p.Name(), func(t *testing.T) {
			for _, variable := range p.SupportedVariables() {
				if variable.ValueSource == "" {
					t.Fatalf("%s variable %q missing ValueSource", p.Name(), variable.Name)
				}
				if variable.ValueKey == "" {
					t.Fatalf("%s variable %q missing ValueKey", p.Name(), variable.Name)
				}
			}
		})
	}
}

func variableSet(vars []provider.TemplateVariable) map[string]bool {
	result := make(map[string]bool, len(vars))
	for _, v := range vars {
		result[v.Name] = true
	}
	return result
}
