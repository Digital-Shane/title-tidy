package ffprobe

import (
	"context"
	"errors"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	ffprobeLib "gopkg.in/vansante/go-ffprobe.v2"
)

func TestFetch_Success(t *testing.T) {
	p := New()
	p.probe = func(ctx context.Context, path string, extraOpts ...string) (*ffprobeLib.ProbeData, error) {
		return &ffprobeLib.ProbeData{
			Format: &ffprobeLib.Format{},
			Streams: []*ffprobeLib.Stream{
				{
					CodecName: "h264",
					CodecType: string(ffprobeLib.StreamVideo),
					Height:    1080,
				},
				{
					CodecName: "aac",
					CodecType: string(ffprobeLib.StreamAudio),
				},
			},
		}, nil
	}

	req := provider.FetchRequest{
		MediaType: provider.MediaTypeMovie,
		Name:      "Example",
		Extra: map[string]interface{}{
			"path": "/videos/example.mkv",
		},
	}

	meta, err := p.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	if meta.Core.MediaType != provider.MediaTypeMovie {
		t.Errorf("Fetch() media type = %v, want %v", meta.Core.MediaType, provider.MediaTypeMovie)
	}

	if got := meta.Extended["video_codec"]; got != "h264" {
		t.Errorf("video_codec = %v, want h264", got)
	}

	if got := meta.Extended["audio_codec"]; got != "aac" {
		t.Errorf("audio_codec = %v, want aac", got)
	}

	if got := meta.Extended["video_resolution"]; got != "1080p" {
		t.Errorf("video_resolution = %v, want 1080p", got)
	}

	if got := meta.Sources["video_codec"]; got != providerName {
		t.Errorf("source(video_codec) = %v, want %v", got, providerName)
	}

	if got := meta.Sources["audio_codec"]; got != providerName {
		t.Errorf("source(audio_codec) = %v, want %v", got, providerName)
	}

	if got := meta.Sources["video_resolution"]; got != providerName {
		t.Errorf("source(video_resolution) = %v, want %v", got, providerName)
	}
}

func TestFetch_MissingPath(t *testing.T) {
	p := New()
	req := provider.FetchRequest{MediaType: provider.MediaTypeEpisode}

	_, err := p.Fetch(context.Background(), req)
	if err == nil {
		t.Fatalf("Fetch() expected error, got nil")
	}

	var provErr *provider.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Fetch() error = %v, want ProviderError", err)
	}

	if provErr.Code != "MISSING_PATH" {
		t.Errorf("ProviderError.Code = %v, want MISSING_PATH", provErr.Code)
	}
}

func TestFetch_SkipsResolutionWhenHeightMissing(t *testing.T) {
	p := New()
	p.probe = func(ctx context.Context, path string, extraOpts ...string) (*ffprobeLib.ProbeData, error) {
		return &ffprobeLib.ProbeData{
			Format: &ffprobeLib.Format{},
			Streams: []*ffprobeLib.Stream{
				{
					CodecName: "h264",
					CodecType: string(ffprobeLib.StreamVideo),
				},
			},
		}, nil
	}

	req := provider.FetchRequest{
		MediaType: provider.MediaTypeMovie,
		Name:      "Example",
		Extra: map[string]interface{}{
			"path": "/videos/example.mkv",
		},
	}

	meta, err := p.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	if _, ok := meta.Extended["video_resolution"]; ok {
		t.Errorf("video_resolution present = %t, want false", ok)
	}

	if _, ok := meta.Sources["video_resolution"]; ok {
		t.Errorf("source(video_resolution) present = %t, want false", ok)
	}
}

func TestFetch_UnsupportedMediaType(t *testing.T) {
	p := New()
	req := provider.FetchRequest{
		MediaType: provider.MediaTypeShow,
		Extra: map[string]interface{}{
			"path": "/videos/show.mkv",
		},
	}

	_, err := p.Fetch(context.Background(), req)
	if err == nil {
		t.Fatalf("Fetch() expected error for unsupported media type")
	}

	var provErr *provider.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Fetch() error = %v, want ProviderError", err)
	}

	if provErr.Code != "UNSUPPORTED_MEDIA_TYPE" {
		t.Errorf("ProviderError.Code = %v, want UNSUPPORTED_MEDIA_TYPE", provErr.Code)
	}
}
