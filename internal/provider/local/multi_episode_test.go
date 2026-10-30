package local

import "testing"

func TestMultiEpisodeParser(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		season int
		first  int
		last   int
	}{
		{name: "consecutive", token: "S01E01E02", season: 1, first: 1, last: 2},
		{name: "jellyfin range", token: "S01E01-E02", season: 1, first: 1, last: 2},
		{name: "numeric range", token: "S01E01-02", season: 1, first: 1, last: 2},
		{name: "three episodes", token: "S01E01E02E03", season: 1, first: 1, last: 3},
		{name: "longer range", token: "S02E05-E10", season: 2, first: 5, last: 10},
		{name: "lowercase", token: "s1e1e2", season: 1, first: 1, last: 2},
		{name: "specials", token: "S00E00E01", season: 0, first: 0, last: 1},
		{name: "three digit episodes", token: "S03E100-E102", season: 3, first: 100, last: 102},
		{name: "x consecutive", token: "1x01x02", season: 1, first: 1, last: 2},
		{name: "x range", token: "1x01-02", season: 1, first: 1, last: 2},
		{name: "single episode", token: "S01E01", season: 1, first: 1},
		{name: "quality suffix", token: "S01E01-1080p", season: 1, first: 1},
		{name: "audio suffix", token: "S01E01 Opus 2.0", season: 1, first: 1},
		{name: "reversed range", token: "S01E02-E01", season: 1, first: 2},
		{name: "duplicate episode", token: "S01E01E01", season: 1, first: 1},
		{name: "out of bounds", token: "S01E01-E301", season: 1, first: 1},
		{name: "tagged range", token: "S01E01 [S02E03-E04]", season: 1, first: 1},
	}
	parser := NewEpisodeParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := "Example.Show.(1979)." + tt.token + ".mkv"
			node := newTestNode(filename, false)
			if !parser.CanParse(filename, node) {
				t.Fatal("file should be recognized as an episode")
			}
			meta, err := parser.Parse(filename, node)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Core.SeasonNum != tt.season || meta.Core.EpisodeNum != tt.first || meta.Core.EpisodeEndNum != tt.last {
				t.Errorf("episode numbers = (%d, %d, %d), want (%d, %d, %d)",
					meta.Core.SeasonNum, meta.Core.EpisodeNum, meta.Core.EpisodeEndNum, tt.season, tt.first, tt.last)
			}
			if meta.Core.Title != "Example Show" || meta.Core.Year != "1979" {
				t.Errorf("show = %q (%s), want Example Show (1979)", meta.Core.Title, meta.Core.Year)
			}
			season, first, found := SeasonEpisodeFromContext(NewParseContext(filename, node))
			if !found || season != tt.season || first != tt.first {
				t.Errorf("detection numbers = (%d, %d, %t), want (%d, %d, true)", season, first, found, tt.season, tt.first)
			}
		})
	}
}
