package config

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	appconfig "github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
)

type providerValidationMsg struct {
	providerName string
	fieldName    string
	value        string
	valid        bool
}

type providerValidateCmd struct {
	providerName string
	fieldName    string
	value        string
}

func validateProviderAPIKey(providerName, fieldName, apiKey string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(apiKey) == "" {
			return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
		}
		if err := appconfig.EnsureBuiltinProviders(); err != nil {
			return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
		}

		prov, ok := provider.GlobalRegistry.Get(providerName)
		if !ok {
			return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
		}
		if err := prov.Configure(validationConfig(prov, apiKey)); err != nil {
			return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
		}

		req := provider.FetchRequest{
			MediaType: provider.MediaTypeMovie,
			Name:      "The Matrix",
			Year:      "1999",
		}
		meta, err := prov.Fetch(context.Background(), req)
		if err != nil || meta == nil {
			var provErr *provider.ProviderError
			if errors.As(err, &provErr) && provErr.Code == "AUTH_FAILED" {
				return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
			}
			return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: false}
		}

		return providerValidationMsg{providerName: providerName, fieldName: fieldName, value: apiKey, valid: true}
	}
}

func debouncedProviderValidate(providerName, fieldName, value string) tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return providerValidateCmd{providerName: providerName, fieldName: fieldName, value: value}
	})
}

func validationConfig(prov provider.Provider, apiKey string) map[string]interface{} {
	cfg := make(map[string]interface{})
	for _, field := range prov.ConfigSchema().Fields {
		switch {
		case field.Type == provider.ConfigFieldTypePassword && field.Required:
			cfg[field.Name] = apiKey
		case field.Type == provider.ConfigFieldTypeBool:
			cfg[field.Name] = false
		case field.Default != nil:
			cfg[field.Name] = field.Default
		}
	}
	return cfg
}
