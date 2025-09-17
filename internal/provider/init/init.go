// Package init handles provider initialization to avoid import cycles
package init

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
)

// LoadBuiltinProviders loads all built-in providers into the global registry
func LoadBuiltinProviders() error {
	// Register local provider first (always enabled)
	localProvider := local.New()
	if err := provider.GlobalRegistry.Register("local", localProvider, 0); err != nil {
		return fmt.Errorf("failed to register local provider: %w", err)
	}
	// Local provider is always enabled
	if err := provider.GlobalRegistry.Enable("local"); err != nil {
		return fmt.Errorf("failed to enable local provider: %w", err)
	}

	// Register TMDB provider
	tmdbProvider := tmdb.New()
	if err := provider.GlobalRegistry.Register("tmdb", tmdbProvider, 100); err != nil {
		return fmt.Errorf("failed to register TMDB provider: %w", err)
	}

	// Future providers

	return nil
}
