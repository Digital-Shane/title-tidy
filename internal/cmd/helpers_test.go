package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

func TestExtractShowNameFromPath(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		removeExtension bool
		wantShow        string
		wantYear        string
	}{
		{
			name:            "episode with extension",
			path:            "Breaking Bad 2008 S01E01.mkv",
			removeExtension: true,
			wantShow:        "Breaking Bad",
			wantYear:        "2008",
		},
		{
			name:            "episode without extension removal",
			path:            "Breaking Bad 2008 S01E01.mkv",
			removeExtension: false,
			wantShow:        "Breaking Bad",
			wantYear:        "2008",
		},
		{
			name:            "season folder name",
			path:            "Game of Thrones 2011/Season 01",
			removeExtension: false,
			wantShow:        "Game of Thrones",
			wantYear:        "2011",
		},
		{
			name:            "no season pattern found",
			path:            "The Office 2005",
			removeExtension: false,
			wantShow:        "The Office",
			wantYear:        "2005",
		},
		{
			name:            "dotted season episode pattern",
			path:            "Stranger Things 2016 1.01.mkv",
			removeExtension: true,
			wantShow:        "Stranger Things",
			wantYear:        "2016",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShow, gotYear := extractShowNameFromPath(tt.path, tt.removeExtension)
			if gotShow != tt.wantShow {
				t.Errorf("extractShowNameFromPath(%q, %v) show = %q, want %q", tt.path, tt.removeExtension, gotShow, tt.wantShow)
			}
			if gotYear != tt.wantYear {
				t.Errorf("extractShowNameFromPath(%q, %v) year = %q, want %q", tt.path, tt.removeExtension, gotYear, tt.wantYear)
			}
		})
	}
}

func TestFetchMetadata(t *testing.T) {
	// Test with nil provider
	result := fetchMetadata(nil, "Breaking Bad", "", false)
	if result != nil {
		t.Errorf("fetchMetadata(nil, \"Breaking Bad\") = %v, want nil", result)
	}

	// Test with empty name
	provider := &provider.TMDBProvider{}
	result = fetchMetadata(provider, "", "", false)
	if result != nil {
		t.Errorf("fetchMetadata(provider, \"\") = %v, want nil", result)
	}

	// Test movie mode
	result = fetchMetadata(provider, "", "", true)
	if result != nil {
		t.Errorf("fetchMetadata(provider, \"\", \"\", true) = %v, want nil", result)
	}
}

func TestFetchSeasonMetadata(t *testing.T) {
	// Test with nil provider
	result := fetchSeasonMetadata(nil, nil, 1)
	if result != nil {
		t.Errorf("fetchSeasonMetadata(nil, nil, 1) = %v, want nil", result)
	}

	// Test with nil show metadata
	provider := &provider.TMDBProvider{}
	result = fetchSeasonMetadata(provider, nil, 1)
	if result != nil {
		t.Errorf("fetchSeasonMetadata(provider, nil, 1) = %v, want nil", result)
	}
}

func TestFetchEpisodeMetadata(t *testing.T) {
	// Test with nil provider
	result := fetchEpisodeMetadata(nil, nil, 1, 1)
	if result != nil {
		t.Errorf("fetchEpisodeMetadata(nil, nil, 1, 1) = %v, want nil", result)
	}

	// Test with nil show metadata
	provider := &provider.TMDBProvider{}
	result = fetchEpisodeMetadata(provider, nil, 1, 1)
	if result != nil {
		t.Errorf("fetchEpisodeMetadata(provider, nil, 1, 1) = %v, want nil", result)
	}
}

func TestCreateFormatContext(t *testing.T) {
	cfg := &config.FormatConfig{
		ShowFolder: "{show} ({year})",
	}

	ctx := createFormatContext(cfg, "Test Show", "Test Movie", "2023", 1, 2, nil)

	if ctx.ShowName != "Test Show" {
		t.Errorf("createFormatContext ShowName = %q, want %q", ctx.ShowName, "Test Show")
	}
	if ctx.MovieName != "Test Movie" {
		t.Errorf("createFormatContext MovieName = %q, want %q", ctx.MovieName, "Test Movie")
	}
	if ctx.Year != "2023" {
		t.Errorf("createFormatContext Year = %q, want %q", ctx.Year, "2023")
	}
	if ctx.Season != 1 {
		t.Errorf("createFormatContext Season = %d, want %d", ctx.Season, 1)
	}
	if ctx.Episode != 2 {
		t.Errorf("createFormatContext Episode = %d, want %d", ctx.Episode, 2)
	}
	if ctx.Metadata != nil {
		t.Errorf("createFormatContext Metadata = %v, want nil", ctx.Metadata)
	}
	if ctx.Config != cfg {
		t.Errorf("createFormatContext Config = %v, want %v", ctx.Config, cfg)
	}
}

