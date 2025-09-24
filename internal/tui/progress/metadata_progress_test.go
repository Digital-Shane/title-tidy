package progress

import (
	"context"
	"errors"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/google/go-cmp/cmp"
)

func TestMetadataProgressShouldRunFFProbe(t *testing.T) {
	movieNode := newMetadataFileNode("movie", "movie.mkv", "/library/movie.mkv", false)
	dirNode := newMetadataFileNode("dir", "movie", "/library/movie", true)
	subtitleNode := newMetadataFileNode("subtitle", "movie.srt", "/library/movie.srt", false)

	tests := []struct {
		name string
		item core.MetadataItem
		want bool
	}{
		{
			name: "NoNode",
			item: core.MetadataItem{MediaType: provider.MediaTypeMovie},
			want: false,
		},
		{
			name: "DirNode",
			item: core.MetadataItem{MediaType: provider.MediaTypeMovie, Node: dirNode},
			want: false,
		},
		{
			name: "NonVideoFile",
			item: core.MetadataItem{MediaType: provider.MediaTypeMovie, Node: subtitleNode},
			want: false,
		},
		{
			name: "MovieVideo",
			item: core.MetadataItem{MediaType: provider.MediaTypeMovie, Node: movieNode},
			want: true,
		},
		{
			name: "EpisodeVideo",
			item: core.MetadataItem{MediaType: provider.MediaTypeEpisode, Node: movieNode},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := core.ShouldRunFFProbe(tc.item)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ShouldRunFFProbe(%s) mismatch (-want +got):%s", tc.name, diff)
			}
		})
	}
}

