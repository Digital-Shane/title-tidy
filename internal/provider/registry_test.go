package provider_test

import (
	"context"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
)

// MockProvider is a test provider implementation
type MockProvider struct {
	name         string
	version      string
	capabilities provider.ProviderCapabilities
	variables    []provider.TemplateVariable
	fetchFunc    func(context.Context, provider.FetchRequest) (*provider.Metadata, error)
	configured   bool
}

func (m *MockProvider) Name() string        { return m.name }
func (m *MockProvider) Version() string     { return m.version }
func (m *MockProvider) Description() string { return "Mock provider for testing" }
func (m *MockProvider) Capabilities() provider.ProviderCapabilities {
	return m.capabilities
}
func (m *MockProvider) SupportedVariables() []provider.TemplateVariable {
	return m.variables
}
func (m *MockProvider) ConfigSchema() provider.ConfigSchema {
	return provider.ConfigSchema{}
}
func (m *MockProvider) Configure(config map[string]interface{}) error {
	m.configured = true
	return nil
}
func (m *MockProvider) Fetch(ctx context.Context, req provider.FetchRequest) (*provider.Metadata, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, req)
	}
	return nil, nil
}

func TestRegistry_Register(t *testing.T) {
	registry := provider.NewRegistry()

	mock := &MockProvider{
		name:    "test",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
		},
	}

	// Test successful registration
	err := registry.Register("test", mock, 100)
	if err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	// Test duplicate registration
	err = registry.Register("test", mock, 100)
	if err == nil {
		t.Error("Register() expected error for duplicate, got nil")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := provider.NewRegistry()

	mock := &MockProvider{
		name:    "test",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
		},
	}

	registry.Register("test", mock, 100)

	// Test getting existing provider
	p, exists := registry.Get("test")
	if !exists {
		t.Error("Get() exists = false, want true")
	}
	if p == nil {
		t.Error("Get() provider = nil, want non-nil")
	}

	// Test getting non-existent provider
	_, exists = registry.Get("nonexistent")
	if exists {
		t.Error("Get() exists = true, want false")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := provider.NewRegistry()

	// Register providers with different priorities
	mock1 := &MockProvider{
		name:    "low",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
		},
	}
	mock2 := &MockProvider{
		name:    "high",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
		},
	}

	registry.Register("low", mock1, 50)
	registry.Register("high", mock2, 100)

	// Test listing returns providers in priority order
	list := registry.List()
	if len(list) != 2 {
		t.Errorf("List() length = %d, want 2", len(list))
	}
	if list[0] != "high" {
		t.Errorf("List()[0] = %s, want 'high'", list[0])
	}
	if list[1] != "low" {
		t.Errorf("List()[1] = %s, want 'low'", list[1])
	}
}

func TestRegistry_Enable(t *testing.T) {
	registry := provider.NewRegistry()

	mock := &MockProvider{
		name:    "test",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes:   []provider.MediaType{provider.MediaTypeMovie},
			RequiresAuth: false,
		},
	}

	registry.Register("test", mock, 100)

	// Test enabling provider
	err := registry.Enable("test")
	if err != nil {
		t.Errorf("Enable() error = %v, want nil", err)
	}

	if !registry.IsEnabled("test") {
		t.Error("IsEnabled() = false, want true")
	}

	// Test enabling non-existent provider
	err = registry.Enable("nonexistent")
	if err == nil {
		t.Error("Enable() expected error for nonexistent provider, got nil")
	}
}

func TestRegistry_Configure(t *testing.T) {
	registry := provider.NewRegistry()

	mock := &MockProvider{
		name:    "test",
		version: "1.0.0",
		capabilities: provider.ProviderCapabilities{
			MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
		},
	}

	registry.Register("test", mock, 100)

	// Test configuring provider
	config := map[string]interface{}{
		"api_key": "test-key",
	}
	err := registry.Configure("test", config)
	if err != nil {
		t.Errorf("Configure() error = %v, want nil", err)
	}

	if !mock.configured {
		t.Error("Provider not configured")
	}

	// Test configuring non-existent provider
	err = registry.Configure("nonexistent", config)
	if err == nil {
		t.Error("Configure() expected error for nonexistent provider, got nil")
	}
}

func TestValidateCapabilities(t *testing.T) {
	// Test valid capabilities
	caps := provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
	}
	err := provider.ValidateCapabilities(caps)
	if err != nil {
		t.Errorf("ValidateCapabilities() error = %v, want nil", err)
	}

	// Test missing media types
	caps = provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{},
	}
	err = provider.ValidateCapabilities(caps)
	if err == nil {
		t.Error("ValidateCapabilities() expected error for no media types, got nil")
	}

	// Test language-agnostic provider
	caps = provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{provider.MediaTypeMovie},
	}
	err = provider.ValidateCapabilities(caps)
	if err != nil {
		t.Errorf("ValidateCapabilities() error = %v for language-agnostic, want nil", err)
	}
}

func TestProviderError(t *testing.T) {
	err := &provider.ProviderError{
		Provider:   "tmdb",
		Code:       "RATE_LIMIT",
		Message:    "API rate limit exceeded",
		Retry:      true,
		RetryAfter: 10,
	}

	// Test Error() method
	errStr := err.Error()
	if !cmp.Equal(errStr, "API rate limit exceeded") {
		t.Errorf("Error() = %s, want 'API rate limit exceeded'", errStr)
	}

	// Test Retry field
	if !err.Retry {
		t.Error("Retry = false, want true")
	}
}
