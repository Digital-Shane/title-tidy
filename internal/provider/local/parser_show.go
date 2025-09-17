package local

import (
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// ShowParser handles parsing of TV show folders
type ShowParser struct{}

// NewShowParser creates a new show parser
func NewShowParser() *ShowParser {
	return &ShowParser{}
}

// Parse extracts show metadata from a folder name
func (p *ShowParser) Parse(name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error) {
	ctx := NewParseContext(name, node)

	// Extract show name and year
	showName, year := ResolveShowInfo(ctx)

	// Create metadata
	metadata := createMetadata(provider.MediaTypeShow, showName, year, 1.0)

	// For shows, the title is the show name
	metadata.Core.Title = showName

	return metadata, nil
}

// CanParse checks if this parser can handle the input
func (p *ShowParser) CanParse(name string, node *treeview.Node[treeview.FileInfo]) bool {
	// Show folders typically don't have specific patterns
	// They're identified more by context (containing season folders)
	// For now, we'll accept any directory that isn't clearly something else

	// Check if it's a directory
	if node != nil && !node.Data().IsDir() {
		return false
	}

	// Reject if it looks like a season folder
	if _, isSeason := ExtractSeasonNumber(name); isSeason {
		return false
	}

	return true
}