func TestMetadataProgressFetchFFProbeMetadata(t *testing.T) {
	videoNode := newMetadataFileNode("video", "movie.mkv", "/library/movie.mkv", false)
	missingPathNode := newMetadataFileNode("missing", "movie.mkv", "", false)
	episodeNode := newMetadataFileNode("episode", "episode.mkv", "/library/episode.mkv", false)

	tests := []struct {
		name             string
		providerPresent  bool
		providerResponse *provider.Metadata
		providerError    error
		item             core.MetadataItem
		wantMeta         *provider.Metadata
		wantProviderCall bool
		wantErrCheck     func(*testing.T, error)
		assertReq        func(*testing.T, provider.FetchRequest)
	}{
		{
			name:            "NoProvider",
			providerPresent: false,
			item: core.MetadataItem{
				MediaType: provider.MediaTypeMovie,
				Node:      videoNode,
			},
			wantMeta:         nil,
			wantProviderCall: false,
		},
		{
			name:            "ShouldNotRun",
			providerPresent: true,
			item: core.MetadataItem{
				MediaType: provider.MediaTypeShow,
				Node:      videoNode,
			},
			wantMeta:         nil,
			wantProviderCall: false,
		},
		{
			name:            "MissingPath",
			providerPresent: true,
			item: core.MetadataItem{
				MediaType: provider.MediaTypeMovie,
				Node:      missingPathNode,
			},
			wantMeta:         nil,
			wantProviderCall: false,
			wantErrCheck: func(t *testing.T, err error) {
				t.Helper()
				var provErr *provider.ProviderError
				if !errors.As(err, &provErr) {
					t.Fatalf("fetchFFProbeMetadata() error = %v, want *provider.ProviderError", err)
				}
				if diff := cmp.Diff("MISSING_PATH", provErr.Code); diff != "" {
					t.Errorf("provider error code mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name:             "MovieRequestIncludesPath",
			providerPresent:  true,
			providerResponse: &provider.Metadata{Core: provider.CoreMetadata{Title: "Movie", MediaType: provider.MediaTypeMovie}},
			item: core.MetadataItem{
				Name:      "Movie",
				Year:      "2021",
				IsMovie:   true,
				MediaType: provider.MediaTypeMovie,
				Node:      videoNode,
			},
			wantMeta:         &provider.Metadata{Core: provider.CoreMetadata{Title: "Movie", MediaType: provider.MediaTypeMovie}},
			wantProviderCall: true,
			assertReq: func(t *testing.T, req provider.FetchRequest) {
				t.Helper()
				if diff := cmp.Diff(provider.MediaTypeMovie, req.MediaType); diff != "" {
					t.Errorf("media type mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff("/library/movie.mkv", req.Extra["path"]); diff != "" {
					t.Errorf("path extra mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff("Movie", req.Name); diff != "" {
					t.Errorf("name mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff("2021", req.Year); diff != "" {
					t.Errorf("year mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name:             "EpisodeRequestIncludesExtras",
			providerPresent:  true,
			providerResponse: &provider.Metadata{Core: provider.CoreMetadata{Title: "Episode", MediaType: provider.MediaTypeEpisode}},
			item: core.MetadataItem{
				Name:      "Episode",
				Season:    1,
				Episode:   2,
				MediaType: provider.MediaTypeEpisode,
				Node:      episodeNode,
			},
			wantMeta:         &provider.Metadata{Core: provider.CoreMetadata{Title: "Episode", MediaType: provider.MediaTypeEpisode}},
			wantProviderCall: true,
			assertReq: func(t *testing.T, req provider.FetchRequest) {
				t.Helper()
				if diff := cmp.Diff(provider.MediaTypeEpisode, req.MediaType); diff != "" {
					t.Errorf("media type mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(1, req.Season); diff != "" {
					t.Errorf("season mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(2, req.Episode); diff != "" {
					t.Errorf("episode mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff("/library/episode.mkv", req.Extra["path"]); diff != "" {
					t.Errorf("path extra mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedReq provider.FetchRequest
			called := false
			var prov provider.Provider
			if tc.providerPresent {
				prov = newMetadataFakeProvider("ffprobe", func(req provider.FetchRequest) (*provider.Metadata, error) {
					called = true
					capturedReq = req
					return tc.providerResponse, tc.providerError
				})
			}

			gotMeta, err := core.FetchFFProbeMetadata(context.Background(), prov, tc.item)

			if tc.wantErrCheck != nil {
				tc.wantErrCheck(t, err)
			} else if err != nil {
				t.Fatalf("fetchFFProbeMetadata() unexpected error = %v", err)
			}

			if diff := cmp.Diff(tc.wantProviderCall, called); diff != "" {
				t.Errorf("provider call mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantMeta, gotMeta); diff != "" {
				t.Errorf("metadata mismatch (-want +got):\n%s", diff)
			}

			if tc.assertReq != nil {
				if !called {
					t.Fatalf("expected provider to be called to assert request")
				}
				tc.assertReq(t, capturedReq)
			}
		})
	}
}

func TestHasMetadataValues(t *testing.T) {
	tests := []struct {
		name string
		meta *provider.Metadata
		want bool
	}{
		{
			name: "Nil",
			meta: nil,
			want: false,
		},
		{
			name: "Empty",
			meta: &provider.Metadata{},
			want: false,
		},
		{
			name: "ExtendedFields",
			meta: &provider.Metadata{Extended: map[string]interface{}{"runtime": 120}},
			want: true,
		},
		{
			name: "Sources",
			meta: &provider.Metadata{Sources: map[string]string{"ffprobe": "path"}},
			want: true,
		},
		{
			name: "IDs",
			meta: &provider.Metadata{IDs: map[string]string{"imdb": "tt123"}},
			want: true,
		},
		{
			name: "CoreFields",
			meta: &provider.Metadata{Core: provider.CoreMetadata{Title: "Test", SeasonNum: 1, EpisodeNum: 2, Year: "2022"}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := core.HasMetadataValues(tc.meta)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("core.HasMetadataValues(%s) mismatch (-want +got):\n%s", tc.name, diff)
			}
		})
	}
}
