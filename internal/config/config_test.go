package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// cmpOpts returns comparison options for stable config comparisons.
func cmpOpts() cmp.Option {
	return cmpopts.EquateEmpty()
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	want := &FormatConfig{
		ShowFolder:           "{title} ({year})",
		SeasonFolder:         "Season {season}",
		Episode:              "S{season}E{episode}",
		Movie:                "{title} ({year})",
		PreserveExistingTags: false,
		LogRetentionDays:     30,
		EnableLogging:        true,
		MetadataWorkerCount:  10,
		Providers:            map[string]ProviderConfig{},
	}

	if diff := cmp.Diff(want, cfg, cmpOpts()); diff != "" {
		t.Errorf("DefaultConfig() mismatch (-want +got):\n%s", diff)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Errorf("ConfigPath() error = %v, want nil", err)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("ConfigPath() = %v, want absolute path", path)
	}

	// Check that it contains the .title-tidy directory
	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".title-tidy" {
		t.Errorf("ConfigPath() = %v, want path containing .title-tidy directory", path)
	}

	// Check that it ends with config.json
	if filepath.Base(path) != "config.json" {
		t.Errorf("ConfigPath() = %v, want path ending with config.json", path)
	}
}

func TestLoadMigratesLegacyProviderConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configDir := filepath.Join(os.Getenv("HOME"), ".title-tidy")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	configData := []byte(`{
		"show_folder": "{title}",
		"tmdb_api_key": "tmdb-key",
		"enable_tmdb_lookup": true,
		"tmdb_language": "es-ES",
		"tmdb_worker_count": 3,
		"omdb_api_key": "omdb-key",
		"enable_omdb_lookup": true,
		"tvdb_api_key": "tvdb-key",
		"enable_tvdb_lookup": true,
		"enable_ffprobe": true
	}`)
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if diff := cmp.Diff(3, cfg.MetadataWorkerCount); diff != "" {
		t.Errorf("MetadataWorkerCount mismatch (-want +got):\n%s", diff)
	}

	assertProvider := func(name string, enabled bool, want map[string]interface{}) {
		t.Helper()
		got := cfg.Provider(name)
		if diff := cmp.Diff(enabled, got.Enabled); diff != "" {
			t.Errorf("%s enabled mismatch (-want +got):\n%s", name, diff)
		}
		for key, wantValue := range want {
			if diff := cmp.Diff(wantValue, got.Config[key]); diff != "" {
				t.Errorf("%s config %s mismatch (-want +got):\n%s", name, key, diff)
			}
		}
	}

	assertProvider("tmdb", true, map[string]interface{}{"api_key": "tmdb-key", "language": "es-ES"})
	assertProvider("omdb", true, map[string]interface{}{"api_key": "omdb-key"})
	assertProvider("tvdb", true, map[string]interface{}{"api_key": "tvdb-key"})
	assertProvider("ffprobe", true, nil)

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal migrated config: %v", err)
	}
	for _, legacyKey := range []string{
		"tmdb_api_key",
		"enable_tmdb_lookup",
		"tmdb_language",
		"tmdb_worker_count",
		"omdb_api_key",
		"enable_omdb_lookup",
		"tvdb_api_key",
		"enable_tvdb_lookup",
		"enable_ffprobe",
	} {
		if _, ok := raw[legacyKey]; ok {
			t.Fatalf("migrated config still contains legacy key %q", legacyKey)
		}
	}
	if _, ok := raw["providers"]; !ok {
		t.Fatal("migrated config missing providers map")
	}
	if _, ok := raw["metadata_worker_count"]; !ok {
		t.Fatal("migrated config missing metadata_worker_count")
	}
}

func TestLoadDropsLegacyTMDBCacheConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configDir := filepath.Join(os.Getenv("HOME"), ".title-tidy")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	configData := []byte(`{
		"providers": {
			"tmdb": {
				"enabled": true,
				"config": {
					"api_key": "tmdb-key",
					"language": "fr-FR",
					"cache_enabled": false,
					"cache_duration": 24
				}
			}
		}
	}`)
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tmdbConfig := cfg.Provider("tmdb").Config
	if _, ok := tmdbConfig["cache_enabled"]; ok {
		t.Fatal("tmdb config kept legacy cache_enabled")
	}
	if _, ok := tmdbConfig["cache_duration"]; ok {
		t.Fatal("tmdb config kept legacy cache_duration")
	}
	if diff := cmp.Diff("tmdb-key", tmdbConfig["api_key"]); diff != "" {
		t.Errorf("tmdb api_key mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("fr-FR", tmdbConfig["language"]); diff != "" {
		t.Errorf("tmdb language mismatch (-want +got):\n%s", diff)
	}
}

func TestSave(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	cfg := &FormatConfig{
		ShowFolder:       "test {title}",
		SeasonFolder:     "test Season {season}",
		Episode:          "test E{episode}",
		Movie:            "test {title}",
		LogRetentionDays: 90,
		EnableLogging:    false,
	}

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Verify file was created
	configFile := filepath.Join(tempDir, ".title-tidy", "config.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	// Parse back to verify content
	var saved FormatConfig
	err = json.Unmarshal(data, &saved)
	if err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if diff := cmp.Diff(cfg, &saved, cmpOpts()); diff != "" {
		t.Errorf("Saved config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoad(t *testing.T) {
	t.Run("file_not_exists", func(t *testing.T) {
		// Create temp dir for config
		tempDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		t.Setenv("HOME", tempDir)
		defer func() { os.Setenv("HOME", oldHome) }()

		cfg, err := Load()
		if err != nil {
			t.Errorf("Load() with no file error = %v, want nil", err)
		}

		// Should return default config
		want := DefaultConfig()
		if diff := cmp.Diff(want, cfg, cmpOpts()); diff != "" {
			t.Errorf("Load() with no file mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("valid_config", func(t *testing.T) {
		// Create temp dir and config file
		tempDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		t.Setenv("HOME", tempDir)
		defer func() { os.Setenv("HOME", oldHome) }()

		configDir := filepath.Join(tempDir, ".title-tidy")
		os.MkdirAll(configDir, 0755)

		testConfig := &FormatConfig{
			ShowFolder:   "{title} - {year}",
			SeasonFolder: "S{season}",
			Episode:      "{code} {title}",
			Movie:        "{title} [{year}]",
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")
		configPath := filepath.Join(configDir, "config.json")
		os.WriteFile(configPath, data, 0644)

		cfg, err := Load()
		if err != nil {
			t.Errorf("Load() error = %v, want nil", err)
		}

		// Expected config should include default values filled in by Load()
		expectedConfig := &FormatConfig{
			ShowFolder:          "{title} - {year}",
			SeasonFolder:        "S{season}",
			Episode:             "{code} {title}",
			Movie:               "{title} [{year}]",
			LogRetentionDays:    30,    // Default value filled in by Load()
			EnableLogging:       false, // Not set in JSON, so false
			MetadataWorkerCount: 10,
			Providers:           map[string]ProviderConfig{},
		}

		if diff := cmp.Diff(expectedConfig, cfg, cmpOpts()); diff != "" {
			t.Errorf("Load() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("partial_config", func(t *testing.T) {
		// Create temp dir and config file with only some fields
		tempDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		t.Setenv("HOME", tempDir)
		defer func() { os.Setenv("HOME", oldHome) }()

		configDir := filepath.Join(tempDir, ".title-tidy")
		os.MkdirAll(configDir, 0755)

		partialConfig := map[string]string{
			"show_folder": "{title}",
			"episode":     "{code}",
		}

		data, _ := json.MarshalIndent(partialConfig, "", "  ")
		configPath := filepath.Join(configDir, "config.json")
		os.WriteFile(configPath, data, 0644)

		cfg, err := Load()
		if err != nil {
			t.Errorf("Load() error = %v, want nil", err)
		}

		// Should fill in missing fields with defaults
		want := &FormatConfig{
			ShowFolder:          "{title}",
			SeasonFolder:        "Season {season}", // default
			Episode:             "{code}",
			Movie:               "{title} ({year})", // default
			LogRetentionDays:    30,                 // default
			EnableLogging:       false,              // Not set in JSON, so false
			MetadataWorkerCount: 10,
			Providers:           map[string]ProviderConfig{},
		}

		if diff := cmp.Diff(want, cfg, cmpOpts()); diff != "" {
			t.Errorf("Load() partial config mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		// Create temp dir and invalid config file
		tempDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		t.Setenv("HOME", tempDir)
		defer func() { os.Setenv("HOME", oldHome) }()

		configDir := filepath.Join(tempDir, ".title-tidy")
		os.MkdirAll(configDir, 0755)

		configPath := filepath.Join(configDir, "config.json")
		os.WriteFile(configPath, []byte("invalid json"), 0644)

		_, err := Load()
		if err == nil {
			t.Error("Load() with invalid JSON error = nil, want error")
		}
	})
}

func TestApplyShowFolderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		show     string
		year     string
		want     string
	}{
		{
			name:     "default_template",
			template: "{title} ({year})",
			show:     "Breaking Bad",
			year:     "2008",
			want:     "Breaking Bad (2008)",
		},
		{
			name:     "show_only",
			template: "{title}",
			show:     "The Wire",
			year:     "2002",
			want:     "The Wire",
		},
		{
			name:     "year_only",
			template: "{year}",
			show:     "Game of Thrones",
			year:     "2011",
			want:     "2011",
		},
		{
			name:     "custom_format",
			template: "{title} - {year}",
			show:     "Game of Thrones",
			year:     "2011",
			want:     "Game of Thrones - 2011",
		},
		{
			name:     "no_placeholders",
			template: "TV Shows",
			show:     "Breaking Bad",
			year:     "2008",
			want:     "TV Shows",
		},
		{
			name:     "empty_values",
			template: "{title} ({year})",
			show:     "",
			year:     "",
			want:     "",
		},
		{
			name:     "escaped_literal_braces",
			template: "\\{{title}\\}",
			show:     "Breaking Bad",
			year:     "2008",
			want:     "{Breaking Bad}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FormatConfig{ShowFolder: tt.template}
			ctx := &FormatContext{
				ShowName: tt.show,
				Year:     tt.year,
			}
			got := cfg.ApplyShowFolderTemplate(ctx)
			if got != tt.want {
				t.Errorf("ApplyShowFolderTemplate(%q, %q) = %q, want %q", tt.show, tt.year, got, tt.want)
			}
		})
	}
}

func TestApplyShowFolderTemplateEscapedBracesWithMetadata(t *testing.T) {
	cfg := &FormatConfig{ShowFolder: "{title} ({year}) \\{imdb-{imdb_id}\\}"}
	ctx := &FormatContext{
		ShowName: "Breaking Bad",
		Year:     "2008",
		Metadata: &provider.Metadata{
			IDs: map[string]string{"imdb_id": "tt0903747"},
		},
	}

	got := cfg.ApplyShowFolderTemplate(ctx)
	want := "Breaking Bad (2008) {imdb-tt0903747}"
	if got != want {
		t.Fatalf("ApplyShowFolderTemplate() = %q, want %q", got, want)
	}
}

func TestApplySeasonFolderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		show     string
		year     string
		season   int
		want     string
	}{
		{
			name:     "default_template",
			template: "{title} - Season {season}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			want:     "Breaking Bad - Season 01",
		},
		{
			name:     "season_with_prefix",
			template: "{title} S{season}",
			show:     "The Wire",
			year:     "2002",
			season:   3,
			want:     "The Wire S03",
		},
		{
			name:     "season_number_only",
			template: "Season {season}",
			show:     "Ignored",
			year:     "2020",
			season:   10,
			want:     "Season 10",
		},
		{
			name:     "all_variables",
			template: "{title} - S{season} - Season {season} - {season}",
			show:     "Test",
			year:     "2021",
			season:   5,
			want:     "Test - S05 - Season 05 - 05",
		},
		{
			name:     "no_placeholders",
			template: "Seasons",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			want:     "Seasons",
		},
		{
			name:     "large_season_number",
			template: "S{season}",
			show:     "Test",
			year:     "2022",
			season:   100,
			want:     "S100",
		},
		{
			name:     "with_year",
			template: "{title} ({year}) - Season {season}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			want:     "Breaking Bad (2008) - Season 01",
		},
		{
			name:     "empty_year",
			template: "{title} ({year}) - Season {season}",
			show:     "Breaking Bad",
			year:     "",
			season:   1,
			want:     "Breaking Bad - Season 01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FormatConfig{SeasonFolder: tt.template}
			ctx := &FormatContext{
				ShowName: tt.show,
				Year:     tt.year,
				Season:   tt.season,
			}
			got := cfg.ApplySeasonFolderTemplate(ctx)
			if got != tt.want {
				t.Errorf("ApplySeasonFolderTemplate(%q, %q, %d) = %q, want %q", tt.show, tt.year, tt.season, got, tt.want)
			}
		})
	}
}

func TestApplyEpisodeTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		show     string
		year     string
		season   int
		episode  int
		want     string
	}{
		{
			name:     "default_template",
			template: "S{season}E{episode}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			episode:  1,
			want:     "S01E01",
		},
		{
			name:     "full_format",
			template: "{title} ({year}) S{season}E{episode}",
			show:     "The Wire",
			year:     "2002",
			season:   2,
			episode:  10,
			want:     "The Wire (2002) S02E10",
		},
		{
			name:     "separate_codes",
			template: "S{season} E{episode}",
			show:     "Ignored",
			year:     "Ignored",
			season:   5,
			episode:  23,
			want:     "S05 E23",
		},
		{
			name:     "all_variables",
			template: "{title} {year} {season} {episode} S{season} E{episode}",
			show:     "Test",
			year:     "2020",
			season:   3,
			episode:  7,
			want:     "Test 2020 03 07 S03 E07",
		},
		{
			name:     "no_placeholders",
			template: "Episode",
			show:     "Test",
			year:     "2020",
			season:   1,
			episode:  1,
			want:     "Episode",
		},
		{
			name:     "large_numbers",
			template: "S{season}E{episode}",
			show:     "Test",
			year:     "2020",
			season:   100,
			episode:  200,
			want:     "S100E200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FormatConfig{Episode: tt.template}
			ctx := &FormatContext{
				ShowName: tt.show,
				Year:     tt.year,
				Season:   tt.season,
				Episode:  tt.episode,
			}
			got := cfg.ApplyEpisodeTemplate(ctx)
			if got != tt.want {
				t.Errorf("ApplyEpisodeTemplate(%q, %q, %d, %d) = %q, want %q",
					tt.show, tt.year, tt.season, tt.episode, got, tt.want)
			}
		})
	}
}

func TestApplyMovieTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		movie    string
		year     string
		want     string
	}{
		{
			name:     "default_template",
			template: "{title} ({year})",
			movie:    "The Matrix",
			year:     "1999",
			want:     "The Matrix (1999)",
		},
		{
			name:     "movie_only",
			template: "{title}",
			movie:    "Inception",
			year:     "2010",
			want:     "Inception",
		},
		{
			name:     "year_only",
			template: "{year}",
			movie:    "The Dark Knight",
			year:     "2008",
			want:     "2008",
		},
		{
			name:     "custom_format",
			template: "{title} - {year}",
			movie:    "The Dark Knight",
			year:     "2008",
			want:     "The Dark Knight - 2008",
		},
		{
			name:     "brackets_format",
			template: "{title} [{year}]",
			movie:    "Interstellar",
			year:     "2014",
			want:     "Interstellar [2014]",
		},
		{
			name:     "no_placeholders",
			template: "Movies",
			movie:    "Test",
			year:     "2020",
			want:     "Movies",
		},
		{
			name:     "empty_values",
			template: "{title} ({year})",
			movie:    "",
			year:     "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FormatConfig{Movie: tt.template}
			ctx := &FormatContext{
				MovieName: tt.movie,
				Year:      tt.year,
			}
			got := cfg.ApplyMovieTemplate(ctx)
			if got != tt.want {
				t.Errorf("ApplyMovieTemplate(%q, %q) = %q, want %q", tt.movie, tt.year, got, tt.want)
			}
		})
	}
}

