package local

import (
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// Parser defines the interface for media parsers
type Parser interface {
	// Parse extracts metadata from a filename or directory name
	Parse(name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error)

	// CanParse checks if this parser can handle the given input
	CanParse(name string, node *treeview.Node[treeview.FileInfo]) bool
}

// ParserEngine manages all parsers and routes parsing requests
type ParserEngine struct {
	detector      *Detector
	showParser    Parser
	seasonParser  Parser
	episodeParser Parser
	movieParser   Parser
}

// NewParserEngine creates a new parser engine with all parsers initialized
func NewParserEngine() *ParserEngine {
	return &ParserEngine{
		detector:      NewDetector(),
		showParser:    NewShowParser(),
		seasonParser:  NewSeasonParser(),
		episodeParser: NewEpisodeParser(),
		movieParser:   NewMovieParser(),
	}
}

// Parse routes the parsing request to the appropriate parser based on media type
func (e *ParserEngine) Parse(mediaType provider.MediaType, name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error) {
	var parser Parser

	switch mediaType {
	case provider.MediaTypeShow:
		parser = e.showParser
	case provider.MediaTypeSeason:
		parser = e.seasonParser
	case provider.MediaTypeEpisode:
		parser = e.episodeParser
	case provider.MediaTypeMovie:
		parser = e.movieParser
	default:
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "UNSUPPORTED_TYPE",
			Message:  "Unsupported media type for parsing",
			Retry:    false,
		}
	}

	return parser.Parse(name, node)
}

// Helper function to create metadata with common fields filled
func createMetadata(mediaType provider.MediaType, title, year string, confidence float64) *provider.Metadata {
	meta := &provider.Metadata{
		Core: provider.CoreMetadata{
			MediaType: mediaType,
			Title:     title,
			Year:      year,
		},
		Extended: make(map[string]interface{}),
		IDs:      make(map[string]string),
		Sources: map[string]string{
			"title": providerName,
			"year":  providerName,
		},
		Confidence: confidence,
	}

	return meta
}

// DetectNode analyses a filesystem node and selects the most appropriate parser.
// It returns the detected media type alongside the parsed metadata.
func (e *ParserEngine) DetectNode(node *treeview.Node[treeview.FileInfo]) (provider.MediaType, *provider.Metadata, error) {
	if node == nil {
		return "", nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "INVALID_INPUT",
			Message:  "node is required for detection",
			Retry:    false,
		}
	}

	ctx := NewParseContext(node.Name(), node)

	mediaType, err := e.detector.Detect(ctx)
	if err != nil {
		return "", nil, err
	}

	meta, err := e.Parse(mediaType, ctx.Name, node)
	if err != nil {
		return "", nil, err
	}

	return mediaType, meta, nil
}

func isLikelyShowNode(node *treeview.Node[treeview.FileInfo]) bool {
	if node == nil {
		return false
	}

	for _, child := range node.Children() {
		if child == nil {
			continue
		}

		if child.Data().IsDir() {
			if _, ok := ExtractSeasonNumber(child.Name()); ok {
				return true
			}
			continue
		}

		if _, _, found := SeasonEpisodeFromContext(NewParseContext(child.Name(), child)); found {
			return true
		}
	}

	return false
}
