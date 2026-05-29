// Package init handles provider initialization to avoid import cycles
package init

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/ffprobe"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/provider/omdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tmdb"
	"github.com/Digital-Shane/title-tidy/internal/provider/tvdb"
)

// LoadBuiltinProviders loads all built-in providers into the global registry
func LoadBuiltinProviders() error {
	if err := registerBuiltin(local.New(), true); err != nil {
		return fmt.Errorf("failed to register local provider: %w", err)
	}
	if err := registerBuiltin(tmdb.New(), false); err != nil {
		return fmt.Errorf("failed to register TMDB provider: %w", err)
	}
	if err := registerBuiltin(tvdb.New(), false); err != nil {
		return fmt.Errorf("failed to register TVDB provider: %w", err)
	}
	if err := registerBuiltin(omdb.New(), false); err != nil {
		return fmt.Errorf("failed to register OMDb provider: %w", err)
	}
	if err := registerBuiltin(ffprobe.New(), false); err != nil {
		return fmt.Errorf("failed to register ffprobe provider: %w", err)
	}

	return nil
}

func registerBuiltin(p provider.Provider, enabled bool) error {
	name := p.Name()
	if _, exists := provider.GlobalRegistry.Get(name); !exists {
		if err := provider.GlobalRegistry.RegisterProvider(p); err != nil {
			return err
		}
	}
	if enabled {
		if err := provider.GlobalRegistry.Enable(name); err != nil {
			return err
		}
	}
	return nil
}
