package provider

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	tmdb "github.com/ryanbradynd05/go-tmdb"
)

// mockTMDBClient implements TMDBClient for testing
type mockTMDBClient struct {
	searchMovieFunc      func(name string, options map[string]string) (*tmdb.MovieSearchResults, error)
	searchTvFunc         func(name string, options map[string]string) (*tmdb.TvSearchResults, error)
	getMovieInfoFunc     func(id int, options map[string]string) (*tmdb.Movie, error)
	getTvInfoFunc        func(id int, options map[string]string) (*tmdb.TV, error)
	getTvSeasonInfoFunc  func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error)
	getTvEpisodeInfoFunc func(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error)
}

func (m *mockTMDBClient) SearchMovie(name string, options map[string]string) (*tmdb.MovieSearchResults, error) {
	if m.searchMovieFunc != nil {
		return m.searchMovieFunc(name, options)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTMDBClient) SearchTv(name string, options map[string]string) (*tmdb.TvSearchResults, error) {
	if m.searchTvFunc != nil {
		return m.searchTvFunc(name, options)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTMDBClient) GetMovieInfo(id int, options map[string]string) (*tmdb.Movie, error) {
	if m.getMovieInfoFunc != nil {
		return m.getMovieInfoFunc(id, options)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTMDBClient) GetTvInfo(id int, options map[string]string) (*tmdb.TV, error) {
	if m.getTvInfoFunc != nil {
		return m.getTvInfoFunc(id, options)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTMDBClient) GetTvSeasonInfo(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
	if m.getTvSeasonInfoFunc != nil {
		return m.getTvSeasonInfoFunc(showID, seasonID, options)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTMDBClient) GetTvEpisodeInfo(showID, seasonNum, episodeNum int, options map[string]string) (*tmdb.TvEpisode, error) {
	if m.getTvEpisodeInfoFunc != nil {
		return m.getTvEpisodeInfoFunc(showID, seasonNum, episodeNum, options)
	}
	return nil, errors.New("not implemented")
}

func TestNewTMDBProvider(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		language string
		wantErr  bool
	}{
		{
			name:     "valid_api_key",
			apiKey:   "test-api-key",
			language: "en-US",
			wantErr:  false,
		},
		{
			name:     "empty_api_key",
			apiKey:   "",
			language: "en-US",
			wantErr:  true,
		},
		{
			name:     "default_language",
			apiKey:   "test-api-key",
			language: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewTMDBProvider(tt.apiKey, tt.language)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewTMDBProvider(%q, %q) error = nil, want error", tt.apiKey, tt.language)
				}
				if !errors.Is(err, ErrInvalidAPIKey) {
					t.Errorf("NewTMDBProvider(%q, %q) error = %v, want %v", tt.apiKey, tt.language, err, ErrInvalidAPIKey)
				}
			} else {
				if err != nil {
					t.Errorf("NewTMDBProvider(%q, %q) error = %v, want nil", tt.apiKey, tt.language, err)
				}
				if provider == nil {
					t.Errorf("NewTMDBProvider(%q, %q) = nil, want provider", tt.apiKey, tt.language)
				}
			}
		})
	}
}

func TestSearchMovie(t *testing.T) {
	tests := []struct {
		name        string
		movieName   string
		movieYear   string
		mockFunc    func(query string, options map[string]string) (*tmdb.MovieSearchResults, error)
		getInfoFunc func(id int, options map[string]string) (*tmdb.Movie, error)
		want        *EnrichedMetadata
		wantErr     bool
	}{
		{
			name:      "successful_search_with_full_details",
			movieName: "The Matrix",
			movieYear: "1999",
			mockFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
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
			},
			getInfoFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
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
			},
			want: &EnrichedMetadata{
				Title:     "The Matrix",
				Year:      "1999",
				Overview:  "A computer hacker learns about the true nature of reality",
				Rating:    8.2,
				Genres:    []string{"Action", "Science Fiction"},
				Runtime:   136,
				Tagline:   "Welcome to the Real World",
				ID:        603,
				LocalName: "The Matrix",
				LocalYear: "1999",
			},
			wantErr: false,
		},
		{
			name:      "successful_search_fallback_to_search_results",
			movieName: "Inception",
			movieYear: "2010",
			mockFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
				return &tmdb.MovieSearchResults{
					Results: []tmdb.MovieShort{
						{
							ID:          27205,
							Title:       "Inception",
							ReleaseDate: "2010-07-16",
							Overview:    "A thief who steals corporate secrets",
							VoteAverage: 8.4,
						},
					},
				}, nil
			},
			getInfoFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
				return nil, errors.New("API error")
			},
			want: &EnrichedMetadata{
				Title:     "Inception",
				Year:      "2010",
				Overview:  "A thief who steals corporate secrets",
				Rating:    8.4,
				ID:        27205,
				LocalName: "Inception",
				LocalYear: "2010",
			},
			wantErr: false,
		},
		{
			name:      "no_results",
			movieName: "NonexistentMovie",
			movieYear: "2099",
			mockFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
				return &tmdb.MovieSearchResults{
					Results: []tmdb.MovieShort{},
				}, nil
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:      "api_error",
			movieName: "The Matrix",
			movieYear: "1999",
			mockFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
				return nil, errors.New("401 Unauthorized")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:      "empty_movie_name",
			movieName: "",
			movieYear: "1999",
			mockFunc:  nil,
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			mockClient := &mockTMDBClient{
				searchMovieFunc:  tt.mockFunc,
				getMovieInfoFunc: tt.getInfoFunc,
			}
			provider.SetClient(mockClient)

			got, err := provider.SearchMovie(tt.movieName, tt.movieYear)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchMovie(%q, %q) error = nil, want error", tt.movieName, tt.movieYear)
				}
			} else {
				if err != nil {
					t.Errorf("SearchMovie(%q, %q) error = %v, want nil", tt.movieName, tt.movieYear, err)
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("SearchMovie(%q, %q) mismatch (-want +got):\n%s", tt.movieName, tt.movieYear, diff)
				}
			}
		})
	}
}

