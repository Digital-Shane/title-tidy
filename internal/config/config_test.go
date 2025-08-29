package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	want := &FormatConfig{
		ShowFolder:          "{show} ({year})",
		SeasonFolder:        "{season_name}",
		Episode:             "{season_code}{episode_code}",
		Movie:               "{movie} ({year})",
		LogRetentionDays:    30,
		EnableLogging:       true,
		TMDBAPIKey:          "",
		EnableTMDBLookup:    false,
		TMDBLanguage:        "en-US",
		PreferLocalMetadata: true,
	}

	if diff := cmp.Diff(want, cfg); diff != "" {
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

func TestLoad_NonExistentFile(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() with non-existent file error = %v, want nil", err)
	}

	// Should return default config
	want := DefaultConfig()
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Load() with non-existent file mismatch (-want +got):\n%s", diff)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Create config directory and file
	configDir := filepath.Join(tempDir, ".title-tidy")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	configData := []byte(`{
		"show_folder": "custom {show}",
		"season_folder": "custom {season_name}",
		"episode": "custom {episode_code}",
		"movie": "custom {movie}",
		"log_retention_days": 60,
		"enable_logging": false
	}`)
	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	want := &FormatConfig{
		ShowFolder:          "custom {show}",
		SeasonFolder:        "custom {season_name}",
		Episode:             "custom {episode_code}",
		Movie:               "custom {movie}",
		LogRetentionDays:    60,
		EnableLogging:       false,
		TMDBAPIKey:          "",
		EnableTMDBLookup:    false,
		TMDBLanguage:        "en-US", // Filled in by Load() with default
		PreferLocalMetadata: false,
	}

	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Load() mismatch (-want +got):\n%s", diff)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Create config directory and file with partial config
	configDir := filepath.Join(tempDir, ".title-tidy")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	configData := []byte(`{
		"show_folder": "custom {show}",
		"log_retention_days": 60
	}`)
	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Should have custom showFolder but default values for missing fields
	if cfg.ShowFolder != "custom {show}" {
		t.Errorf("Load() ShowFolder = %q, want %q", cfg.ShowFolder, "custom {show}")
	}
	if cfg.SeasonFolder != "{season_name}" {
		t.Errorf("Load() SeasonFolder = %q, want default %q", cfg.SeasonFolder, "{season_name}")
	}
	if cfg.Episode != "{season_code}{episode_code}" {
		t.Errorf("Load() Episode = %q, want default %q", cfg.Episode, "{season_code}{episode_code}")
	}
	if cfg.Movie != "{movie} ({year})" {
		t.Errorf("Load() Movie = %q, want default %q", cfg.Movie, "{movie} ({year})")
	}
	if cfg.LogRetentionDays != 60 {
		t.Errorf("Load() LogRetentionDays = %d, want %d", cfg.LogRetentionDays, 60)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Create config directory and invalid JSON file
	configDir := filepath.Join(tempDir, ".title-tidy")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	configData := []byte(`{invalid json}`)
	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() with invalid JSON error = nil, want error")
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
		ShowFolder:       "test {show}",
		SeasonFolder:     "test {season_name}",
		Episode:          "test {episode_code}",
		Movie:            "test {movie}",
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

	if diff := cmp.Diff(cfg, &saved); diff != "" {
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
		if diff := cmp.Diff(want, cfg); diff != "" {
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
			ShowFolder:   "{show} - {year}",
			SeasonFolder: "S{season}",
			Episode:      "{code} {show}",
			Movie:        "{movie} [{year}]",
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
			ShowFolder:          "{show} - {year}",
			SeasonFolder:        "S{season}",
			Episode:             "{code} {show}",
			Movie:               "{movie} [{year}]",
			LogRetentionDays:    30,    // Default value filled in by Load()
			EnableLogging:       false, // Not set in JSON, so false
			TMDBAPIKey:          "",
			EnableTMDBLookup:    false,
			TMDBLanguage:        "en-US", // Default value filled in by Load()
			PreferLocalMetadata: false,   // Not set in JSON, so false
		}

		if diff := cmp.Diff(expectedConfig, cfg); diff != "" {
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
			"show_folder": "{show}",
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
			ShowFolder:          "{show}",
			SeasonFolder:        "{season_name}", // default
			Episode:             "{code}",
			Movie:               "{movie} ({year})", // default
			LogRetentionDays:    30,                 // default
			EnableLogging:       false,              // Not set in JSON, so false
			TMDBAPIKey:          "",
			EnableTMDBLookup:    false,
			TMDBLanguage:        "en-US", // default
			PreferLocalMetadata: false,   // Not set in JSON, so false
		}

		if diff := cmp.Diff(want, cfg); diff != "" {
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

func TestFormatConfig_Save(t *testing.T) {
	t.Run("save_new_config", func(t *testing.T) {
		// Create temp dir for config
		tempDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		t.Setenv("HOME", tempDir)
		defer func() { os.Setenv("HOME", oldHome) }()

		cfg := &FormatConfig{
			ShowFolder:   "{show} - {year}",
			SeasonFolder: "Season {season}",
			Episode:      "{show} {code}",
			Movie:        "{movie} [{year}]",
		}

		err := cfg.Save()
		if err != nil {
			t.Errorf("Save() error = %v, want nil", err)
		}

		// Verify file was created
		configPath := filepath.Join(tempDir, ".title-tidy", "config.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Save() did not create config file")
		}

		// Verify content
		data, _ := os.ReadFile(configPath)
		var loaded FormatConfig
		json.Unmarshal(data, &loaded)

		if diff := cmp.Diff(cfg, &loaded); diff != "" {
			t.Errorf("Saved config mismatch (-want +got):\n%s", diff)
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
			template: "{show} ({year})",
			show:     "Breaking Bad",
			year:     "2008",
			want:     "Breaking Bad (2008)",
		},
		{
			name:     "show_only",
			template: "{show}",
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
			template: "{show} - {year}",
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
			template: "{show} ({year})",
			show:     "",
			year:     "",
			want:     "",
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
			template: "{show} - {season_name}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			want:     "Breaking Bad - Season 01",
		},
		{
			name:     "season_code",
			template: "{show} {season_code}",
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
			template: "{show} - {season_code} - {season_name} - {season}",
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
			template: "{season_code}",
			show:     "Test",
			year:     "2022",
			season:   100,
			want:     "S100",
		},
		{
			name:     "with_year",
			template: "{show} ({year}) - {season_name}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			want:     "Breaking Bad (2008) - Season 01",
		},
		{
			name:     "empty_year",
			template: "{show} ({year}) - {season_name}",
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
			template: "{season_code}{episode_code}",
			show:     "Breaking Bad",
			year:     "2008",
			season:   1,
			episode:  1,
			want:     "S01E01",
		},
		{
			name:     "full_format",
			template: "{show} ({year}) {season_code}{episode_code}",
			show:     "The Wire",
			year:     "2002",
			season:   2,
			episode:  10,
			want:     "The Wire (2002) S02E10",
		},
		{
			name:     "separate_codes",
			template: "{season_code} {episode_code}",
			show:     "Ignored",
			year:     "Ignored",
			season:   5,
			episode:  23,
			want:     "S05 E23",
		},
		{
			name:     "all_variables",
			template: "{show} {year} {season} {episode} {season_code} {episode_code}",
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
			template: "{season_code}{episode_code}",
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
			template: "{movie} ({year})",
			movie:    "The Matrix",
			year:     "1999",
			want:     "The Matrix (1999)",
		},
		{
			name:     "movie_only",
			template: "{movie}",
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
			template: "{movie} - {year}",
			movie:    "The Dark Knight",
			year:     "2008",
			want:     "The Dark Knight - 2008",
		},
		{
			name:     "brackets_format",
			template: "{movie} [{year}]",
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
			template: "{movie} ({year})",
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

func TestCleanName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty_parentheses",
			input: "Movie ()",
			want:  "Movie",
		},
		{
			name:  "empty_brackets",
			input: "Movie []",
			want:  "Movie",
		},
		{
			name:  "empty_braces",
			input: "Movie {}",
			want:  "Movie",
		},
		{
			name:  "empty_angle_brackets",
			input: "Movie <>",
			want:  "Movie",
		},
		{
			name:  "spaces_in_parentheses",
			input: "Movie (  )",
			want:  "Movie",
		},
		{
			name:  "spaces_in_brackets",
			input: "Movie [   ]",
			want:  "Movie",
		},
		{
			name:  "spaces_in_braces",
			input: "Movie { }",
			want:  "Movie",
		},
		{
			name:  "spaces_in_angle_brackets",
			input: "Movie <  >",
			want:  "Movie",
		},
		{
			name:  "multiple_empty_brackets",
			input: "Movie () [] {}",
			want:  "Movie",
		},
		{
			name:  "nested_empty_brackets",
			input: "Movie ([{}])",
			want:  "Movie",
		},
		{
			name:  "keep_non_empty_brackets",
			input: "Movie (2020) [HD]",
			want:  "Movie (2020) [HD]",
		},
		{
			name:  "leading_dash",
			input: "- Movie",
			want:  "Movie",
		},
		{
			name:  "trailing_dash",
			input: "Movie -",
			want:  "Movie",
		},
		{
			name:  "both_dashes",
			input: "- Movie -",
			want:  "Movie",
		},
		{
			name:  "extra_spaces",
			input: "  Movie   ",
			want:  "Movie",
		},
		{
			name:  "complex_mix",
			input: "- Movie () [  ] { } <> -",
			want:  "Movie",
		},
		{
			name:  "empty_string",
			input: "",
			want:  "",
		},
		{
			name:  "only_brackets",
			input: "() [] {} <>",
			want:  "",
		},
		{
			name:  "template_with_missing_year",
			input: "The Matrix ()",
			want:  "The Matrix",
		},
		{
			name:  "template_with_missing_fields",
			input: "Show [] - Season {}",
			want:  "Show - Season",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanName(tt.input)
			if got != tt.want {
				t.Errorf("CleanName(%q) = %q, want %q", tt.input, got, tt.want)
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
				ShowFolder:   "{show} ({year})",
				SeasonFolder: "{season_name}",
				Episode:      "{season_code}{episode_code}",
				Movie:        "{movie} ({year})",
			},
			expected: false,
		},
		{
			name: "has_episode_title",
			config: &FormatConfig{
				ShowFolder:   "{show} ({year})",
				SeasonFolder: "{season_name}",
				Episode:      "{season_code}{episode_code} - {episode_title}",
				Movie:        "{movie} ({year})",
			},
			expected: true,
		},
		{
			name: "has_rating",
			config: &FormatConfig{
				ShowFolder:   "{show} ({year}) [{rating}]",
				SeasonFolder: "{season_name}",
				Episode:      "{season_code}{episode_code}",
				Movie:        "{movie} ({year})",
			},
			expected: true,
		},
		{
			name: "has_genres",
			config: &FormatConfig{
				ShowFolder:   "{show} ({year})",
				SeasonFolder: "{season_name}",
				Episode:      "{season_code}{episode_code}",
				Movie:        "{movie} ({year}) - {genres}",
			},
			expected: true,
		},
		{
			name: "has_title_variable",
			config: &FormatConfig{
				ShowFolder:   "{title} ({year})",
				SeasonFolder: "{season_name}",
				Episode:      "{season_code}{episode_code}",
				Movie:        "{movie} ({year})",
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
		ShowFolder:          "{show} ({year}) [{rating}]",
		SeasonFolder:        "{season_name} - {genres}",
		Episode:             "{season_code}{episode_code} - {episode_title}",
		Movie:               "{movie} ({year}) - {tagline}",
		PreferLocalMetadata: false,
	}

	metadata := &provider.EnrichedMetadata{
		Title:       "The Matrix Reloaded",
		ShowName:    "Breaking Bad",
		Year:        "2003",
		Rating:      8.7,
		Genres:      []string{"Action", "Sci-Fi"},
		EpisodeName: "Ozymandias",
		Tagline:     "Free your mind",
	}

	t.Run("ShowFolderWithMetadata", func(t *testing.T) {
		ctx := &FormatContext{
			ShowName: "Breaking Bad",
			Year:     "2008",
			Metadata: metadata,
			Config:   cfg,
		}
		got := cfg.ApplyShowFolderTemplate(ctx)
		want := "Breaking Bad (2003) [8.7]"
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

	t.Run("PreferLocalMetadata", func(t *testing.T) {
		cfgLocal := &FormatConfig{
			ShowFolder:          "{show} ({year})",
			PreferLocalMetadata: true,
		}
		ctx := &FormatContext{
			ShowName: "Local Show",
			Year:     "2020",
			Metadata: metadata,
			Config:   cfgLocal,
		}
		got := cfgLocal.ApplyShowFolderTemplate(ctx)
		want := "Local Show (2020)"
		if got != want {
			t.Errorf("ApplyShowFolderTemplateWithContext() with PreferLocal = %q, want %q", got, want)
		}
	})
}

func TestResolveVariableComprehensive(t *testing.T) {
	metadata := &provider.EnrichedMetadata{
		Title:       "The Matrix",
		ShowName:    "Breaking Bad",
		Year:        "2003",
		Rating:      8.7,
		Genres:      []string{"Action", "Sci-Fi"},
		EpisodeName: "Ozymandias",
		EpisodeAir:  "2013-09-15",
		SeasonName:  "Season Five",
		Runtime:     136,
		Tagline:     "Free your mind",
		Overview:    "A computer hacker learns from mysterious rebels about the true nature of his reality and his role in the war against its controllers.",
	}

	tests := []struct {
		name     string
		template string
		ctx      *FormatContext
		want     string
	}{
		{
			name:     "all_movie_variables",
			template: "{movie} - {title} - {year} - {rating} - {genres} - {runtime} - {tagline}",
			ctx: &FormatContext{
				MovieName: "Local Movie",
				Year:      "1999",
				Metadata:  metadata,
			},
			want: "The Matrix - The Matrix - 2003 - 8.7 - Action, Sci-Fi - 136 - Free your mind",
		},
		{
			name:     "all_show_variables",
			template: "{show} - {title} - {year} - {season_name}",
			ctx: &FormatContext{
				ShowName: "Local Show",
				Year:     "2008",
				Season:   5,
				Metadata: metadata,
			},
			want: "Breaking Bad - The Matrix - 2003 - Season Five",
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
			name:     "overview_truncation",
			template: "{overview}",
			ctx: &FormatContext{
				Metadata: metadata,
			},
			want: "A computer hacker learns from mysterious rebels about the true nature of his reality and his role...",
		},
		{
			name:     "empty_metadata_fields",
			template: "{episode_title} - {air_date} - {tagline}",
			ctx: &FormatContext{
				Metadata: &provider.EnrichedMetadata{},
			},
			want: "",
		},
		{
			name:     "season_codes",
			template: "{season} - {season_code} - {episode} - {episode_code}",
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
				Metadata: &provider.EnrichedMetadata{Rating: 0},
			},
			want: "",
		},
		{
			name:     "title_fallback_to_movie",
			template: "{title}",
			ctx: &FormatContext{
				MovieName: "Fallback Movie",
				Metadata:  &provider.EnrichedMetadata{},
			},
			want: "Fallback Movie",
		},
		{
			name:     "title_fallback_to_show",
			template: "{title}",
			ctx: &FormatContext{
				ShowName: "Fallback Show",
				Metadata: &provider.EnrichedMetadata{},
			},
			want: "Fallback Show",
		},
		{
			name:     "prefer_local_metadata",
			template: "{show} - {year}",
			ctx: &FormatContext{
				ShowName: "Local Show",
				Year:     "2020",
				Metadata: metadata,
				Config:   &FormatConfig{PreferLocalMetadata: true},
			},
			want: "Local Show - 2020",
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

func TestExtractNameAndYear(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantYear string
	}{
		{
			name:     "movie_with_year",
			input:    "The Matrix (1999)",
			wantName: "The Matrix",
			wantYear: "1999",
		},
		{
			name:     "movie_with_year_brackets",
			input:    "The Matrix [1999]",
			wantName: "The Matrix",
			wantYear: "1999",
		},
		{
			name:     "movie_no_year",
			input:    "The Matrix",
			wantName: "The Matrix",
			wantYear: "",
		},
		{
			name:     "year_only",
			input:    "(2020)",
			wantName: "",
			wantYear: "2020",
		},
		{
			name:     "complex_name",
			input:    "The.Matrix.1999.1080p.BluRay",
			wantName: "The Matrix",
			wantYear: "1999",
		},
		{
			name:     "dots_and_underscores",
			input:    "The_Matrix_1999",
			wantName: "The Matrix 1999",
			wantYear: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotYear := ExtractNameAndYear(tt.input)
			if gotName != tt.wantName || gotYear != tt.wantYear {
				t.Errorf("ExtractNameAndYear(%q) = (%q, %q), want (%q, %q)",
					tt.input, gotName, gotYear, tt.wantName, tt.wantYear)
			}
		})
	}
}
