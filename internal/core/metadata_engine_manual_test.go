package core

import (
	"context"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/mhmtszr/concurrent-swiss-map"
)

type retryTestProvider struct{}

func (retryTestProvider) Name() string                                    { return "retry-test" }
func (retryTestProvider) Description() string                             { return "retry-test" }
func (retryTestProvider) SupportedVariables() []provider.TemplateVariable { return nil }
func (retryTestProvider) Configure(map[string]interface{}) error          { return nil }
func (retryTestProvider) ConfigSchema() provider.ConfigSchema             { return provider.ConfigSchema{} }
func (retryTestProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{
			provider.MediaTypeMovie,
			provider.MediaTypeShow,
		},
	}
}

func (retryTestProvider) Fetch(ctx context.Context, req provider.FetchRequest) (*provider.Metadata, error) {
	_ = ctx
	if req.Name == "Manual Success" {
		return &provider.Metadata{Core: provider.CoreMetadata{Title: req.Name, MediaType: req.MediaType}}, nil
	}
	return nil, &provider.ProviderError{Provider: "retry-test", Code: "NOT_FOUND", Message: "not found", Retry: false}
}

func TestMetadataEngineRetryProviderClearsFailures(t *testing.T) {
	t.Parallel()

	item := MetadataItem{
		Name:      "Manual Movie",
		Year:      "2022",
		IsMovie:   true,
		MediaType: provider.MediaTypeMovie,
		Key:       provider.GenerateMetadataKey("movie", "Manual Movie", "2022", 0, 0),
	}
	notFound := &provider.ProviderError{Provider: "retry-test", Code: "NOT_FOUND", Message: "missing", Retry: false}

	engine := &MetadataEngine{
		tmdbProvider: retryTestProvider{},
		metadata:     csmap.Create[string, *provider.Metadata](),
	}

	engine.processResult(MetadataResult{
		Item:    item,
		Errs:    []error{notFound},
		TMDBErr: notFound,
	})

	failures := engine.ProviderFailures()
	if diff := cmp.Diff(1, len(failures)); diff != "" {
		t.Fatalf("ProviderFailures length mismatch (-want +got):\n%s", diff)
	}
	if failures[0].Provider != MetadataProviderTMDB {
		t.Fatalf("failure provider = %s, want TMDB", failures[0].Provider)
	}

	result, err := engine.RetryProvider(context.Background(), item.Key, MetadataProviderTMDB, "Manual Success")
	if err != nil {
		t.Fatalf("RetryProvider() unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("RetryProvider() returned failure %v, want nil", result)
	}

	if diff := cmp.Diff(0, len(engine.ProviderFailures())); diff != "" {
		t.Errorf("ProviderFailures after retry (-want +got):\n%s", diff)
	}
	meta := engine.Metadata()[item.Key]
	if meta == nil {
		t.Fatalf("Metadata()[%q] = nil, want populated", item.Key)
	}
	if diff := cmp.Diff("Manual Success", meta.Core.Title); diff != "" {
		t.Errorf("metadata title mismatch (-want +got):\n%s", diff)
	}
	summary := engine.SummarySnapshot()
	if diff := cmp.Diff(0, summary.ErrorCount); diff != "" {
		t.Errorf("summary.ErrorCount mismatch (-want +got):\n%s", diff)
	}
}

func TestMetadataEngineNonRetryableErrorsNotTracked(t *testing.T) {
	t.Parallel()

	item := MetadataItem{
		Name:      "Manual Movie",
		Year:      "2022",
		IsMovie:   true,
		MediaType: provider.MediaTypeMovie,
		Key:       provider.GenerateMetadataKey("movie", "Manual Movie", "2022", 0, 0),
	}
	authErr := &provider.ProviderError{Provider: "retry-test", Code: "AUTH_FAILED", Message: "bad key", Retry: false}

	engine := &MetadataEngine{
		metadata: csmap.Create[string, *provider.Metadata](),
	}

	engine.processResult(MetadataResult{
		Item:    item,
		Errs:    []error{authErr},
		TMDBErr: authErr,
	})

	if diff := cmp.Diff(0, len(engine.ProviderFailures())); diff != "" {
		t.Errorf("ProviderFailures length mismatch (-want +got):\n%s", diff)
	}
	summary := engine.SummarySnapshot()
	if diff := cmp.Diff(0, summary.ErrorCount); diff != "" {
		t.Errorf("summary.ErrorCount mismatch (-want +got):\n%s", diff)
	}
}