func TestSearchTVShow(t *testing.T) {
	tests := []struct {
		name        string
		showName    string
		mockFunc    func(query string, options map[string]string) (*tmdb.TvSearchResults, error)
		getInfoFunc func(id int, options map[string]string) (*tmdb.TV, error)
		want        *EnrichedMetadata
		wantErr     bool
	}{
		{
			name:     "successful_search_with_full_details",
			showName: "Breaking Bad",
			mockFunc: func(query string, options map[string]string) (*tmdb.TvSearchResults, error) {
				return &tmdb.TvSearchResults{
					Results: []struct {
						BackdropPath  string `json:"backdrop_path"`
						ID            int
						OriginalName  string   `json:"original_name"`
						FirstAirDate  string   `json:"first_air_date"`
						OriginCountry []string `json:"origin_country"`
						PosterPath    string   `json:"poster_path"`
						Popularity    float32
						Name          string
						VoteAverage   float32 `json:"vote_average"`
						VoteCount     uint32  `json:"vote_count"`
					}{
						{
							ID:           1396,
							Name:         "Breaking Bad",
							FirstAirDate: "2008-01-20",
							VoteAverage:  8.9,
						},
					},
				}, nil
			},
			getInfoFunc: func(id int, options map[string]string) (*tmdb.TV, error) {
				return &tmdb.TV{
					ID:              1396,
					Name:            "Breaking Bad",
					FirstAirDate:    "2008-01-20",
					Overview:        "A high school chemistry teacher turned meth maker",
					VoteAverage:     8.9,
					EpisodeRunTime:  []int{45, 47},
					NumberOfSeasons: 5,
					Genres: []struct {
						ID   int
						Name string
					}{
						{ID: 18, Name: "Drama"},
						{ID: 80, Name: "Crime"},
					},
				}, nil
			},
			want: &EnrichedMetadata{
				ShowName:    "Breaking Bad",
				Title:       "Breaking Bad",
				Year:        "2008",
				Overview:    "A high school chemistry teacher turned meth maker",
				Rating:      8.9,
				Genres:      []string{"Drama", "Crime"},
				Runtime:     45,
				SeasonCount: 5,
				ID:          1396,
				LocalName:   "Breaking Bad",
			},
			wantErr: false,
		},
		{
			name:     "no_results",
			showName: "NonexistentShow",
			mockFunc: func(query string, options map[string]string) (*tmdb.TvSearchResults, error) {
				return &tmdb.TvSearchResults{
					Results: []struct {
						BackdropPath  string `json:"backdrop_path"`
						ID            int
						OriginalName  string   `json:"original_name"`
						FirstAirDate  string   `json:"first_air_date"`
						OriginCountry []string `json:"origin_country"`
						PosterPath    string   `json:"poster_path"`
						Popularity    float32
						Name          string
						VoteAverage   float32 `json:"vote_average"`
						VoteCount     uint32  `json:"vote_count"`
					}{},
				}, nil
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:     "successful_search_fallback_to_search_results",
			showName: "Game of Thrones",
			mockFunc: func(query string, options map[string]string) (*tmdb.TvSearchResults, error) {
				return &tmdb.TvSearchResults{
					Results: []struct {
						BackdropPath  string `json:"backdrop_path"`
						ID            int
						OriginalName  string   `json:"original_name"`
						FirstAirDate  string   `json:"first_air_date"`
						OriginCountry []string `json:"origin_country"`
						PosterPath    string   `json:"poster_path"`
						Popularity    float32
						Name          string
						VoteAverage   float32 `json:"vote_average"`
						VoteCount     uint32  `json:"vote_count"`
					}{
						{
							ID:           1399,
							Name:         "Game of Thrones",
							FirstAirDate: "2011-04-17",
							VoteAverage:  9.3,
						},
					},
				}, nil
			},
			getInfoFunc: func(id int, options map[string]string) (*tmdb.TV, error) {
				return nil, errors.New("API error")
			},
			want: &EnrichedMetadata{
				ShowName:  "Game of Thrones",
				Title:     "Game of Thrones",
				Year:      "2011",
				Rating:    9.3,
				ID:        1399,
				LocalName: "Game of Thrones",
			},
			wantErr: false,
		},
		{
			name:     "api_error",
			showName: "Test Show",
			mockFunc: func(query string, options map[string]string) (*tmdb.TvSearchResults, error) {
				return nil, errors.New("503 Service Unavailable")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:     "empty_show_name",
			showName: "",
			mockFunc: nil,
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			mockClient := &mockTMDBClient{
				searchTvFunc:  tt.mockFunc,
				getTvInfoFunc: tt.getInfoFunc,
			}
			provider.SetClient(mockClient)

			got, err := provider.SearchTVShow(tt.showName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchTVShow(%q) error = nil, want error", tt.showName)
				}
			} else {
				if err != nil {
					t.Errorf("SearchTVShow(%q) error = %v, want nil", tt.showName, err)
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("SearchTVShow(%q) mismatch (-want +got):\n%s", tt.showName, diff)
				}
			}
		})
	}
}

