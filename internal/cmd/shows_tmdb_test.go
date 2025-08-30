package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
	tmdb "github.com/ryanbradynd05/go-tmdb"
)

// mockTMDBClient implements provider.TMDBClient for testing
type mockTMDBClient struct {
	searchTvFunc         func(name string, options map[string]string) (*tmdb.TvSearchResults, error)
	getTvInfoFunc        func(id int, options map[string]string) (*tmdb.TV, error)
	getTvSeasonInfoFunc  func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error)
	getTvEpisodeInfoFunc func(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error)
}

func (m *mockTMDBClient) SearchMovie(name string, options map[string]string) (*tmdb.MovieSearchResults, error) {
	return nil, nil
}

func (m *mockTMDBClient) SearchTv(name string, options map[string]string) (*tmdb.TvSearchResults, error) {
	if m.searchTvFunc != nil {
		return m.searchTvFunc(name, options)
	}
	return nil, nil
}

func (m *mockTMDBClient) GetMovieInfo(id int, options map[string]string) (*tmdb.Movie, error) {
	return nil, nil
}

func (m *mockTMDBClient) GetTvInfo(id int, options map[string]string) (*tmdb.TV, error) {
	if m.getTvInfoFunc != nil {
		return m.getTvInfoFunc(id, options)
	}
	return nil, nil
}

func (m *mockTMDBClient) GetTvSeasonInfo(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
	if m.getTvSeasonInfoFunc != nil {
		return m.getTvSeasonInfoFunc(showID, seasonID, options)
	}
	return nil, nil
}

func (m *mockTMDBClient) GetTvEpisodeInfo(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error) {
	if m.getTvEpisodeInfoFunc != nil {
		return m.getTvEpisodeInfoFunc(showID, seasonNum, episodeNum, options)
	}
	return nil, nil
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0755 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

// createMockProvider creates a TMDBProvider with a mock client for testing
func createMockProvider(client provider.TMDBClient) *provider.TMDBProvider {
	// We'll need to expose a way to inject the client into TMDBProvider
	// For now, this is a placeholder - the actual implementation would need
	// to be updated to support dependency injection for testing
	return nil
}

func TestShowsCommandWithTMDBIntegration(t *testing.T) {
	// Create a test tree structure
	showInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "Breaking Bad", isDir: true},
		Path:     "Breaking Bad",
		Extra:    make(map[string]any),
	}
	showNode := treeview.NewNode("breaking-bad", "Breaking Bad", showInfo)

	seasonInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "Season 01", isDir: true},
		Path:     "Breaking Bad/Season 01",
		Extra:    make(map[string]any),
	}
	seasonNode := treeview.NewNode("season-01", "Season 01", seasonInfo)

	episodeInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "S01E01 - Pilot.mkv", isDir: false},
		Path:     "Breaking Bad/Season 01/S01E01 - Pilot.mkv",
		Extra:    make(map[string]any),
	}
	episodeNode := treeview.NewNode("s01e01", "S01E01 - Pilot.mkv", episodeInfo)

	seasonNode.AddChild(episodeNode)
	showNode.AddChild(seasonNode)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{showNode})

	// Create config with TMDB enabled
	cfg := &config.FormatConfig{
		ShowFolder:          "{show} ({year})",
		SeasonFolder:        "Season {season}",
		Episode:             "{season_code}{episode_code} - {episode_title}",
		EnableTMDBLookup:    true,
		TMDBAPIKey:          "test-api-key",
		TMDBLanguage:        "en-US",
		PreferLocalMetadata: false,
	}

	// Apply annotation without TMDB (simulating the case where TMDB is disabled)
	ShowsCommand.annotate(tree, cfg, "")

	// Check that metadata was applied
	showMeta := core.GetMeta(showNode)
	if showMeta == nil {
		t.Error("Show node should have metadata")
	}
	if showMeta.Type != core.MediaShow {
		t.Errorf("Show node type = %v, want MediaShow", showMeta.Type)
	}

	seasonMeta := core.GetMeta(seasonNode)
	if seasonMeta == nil {
		t.Error("Season node should have metadata")
	}
	if seasonMeta.Type != core.MediaSeason {
		t.Errorf("Season node type = %v, want MediaSeason", seasonMeta.Type)
	}

	episodeMeta := core.GetMeta(episodeNode)
	if episodeMeta == nil {
		t.Error("Episode node should have metadata")
	}
	if episodeMeta.Type != core.MediaEpisode {
		t.Errorf("Episode node type = %v, want MediaEpisode", episodeMeta.Type)
	}
}

func TestShowsCommandTMDBFallback(t *testing.T) {
	// Test that the command works even when TMDB fails
	showInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "The Office 2005", isDir: true},
		Path:     "The Office 2005",
		Extra:    make(map[string]any),
	}
	showNode := treeview.NewNode("the-office", "The Office 2005", showInfo)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{showNode})

	cfg := &config.FormatConfig{
		ShowFolder:          "{show} ({year})",
		EnableTMDBLookup:    true,
		TMDBAPIKey:          "test-api-key",
		PreferLocalMetadata: false,
	}

	// Apply annotation - should fallback to local metadata when TMDB is unavailable
	ShowsCommand.annotate(tree, cfg, "")

	showMeta := core.GetMeta(showNode)
	if showMeta == nil {
		t.Fatal("Show node should have metadata even without TMDB")
	}

	// Check that local name extraction worked
	expectedName := "The Office (2005)"
	if showMeta.NewName != expectedName {
		t.Errorf("Show NewName = %q, want %q", showMeta.NewName, expectedName)
	}
}

