// Package init handles provider initialization to avoid import cycles
package init

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/core"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
)

// LoadBuiltinProviders loads all built-in providers into the global registry
func LoadBuiltinProviders() error {
	// Register core provider first (always enabled)
	coreProvider := core.New()
	if err := provider.GlobalRegistry.Register("core", coreProvider, 0); err != nil {
		return fmt.Errorf("failed to register core provider: %w", err)
	}
	// Core provider is always enabled
	if err := provider.GlobalRegistry.Enable("core"); err != nil {
		return fmt.Errorf("failed to enable core provider: %w", err)
	}

	// Register TMDB provider
	tmdbProvider := tmdb.New()
	if err := provider.GlobalRegistry.Register("tmdb", tmdbProvider, 100); err != nil {
		return fmt.Errorf("failed to register TMDB provider: %w", err)
	}

	// Future providers

	return nil
}
