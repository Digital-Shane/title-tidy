package provider

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testMetadataCache struct {
	data map[string]*Metadata
}

func newTestMetadataCache() *testMetadataCache {
	return &testMetadataCache{data: make(map[string]*Metadata)}
}

func (c *testMetadataCache) Get(key string) (*Metadata, bool) {
	meta, ok := c.data[key]
	return meta, ok
}

func (c *testMetadataCache) Set(key string, meta *Metadata) {
	c.data[key] = meta
}

type stubProvider struct {
	fetch func(context.Context, FetchRequest) (*Metadata, error)
}

func (p *stubProvider) Name() string                           { return "stub" }
func (p *stubProvider) Description() string                    { return "stub provider" }
func (p *stubProvider) Capabilities() ProviderCapabilities     { return ProviderCapabilities{} }
func (p *stubProvider) SupportedVariables() []TemplateVariable { return nil }
func (p *stubProvider) Configure(map[string]interface{}) error { return nil }
func (p *stubProvider) ConfigSchema() ConfigSchema             { return ConfigSchema{} }

func (p *stubProvider) Fetch(ctx context.Context, req FetchRequest) (*Metadata, error) {
	if p.fetch == nil {
		return nil, nil
	}
	return p.fetch(ctx, req)
}

func TestGenerateMetadataKey(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mediaType string
		name      string
		year      string
		season    int
		episode   int
		want      string
	}{
		"movie": {
			mediaType: "movie",
			name:      "Avatar",
			year:      "2009",
			want:      "movie:Avatar:2009",
		},
		"show": {
			mediaType: "show",
			name:      "Breaking Bad",
			year:      "2008",
			want:      "show:Breaking Bad:2008",
		},
		"season": {
			mediaType: "season",
			name:      "Breaking Bad",
			year:      "2008",
			season:    1,
			want:      "season:Breaking Bad:2008:1",
		},
		"episode": {
			mediaType: "episode",
			name:      "Breaking Bad",
			year:      "2008",
			season:    1,
			episode:   5,
			want:      "episode:Breaking Bad:2008:1:5",
		},
		"unknown": {
			mediaType: "unknown",
			name:      "Test",
			year:      "2020",
			want:      "",
		},
	}

	for name, tc := range tests {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := GenerateMetadataKey(tc.mediaType, tc.name, tc.year, tc.season, tc.episode)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GenerateMetadataKey mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFetchMetadataWithDependenciesEarlyExit(t *testing.T) {
	t.Parallel()

	cache := newTestMetadataCache()

	got, err := FetchMetadataWithDependencies(context.Background(), nil, "", "2020", 1, 1, false, cache)
	if err != nil {
		t.Fatalf("FetchMetadataWithDependencies(nil) returned error %v, want nil", err)
	}
	if got != nil {
		t.Errorf("FetchMetadataWithDependencies(nil) = %v, want nil", got)
	}
}

func TestFetchMetadataWithDependenciesEpisode(t *testing.T) {
	t.Parallel()

	cache := newTestMetadataCache()
	showRequest := FetchRequest{MediaType: MediaTypeShow, Name: "Test Show"}
	episodeRequest := FetchRequest{MediaType: MediaTypeEpisode, ID: "show-123", Name: "Test Show", Year: "2020", Season: 1, Episode: 5}

	calls := make([]FetchRequest, 0, 2)
	stub := &stubProvider{
		fetch: func(_ context.Context, req FetchRequest) (*Metadata, error) {
			calls = append(calls, req)
			switch len(calls) {
			case 1:
				if diff := cmp.Diff(showRequest, req); diff != "" {
					t.Fatalf("show request mismatch (-want +got):\n%s", diff)
				}
				return &Metadata{IDs: map[string]string{"tmdb_id": "show-123"}}, nil
			case 2:
				if diff := cmp.Diff(episodeRequest, req); diff != "" {
					t.Fatalf("episode request mismatch (-want +got):\n%s", diff)
				}
				return &Metadata{IDs: map[string]string{"tmdb_episode": "episode-123"}}, nil
			default:
				t.Fatalf("unexpected extra fetch: %+v", req)
				return nil, nil
			}
		},
	}

	got, err := FetchMetadataWithDependencies(context.Background(), stub, "Test Show", "2020", 1, 5, false, cache)
	if err != nil {
		t.Fatalf("FetchMetadataWithDependencies returned error %v, want nil", err)
	}

	want := &Metadata{IDs: map[string]string{"tmdb_episode": "episode-123"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FetchMetadataWithDependencies episode (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]FetchRequest{showRequest, episodeRequest}, calls); diff != "" {
		t.Errorf("fetch call order (-want +got):\n%s", diff)
	}

	cached, ok := cache.Get(GenerateMetadataKey("show", "Test Show", "2020", 0, 0))
	if !ok {
		t.Fatalf("show metadata not cached")
	}
	wantCached := &Metadata{IDs: map[string]string{"tmdb_id": "show-123"}}
	if diff := cmp.Diff(wantCached, cached); diff != "" {
		t.Errorf("cached show metadata (-want +got):\n%s", diff)
	}
}