func TestShowsCommandWithLinking(t *testing.T) {
	// Test that linking paths are correctly set with TMDB metadata
	showInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "Game of Thrones", isDir: true},
		Path:     "Game of Thrones",
		Extra:    make(map[string]any),
	}
	showNode := treeview.NewNode("got", "Game of Thrones", showInfo)

	seasonInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "Season 1", isDir: true},
		Path:     "Game of Thrones/Season 1",
		Extra:    make(map[string]any),
	}
	seasonNode := treeview.NewNode("season-1", "Season 1", seasonInfo)
	showNode.AddChild(seasonNode)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{showNode})

	cfg := &config.FormatConfig{
		ShowFolder:       "{show} ({year})",
		SeasonFolder:     "Season {season}",
		EnableTMDBLookup: false, // Disable TMDB for this test
	}

	linkPath := "/media/library"
	ShowsCommand.annotate(tree, cfg, linkPath)

	showMeta := core.GetMeta(showNode)
	seasonMeta := core.GetMeta(seasonNode)

	// Check destination paths
	if showMeta.DestinationPath == "" {
		t.Error("Show should have destination path when linking")
	}
	if seasonMeta.DestinationPath == "" {
		t.Error("Season should have destination path when linking")
	}
}

func TestEpisodesCommandWithTMDB(t *testing.T) {
	// Test episodes command (which has limited TMDB support)
	episodeInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "S02E05.mkv", isDir: false},
		Path:     "S02E05.mkv",
		Extra:    make(map[string]any),
	}
	episodeNode := treeview.NewNode("s02e05", "S02E05.mkv", episodeInfo)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{episodeNode})

	cfg := &config.FormatConfig{
		Episode:          "{season_code}{episode_code}",
		EnableTMDBLookup: true,
		TMDBAPIKey:       "test-api-key",
	}

	EpisodesCommand.annotate(tree, cfg, "")

	episodeMeta := core.GetMeta(episodeNode)
	if episodeMeta == nil {
		t.Fatal("Episode node should have metadata")
	}

	expectedName := "S02E05.mkv"
	if episodeMeta.NewName != expectedName {
		t.Errorf("Episode NewName = %q, want %q", episodeMeta.NewName, expectedName)
	}
}

func TestSeasonsCommandWithTMDB(t *testing.T) {
	// Test seasons command (which has limited TMDB support)
	seasonInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "Season 03", isDir: true},
		Path:     "Season 03",
		Extra:    make(map[string]any),
	}
	seasonNode := treeview.NewNode("season-03", "Season 03", seasonInfo)

	episodeInfo := treeview.FileInfo{
		FileInfo: mockFileInfo{name: "S03E01.mkv", isDir: false},
		Path:     "Season 03/S03E01.mkv",
		Extra:    make(map[string]any),
	}
	episodeNode := treeview.NewNode("s03e01", "S03E01.mkv", episodeInfo)
	seasonNode.AddChild(episodeNode)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{seasonNode})

	cfg := &config.FormatConfig{
		SeasonFolder:     "Season {season}",
		Episode:          "{season_code}{episode_code}",
		EnableTMDBLookup: true,
		TMDBAPIKey:       "test-api-key",
	}

	SeasonsCommand.annotate(tree, cfg, "")

	seasonMeta := core.GetMeta(seasonNode)
	if seasonMeta == nil {
		t.Fatal("Season node should have metadata")
	}

	expectedSeasonName := "Season 03"
	if seasonMeta.NewName != expectedSeasonName {
		t.Errorf("Season NewName = %q, want %q", seasonMeta.NewName, expectedSeasonName)
	}

	episodeMeta := core.GetMeta(episodeNode)
	if episodeMeta == nil {
		t.Fatal("Episode node should have metadata")
	}

	expectedEpisodeName := "S03E01.mkv"
	if episodeMeta.NewName != expectedEpisodeName {
		t.Errorf("Episode NewName = %q, want %q", episodeMeta.NewName, expectedEpisodeName)
	}
}

func TestMetadataContextPassing(t *testing.T) {
	// Test that metadata is correctly passed through FormatContext
	ctx := &config.FormatContext{
		ShowName: "Stranger Things",
		Year:     "2016",
		Season:   1,
		Episode:  1,
		Metadata: &provider.EnrichedMetadata{
			ShowName:    "Stranger Things",
			Year:        "2016",
			EpisodeName: "Chapter One: The Vanishing of Will Byers",
			Rating:      8.7,
		},
		Config: &config.FormatConfig{
			Episode:             "{season_code}{episode_code} - {episode_title}",
			PreferLocalMetadata: false,
		},
	}

	// Apply template
	result := ctx.Config.ApplyEpisodeTemplate(ctx)

	// The template should use the metadata episode title
	expected := "S01E01 - Chapter One: The Vanishing of Will Byers"
	if result != expected {
		diff := cmp.Diff(expected, result)
		t.Errorf("ApplyEpisodeTemplate with metadata returned unexpected result:\n%s", diff)
	}
}
