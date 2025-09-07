package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
	tmdb "github.com/ryanbradynd05/go-tmdb"
)

// mockMovieTMDBClient implements TMDBClient for testing
type mockMovieTMDBClient struct {
	searchMovieFunc func(name string, options map[string]string) (*tmdb.MovieSearchResults, error)
	getMovieFunc    func(id int, options map[string]string) (*tmdb.Movie, error)
}

func (m *mockMovieTMDBClient) SearchMovie(name string, options map[string]string) (*tmdb.MovieSearchResults, error) {
	if m.searchMovieFunc != nil {
		return m.searchMovieFunc(name, options)
	}
	return nil, nil
}

func (m *mockMovieTMDBClient) GetMovieInfo(id int, options map[string]string) (*tmdb.Movie, error) {
	if m.getMovieFunc != nil {
		return m.getMovieFunc(id, options)
	}
	return nil, nil
}

func (m *mockMovieTMDBClient) SearchTv(name string, options map[string]string) (*tmdb.TvSearchResults, error) {
	return nil, nil
}

func (m *mockMovieTMDBClient) GetTvInfo(id int, options map[string]string) (*tmdb.TV, error) {
	return nil, nil
}

func (m *mockMovieTMDBClient) GetTvSeasonInfo(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
	return nil, nil
}

func (m *mockMovieTMDBClient) GetTvEpisodeInfo(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error) {
	return nil, nil
}

func TestMovieRenameWithTMDB(t *testing.T) {
	// Create a mock TMDB client
	mockClient := &mockMovieTMDBClient{
		searchMovieFunc: func(name string, options map[string]string) (*tmdb.MovieSearchResults, error) {
			if name == "The Matrix" {
				return &tmdb.MovieSearchResults{
					Results: []tmdb.MovieShort{
						{
							ID:          603,
							Title:       "The Matrix",
							ReleaseDate: "1999-03-31",
							Overview:    "A computer hacker learns about the true nature of reality",
							VoteAverage: 8.2,
						},
					},
				}, nil
			}
			return &tmdb.MovieSearchResults{}, nil
		},
		getMovieFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
			if id == 603 {
				return &tmdb.Movie{
					ID:          603,
					Title:       "The Matrix",
					ReleaseDate: "1999-03-31",
					Overview:    "A computer hacker learns about the true nature of reality",
					VoteAverage: 8.2,
					Runtime:     136,
					Tagline:     "Welcome to the Real World",
					Genres: []struct {
						ID   int
						Name string
					}{
						{ID: 28, Name: "Action"},
						{ID: 878, Name: "Science Fiction"},
					},
				}, nil
			}
			return nil, nil
		},
	}

	// Create provider and set mock client
	tmdbProvider, _ := provider.NewTMDBProvider("test_key", "en-US")
	tmdbProvider.SetClient(mockClient)

	t.Run("MoviePreprocessWithMetadata", func(t *testing.T) {
		cfg := &config.FormatConfig{
			Movie:            "{title} ({year})",
			EnableTMDBLookup: true,
			TMDBAPIKey:       "test_key",
		}

		// Test with the applyRename function directly
		newName, _ := applyRename(cfg, tmdbProvider, "The Matrix", "1999", true)
		want := "The Matrix (1999)"
		if newName != want {
			t.Errorf("applyRename() with TMDB = %q, want %q", newName, want)
		}

		// Test MoviePreprocess without TMDB (normal operation)
		cfg.EnableTMDBLookup = false
		nodes := []*treeview.Node[treeview.FileInfo]{
			treeview.NewNode("The.Matrix.1999.1080p.mkv", "The.Matrix.1999.1080p.mkv",
				treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("The.Matrix.1999.1080p.mkv", false)}),
		}

		cmdCfg := &CommandConfig{Config: cfg}
		result := MoviePreprocess(nodes, cmdCfg)

		if len(result) != 1 {
			t.Errorf("MoviePreprocess() returned %d nodes, want 1", len(result))
		}

		// Check virtual directory was created
		if !result[0].Data().IsDir() {
			t.Error("MoviePreprocess() did not create virtual directory")
		}

		meta := core.GetMeta(result[0])
		if meta == nil {
			t.Fatal("MoviePreprocess() did not attach metadata")
		}

		// Without TMDB, it should just clean the name
		if meta.NewName != "The Matrix (1999)" {
			t.Errorf("MoviePreprocess() NewName = %q, want %q", meta.NewName, "The Matrix (1999)")
		}
	})

	t.Run("MovieAnnotateWithMetadata", func(t *testing.T) {
		cfg := &config.FormatConfig{
			Movie:            "{title} ({year})",
			EnableTMDBLookup: false, // MovieAnnotate doesn't use TMDB directly in current implementation
			TMDBAPIKey:       "test_key",
		}

		// Create a tree with a movie directory
		root := treeview.NewNode("The.Matrix.1999", "The.Matrix.1999",
			treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("The.Matrix.1999", true)})

		tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{root})

		MovieAnnotate(tree, cfg, "", nil)

		meta := core.GetMeta(root)
		if meta == nil {
			t.Fatal("MovieAnnotate() did not attach metadata")
		}

		want := "The Matrix (1999)"
		if meta.NewName != want {
			t.Errorf("MovieAnnotate() NewName = %q, want %q", meta.NewName, want)
		}
	})

	t.Run("HelperFunctions", func(t *testing.T) {
		cfg := &config.FormatConfig{
			Movie:            "{title} ({year})",
			EnableTMDBLookup: true,
		}

		// Test fetchMetadata for movies
		metadata := fetchMetadata(tmdbProvider, "The Matrix", "1999", true)
		if metadata == nil {
			t.Fatal("fetchMetadata() returned nil")
		}

		if metadata.Title != "The Matrix" {
			t.Errorf("fetchMetadata() Title = %q, want %q", metadata.Title, "The Matrix")
		}

		// Test applyRename for movies
		newName, meta := applyRename(cfg, tmdbProvider, "The Matrix", "1999", true)

		wantName := "The Matrix (1999)"
		if newName != wantName {
			t.Errorf("applyRename() name = %q, want %q", newName, wantName)
		}

		if meta == nil {
			t.Error("applyRename() metadata = nil, want non-nil")
		}
	})
}

