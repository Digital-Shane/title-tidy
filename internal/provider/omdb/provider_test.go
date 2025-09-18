package omdb

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, body string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func TestConfigureRequiresAPIKey(t *testing.T) {
	prov := New()
	if err := prov.Configure(map[string]interface{}{}); err == nil {
		t.Fatal("expected error when api_key is missing")
	}
}

func TestFetchMovie(t *testing.T) {
	prov := New()
	prov.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{
            "Title": "Interstellar",
            "Year": "2014",
            "Runtime": "169 min",
            "Genre": "Adventure, Drama, Sci-Fi",
            "Plot": "A team of explorers travel through a wormhole in space in an attempt to ensure humanity's survival.",
            "Language": "English",
            "Country": "USA",
            "imdbRating": "8.6",
            "imdbID": "tt0816692",
            "Production": "Paramount Pictures",
            "Type": "movie",
            "Response": "True"
        }`), nil
	})

	if err := prov.Configure(map[string]interface{}{"api_key": "testing"}); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	meta, err := prov.Fetch(context.Background(), provider.FetchRequest{
		MediaType: provider.MediaTypeMovie,
		Name:      "Interstellar",
		Year:      "2014",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if meta.Core.Title != "Interstellar" {
		t.Fatalf("Title = %q, want Interstellar", meta.Core.Title)
	}

	if meta.Core.Year != "2014" {
		t.Fatalf("Year = %q, want 2014", meta.Core.Year)
	}

	if meta.Core.Rating == 0 {
		t.Fatal("expected rating to be parsed")
	}

	if got := meta.IDs["imdb_id"]; got != "tt0816692" {
		t.Fatalf("imdb_id = %q, want tt0816692", got)
	}

	if meta.Extended["runtime"].(int) != 169 {
		t.Fatalf("runtime = %v, want 169", meta.Extended["runtime"])
	}
}

func TestFetchEpisode(t *testing.T) {
	prov := New()
	prov.httpClient = newTestClient(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		if q.Get("Episode") == "1" {
			return jsonResponse(200, `{
                "Title": "Winter Is Coming",
                "Year": "2011",
                "Released": "17 Apr 2011",
                "Runtime": "62 min",
                "Genre": "Action, Adventure, Drama",
                "Plot": "Episode plot",
                "Language": "English",
                "Country": "United States",
                "imdbRating": "8.9",
                "imdbID": "tt1480055",
                "seriesID": "tt0944947",
                "Type": "episode",
                "Response": "True"
            }`), nil
		}
		return jsonResponse(200, `{"Response": "False", "Error": "Episode not found"}`), nil
	})

	if err := prov.Configure(map[string]interface{}{"api_key": "testing"}); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	meta, err := prov.Fetch(context.Background(), provider.FetchRequest{
		MediaType: provider.MediaTypeEpisode,
		Name:      "Game of Thrones",
		Season:    1,
		Episode:   1,
		ID:        "tt0944947",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if meta.Core.EpisodeName != "Winter Is Coming" {
		t.Fatalf("EpisodeName = %q, want Winter Is Coming", meta.Core.EpisodeName)
	}

	if meta.Core.SeasonNum != 1 || meta.Core.EpisodeNum != 1 {
		t.Fatalf("unexpected season/episode numbers: %+v", meta.Core)
	}

	if meta.Core.Rating == 0 {
		t.Fatal("expected rating to be parsed for episode")
	}

	if got := meta.IDs["imdb_id"]; got != "tt1480055" {
		t.Fatalf("imdb_id = %q, want tt1480055", got)
	}
}