func TestGetEpisodeInfo(t *testing.T) {
	tests := []struct {
		name      string
		showID    int
		season    int
		episode   int
		mockFunc  func(tvID int, seasonNumber int, episodeNumber int, options map[string]string) (*tmdb.TvEpisode, error)
		getTvFunc func(id int, options map[string]string) (*tmdb.TV, error)
		want      *EnrichedMetadata
		wantErr   bool
	}{
		{
			name:    "successful_episode_fetch",
			showID:  1396,
			season:  1,
			episode: 1,
			mockFunc: func(tvID int, seasonNumber int, episodeNumber int, options map[string]string) (*tmdb.TvEpisode, error) {
				return &tmdb.TvEpisode{
					Name:          "Pilot",
					AirDate:       "2008-01-20",
					Overview:      "Walter White, a struggling high school chemistry teacher",
					VoteAverage:   8.5,
					SeasonNumber:  1,
					EpisodeNumber: 1,
				}, nil
			},
			getTvFunc: func(id int, options map[string]string) (*tmdb.TV, error) {
				return &tmdb.TV{
					ID:           1396,
					Name:         "Breaking Bad",
					FirstAirDate: "2008-01-20",
				}, nil
			},
			want: &EnrichedMetadata{
				EpisodeName:  "Pilot",
				EpisodeAir:   "2008-01-20",
				Overview:     "Walter White, a struggling high school chemistry teacher",
				Rating:       8.5,
				SeasonNum:    1,
				EpisodeNum:   1,
				ID:           1396,
				LocalSeason:  1,
				LocalEpisode: 1,
				ShowName:     "Breaking Bad",
				Title:        "Breaking Bad",
				Year:         "2008",
			},
			wantErr: false,
		},
		{
			name:     "invalid_parameters",
			showID:   0,
			season:   1,
			episode:  1,
			mockFunc: nil,
			want:     nil,
			wantErr:  true,
		},
		{
			name:    "api_error",
			showID:  1396,
			season:  1,
			episode: 1,
			mockFunc: func(tvID int, seasonNumber int, episodeNumber int, options map[string]string) (*tmdb.TvEpisode, error) {
				return nil, errors.New("404 Not Found")
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			mockClient := &mockTMDBClient{
				getTvEpisodeInfoFunc: tt.mockFunc,
				getTvInfoFunc:        tt.getTvFunc,
			}
			provider.SetClient(mockClient)

			got, err := provider.GetEpisodeInfo(tt.showID, tt.season, tt.episode)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetEpisodeInfo(%d, %d, %d) error = nil, want error", tt.showID, tt.season, tt.episode)
				}
			} else {
				if err != nil {
					t.Errorf("GetEpisodeInfo(%d, %d, %d) error = %v, want nil", tt.showID, tt.season, tt.episode, err)
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("GetEpisodeInfo(%d, %d, %d) mismatch (-want +got):\n%s", tt.showID, tt.season, tt.episode, diff)
				}
			}
		})
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		wantErr  error
	}{
		{
			name:     "unauthorized_error",
			inputErr: errors.New("401 Unauthorized"),
			wantErr:  ErrInvalidAPIKey,
		},
		{
			name:     "rate_limit_error",
			inputErr: errors.New("429 Too Many Requests"),
			wantErr:  ErrRateLimited,
		},
		{
			name:     "service_unavailable",
			inputErr: errors.New("503 Service Unavailable"),
			wantErr:  ErrAPIUnavailable,
		},
		{
			name:     "generic_error",
			inputErr: errors.New("some other error"),
			wantErr:  errors.New("TMDB API error: some other error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			got := provider.mapError(tt.inputErr)

			if tt.wantErr == ErrInvalidAPIKey || tt.wantErr == ErrRateLimited || tt.wantErr == ErrAPIUnavailable {
				if !errors.Is(got, tt.wantErr) {
					t.Errorf("mapError(%v) = %v, want %v", tt.inputErr, got, tt.wantErr)
				}
			} else {
				if got.Error() != tt.wantErr.Error() {
					t.Errorf("mapError(%v) = %v, want %v", tt.inputErr, got, tt.wantErr)
				}
			}
		})
	}
}

