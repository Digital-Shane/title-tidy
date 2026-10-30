package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview/v2"
	tmdbapi "github.com/ryanbradynd05/go-tmdb"
)

type movieTitleLookupClient struct {
	TMDBClient
	t           *testing.T
	title       string
	searchYear  string
	releaseDate string
}

func (c movieTitleLookupClient) SearchMovie(name string, options map[string]string) (*tmdbapi.MovieSearchResults, error) {
	c.t.Helper()
	if name != c.title || options["year"] != c.searchYear {
		c.t.Errorf("movie search = (%q, year %q), want (%q, year %q)", name, options["year"], c.title, c.searchYear)
	}
	var results tmdbapi.MovieSearchResults
	err := json.Unmarshal([]byte(`{"results":[{"id":123}]}`), &results)
	return &results, err
}

func (c movieTitleLookupClient) GetMovieInfo(_ int, _ map[string]string) (*tmdbapi.Movie, error) {
	return &tmdbapi.Movie{ID: 123, Title: c.title, ReleaseDate: c.releaseDate}, nil
}

func TestMovieTitlesContainingYearsReachSearchIntact(t *testing.T) {
	for _, tt := range []struct {
		filename    string
		title       string
		searchYear  string
		releaseDate string
	}{
		{"1917.2019.720p.mp4", "1917", "2019", "2019-12-25"},
		{"2010: The Year We Make Contact.mp4", "2010: The Year We Make Contact", "", "1984-12-06"},
		{"Tamala 2010: A Punk Cat in Space.mp4", "Tamala 2010: A Punk Cat in Space", "", "2002-10-19"},
	} {
		t.Run(tt.filename, func(t *testing.T) {
			prov := New()
			prov.client = movieTitleLookupClient{
				t: t, title: tt.title, searchYear: tt.searchYear, releaseDate: tt.releaseDate,
			}
			prov.rateLimiter = newRateLimiter(100, time.Second)
			node := treeview.NewNode(tt.filename, tt.filename, treeview.FileInfo{
				FileInfo: core.NewSimpleFileInfo(tt.filename, false),
				Path:     tt.filename,
			})
			engine := core.NewMetadataEngine(core.MetadataEngineConfig{
				Tree:        treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node}),
				WorkerCount: 1,
				Providers:   []provider.RuntimeConfig{{Name: prov.Name(), Provider: prov, Enabled: true}},
			})
			for range engine.Start(context.Background()) {
			}

			key := provider.GenerateMetadataKey("movie", tt.title, tt.searchYear, 0, 0)
			meta := engine.Metadata()[key]
			if meta == nil || meta.Core.Title != tt.title || meta.Core.Year != tt.releaseDate[:4] {
				t.Fatalf("movie metadata = %+v, want %q (%s)", meta, tt.title, tt.releaseDate[:4])
			}
			if engine.SummarySnapshot().ErrorCount != 0 {
				t.Fatalf("movie lookup failures = %+v", engine.ProviderFailures())
			}
		})
	}
}

type episodeLookupClient struct{ TMDBClient }

func (episodeLookupClient) SearchTv(name string, _ map[string]string) (*tmdbapi.TvSearchResults, error) {
	var results tmdbapi.TvSearchResults
	err := json.Unmarshal([]byte(`{"results":[{"id":123,"name":"Example Show"}]}`), &results)
	if name == "Correct Show" {
		results.Results[0].ID = 456
		results.Results[0].Name = name
	}
	return &results, err
}

func (episodeLookupClient) GetTvInfo(id int, _ map[string]string) (*tmdbapi.TV, error) {
	name := "Example Show"
	if id == 456 {
		name = "Correct Show"
	}
	return &tmdbapi.TV{ID: id, Name: name}, nil
}

func (episodeLookupClient) GetTvEpisodeInfo(id, season, episode int, _ map[string]string) (*tmdbapi.TvEpisode, error) {
	if id == 123 && season == 2 {
		// Match go-tmdb: it returns a non-nil episode alongside the API error.
		return &tmdbapi.TvEpisode{}, errors.New("Code (34): The resource you requested could not be found.")
	}
	return &tmdbapi.TvEpisode{Name: "Pilot", SeasonNumber: season, EpisodeNumber: episode}, nil
}

func TestMissingEpisodeCanBeResolvedWithManualSearch(t *testing.T) {
	prov := New()
	prov.client = episodeLookupClient{}
	prov.rateLimiter = newRateLimiter(100, time.Second)
	var nodes []*treeview.Node[treeview.FileInfo]
	for _, name := range []string{"Example.Show.S01E01.mkv", "Example.Show.S02E01.mkv"} {
		nodes = append(nodes, treeview.NewNode(name, name, treeview.FileInfo{
			FileInfo: core.NewSimpleFileInfo(name, false),
			Path:     name,
		}))
	}
	engine := core.NewMetadataEngine(core.MetadataEngineConfig{
		Tree:        treeview.NewTree(nodes),
		WorkerCount: 1,
		Providers:   []provider.RuntimeConfig{{Name: prov.Name(), Provider: prov, Enabled: true}},
	})
	for range engine.Start(context.Background()) {
	}

	goodKey := provider.GenerateMetadataKey("episode", "Example Show", "", 1, 1)
	badKey := provider.GenerateMetadataKey("episode", "Example Show", "", 2, 1)
	if good := engine.Metadata()[goodKey]; good == nil || good.Core.EpisodeName != "Pilot" {
		t.Fatalf("successful episode metadata = %+v, want Pilot", good)
	}
	if engine.Metadata()[badKey] != nil {
		t.Fatal("missing episode should not be stored as successful metadata")
	}
	failures := engine.ProviderFailures()
	if len(failures) != 1 || failures[0].Item.Key != badKey || failures[0].Query != "Example Show" {
		t.Fatalf("manual search failures = %+v, want the missing episode", failures)
	}
	var provErr *provider.ProviderError
	if !errors.As(failures[0].Err, &provErr) || provErr.Code != "NOT_FOUND" {
		t.Fatalf("manual search error = %v, want NOT_FOUND", failures[0].Err)
	}
	if engine.SummarySnapshot().ErrorCount != 1 {
		t.Error("missing episode should count as an unresolved error")
	}

	failure, err := engine.RetryProvider(context.Background(), badKey, core.MetadataProviderType(prov.Name()), "Correct Show")
	if err != nil || failure != nil {
		t.Fatalf("manual retry = (%v, %v), want success", failure, err)
	}
	if len(engine.ProviderFailures()) != 0 {
		t.Fatal("resolved episode should be removed from manual search failures")
	}
	if meta := engine.Metadata()[badKey]; meta == nil || meta.Core.EpisodeName != "Pilot" || meta.Core.Title != "Correct Show" {
		t.Fatalf("metadata at original episode key = %+v, want the corrected show and episode title", meta)
	}
}