func TestMovieRenameWithoutTMDB(t *testing.T) {
	cfg := &config.FormatConfig{
		Movie:            "{title} ({year})",
		EnableTMDBLookup: false,
	}

	// Test that it still works without TMDB
	nodes := []*treeview.Node[treeview.FileInfo]{
		treeview.NewNode("Inception.2010.BluRay.mkv", "Inception.2010.BluRay.mkv",
			treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("Inception.2010.BluRay.mkv", false)}),
	}

	cmdCfg := &CommandConfig{Config: cfg}
	result := MoviePreprocess(nodes, cmdCfg)

	if len(result) != 1 {
		t.Errorf("MoviePreprocess() without TMDB returned %d nodes, want 1", len(result))
	}

	meta := core.GetMeta(result[0])
	if meta == nil {
		t.Fatal("MoviePreprocess() without TMDB did not attach metadata")
	}

	want := "Inception (2010)"
	if meta.NewName != want {
		t.Errorf("MoviePreprocess() without TMDB NewName = %q, want %q", meta.NewName, want)
	}
}

func TestMovieMetadataVariables(t *testing.T) {
	// Create a mock TMDB client with rich metadata
	mockClient := &mockMovieTMDBClient{
		searchMovieFunc: func(name string, options map[string]string) (*tmdb.MovieSearchResults, error) {
			return &tmdb.MovieSearchResults{
				Results: []tmdb.MovieShort{
					{
						ID:          1234,
						Title:       "Test Movie",
						ReleaseDate: "2023-05-15",
						VoteAverage: 7.5,
					},
				},
			}, nil
		},
		getMovieFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
			return &tmdb.Movie{
				ID:          1234,
				Title:       "Test Movie",
				ReleaseDate: "2023-05-15",
				VoteAverage: 7.5,
				Runtime:     120,
				Tagline:     "An amazing test movie",
				Genres: []struct {
					ID   int
					Name string
				}{
					{ID: 28, Name: "Action"},
					{ID: 12, Name: "Adventure"},
				},
			}, nil
		},
	}

	tmdbProvider, _ := provider.NewTMDBProvider("test_key", "en-US")
	tmdbProvider.SetClient(mockClient)

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "Title and year",
			template: "{title} ({year})",
			want:     "Test Movie (2023)",
		},
		{
			name:     "With rating",
			template: "{title} [{rating}]",
			want:     "Test Movie [7.5]",
		},
		{
			name:     "With runtime",
			template: "{title} - {runtime}min",
			want:     "Test Movie - 120min",
		},
		{
			name:     "With genres",
			template: "{title} - {genres}",
			want:     "Test Movie - Action, Adventure",
		},
		{
			name:     "With tagline",
			template: "{title} - {tagline}",
			want:     "Test Movie - An amazing test movie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.FormatConfig{
				Movie:            tt.template,
				EnableTMDBLookup: true,
			}

			newName, _ := applyRename(cfg, tmdbProvider, "Test Movie", "2023", true)

			if newName != tt.want {
				t.Errorf("applyRename() with template %q = %q, want %q", tt.template, newName, tt.want)
			}
		})
	}
}
