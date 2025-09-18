package ffprobe

import (
	"context"
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"gopkg.in/vansante/go-ffprobe.v2"
)

const (
	providerName = "ffprobe"
	filePathKey  = "path"
)

// probeFunc defines the function signature used to execute ffprobe.
type probeFunc func(ctx context.Context, path string, extraOpts ...string) (*ffprobe.ProbeData, error)

// Provider implements the provider.Provider interface for ffprobe-based metadata.
type Provider struct {
	probe probeFunc
}

// New creates a new ffprobe provider instance with default configuration.
func New() *Provider {
	return &Provider{
		probe: ffprobe.ProbeURL,
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return providerName
}

// Description returns the provider description.
func (p *Provider) Description() string {
	return "Technical media metadata from ffprobe"
}

// Capabilities returns what this provider can do.
func (p *Provider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{
			provider.MediaTypeMovie,
			provider.MediaTypeEpisode,
		},
		RequiresAuth: false,
		Priority:     50,
	}
}

// SupportedVariables returns the template variables supplied by this provider.
func (p *Provider) SupportedVariables() []provider.TemplateVariable {
	mediaTypes := []provider.MediaType{provider.MediaTypeMovie, provider.MediaTypeEpisode}

	return []provider.TemplateVariable{
		{
			Name:        "video_codec",
			DisplayName: "Video Codec",
			Description: "Primary video codec reported by ffprobe",
			MediaTypes:  mediaTypes,
			Example:     "h264",
			Category:    "technical",
			Provider:    providerName,
		},
		{
			Name:        "audio_codec",
			DisplayName: "Audio Codec",
			Description: "Primary audio codec reported by ffprobe",
			MediaTypes:  mediaTypes,
			Example:     "aac",
			Category:    "technical",
			Provider:    providerName,
		},
	}
}

// ConfigSchema returns the configuration schema for this provider.
func (p *Provider) ConfigSchema() provider.ConfigSchema {
	// Currently no configurable fields beyond enable/disable.
	return provider.ConfigSchema{Fields: []provider.ConfigField{}}
}

// Configure applies configuration to the provider.
func (p *Provider) Configure(config map[string]interface{}) error {
	if config == nil {
		return nil
	}

	return nil
}

// Fetch retrieves technical metadata for the provided media file.
func (p *Provider) Fetch(ctx context.Context, request provider.FetchRequest) (*provider.Metadata, error) {
	if request.MediaType != provider.MediaTypeMovie && request.MediaType != provider.MediaTypeEpisode {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "UNSUPPORTED_MEDIA_TYPE",
			Message:  fmt.Sprintf("ffprobe does not handle media type %s", request.MediaType),
			Retry:    false,
		}
	}

	path, err := extractPath(request.Extra)
	if err != nil {
		return nil, err
	}

	data, err := p.probe(context.Background(), path)
	if err != nil {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "PROBE_FAILED",
			Message:  fmt.Sprintf("ffprobe failed for %s: %v", path, err),
			Retry:    false,
		}
	}

	meta := p.buildMetadata(request, data)
	return meta, nil
}

func (p *Provider) buildMetadata(request provider.FetchRequest, data *ffprobe.ProbeData) *provider.Metadata {
	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			MediaType: request.MediaType,
			Title:     request.Name,
		},
		Extended:   make(map[string]interface{}),
		Sources:    make(map[string]string),
		IDs:        make(map[string]string),
		Confidence: 1.0,
	}

	if data == nil || data.Format == nil {
		return meta
	}

	if videoStream := data.FirstVideoStream(); videoStream != nil {
		codec := pickCodecName(videoStream)
		if codec != "" {
			meta.Extended["video_codec"] = codec
			meta.Sources["video_codec"] = providerName
		}
	}

	if audioStream := data.FirstAudioStream(); audioStream != nil {
		codec := pickCodecName(audioStream)
		if codec != "" {
			meta.Extended["audio_codec"] = codec
			meta.Sources["audio_codec"] = providerName
		}
	}

	return meta
}

func extractPath(extra map[string]interface{}) (string, error) {
	if extra == nil {
		return "", &provider.ProviderError{
			Provider: providerName,
			Code:     "MISSING_PATH",
			Message:  "ffprobe requires a file path",
			Retry:    false,
		}
	}

	if path, ok := extra[filePathKey]; ok {
		switch v := path.(type) {
		case string:
			if v != "" {
				return v, nil
			}
		case fmt.Stringer:
			value := v.String()
			if value != "" {
				return value, nil
			}
		}
	}

	return "", &provider.ProviderError{
		Provider: providerName,
		Code:     "MISSING_PATH",
		Message:  "ffprobe requires a non-empty file path",
		Retry:    false,
	}
}

func pickCodecName(stream *ffprobe.Stream) string {
	if stream == nil {
		return ""
	}
	if stream.CodecName != "" {
		return stream.CodecName
	}
	return stream.CodecLongName
}
