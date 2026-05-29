package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	providerInit "github.com/Digital-Shane/title-tidy/internal/provider/init"
)

// EnsureBuiltinProviders makes the built-in providers available through the
// provider registry. The loader is idempotent, so callers can use this before
// reading provider facts without coordinating initialization order.
func EnsureBuiltinProviders() error {
	return providerInit.LoadBuiltinProviders()
}

// ProvidersByDisplayOrder returns registered providers sorted for UI display.
func ProvidersByDisplayOrder(reg *provider.Registry) []provider.Provider {
	reg = providerRegistryOrGlobal(reg)
	providers := reg.Providers()
	sort.SliceStable(providers, func(i, j int) bool {
		ci := providers[i].Capabilities()
		cj := providers[j].Capabilities()
		if ci.DisplayOrder != cj.DisplayOrder {
			return ci.DisplayOrder < cj.DisplayOrder
		}
		if ci.Priority != cj.Priority {
			return ci.Priority > cj.Priority
		}
		return providers[i].Name() < providers[j].Name()
	})
	return providers
}

// ProviderDisplayName returns the provider-owned display name, falling back to
// the provider's stable registry name.
func ProviderDisplayName(p provider.Provider) string {
	if p == nil {
		return ""
	}
	if displayName := strings.TrimSpace(p.Capabilities().DisplayName); displayName != "" {
		return displayName
	}
	return p.Name()
}

// NormalizeProviderConfigs keeps only user-owned provider configuration values.
// Provider schema defaults are applied at runtime by ProviderConfigValues.
func NormalizeProviderConfigs(values map[string]ProviderConfig, reg *provider.Registry) map[string]ProviderConfig {
	reg = providerRegistryOrGlobal(reg)
	normalized := make(map[string]ProviderConfig)
	for name, userConfig := range values {
		p, ok := reg.Get(name)
		if !ok || p == nil || p.Capabilities().Local {
			continue
		}

		cfg := ProviderConfig{Enabled: userConfig.Enabled}
		overrides := providerConfigOverrides(p, userConfig.Config)
		if len(overrides) > 0 {
			cfg.Config = overrides
		}
		if cfg.Enabled || len(cfg.Config) > 0 {
			normalized[name] = cfg
		}
	}
	return normalized
}

// Provider returns stored user config for a provider name.
func (cfg *FormatConfig) Provider(name string) ProviderConfig {
	if cfg == nil || cfg.Providers == nil {
		return ProviderConfig{}
	}
	if value, ok := cfg.Providers[name]; ok {
		return value
	}
	return ProviderConfig{}
}

// SetProviderEnabled updates the enabled flag for one provider.
func (cfg *FormatConfig) SetProviderEnabled(name string, enabled bool) {
	if cfg == nil {
		return
	}
	cfg.ensureProviderMap()
	provCfg := cfg.Provider(name)
	provCfg.Enabled = enabled
	cfg.Providers[name] = provCfg
}

// SetProviderValue updates one provider config field.
func (cfg *FormatConfig) SetProviderValue(providerName, fieldName string, value interface{}) {
	if cfg == nil {
		return
	}
	cfg.ensureProviderMap()
	provCfg := cfg.Provider(providerName)
	if provCfg.Config == nil {
		provCfg.Config = make(map[string]interface{})
	}
	provCfg.Config[fieldName] = value
	cfg.Providers[providerName] = provCfg
}

func (cfg *FormatConfig) applyLegacyProviderConfig(
	tmdbAPIKey string,
	enableTMDBLookup *bool,
	tmdbLanguage string,
	omdbAPIKey string,
	enableOMDBLookup *bool,
	tvdbAPIKey string,
	enableTVDBLookup *bool,
	enableFFProbe *bool,
) {
	if tmdbAPIKey != "" {
		cfg.SetProviderValue("tmdb", "api_key", tmdbAPIKey)
	}
	if tmdbLanguage != "" {
		cfg.SetProviderValue("tmdb", "language", tmdbLanguage)
	}
	if enableTMDBLookup != nil {
		cfg.SetProviderEnabled("tmdb", *enableTMDBLookup)
	}

	if omdbAPIKey != "" {
		cfg.SetProviderValue("omdb", "api_key", omdbAPIKey)
	}
	if enableOMDBLookup != nil {
		cfg.SetProviderEnabled("omdb", *enableOMDBLookup)
	}

	if tvdbAPIKey != "" {
		cfg.SetProviderValue("tvdb", "api_key", tvdbAPIKey)
	}
	if enableTVDBLookup != nil {
		cfg.SetProviderEnabled("tvdb", *enableTVDBLookup)
	}

	if enableFFProbe != nil {
		cfg.SetProviderEnabled("ffprobe", *enableFFProbe)
	}
}

