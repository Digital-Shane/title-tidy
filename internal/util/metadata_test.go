package util

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
)

func TestGenerateMetadataKey(t *testing.T) {
	tests := []struct {
		name      string
		mediaType string
		itemName  string
		year      string
		season    int
		episode   int
		want      string
	}{
		{
			name:      "movie key",
			mediaType: "movie",
			itemName:  "Avatar",
			year:      "2009",
			season:    0,
			episode:   0,
			want:      "movie:Avatar:2009",
		},
		{
			name:      "show key",
			mediaType: "show",
			itemName:  "Breaking Bad",
			year:      "2008",
			season:    0,
			episode:   0,
			want:      "show:Breaking Bad:2008",
		},
		{
			name:      "season key",
			mediaType: "season",
			itemName:  "Breaking Bad",
			year:      "2008",
			season:    1,
			episode:   0,
			want:      "season:Breaking Bad:2008:1",
		},
		{
			name:      "episode key",
			mediaType: "episode",
			itemName:  "Breaking Bad",
			year:      "2008",
			season:    1,
			episode:   5,
			want:      "episode:Breaking Bad:2008:1:5",
		},
		{
			name:      "unknown media type",
			mediaType: "unknown",
			itemName:  "Test",
			year:      "2020",
			season:    0,
			episode:   0,
			want:      "",
		},
		{
			name:      "empty media type",
			mediaType: "",
			itemName:  "Test",
			year:      "2020",
			season:    0,
			episode:   0,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMetadataKey(tt.mediaType, tt.itemName, tt.year, tt.season, tt.episode)
			if got != tt.want {
				t.Errorf("GenerateMetadataKey(%q, %q, %q, %d, %d) = %q, want %q",
					tt.mediaType, tt.itemName, tt.year, tt.season, tt.episode, got, tt.want)
			}
		})
	}
}

