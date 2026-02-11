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
