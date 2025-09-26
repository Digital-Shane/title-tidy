package config

import (
	"context"
	"errors"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/charmbracelet/bubbletea"
)

type tmdbValidationMsg struct {
	apiKey string
	valid  bool
}

type tmdbValidateCmd struct {
	apiKey string
}

type omdbValidationMsg struct {
	apiKey string
	valid  bool
}

type omdbValidateCmd struct {
	apiKey string
}

func validateTMDBAPIKey(apiKey string) tea.Cmd {
	return func() tea.Msg {
		if apiKey == "" {
			return tmdbValidationMsg{apiKey: "", valid: false}
		}

		prov := tmdb.New()
		cfg := map[string]interface{}{
			"api_key":       apiKey,
			"language":      "en-US",
			"cache_enabled": false,
		}
		if err := prov.Configure(cfg); err != nil {
			return tmdbValidationMsg{apiKey: apiKey, valid: false}
		}

		req := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      "The Matrix",
			Year:      "1999",
		}
		if _, err := prov.Fetch(context.Background(), req); err != nil {
			var provErr *provider.ProviderError
			if errors.As(err, &provErr) && provErr.Code == "AUTH_FAILED" {
				return tmdbValidationMsg{apiKey: apiKey, valid: false}
			}
			return tmdbValidationMsg{apiKey: apiKey, valid: false}
		}

		return tmdbValidationMsg{apiKey: apiKey, valid: true}
	}
}

func debouncedTMDBValidate(apiKey string) tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return tmdbValidateCmd{apiKey: apiKey}
	})
}

func validateOMDBAPIKey(apiKey string) tea.Cmd {
	return func() tea.Msg {
		if apiKey == "" {
			return omdbValidationMsg{apiKey: "", valid: false}
		}

		prov := omdb.New()
		if err := prov.Configure(map[string]interface{}{"api_key": apiKey}); err != nil {
			return omdbValidationMsg{apiKey: apiKey, valid: false}
		}

		req := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      "The Matrix",
			Year:      "1999",
		}
		meta, err := prov.Fetch(context.Background(), req)
		if err != nil || meta == nil {
			return omdbValidationMsg{apiKey: apiKey, valid: false}
		}
		return omdbValidationMsg{apiKey: apiKey, valid: true}
	}
}

func debouncedOMDBValidate(apiKey string) tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return omdbValidateCmd{apiKey: apiKey}
	})
}
