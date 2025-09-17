package local

import "github.com/Digital-Shane/title-tidy/internal/provider"

// Detector encapsulates heuristics for identifying media types from local nodes.
type Detector struct{}

// NewDetector creates a fresh detector instance.
func NewDetector() *Detector {
	return &Detector{}
}

// Detect resolves the most likely media type for the provided context.
func (d *Detector) Detect(ctx ParseContext) (provider.MediaType, error) {
	if ctx.Node == nil {
		return "", &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_INPUT",
			Message:  "node is required for detection",
			Retry:    false,
		}
	}

	if ctx.IsFile {
		return d.detectFile(ctx)
	}

	return d.detectDirectory(ctx)
}

func (d *Detector) detectFile(ctx ParseContext) (provider.MediaType, error) {
	if !IsVideo(ctx.Name) && !IsSubtitle(ctx.Name) {
		return "", &provider.ProviderError{
			Provider: providerName,
			Code:     "UNSUPPORTED_NODE",
			Message:  "unsupported file type for detection",
			Retry:    false,
		}
	}

	if _, _, ok := SeasonEpisodeFromContext(ctx); ok {
		return provider.MediaTypeEpisode, nil
	}

	if IsVideo(ctx.Name) {
		return provider.MediaTypeMovie, nil
	}

	return "", &provider.ProviderError{
		Provider: providerName,
		Code:     "UNSUPPORTED_NODE",
		Message:  "unable to detect media type for file",
		Retry:    false,
	}
}

func (d *Detector) detectDirectory(ctx ParseContext) (provider.MediaType, error) {
	if _, isSeason := ExtractSeasonNumber(ctx.Name); isSeason {
		return provider.MediaTypeSeason, nil
	}

	if isLikelyShowNode(ctx.Node) {
		return provider.MediaTypeShow, nil
	}

	// Default: treat as movie directory (aligns with prior behavior).
	return provider.MediaTypeMovie, nil
}