func TestCaching(t *testing.T) {
	provider, _ := NewTMDBProvider("test-api-key", "en-US")

	callCount := 0
	mockClient := &mockTMDBClient{
		searchMovieFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
			callCount++
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
		},
		getMovieInfoFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
			return nil, errors.New("API error")
		},
	}
	provider.SetClient(mockClient)

	// First call should hit the API
	result1, err := provider.SearchMovie("The Matrix", "1999")
	if err != nil {
		t.Errorf("First SearchMovie call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("First call: API call count = %d, want 1", callCount)
	}

	// Second call should hit the cache
	result2, err := provider.SearchMovie("The Matrix", "1999")
	if err != nil {
		t.Errorf("Second SearchMovie call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Second call: API call count = %d, want 1 (should use cache)", callCount)
	}

	// Results should be identical
	if diff := cmp.Diff(result1, result2); diff != "" {
		t.Errorf("Cached result mismatch (-first +second):\n%s", diff)
	}

	// Different parameters should hit the API again
	_, err = provider.SearchMovie("The Matrix", "2000")
	if err != nil {
		t.Errorf("Third SearchMovie call failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Third call: API call count = %d, want 2", callCount)
	}
}

func TestGetSeasonInfo(t *testing.T) {
	tests := []struct {
		name      string
		showID    int
		seasonNum int
		mockFunc  func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error)
		want      *EnrichedMetadata
		wantErr   bool
	}{
		{
			name:      "successful_season_fetch",
			showID:    1396,
			seasonNum: 1,
			mockFunc: func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
				return &tmdb.TvSeason{
					SeasonNumber: 1,
					Name:         "Season 1",
					Overview:     "The first season of Breaking Bad",
					AirDate:      "2008-01-20",
				}, nil
			},
			want: &EnrichedMetadata{
				SeasonName:  "Season 1",
				SeasonNum:   1,
				Overview:    "The first season of Breaking Bad",
				EpisodeAir:  "2008-01-20",
				ID:          1396,
				LocalSeason: 1,
			},
			wantErr: false,
		},
		{
			name:      "invalid_show_id",
			showID:    0,
			seasonNum: 1,
			mockFunc:  nil,
			want:      nil,
			wantErr:   true,
		},
		{
			name:      "invalid_season_number",
			showID:    1396,
			seasonNum: -1,
			mockFunc:  nil,
			want:      nil,
			wantErr:   true,
		},
		{
			name:      "api_error",
			showID:    1396,
			seasonNum: 1,
			mockFunc: func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
				return nil, errors.New("404 Not Found")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:      "no_season_data",
			showID:    1396,
			seasonNum: 1,
			mockFunc: func(showID, seasonID int, options map[string]string) (*tmdb.TvSeason, error) {
				return nil, nil
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			mockClient := &mockTMDBClient{
				getTvSeasonInfoFunc: tt.mockFunc,
			}
			provider.SetClient(mockClient)

			got, err := provider.GetSeasonInfo(tt.showID, tt.seasonNum)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetSeasonInfo(%d, %d) error = nil, want error", tt.showID, tt.seasonNum)
				}
			} else {
				if err != nil {
					t.Errorf("GetSeasonInfo(%d, %d) error = %v, want nil", tt.showID, tt.seasonNum, err)
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("GetSeasonInfo(%d, %d) mismatch (-want +got):\n%s", tt.showID, tt.seasonNum, diff)
				}
			}
		})
	}
}

