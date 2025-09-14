package media

import (
	"regexp"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

// Helper to build a season parent node with a single child (episode file)
func buildEpisodeNode(parentName, fileName string) *treeview.Node[treeview.FileInfo] {
	p := treeview.NewNode(parentName, parentName, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(parentName, true), Path: parentName})
	c := treeview.NewNode(fileName, fileName, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(fileName, false), Path: parentName + "/" + fileName})
	p.AddChild(c)
	return c
}

func TestIsVideo(t *testing.T) {
	// t.Parallel() // avoid race with potential global regex compilation (safe but keep serial for clarity)
	tests := []struct {
		in   string
		want bool
	}{
		{"movie.mkv", true},
		{"clip.MP4", true},
		{"trailer.webm", true},
		{"notes.txt", false},
		{"archive.zip", false},
	}
	for _, tc := range tests {
		if got := IsVideo(tc.in); got != tc.want {
			t.Errorf("IsVideo(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsSubtitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"episode.en.srt", true},
		{"episode.SRT", true},
		{"movie.eng.sub", true},
		{"notes.txt", false},
		{"movie.mkv", false},
	}
	for _, tc := range tests {
		if got := IsSubtitle(tc.in); got != tc.want {
			t.Errorf("IsSubtitle(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsSample(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"sample.mkv", true},
		{"Movie.Sample.mp4", true},
		{"SAMPLE-video.avi", true},
		{"Samples", true},
		{"movie-sample", true},
		{"regular.mkv", false},
		{"example.txt", false},
		{"sampling.doc", false},
	}
	for _, tc := range tests {
		if got := IsSample(tc.in); got != tc.want {
			t.Errorf("IsSample(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestExtractSubtitleSuffix(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"show.S01E01.en.srt", ".en.srt"},
		{"show.S01E01.srt", ".srt"},
		{"movie.eng.srt", ".eng.srt"},
		{"movie.en-US.srt", ".en-US.srt"},
		{"movie.mp4", ""},
		{"noext", ""},
	}
	for _, tc := range tests {
		if got := extractSubtitleSuffix(tc.in); got != tc.want {
			t.Errorf("ExtractSubtitleSuffix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractSeasonNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"Season 02", 2, true},
		{"s1", 1, true},
		{"season-3", 3, true},
		{"5", 5, true},
		{"Season_11 Extras", 11, true},
		{"Specials", 0, false},
	}
	for _, tc := range tests {
		got, ok := ExtractSeasonNumber(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ExtractSeasonNumber(%q) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestExtractExtension(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"file.mkv", ".mkv"},
		{"archive.tar.gz", ".gz"},
		{"noext", ""},
		{"trailingdot.", "."},
	}
	for _, tc := range tests {
		if got := extractExtension(tc.in); got != tc.want {
			t.Errorf("ExtractExtension(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseSeasonEpisode_DirectPatterns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    string
		wantS int
		wantE int
		ok    bool
	}{
		{"StandardUpper", "Show.Name.S01E02.mkv", 1, 2, true},
		{"StandardLower", "show.name.s01e06.mkv", 1, 6, true},
		{"Alt1x", "Show.Name.1x07.mkv", 1, 7, true},
		{"DottedPadded", "1.04.1080p.mkv", 1, 4, true},
		{"DottedUnpadded", "2.4.720p.mkv", 2, 4, true},
		{"DottedSeason10", "10.12.mkv", 10, 12, true},
		{"RejectYearLike", "2024.05.Doc.mkv", 0, 0, false}, // season > 100 rejected
		{"SeasonEpisodeZeroes", "S00E00.mkv", 0, 0, true},  // accepted (zero season/episode allowed by regex path)
		{"TooLargeSeason", "101.02.mkv", 0, 0, false},      // dotted season > 100 -> reject & no other pattern
	}
	for _, tc := range tests {
		c := tc
		t.Run(c.name, func(t *testing.T) {
			s, e, ok := ParseSeasonEpisode(c.in, nil)
			if diff := cmp.Diff(struct {
				S, E int
				Ok   bool
			}{c.wantS, c.wantE, c.ok}, struct {
				S, E int
				Ok   bool
			}{s, e, ok}); diff != "" {
				t.Fatalf("ParseSeasonEpisode(%q) mismatch (-want +got)\n%s", c.in, diff)
			}
		})
	}
}

func TestParseSeasonEpisode_FallbackContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		parent   string
		filename string
		wantS    int
		wantE    int
		ok       bool
	}{
		{"EpisodeNumberWithSeasonParent", "Season 2", "Episode 12.mkv", 2, 12, true},
		{"EpisodeNumberWithLowerSParent", "s3", "E5.mkv", 3, 5, true},
		{"ParentNoSeason", "Extras", "E12.mkv", 0, 0, false},
	}
	for _, tc := range tests {
		c := tc
		t.Run(c.name, func(t *testing.T) {
			node := buildEpisodeNode(c.parent, c.filename)
			s, e, ok := ParseSeasonEpisode(c.filename, node)
			if s != c.wantS || e != c.wantE || ok != c.ok {
				t.Errorf("ParseSeasonEpisode(%q,parent=%q) = (%d,%d,%v), want (%d,%d,%v)", c.filename, c.parent, s, e, ok, c.wantS, c.wantE, c.ok)
			}
		})
	}
}

func TestParseSeasonEpisode_FallbackFailure_NoParentSeason(t *testing.T) {
	t.Parallel()
	node := treeview.NewNode("Episode 4.mkv", "Episode 4.mkv", treeview.FileInfo{FileInfo: core.NewSimpleFileInfo("Episode 4.mkv", false), Path: "Episode 4.mkv"})
	if s, e, ok := ParseSeasonEpisode("Episode 4.mkv", node); ok {
		t.Errorf("ParseSeasonEpisode(noParent) = (%d,%d,%v), want failure", s, e, ok)
	}
}

func TestParseSeasonEpisode_FallbackFailure_NilNode(t *testing.T) {
	t.Parallel()
	if s, e, ok := ParseSeasonEpisode("Episode 4.mkv", nil); ok {
		t.Errorf("ParseSeasonEpisode(nilNode) = (%d,%d,%v), want failure", s, e, ok)
	}
}

func TestFirstIntFromRegexps_EmptySubmatch(t *testing.T) {
	t.Parallel()

	// Test with regex that has empty capturing groups to hit line 150-151
	// This regex matches but has empty first capture group
	re := regexp.MustCompile(`(\d*)test(\d+)`)

	// Input where first group matches empty string but second matches number
	got, ok := firstIntFromRegexps("test123", re)
	if !ok || got != 123 {
		t.Errorf("firstIntFromRegexps with empty submatch = (%d,%v), want (123,true)", got, ok)
	}
}

func TestExtractSubtitleSuffixEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "non_subtitle_file",
			filename: "video.mp4",
			expected: "",
		},
		{
			name:     "subtitle_without_language",
			filename: "movie.srt",
			expected: ".srt",
		},
		{
			name:     "subtitle_with_invalid_pattern",
			filename: "movie.123.srt",
			expected: ".srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubtitleSuffix(tt.filename)
			if result != tt.expected {
				t.Errorf("extractSubtitleSuffix(%q) = %q, want %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestParseSeasonEpisode_NoEpisodeNumber(t *testing.T) {
	filename := "random_file.mp4"
	season, episode, ok := ParseSeasonEpisode(filename, nil)
	if ok {
		t.Errorf("ParseSeasonEpisode(%q, nil) = (%d, %d, true), want (0, 0, false)", filename, season, episode)
	}
}

func TestExtractShowInfo(t *testing.T) {
	tests := []struct {
		name         string
		nodeName     string
		parentName   string
		isFile       bool
		wantShowName string
		wantYear     string
	}{
		{
			name:         "EpisodeFileWithShowInName",
			nodeName:     "Breaking Bad S01E01 Pilot.mkv",
			isFile:       true,
			wantShowName: "Breaking Bad",
			wantYear:     "",
		},
		{
			name:         "EpisodeFileWithShowAndYear",
			nodeName:     "The Office (2005) S02E03.mp4",
			isFile:       true,
			wantShowName: "The Office",
			wantYear:     "2005",
		},
		{
			name:         "EpisodeFileNoShowFallbackToParent",
			nodeName:     "S01E01.mkv",
			parentName:   "Breaking Bad",
			isFile:       true,
			wantShowName: "Breaking Bad",
			wantYear:     "",
		},
		{
			name:         "SeasonFolderWithShow",
			nodeName:     "Breaking Bad Season 1",
			isFile:       false,
			wantShowName: "Breaking Bad",
			wantYear:     "",
		},
		{
			name:         "SeasonFolderNoShow",
			nodeName:     "Season 01",
			parentName:   "The Office (2005)",
			isFile:       false,
			wantShowName: "The Office",
			wantYear:     "2005",
		},
		{
			name:         "ShowFolder",
			nodeName:     "Stranger Things (2016)",
			isFile:       false,
			wantShowName: "Stranger Things",
			wantYear:     "2016",
		},
		{
			name:         "EpisodeWithDottedFormat",
			nodeName:     "The.Office.1.04.The.Alliance.mkv",
			isFile:       true,
			wantShowName: "The Office",
			wantYear:     "",
		},
		{
			name:         "EpisodeWithMultipleWordsShow",
			nodeName:     "Game of Thrones S08E06 The Iron Throne.mkv",
			isFile:       true,
			wantShowName: "Game of Thrones",
			wantYear:     "",
		},
		{
			name:         "DottedFormatWithTrailingSeparator",
			nodeName:     "Better.Call.Saul.1.04.1080p.mkv",
			isFile:       true,
			wantShowName: "Better Call Saul",
			wantYear:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create node structure
			fileInfo := treeview.FileInfo{
				FileInfo: core.NewSimpleFileInfo(tt.nodeName, !tt.isFile),
				Path:     tt.nodeName,
			}
			node := treeview.NewNode(tt.nodeName, tt.nodeName, fileInfo)

			// Add parent if specified
			if tt.parentName != "" {
				parentInfo := treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo(tt.parentName, true),
					Path:     tt.parentName,
				}
				parent := treeview.NewNode(tt.parentName, tt.parentName, parentInfo)
				parent.AddChild(node)
			}

			gotShowName, gotYear := ExtractShowInfo(node, tt.isFile)

			if gotShowName != tt.wantShowName {
				t.Errorf("ExtractShowInfo(%q) showName = %q, want %q", tt.nodeName, gotShowName, tt.wantShowName)
			}
			if gotYear != tt.wantYear {
				t.Errorf("ExtractShowInfo(%q) year = %q, want %q", tt.nodeName, gotYear, tt.wantYear)
			}
		})
	}
}

func TestProcessEpisodeNode(t *testing.T) {
	tests := []struct {
		name         string
		nodeName     string
		parentName   string
		wantShowName string
		wantYear     string
		wantSeason   int
		wantEpisode  int
		wantFound    bool
	}{
		{
			name:         "StandardEpisodeWithShow",
			nodeName:     "Breaking Bad S01E01 Pilot.mkv",
			wantShowName: "Breaking Bad",
			wantYear:     "",
			wantSeason:   1,
			wantEpisode:  1,
			wantFound:    true,
		},
		{
			name:         "EpisodeWithShowAndYear",
			nodeName:     "The Office (2005) S02E03 Office Olympics.mp4",
			wantShowName: "The Office",
			wantYear:     "2005",
			wantSeason:   2,
			wantEpisode:  3,
			wantFound:    true,
		},
		{
			name:         "EpisodeNoShowInFileUsesParent",
			nodeName:     "S03E05.mkv",
			parentName:   "Season 03",
			wantShowName: "",
			wantYear:     "",
			wantSeason:   3,
			wantEpisode:  5,
			wantFound:    true,
		},
		{
			name:         "EpisodeWithXFormat",
			nodeName:     "Breaking Bad 1x07 A No-Rough-Stuff-Type Deal.mkv",
			wantShowName: "Breaking Bad",
			wantYear:     "",
			wantSeason:   1,
			wantEpisode:  7,
			wantFound:    true,
		},
		{
			name:         "DottedFormatEpisode",
			nodeName:     "The.Office.1.04.The.Alliance.mkv",
			wantShowName: "The Office",
			wantYear:     "",
			wantSeason:   1,
			wantEpisode:  4,
			wantFound:    true,
		},
		{
			name:         "DirectoryNode",
			nodeName:     "Season 01",
			wantShowName: "",
			wantYear:     "",
			wantSeason:   0,
			wantEpisode:  0,
			wantFound:    false,
		},
		{
			name:         "NoEpisodePattern",
			nodeName:     "Behind the Scenes.mkv",
			wantShowName: "",
			wantYear:     "",
			wantSeason:   0,
			wantEpisode:  0,
			wantFound:    false,
		},
		{
			name:         "BetterCallSaulDotted",
			nodeName:     "Better.Call.Saul.1.04.1080p.mkv",
			wantShowName: "Better Call Saul",
			wantYear:     "",
			wantSeason:   1,
			wantEpisode:  4,
			wantFound:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create node structure
			fileInfo := treeview.FileInfo{
				FileInfo: core.NewSimpleFileInfo(tt.nodeName, tt.nodeName == "Season 01"),
				Path:     tt.nodeName,
			}
			node := treeview.NewNode(tt.nodeName, tt.nodeName, fileInfo)

			// Add parent if specified
			if tt.parentName != "" {
				parentInfo := treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo(tt.parentName, true),
					Path:     tt.parentName,
				}
				parent := treeview.NewNode(tt.parentName, tt.parentName, parentInfo)
				parent.AddChild(node)
			}

			gotShowName, gotYear, gotSeason, gotEpisode, gotFound := ProcessEpisodeNode(node)

			if gotFound != tt.wantFound {
				t.Errorf("ProcessEpisodeNode(%q) found = %v, want %v", tt.nodeName, gotFound, tt.wantFound)
			}
			if gotShowName != tt.wantShowName {
				t.Errorf("ProcessEpisodeNode(%q) showName = %q, want %q", tt.nodeName, gotShowName, tt.wantShowName)
			}
			if gotYear != tt.wantYear {
				t.Errorf("ProcessEpisodeNode(%q) year = %q, want %q", tt.nodeName, gotYear, tt.wantYear)
			}
			if gotSeason != tt.wantSeason {
				t.Errorf("ProcessEpisodeNode(%q) season = %d, want %d", tt.nodeName, gotSeason, tt.wantSeason)
			}
			if gotEpisode != tt.wantEpisode {
				t.Errorf("ProcessEpisodeNode(%q) episode = %d, want %d", tt.nodeName, gotEpisode, tt.wantEpisode)
			}
		})
	}
}

func TestExtractShowInfoHierarchy(t *testing.T) {
	// Test the hierarchy traversal for complex structures
	tests := []struct {
		name         string
		structure    []string // [episode, season, show] names
		wantShowName string
		wantYear     string
	}{
		{
			name:         "EpisodeInSeasonInShow",
			structure:    []string{"S01E01.mkv", "Season 01", "Breaking Bad (2008)"},
			wantShowName: "Breaking Bad",
			wantYear:     "2008",
		},
		{
			name:         "EpisodeWithShowInSeasonFolder",
			structure:    []string{"01.mkv", "Breaking Bad Season 01", "TV Shows"},
			wantShowName: "Breaking Bad",
			wantYear:     "",
		},
		{
			name:         "EpisodeWithFullShowName",
			structure:    []string{"Breaking Bad S01E01.mkv", "Season 01", "TV Shows"},
			wantShowName: "Breaking Bad",
			wantYear:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build hierarchy from bottom up
			var node *treeview.Node[treeview.FileInfo]

			for i, name := range tt.structure {
				fileInfo := treeview.FileInfo{
					FileInfo: core.NewSimpleFileInfo(name, i > 0),
					Path:     name,
				}
				newNode := treeview.NewNode(name, name, fileInfo)

				if node != nil {
					newNode.AddChild(node)
				}
				node = newNode
			}

			// Get the episode node (deepest child)
			for node.Children() != nil && len(node.Children()) > 0 {
				node = node.Children()[0]
			}

			gotShowName, gotYear := ExtractShowInfo(node, true)

			if diff := cmp.Diff(tt.wantShowName, gotShowName); diff != "" {
				t.Errorf("ExtractShowInfo hierarchy mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantYear, gotYear); diff != "" {
				t.Errorf("ExtractShowInfo year mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