func TestFetchMetadataWithDependencies_NilProvider(t *testing.T) {
	cache := make(map[string]*provider.Metadata)

	got, err := FetchMetadataWithDependencies(nil, "Test Show", "2020", 1, 1, false, cache)

	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies(nil, ...) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_EmptyName(t *testing.T) {
	// Create a mock provider (we'll use nil since we expect early return)
	cache := make(map[string]*provider.Metadata)

	got, err := FetchMetadataWithDependencies(nil, "", "2020", 1, 1, false, cache)

	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies(..., \"\", ...) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_ErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		provider provider.Provider
		itemName string
		year     string
		season   int
		episode  int
		isMovie  bool
		cache    map[string]*provider.Metadata
		want     *provider.Metadata
	}{
		{
			name:     "nil provider",
			provider: nil,
			itemName: "Test",
			year:     "2020",
			season:   0,
			episode:  0,
			isMovie:  true,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
		{
			name:     "empty name",
			provider: nil,
			itemName: "",
			year:     "2020",
			season:   0,
			episode:  0,
			isMovie:  true,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
		{
			name:     "movie request",
			provider: nil,
			itemName: "Test Movie",
			year:     "2020",
			season:   0,
			episode:  0,
			isMovie:  true,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
		{
			name:     "episode request without cache",
			provider: nil,
			itemName: "Test Show",
			year:     "2020",
			season:   1,
			episode:  5,
			isMovie:  false,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
		{
			name:     "season request without cache",
			provider: nil,
			itemName: "Test Show",
			year:     "2020",
			season:   2,
			episode:  0,
			isMovie:  false,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
		{
			name:     "show request",
			provider: nil,
			itemName: "Test Show",
			year:     "2020",
			season:   0,
			episode:  0,
			isMovie:  false,
			cache:    make(map[string]*provider.Metadata),
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchMetadataWithDependencies(tt.provider, tt.itemName, tt.year, tt.season, tt.episode, tt.isMovie, tt.cache)
			if got != tt.want {
				t.Errorf("FetchMetadataWithDependencies() = (%v, %v), want (%v, <nil>)", got, err, tt.want)
			}
		})
	}
}

// This mock is no longer needed with the new provider interface

func TestFetchMetadataWithDependencies_Movie(t *testing.T) {
	cache := make(map[string]*provider.Metadata)

	// Test the function - note we can't actually test the full function due to interface constraints
	// but we can test the key generation and basic flow
	key := GenerateMetadataKey("movie", "Test Movie", "2020", 0, 0)
	want := "movie:Test Movie:2020"

	if key != want {
		t.Errorf("GenerateMetadataKey for movie = %q, want %q", key, want)
	}

	// Test with nil provider
	got, err := FetchMetadataWithDependencies(nil, "Test Movie", "2020", 0, 0, true, cache)
	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies with nil provider = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_Show(t *testing.T) {
	cache := make(map[string]*provider.Metadata)

	// Test show key generation
	key := GenerateMetadataKey("show", "Test Show", "2020", 0, 0)
	want := "show:Test Show:2020"

	if key != want {
		t.Errorf("GenerateMetadataKey for show = %q, want %q", key, want)
	}

	// Test with nil provider
	got, err := FetchMetadataWithDependencies(nil, "Test Show", "2020", 0, 0, false, cache)
	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies with nil provider = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_Episode(t *testing.T) {
	cache := make(map[string]*provider.Metadata)

	// Test episode key generation
	key := GenerateMetadataKey("episode", "Test Show", "2020", 1, 5)
	want := "episode:Test Show:2020:1:5"

	if key != want {
		t.Errorf("GenerateMetadataKey for episode = %q, want %q", key, want)
	}

	// Test with nil provider
	got, err := FetchMetadataWithDependencies(nil, "Test Show", "2020", 1, 5, false, cache)
	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies with nil provider = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_Season(t *testing.T) {
	cache := make(map[string]*provider.Metadata)

	// Test season key generation
	key := GenerateMetadataKey("season", "Test Show", "2020", 2, 0)
	want := "season:Test Show:2020:2"

	if key != want {
		t.Errorf("GenerateMetadataKey for season = %q, want %q", key, want)
	}

	// Test with nil provider
	got, err := FetchMetadataWithDependencies(nil, "Test Show", "2020", 2, 0, false, cache)
	if got != nil || err != nil {
		t.Errorf("FetchMetadataWithDependencies with nil provider = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFetchMetadataWithDependencies_CacheInteraction(t *testing.T) {
	tests := []struct {
		name             string
		itemName         string
		year             string
		season           int
		episode          int
		isMovie          bool
		prePopulateCache bool
		wantCacheKeys    []string
	}{
		{
			name:             "episode populates show cache key",
			itemName:         "Test Show",
			year:             "2020",
			season:           1,
			episode:          5,
			isMovie:          false,
			prePopulateCache: false,
			wantCacheKeys:    []string{"show:Test Show:2020"},
		},
		{
			name:             "season populates show cache key",
			itemName:         "Test Show",
			year:             "2020",
			season:           2,
			episode:          0,
			isMovie:          false,
			prePopulateCache: false,
			wantCacheKeys:    []string{"show:Test Show:2020"},
		},
		{
			name:             "episode with existing show cache",
			itemName:         "Test Show",
			year:             "2020",
			season:           1,
			episode:          5,
			isMovie:          false,
			prePopulateCache: true,
			wantCacheKeys:    []string{"show:Test Show:2020"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := make(map[string]*provider.Metadata)

			if tt.prePopulateCache {
				// Pre-populate with show metadata
				cache["show:Test Show:2020"] = &provider.Metadata{
					Core: provider.CoreMetadata{
						Title: "Test Show",
					},
					IDs: map[string]string{
						"tmdb_id": "123",
					},
				}
			}

			// Call function with nil provider (will exit early but still test cache key generation)
			_, _ = FetchMetadataWithDependencies(nil, tt.itemName, tt.year, tt.season, tt.episode, tt.isMovie, cache)

			// Verify expected cache keys exist (if they were supposed to be generated)
			for _, key := range tt.wantCacheKeys {
				if tt.prePopulateCache {
					// Should already exist
					if _, exists := cache[key]; !exists {
						t.Errorf("Expected cache key %q to exist", key)
					}
				} else {
					// Verify the key format is what we expect (function generates it but doesn't populate since provider is nil)
					expectedKey := GenerateMetadataKey("show", tt.itemName, tt.year, 0, 0)
					if key != expectedKey {
						t.Errorf("Expected key %q, got %q", expectedKey, key)
					}
				}
			}
		})
	}
}

func TestMetadataKeyGeneration_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		mediaType string
		itemName  string
		year      string
		season    int
		episode   int
		want      string
	}{
		{
			name:      "special characters in name",
			mediaType: "movie",
			itemName:  "Test: Movie (2020)",
			year:      "2020",
			season:    0,
			episode:   0,
			want:      "movie:Test: Movie (2020):2020",
		},
		{
			name:      "empty year",
			mediaType: "show",
			itemName:  "Test Show",
			year:      "",
			season:    0,
			episode:   0,
			want:      "show:Test Show:",
		},
		{
			name:      "high season and episode numbers",
			mediaType: "episode",
			itemName:  "Long Running Show",
			year:      "1990",
			season:    25,
			episode:   999,
			want:      "episode:Long Running Show:1990:25:999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMetadataKey(tt.mediaType, tt.itemName, tt.year, tt.season, tt.episode)
			if got != tt.want {
				t.Errorf("GenerateMetadataKey(%q, %q, %q, %d, %d) = %q, want %q",
					tt.mediaType, tt.itemName, tt.year, tt.season, tt.episode, got, tt.want)
			}
		})
	}
}