func TestNeedsMetadata(t *testing.T) {
	tests := []struct {
		name     string
		config   *FormatConfig
		expected bool
	}{
		{
			name: "no_metadata_variables",
			config: &FormatConfig{
				ShowFolder:   "Show",
				SeasonFolder: "Season",
				Episode:      "Episode",
				Movie:        "Movie",
			},
			expected: false,
		},
		{
			name: "has_title_variable",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year})",
				SeasonFolder: "Season {season}",
				Episode:      "S{season}E{episode}",
				Movie:        "{title} ({year})",
			},
			expected: true,
		},
		{
			name: "has_episode_title",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year})",
				SeasonFolder: "Season {season}",
				Episode:      "{season_code}{episode_code} - {episode_title}",
				Movie:        "{title} ({year})",
			},
			expected: true,
		},
		{
			name: "has_rating",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year}) [{rating}]",
				SeasonFolder: "Season {season}",
				Episode:      "S{season}E{episode}",
				Movie:        "{title} ({year})",
			},
			expected: true,
		},
		{
			name: "has_genres",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year})",
				SeasonFolder: "Season {season}",
				Episode:      "S{season}E{episode}",
				Movie:        "{title} ({year}) - {genres}",
			},
			expected: true,
		},
		{
			name: "has_video_codec",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year})",
				SeasonFolder: "Season {season}",
				Episode:      "S{season}E{episode}",
				Movie:        "{title} ({year}) [{video_codec}]",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.NeedsMetadata()
			if got != tt.expected {
				t.Errorf("NeedsMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestApplyTemplateWithMetadata(t *testing.T) {
	cfg := &FormatConfig{
		ShowFolder:   "{title} ({year}) [{rating}]",
		SeasonFolder: "Season {season} - {genres}",
		Episode:      "S{season}E{episode} - {episode_title}",
		Movie:        "{title} ({year}) - {tagline}",
	}

	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       "The Matrix Reloaded",
			Year:        "2003",
			Rating:      8.7,
			Genres:      []string{"Action", "Sci-Fi"},
			EpisodeName: "Ozymandias",
		},
		Extended: map[string]interface{}{
			"tagline": "Free your mind",
		},
	}

	t.Run("ShowFolderWithMetadata", func(t *testing.T) {
		ctx := &FormatContext{
			ShowName: "Breaking Bad",
			Year:     "2008",
			Metadata: metadata,
			Config:   cfg,
		}
		got := cfg.ApplyShowFolderTemplate(ctx)
		want := "The Matrix Reloaded (2003) [8.7]"
		if got != want {
			t.Errorf("ApplyShowFolderTemplateWithContext() = %q, want %q", got, want)
		}
	})

	t.Run("EpisodeWithMetadata", func(t *testing.T) {
		ctx := &FormatContext{
			ShowName: "Breaking Bad",
			Year:     "2008",
			Season:   5,
			Episode:  14,
			Metadata: metadata,
			Config:   cfg,
		}
		got := cfg.ApplyEpisodeTemplate(ctx)
		want := "S05E14 - Ozymandias"
		if got != want {
			t.Errorf("ApplyEpisodeTemplateWithContext() = %q, want %q", got, want)
		}
	})

	t.Run("MovieWithMetadata", func(t *testing.T) {
		ctx := &FormatContext{
			MovieName: "The Matrix",
			Year:      "1999",
			Metadata:  metadata,
			Config:    cfg,
		}
		got := cfg.ApplyMovieTemplate(ctx)
		want := "The Matrix Reloaded (2003) - Free your mind"
		if got != want {
			t.Errorf("ApplyMovieTemplateWithContext() = %q, want %q", got, want)
		}
	})

	t.Run("FallbackToLocalWhenNoMetadata", func(t *testing.T) {
		ctx := &FormatContext{
			ShowName: "Breaking Bad",
			Year:     "2008",
			Season:   5,
			Episode:  14,
			Metadata: nil,
			Config:   cfg,
		}
		got := cfg.ApplyEpisodeTemplate(ctx)
		want := "S05E14" // CleanName removes the trailing " -"
		if got != want {
			t.Errorf("ApplyEpisodeTemplateWithContext() without metadata = %q, want %q", got, want)
		}
	})

}

