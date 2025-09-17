package local

import (
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// SeasonParser handles parsing of season folders
type SeasonParser struct{}

// NewSeasonParser creates a new season parser
func NewSeasonParser() *SeasonParser {
	return &SeasonParser{}
}

// Parse extracts season metadata from a folder name
func (p *SeasonParser) Parse(name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error) {
	ctx := NewParseContext(name, node)

	// Extract season number
	seasonNum, found := ExtractSeasonNumber(ctx.Name)
	if !found {
		// If not found in name, default to 0 or error
		return nil, &provider.ProviderError{
			Provider: providerName,
			Code:     "PARSE_ERROR",
			Message:  "Could not extract season number from: " + name,
			Retry:    false,
		}
	}

	// Try to extract show name from context
	showName, year := ResolveShowInfo(ctx)

	// Create metadata
	metadata := createMetadata(provider.MediaTypeSeason, showName, year, 1.0)
	metadata.Core.SeasonNum = seasonNum

	// Add season to sources
	metadata.Sources["season"] = providerName

	return metadata, nil
}

// CanParse checks if this parser can handle the input
func (p *SeasonParser) CanParse(name string, node *treeview.Node[treeview.FileInfo]) bool {
	// Check if it's a directory
	if node != nil && !node.Data().IsDir() {
		return false
	}

	// Check if it contains a season number
	_, isSeason := ExtractSeasonNumber(name)
	return isSeason
}
