package local

import (
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
)

// MovieParser handles parsing of movie files and folders
type MovieParser struct{}

// NewMovieParser creates a new movie parser
func NewMovieParser() *MovieParser {
	return &MovieParser{}
}

// Parse extracts movie metadata from a filename or folder name
func (p *MovieParser) Parse(name string, node *treeview.Node[treeview.FileInfo]) (*provider.Metadata, error) {
	ctx := NewParseContext(name, node)

	// Extract movie name and year
	movieName, year := p.extractMovieInfo(ctx)

	if movieName == "" {
		// Clean it up from the working name
		movieName, year = ctx.TitleAndYear()
	}

	// Create metadata
	metadata := createMetadata(provider.MediaTypeMovie, movieName, year, 1.0)

	// For movies, ensure we set the title correctly
	metadata.Core.Title = movieName

	return metadata, nil
}

// CanParse checks if this parser can handle the input
func (p *MovieParser) CanParse(name string, node *treeview.Node[treeview.FileInfo]) bool {
	// Movies can be either video files or directories
	if node != nil && !node.Data().IsDir() {
		// If it's a file, it should be a video file
		if !IsVideo(name) {
			return false
		}

		// Reject if it looks like an episode
		_, _, isEpisode := SeasonEpisodeFromContext(NewParseContext(name, node))
		if isEpisode {
			return false
		}
	}

	// Accept as potential movie
	return true
}

// extractMovieInfo extracts movie name and year from a path
func (p *MovieParser) extractMovieInfo(ctx ParseContext) (movieName, year string) {
	return ExtractNameAndYear(ctx.WorkingName())
}