func TestResolveVariableComprehensive(t *testing.T) {
	metadata := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title:       "The Matrix",
			Year:        "2003",
			Rating:      8.7,
			Genres:      []string{"Action", "Sci-Fi"},
			EpisodeName: "Ozymandias",
			Overview:    "A computer hacker learns from mysterious rebels about the true nature of his reality and his role in the war against its controllers.",
		},
		Extended: map[string]interface{}{
			"tagline":  "Free your mind",
			"runtime":  136,
			"air_date": "2013-09-15",
		},
	}

	tests := []struct {
		name     string
		template string
		ctx      *FormatContext
		want     string
	}{
		{
			name:     "all_movie_variables",
			template: "{title} - {year} - {rating} - {genres} - {runtime} - {tagline}",
			ctx: &FormatContext{
				MovieName: "Local Movie",
				Year:      "1999",
				Metadata:  metadata,
			},
			want: "The Matrix - 2003 - 8.7 - Action, Sci-Fi - 136 - Free your mind",
		},
		{
			name:     "all_show_variables",
			template: "{title} - {year} - Season {season}",
			ctx: &FormatContext{
				ShowName: "Local Show",
				Year:     "2008",
				Season:   5,
				Metadata: metadata,
			},
			want: "The Matrix - 2003 - Season 05",
		},
		{
			name:     "all_episode_variables",
			template: "{episode_title} - {air_date} - {runtime}",
			ctx: &FormatContext{
				Episode:  14,
				Metadata: metadata,
			},
			want: "Ozymandias - 2013-09-15 - 136",
		},
		{
			name:     "empty_metadata_fields",
			template: "{episode_title} - {air_date} - {tagline}",
			ctx: &FormatContext{
				Metadata: &provider.Metadata{},
			},
			want: "",
		},
		{
			name:     "season_episode_formatting",
			template: "{season} - S{season} - {episode} - E{episode}",
			ctx: &FormatContext{
				Season:  5,
				Episode: 14,
			},
			want: "05 - S05 - 14 - E14",
		},
		{
			name:     "zero_values",
			template: "{season} - {episode} - {rating}",
			ctx: &FormatContext{
				Season:   0,
				Episode:  0,
				Metadata: &provider.Metadata{Core: provider.CoreMetadata{Rating: 0}},
			},
			want: "00 - 00", // Season 0 and Episode 0 are valid (specials/extras)
		},
		{
			name:     "title_fallback_to_movie",
			template: "{title}",
			ctx: &FormatContext{
				MovieName: "Fallback Movie",
				Metadata:  &provider.Metadata{},
			},
			want: "Fallback Movie",
		},
		{
			name:     "title_variable_usage",
			template: "{title}",
			ctx: &FormatContext{
				ShowName: "Fallback Show",
				Metadata: &provider.Metadata{},
			},
			want: "Fallback Show",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FormatConfig{
				ShowFolder:   tt.template,
				SeasonFolder: tt.template,
				Episode:      tt.template,
				Movie:        tt.template,
			}
			// Use the appropriate template function based on context
			var got string
			if tt.ctx.MovieName != "" {
				got = cfg.ApplyMovieTemplate(tt.ctx)
			} else if tt.ctx.Episode > 0 {
				got = cfg.ApplyEpisodeTemplate(tt.ctx)
			} else if tt.ctx.Season > 0 {
				got = cfg.ApplySeasonFolderTemplate(tt.ctx)
			} else {
				got = cfg.ApplyShowFolderTemplate(tt.ctx)
			}

			if got != tt.want {
				t.Errorf("Template resolution = %q, want %q", got, tt.want)
			}
		})
	}
}
