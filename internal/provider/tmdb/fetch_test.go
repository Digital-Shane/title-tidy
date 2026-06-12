package tmdb

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
	tmdbapi "github.com/ryanbradynd05/go-tmdb"
)

func TestMovieToMetadataIncludesRuntime(t *testing.T) {
	prov := New()
	meta := prov.movieToMetadata(&tmdbapi.Movie{
		Title:   "The Matrix",
		Runtime: 136,
	})

	if diff := cmp.Diff(uint32(136), meta.Extended["runtime"]); diff != "" {
		t.Fatalf("runtime mismatch (-want +got):\n%s", diff)
	}
}

func TestTVToMetadataIncludesRuntime(t *testing.T) {
	prov := New()
	meta := prov.tvToMetadata(&tmdbapi.TV{
		Name:           "Breaking Bad",
		EpisodeRunTime: []int{48},
	})

	if diff := cmp.Diff(48, meta.Extended["runtime"]); diff != "" {
		t.Fatalf("runtime mismatch (-want +got):\n%s", diff)
	}
}

func TestEpisodeToMetadataIncludesAirDate(t *testing.T) {
	prov := New()
	meta := prov.episodeToMetadata(&tmdbapi.TvEpisode{
		Name:          "Pilot",
		AirDate:       "2008-01-20",
		SeasonNumber:  1,
		EpisodeNumber: 1,
	}, nil, 123)

	if diff := cmp.Diff("2008-01-20", meta.Extended["air_date"]); diff != "" {
		t.Fatalf("air_date mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(providerName, meta.Sources["air_date"]); diff != "" {
		t.Fatalf("air_date source mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(provider.MediaTypeEpisode, meta.Core.MediaType); diff != "" {
		t.Fatalf("media type mismatch (-want +got):\n%s", diff)
	}
}
