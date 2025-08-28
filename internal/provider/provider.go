package provider

// MetadataProvider defines the interface for fetching metadata from external sources
type MetadataProvider interface {
	SearchMovie(name, year string) (*EnrichedMetadata, error)
	SearchTVShow(name string) (*EnrichedMetadata, error)
	GetSeasonInfo(showID, seasonNum int) (*EnrichedMetadata, error)
	GetEpisodeInfo(showID, season, episode int) (*EnrichedMetadata, error)
}

// EnrichedMetadata contains metadata fetched from external providers
type EnrichedMetadata struct {
	// Movie/Show common fields
	Title    string
	Year     string
	Overview string
	Rating   float32
	Genres   []string
	Runtime  int
	Tagline  string
	ID       int

	// TV specific fields
	ShowName    string
	SeasonName  string
	EpisodeName string
	EpisodeAir  string
	SeasonCount int
	SeasonNum   int
	EpisodeNum  int

	// Original parsed data (fallback)
	LocalName    string
	LocalYear    string
	LocalSeason  int
	LocalEpisode int
}