func (cfg *FormatConfig) ensureProviderMap() {
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
}

// ProviderEnabled reports whether this app config enables the provider.
func (cfg *FormatConfig) ProviderEnabled(p provider.Provider) bool {
	if cfg == nil || p == nil {
		return false
	}
	if p.Capabilities().Local {
		return true
	}
	return cfg.Provider(p.Name()).Enabled
}

// ProviderConfigValues returns provider schema field values from app config,
// using provider defaults for fields not persisted by the app config.
func (cfg *FormatConfig) ProviderConfigValues(p provider.Provider) map[string]interface{} {
	values := make(map[string]interface{})
	if cfg == nil || p == nil {
		return values
	}

	provCfg := cfg.Provider(p.Name())
	for _, field := range p.ConfigSchema().Fields {
		if value, ok := provCfg.Config[field.Name]; ok {
			values[field.Name] = value
			continue
		}
		if field.Default != nil {
			values[field.Name] = field.Default
		}
	}
	return values
}

// ProviderRuntimeConfigs adapts the app config into generic provider runtime
// configs consumed by metadata fetching.
func (cfg *FormatConfig) ProviderRuntimeConfigs(reg *provider.Registry) []provider.RuntimeConfig {
	reg = providerRegistryOrGlobal(reg)

	configs := make([]provider.RuntimeConfig, 0)
	for _, p := range reg.Providers() {
		if p == nil || p.Capabilities().Local {
			continue
		}

		values := cfg.ProviderConfigValues(p)
		enabled := cfg.ProviderEnabled(p)
		if enabled && !ProviderConfigComplete(p, values) {
			enabled = false
		}

		configs = append(configs, provider.RuntimeConfig{
			Name:     p.Name(),
			Enabled:  enabled,
			Values:   values,
			Provider: p,
		})
	}
	return configs
}

// ConfigureProviderRegistry applies enabled app provider configuration to the
// registry. It returns per-provider warnings instead of failing the whole UI.
func (cfg *FormatConfig) ConfigureProviderRegistry(reg *provider.Registry) []error {
	reg = providerRegistryOrGlobal(reg)

	var errs []error
	for _, runtime := range cfg.ProviderRuntimeConfigs(reg) {
		if !runtime.Enabled {
			continue
		}
		if len(runtime.Values) > 0 {
			if err := reg.Configure(runtime.Name, runtime.Values); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", runtime.Name, err))
				continue
			}
		}
		if err := reg.Enable(runtime.Name); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", runtime.Name, err))
		}
	}
	return errs
}

// ProviderConfigComplete reports whether all required schema fields are present.
func ProviderConfigComplete(p provider.Provider, values map[string]interface{}) bool {
	if p == nil {
		return false
	}
	for _, field := range p.ConfigSchema().Fields {
		if !field.Required {
			continue
		}
		if isEmptyConfigValue(values[field.Name]) {
			return false
		}
	}
	return true
}

func providerDefaults(p provider.Provider) map[string]interface{} {
	values := make(map[string]interface{})
	if p == nil {
		return values
	}
	for _, field := range p.ConfigSchema().Fields {
		if field.Default != nil {
			values[field.Name] = field.Default
		}
	}
	return values
}

func providerConfigOverrides(p provider.Provider, values map[string]interface{}) map[string]interface{} {
	overrides := make(map[string]interface{})
	if p == nil || len(values) == 0 {
		return overrides
	}

	defaults := providerDefaults(p)
	for _, field := range p.ConfigSchema().Fields {
		value, ok := values[field.Name]
		if !ok {
			continue
		}
		if isEmptyConfigValue(value) && defaults[field.Name] == nil {
			continue
		}
		if configValuesEqual(value, defaults[field.Name]) {
			continue
		}
		overrides[field.Name] = value
	}
	return overrides
}

func configValuesEqual(value, defaultValue interface{}) bool {
	if value == nil || defaultValue == nil {
		return value == defaultValue
	}
	return fmt.Sprint(value) == fmt.Sprint(defaultValue)
}

func isEmptyConfigValue(value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case int:
		return v == 0
	case float64:
		return v == 0
	case bool:
		return false
	}
	return false
}

func providerRegistryOrGlobal(reg *provider.Registry) *provider.Registry {
	if reg != nil {
		return reg
	}
	_ = EnsureBuiltinProviders()
	return provider.GlobalRegistry
}