func TestGetIDFromMetadata(t *testing.T) {
	tests := []struct {
		name           string
		meta           *EnrichedMetadata
		mediaType      string
		searchMovieErr error
		searchTVErr    error
		want           int
		wantErr        bool
	}{
		{
			name: "existing_id_in_metadata",
			meta: &EnrichedMetadata{
				ID: 603,
			},
			mediaType: "movie",
			want:      603,
			wantErr:   false,
		},
		{
			name: "search_movie_success",
			meta: &EnrichedMetadata{
				LocalName: "The Matrix",
				LocalYear: "1999",
			},
			mediaType:      "movie",
			searchMovieErr: nil,
			want:           603,
			wantErr:        false,
		},
		{
			name: "search_movie_error",
			meta: &EnrichedMetadata{
				LocalName: "NonexistentMovie",
				LocalYear: "2099",
			},
			mediaType:      "movie",
			searchMovieErr: ErrNoResults,
			want:           0,
			wantErr:        true,
		},
		{
			name: "search_tv_success",
			meta: &EnrichedMetadata{
				LocalName: "Breaking Bad",
			},
			mediaType:   "tv",
			searchTVErr: nil,
			want:        1396,
			wantErr:     false,
		},
		{
			name: "search_tv_error",
			meta: &EnrichedMetadata{
				LocalName: "NonexistentShow",
			},
			mediaType:   "tv",
			searchTVErr: ErrNoResults,
			want:        0,
			wantErr:     true,
		},
		{
			name: "unknown_media_type",
			meta: &EnrichedMetadata{
				LocalName: "Something",
			},
			mediaType: "unknown",
			want:      0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewTMDBProvider("test-api-key", "en-US")
			mockClient := &mockTMDBClient{
				searchMovieFunc: func(query string, options map[string]string) (*tmdb.MovieSearchResults, error) {
					if tt.searchMovieErr != nil {
						return nil, tt.searchMovieErr
					}
					return &tmdb.MovieSearchResults{
						Results: []tmdb.MovieShort{
							{ID: 603, Title: "The Matrix"},
						},
					}, nil
				},
				searchTvFunc: func(query string, options map[string]string) (*tmdb.TvSearchResults, error) {
					if tt.searchTVErr != nil {
						return nil, tt.searchTVErr
					}
					return &tmdb.TvSearchResults{
						Results: []struct {
							BackdropPath  string `json:"backdrop_path"`
							ID            int
							OriginalName  string   `json:"original_name"`
							FirstAirDate  string   `json:"first_air_date"`
							OriginCountry []string `json:"origin_country"`
							PosterPath    string   `json:"poster_path"`
							Popularity    float32
							Name          string
							VoteAverage   float32 `json:"vote_average"`
							VoteCount     uint32  `json:"vote_count"`
						}{
							{ID: 1396, Name: "Breaking Bad"},
						},
					}, nil
				},
				getMovieInfoFunc: func(id int, options map[string]string) (*tmdb.Movie, error) {
					return nil, errors.New("API error")
				},
				getTvInfoFunc: func(id int, options map[string]string) (*tmdb.TV, error) {
					return nil, errors.New("API error")
				},
			}
			provider.SetClient(mockClient)

			got, err := provider.GetIDFromMetadata(tt.meta, tt.mediaType)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetIDFromMetadata(%v, %q) error = nil, want error", tt.meta, tt.mediaType)
				}
			} else {
				if err != nil {
					t.Errorf("GetIDFromMetadata(%v, %q) error = %v, want nil", tt.meta, tt.mediaType, err)
				}
				if got != tt.want {
					t.Errorf("GetIDFromMetadata(%v, %q) = %d, want %d", tt.meta, tt.mediaType, got, tt.want)
				}
			}
		})
	}
}