func TestSetDestinationPath(t *testing.T) {
	// Create a mock file info
	fileInfo := mockFileInfo{name: "test.mkv", isDir: false}
	treeFileInfo := treeview.FileInfo{
		FileInfo: fileInfo,
		Path:     "test.mkv",
		Extra:    make(map[string]any),
	}
	node := treeview.NewNode("test", "test.mkv", treeFileInfo)

	// Test with empty linkPath (should do nothing)
	setDestinationPath(node, "", "", "newname.mkv")

	// Test with linkPath but no parentPath
	setDestinationPath(node, "/media/library", "", "newname.mkv")

	// Test with both linkPath and parentPath
	setDestinationPath(node, "/media/library", "Show (2023)", "newname.mkv")
}

func TestApplySeasonRenameWithMetadata(t *testing.T) {
	cfg := &config.FormatConfig{
		SeasonFolder: "Season {season}",
	}

	// Test with mock show metadata that has metadata fetching
	mockProvider := &provider.TMDBProvider{}

	// Create a simple show metadata structure without using literal syntax
	var showMeta *provider.EnrichedMetadata

	result := applySeasonRename(cfg, mockProvider, showMeta, "Test Show", "2023", 2)
	expected := "Season 02"
	if result != expected {
		t.Errorf("applySeasonRename with metadata result = %q, want %q", result, expected)
	}
}

func TestApplyEpisodeRename(t *testing.T) {
	// Create a test node
	fileInfo := mockFileInfo{name: "S01E01.mkv", isDir: false}
	treeFileInfo := treeview.FileInfo{
		FileInfo: fileInfo,
		Path:     "S01E01.mkv",
		Extra:    make(map[string]any),
	}
	node := treeview.NewNode("s01e01", "S01E01.mkv", treeFileInfo)

	cfg := &config.FormatConfig{
		Episode: "{season_code}{episode_code} - {episode_title}",
	}

	// Test without metadata
	result := applyEpisodeRename(node, cfg, nil, nil, "Test Show", "2023", 1, 1)
	if result == "" {
		t.Error("applyEpisodeRename should return a formatted name")
	}
	if !cmp.Equal(result[len(result)-4:], ".mkv") {
		t.Errorf("applyEpisodeRename should preserve extension, got %q", result)
	}
}

func TestApplySeasonRename(t *testing.T) {
	cfg := &config.FormatConfig{
		SeasonFolder: "Season {season}",
	}

	// Test without metadata
	result := applySeasonRename(cfg, nil, nil, "Test Show", "2023", 1)
	expected := "Season 01"
	if result != expected {
		t.Errorf("applySeasonRename(%v, nil, nil, \"Test Show\", \"2023\", 1) = %q, want %q", cfg, result, expected)
	}
}

func TestApplyRename(t *testing.T) {
	cfg := &config.FormatConfig{
		ShowFolder: "{show} ({year})",
		Movie:      "{movie} ({year})",
	}

	// Test show rename without TMDB provider
	result, metadata := applyRename(cfg, nil, "Test Show", "2023", false)
	expected := "Test Show (2023)"
	if result != expected {
		t.Errorf("applyRename show result = %q, want %q", result, expected)
	}
	if metadata != nil {
		t.Errorf("applyRename show metadata = %v, want nil", metadata)
	}

	// Test movie rename without TMDB provider
	result, metadata = applyRename(cfg, nil, "Test Movie", "2023", true)
	expected = "Test Movie (2023)"
	if result != expected {
		t.Errorf("applyRename movie result = %q, want %q", result, expected)
	}
	if metadata != nil {
		t.Errorf("applyRename movie metadata = %v, want nil", metadata)
	}
}
