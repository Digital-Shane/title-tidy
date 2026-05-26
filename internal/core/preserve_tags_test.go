package core

import "testing"

func TestPreserveExistingBracketTags(t *testing.T) {
	tests := []struct {
		name      string
		generated string
		source    string
		enabled   bool
		want      string
	}{
		{
			name:      "disabled",
			generated: "S01E01",
			source:    "Show.S01E01.[Extended]",
			enabled:   false,
			want:      "S01E01",
		},
		{
			name:      "preserves_single_tag",
			generated: "S01E01",
			source:    "Show.S01E01.[Extended]",
			enabled:   true,
			want:      "S01E01[Extended]",
		},
		{
			name:      "preserves_multiple_tags",
			generated: "Movie (1999) [imdbid-tt0133093]",
			source:    "Movie - [Uncut][h265]",
			enabled:   true,
			want:      "Movie (1999) [imdbid-tt0133093][Uncut][h265]",
		},
		{
			name:      "deduplicates_case_insensitive",
			generated: "Movie (1999) [uncut]",
			source:    "Movie - [Uncut]",
			enabled:   true,
			want:      "Movie (1999) [uncut]",
		},
		{
			name:      "preserves_anime_source_tags",
			generated: "S00E17",
			source:    "[sam] Kaichou wa Maid-sama! - 17 [BD 1080p FLAC] [0E123677]",
			enabled:   true,
			want:      "S00E17[sam][BD 1080p FLAC][0E123677]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreserveExistingBracketTags(tt.generated, tt.source, tt.enabled)
			if got != tt.want {
				t.Errorf("PreserveExistingBracketTags(%q, %q, %v) = %q, want %q", tt.generated, tt.source, tt.enabled, got, tt.want)
			}
		})
	}
}
