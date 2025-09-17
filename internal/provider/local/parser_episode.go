package local

import (
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// EpisodeParser handles parsing of episode files
type EpisodeParser struct{}

// NewEpisodeParser creates a new episode parser
func NewEpisodeParser() *EpisodeParser {
	return &EpisodeParser{}
}

// Parse extracts episode metadata from a filename
func (p *EpisodeParser) Parse(name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error) {
	ctx := NewParseContext(name, node)

	// Parse season and episode numbers
	season, episode, found := SeasonEpisodeFromContext(ctx)
	if !found {
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "PARSE_ERROR",
			Message:  "Could not extract season and episode numbers from: " + name,
			Retry:    false,
		}
	}

	// Extract show name from the filename or parent directories
	showName, year := p.extractShowInfo(ctx)

	// Create metadata
	metadata := createMetadata(provider.MediaTypeEpisode, showName, year, 1.0)
	metadata.Core.SeasonNum = season
	metadata.Core.EpisodeNum = episode

	// Add to sources
	metadata.Sources["season"] = providerName
	metadata.Sources["episode"] = providerName

	return metadata, nil
}

// CanParse checks if this parser can handle the input
func (p *EpisodeParser) CanParse(name string, node *treeview.Node[treeview.FileInfo]) bool {
	// Episodes should be video files
	if !IsVideo(name) && !IsSubtitle(name) {
		return false
	}

	// Try to parse season/episode
	_, _, found := p.parseSeasonEpisode(name, node)
	return found
}

// parseSeasonEpisode extracts season and episode numbers using multiple strategies
func (p *EpisodeParser) parseSeasonEpisode(input string, node *treeview.Node[treeview.FileInfo]) (int, int, bool) {
	return SeasonEpisodeFromContext(NewParseContext(input, node))
}

// extractShowInfo extracts show name and year from filename and parent directories
func (p *EpisodeParser) extractShowInfo(ctx ParseContext) (showName, year string) {
	return ResolveShowInfo(ctx)
}