func TestParseYear(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "year_in_middle",
			input: "The Matrix (1999) Reloaded",
			want:  "1999",
		},
		{
			name:  "year_at_end",
			input: "Breaking Bad 2008",
			want:  "2008",
		},
		{
			name:  "year_at_start",
			input: "2010 Inception",
			want:  "2010",
		},
		{
			name:  "multiple_years_returns_first",
			input: "Movie 1999 and 2000",
			want:  "1999",
		},
		{
			name:  "no_year",
			input: "Movie Name Without Year",
			want:  "",
		},
		{
			name:  "invalid_year_too_low",
			input: "Movie 1800",
			want:  "",
		},
		{
			name:  "invalid_year_too_high",
			input: "Movie 2200",
			want:  "",
		},
		{
			name:  "edge_case_year_1900",
			input: "Old Movie 1900",
			want:  "1900",
		},
		{
			name:  "edge_case_year_2100",
			input: "Future Movie 2100",
			want:  "2100",
		},
		{
			name:  "empty_string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseYear(tt.input)
			if got != tt.want {
				t.Errorf("ParseYear(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapErrorWithNil(t *testing.T) {
	provider, _ := NewTMDBProvider("test-api-key", "en-US")

	got := provider.mapError(nil)
	if got != nil {
		t.Errorf("mapError(nil) = %v, want nil", got)
	}
}
