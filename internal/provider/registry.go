package provider

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages all available providers
type Registry struct {
	mu            sync.RWMutex
	providers     map[string]Provider
	priorities    map[string]int
	enabledStatus map[string]bool
	configs       map[string]map[string]interface{}
}

// GlobalRegistry is the default registry instance
var GlobalRegistry = NewRegistry()

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers:     make(map[string]Provider),
		priorities:    make(map[string]int),
		enabledStatus: make(map[string]bool),
		configs:       make(map[string]map[string]interface{}),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, provider Provider, priority int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	// Validate provider capabilities
	if err := ValidateCapabilities(provider.Capabilities()); err != nil {
		return fmt.Errorf("invalid provider capabilities for %s: %w", name, err)
	}

	r.providers[name] = provider
	r.priorities[name] = priority
	r.enabledStatus[name] = false // Disabled by default

	return nil
}

// RegisterProvider adds a provider using the facts supplied by the provider
// itself.
func (r *Registry) RegisterProvider(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	caps := provider.Capabilities()
	return r.Register(provider.Name(), provider, caps.Priority)
}

// Get returns a provider by name
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	return provider, exists
}

// List returns all registered providers
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	// Sort by priority
	sort.Slice(names, func(i, j int) bool {
		return r.priorities[names[i]] > r.priorities[names[j]]
	})

	return names
}

// Providers returns all registered providers sorted by registry priority.
func (r *Registry) Providers() []Provider {
	names := r.List()
	providers := make([]Provider, 0, len(names))
	for _, name := range names {
		if p, ok := r.Get(name); ok {
			providers = append(providers, p)
		}
	}
	return providers
}

// Priority returns the registry priority for a provider.
func (r *Registry) Priority(name string) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	priority, exists := r.priorities[name]
	return priority, exists
}

// IsEnabled reports whether a provider is currently enabled.
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.enabledStatus[name]
}

// Enable enables a provider
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Validate configuration if required
	if provider.Capabilities().RequiresAuth {
		if config, hasConfig := r.configs[name]; !hasConfig || len(config) == 0 {
			return fmt.Errorf("provider %s requires configuration", name)
		}
	}

	r.enabledStatus[name] = true
	return nil
}

// Configure sets configuration for a provider
func (r *Registry) Configure(name string, config map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Apply configuration to provider
	if err := provider.Configure(config); err != nil {
		return fmt.Errorf("failed to configure provider %s: %w", name, err)
	}

	// Store configuration
	r.configs[name] = config

	return nil
}
