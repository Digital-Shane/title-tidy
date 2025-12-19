package local

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/treeview"
	"github.com/google/go-cmp/cmp"
)

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	time  time.Time
	isDir bool
	sys   interface{}
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.time }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return m.sys }

func newTestNode(name string, isDir bool) *treeview.Node[treeview.FileInfo] {
	node := treeview.NewNodeSimple(name, treeview.FileInfo{
		FileInfo: &mockFileInfo{
			name:  name,
			isDir: isDir,
		},
		Path: name,
	})
	return node
}

func TestExtractNameAndYear(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantYear string
	}{
		{
			name:     "MovieWithYear",
			input:    "The Matrix (1999)",
			wantName: "The Matrix",
			wantYear: "1999",
		},
		{
			name:     "MovieWithYearNoBrackets",
			input:    "Inception 2010",
			wantName: "Inception",
			wantYear: "2010",
		},
		{
			name:     "ShowWithDots",
			input:    "Breaking.Bad.2008",
			wantName: "Breaking Bad",
			wantYear: "2008",
		},
		{
			name:     "ShowWithEncodingTags",
			input:    "Game.of.Thrones.S01E01.1080p.BluRay.x264",
			wantName: "Game of Thrones S01E01",
			wantYear: "",
		},
		{
			name:     "MovieWithQualityTags",
			input:    "Avatar.2009.1080p.BluRay.x264-YIFY",
			wantName: "Avatar",
			wantYear: "2009",
		},
		{
			name:     "ShowNoYear",
			input:    "The Office",
			wantName: "The Office",
			wantYear: "",
		},
		{
			name:     "YearRange",
			input:    "Stranger Things (2016-2024)",
			wantName: "Stranger Things",
			wantYear: "2016",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotYear := ExtractNameAndYear(tt.input)
			if gotName != tt.wantName {
				t.Errorf("ExtractNameAndYear(%q) name = %v, want %v", tt.input, gotName, tt.wantName)
			}
			if gotYear != tt.wantYear {
				t.Errorf("ExtractNameAndYear(%q) year = %v, want %v", tt.input, gotYear, tt.wantYear)
			}
		})
	}
}

func TestExtractSeasonNumber(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNum   int
		wantFound bool
	}{
		{"ExplicitSeason", "Season 01", 1, true},
		{"SeasonWithSpace", "Season 2", 2, true},
		{"ShortForm", "S01", 1, true},
		{"JustNumber", "3", 3, true},
		{"WithShow", "Breaking Bad Season 1", 1, true},
		{"YearNotSeason", "2019", 0, false},
		{"NoSeason", "Random Text", 0, false},
		{"SeasonZero", "Season 0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotFound := ExtractSeasonNumber(tt.input)
			if gotNum != tt.wantNum || gotFound != tt.wantFound {
				t.Errorf("ExtractSeasonNumber(%q) = (%v, %v), want (%v, %v)",
					tt.input, gotNum, gotFound, tt.wantNum, tt.wantFound)
			}
		})
	}
}

func TestEpisodeParser(t *testing.T) {
	parser := NewEpisodeParser()

	tests := []struct {
		name         string
		filename     string
		wantSeason   int
		wantEpisode  int
		wantShow     string
		wantCanParse bool
	}{
		{
			name:         "StandardFormat",
			filename:     "Breaking.Bad.S01E01.Pilot.mkv",
			wantSeason:   1,
			wantEpisode:  1,
			wantShow:     "Breaking Bad",
			wantCanParse: true,
		},
		{
			name:         "XFormat",
			filename:     "Game.of.Thrones.1x09.Baelor.mp4",
			wantSeason:   1,
			wantEpisode:  9,
			wantShow:     "Game of Thrones",
			wantCanParse: true,
		},
		{
			name:         "DottedFormat",
			filename:     "The.Wire.1.04.Old.Cases.avi",
			wantSeason:   1,
			wantEpisode:  4,
			wantShow:     "The Wire",
			wantCanParse: true,
		},
		{
			name:         "NotAnEpisode",
			filename:     "Inception.2010.1080p.mkv",
			wantSeason:   0,
			wantEpisode:  0,
			wantShow:     "",
			wantCanParse: false,
		},
		{
			name:         "EpisodeOnly",
			filename:     "E05.mkv",
			wantSeason:   0,
			wantEpisode:  5,
			wantShow:     "",
			wantCanParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := parser.CanParse(tt.filename, nil)
			if canParse != tt.wantCanParse {
				t.Errorf("CanParse(%q) = %v, want %v", tt.filename, canParse, tt.wantCanParse)
			}

			if !tt.wantCanParse {
				return
			}

			// Test Parse
			meta, err := parser.Parse(tt.filename, nil)
			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.filename, err)
				return
			}

			if meta.Core.SeasonNum != tt.wantSeason {
				t.Errorf("Parse(%q) season = %v, want %v", tt.filename, meta.Core.SeasonNum, tt.wantSeason)
			}
			if meta.Core.EpisodeNum != tt.wantEpisode {
				t.Errorf("Parse(%q) episode = %v, want %v", tt.filename, meta.Core.EpisodeNum, tt.wantEpisode)
			}
			if tt.wantShow != "" && meta.Core.Title != tt.wantShow {
				t.Errorf("Parse(%q) show = %v, want %v", tt.filename, meta.Core.Title, tt.wantShow)
			}
		})
	}
}

func TestSeasonEpisodeFromContext(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		parentName  string
		wantSeason  int
		wantEpisode int
		wantFound   bool
	}{
		{
			name:        "Standard",
			filename:    "Breaking.Bad.S01E01.mkv",
			wantSeason:  1,
			wantEpisode: 1,
			wantFound:   true,
		},
		{
			name:        "ExplicitWithAudioChannels",
			filename:    "Show.S01E02.AAC.5.1.1080p.mkv",
			wantSeason:  1,
			wantEpisode: 2,
			wantFound:   true,
		},
		{
			name:        "Dotted",
			filename:    "The.Wire.1.04.avi",
			wantSeason:  1,
			wantEpisode: 4,
			wantFound:   true,
		},
		{
			name:        "EpisodeWithParentSeason",
			filename:    "E05.mkv",
			parentName:  "Season 02",
			wantSeason:  2,
			wantEpisode: 5,
			wantFound:   true,
		},
		{
			name:        "EpisodeOnly",
			filename:    "Episode 7.mp4",
			wantSeason:  0,
			wantEpisode: 7,
			wantFound:   true,
		},
		{
			name:      "NoMatch",
			filename:  "Notes.txt",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newTestNode(tt.filename, false)
			if tt.parentName != "" {
				parent := newTestNode(tt.parentName, true)
				parent.AddChild(node)
			}

			ctx := NewParseContext(tt.filename, node)
			gotSeason, gotEpisode, gotFound := SeasonEpisodeFromContext(ctx)

			if gotFound != tt.wantFound {
				t.Errorf("SeasonEpisodeFromContext(%q) found = %v, want %v", tt.filename, gotFound, tt.wantFound)
				return
			}

			if !tt.wantFound {
				return
			}

			if gotSeason != tt.wantSeason {
				t.Errorf("SeasonEpisodeFromContext(%q) season = %v, want %v", tt.filename, gotSeason, tt.wantSeason)
			}
			if gotEpisode != tt.wantEpisode {
				t.Errorf("SeasonEpisodeFromContext(%q) episode = %v, want %v", tt.filename, gotEpisode, tt.wantEpisode)
			}
		})
	}
}

func TestMovieParser(t *testing.T) {
	parser := NewMovieParser()

	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantYear  string
	}{
		{
			name:      "MovieWithYear",
			input:     "The.Matrix.1999.1080p.BluRay.mkv",
			wantTitle: "The Matrix",
			wantYear:  "1999",
		},
		{
			name:      "MovieFolder",
			input:     "Inception (2010)",
			wantTitle: "Inception",
			wantYear:  "2010",
		},
		{
			name:      "MovieNoYear",
			input:     "Avatar.mkv",
			wantTitle: "Avatar",
			wantYear:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock node for file detection
			isDir := !IsVideo(tt.input)
			node := &treeview.Node[treeview.FileInfo]{}
			node.SetData(treeview.FileInfo{
				FileInfo: &mockFileInfo{
					name:  tt.input,
					isDir: isDir,
				},
				Path: tt.input,
			})

			meta, err := parser.Parse(tt.input, node)
			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
				return
			}

			if meta.Core.Title != tt.wantTitle {
				t.Errorf("Parse(%q) title = %v, want %v", tt.input, meta.Core.Title, tt.wantTitle)
			}
			if meta.Core.Year != tt.wantYear {
				t.Errorf("Parse(%q) year = %v, want %v", tt.input, meta.Core.Year, tt.wantYear)
			}
		})
	}
}

func TestResolveShowInfo(t *testing.T) {
	tests := []struct {
		name      string
		buildNode func() *treeview.Node[treeview.FileInfo]
		wantTitle string
		wantYear  string
	}{
		{
			name: "EpisodeFromFilename",
			buildNode: func() *treeview.Node[treeview.FileInfo] {
				return newTestNode("Breaking.Bad.S01E01.mkv", false)
			},
			wantTitle: "Breaking Bad",
			wantYear:  "",
		},
		{
			name: "EpisodeFromParents",
			buildNode: func() *treeview.Node[treeview.FileInfo] {
				show := newTestNode("Breaking Bad (2008)", true)
				season := newTestNode("Season 01", true)
				episode := newTestNode("Pilot.mkv", false)
				season.AddChild(episode)
				show.AddChild(season)
				return episode
			},
			wantTitle: "Breaking Bad",
			wantYear:  "2008",
		},
		{
			name: "SeasonFolder",
			buildNode: func() *treeview.Node[treeview.FileInfo] {
				show := newTestNode("Breaking Bad (2008)", true)
				season := newTestNode("Season 02", true)
				show.AddChild(season)
				return season
			},
			wantTitle: "Breaking Bad",
			wantYear:  "2008",
		},
		{
			name: "ShowFolder",
			buildNode: func() *treeview.Node[treeview.FileInfo] {
				return newTestNode("The Office (2005)", true)
			},
			wantTitle: "The Office",
			wantYear:  "2005",
		},
		{
			name: "SeasonWithoutParent",
			buildNode: func() *treeview.Node[treeview.FileInfo] {
				return newTestNode("Season 01", true)
			},
			wantTitle: "",
			wantYear:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.buildNode()
			ctx := NewParseContext(node.Name(), node)
			title, year := ResolveShowInfo(ctx)

			if title != tt.wantTitle {
				t.Errorf("ResolveShowInfo(%q) title = %v, want %v", node.Name(), title, tt.wantTitle)
			}
			if year != tt.wantYear {
				t.Errorf("ResolveShowInfo(%q) year = %v, want %v", node.Name(), year, tt.wantYear)
			}
		})
	}
}

func TestProviderFetch(t *testing.T) {
	p := New()
	ctx := context.Background()

	tests := []struct {
		name        string
		request     provider.FetchRequest
		wantTitle   string
		wantSeason  int
		wantEpisode int
		wantYear    string
		wantErr     bool
	}{
		{
			name: "FetchShow",
			request: provider.FetchRequest{
				MediaType: provider.MediaTypeShow,
				Name:      "Breaking Bad (2008)",
			},
			wantTitle: "Breaking Bad",
			wantYear:  "2008",
		},
		{
			name: "FetchSeason",
			request: provider.FetchRequest{
				MediaType: provider.MediaTypeSeason,
				Name:      "Season 02",
			},
			wantSeason: 2,
		},
		{
			name: "FetchEpisode",
			request: provider.FetchRequest{
				MediaType: provider.MediaTypeEpisode,
				Name:      "Breaking.Bad.S01E01.mkv",
			},
			wantTitle:   "Breaking Bad",
			wantSeason:  1,
			wantEpisode: 1,
		},
		{
			name: "FetchMovie",
			request: provider.FetchRequest{
				MediaType: provider.MediaTypeMovie,
				Name:      "The Matrix (1999)",
			},
			wantTitle: "The Matrix",
			wantYear:  "1999",
		},
		{
			name: "EmptyName",
			request: provider.FetchRequest{
				MediaType: provider.MediaTypeMovie,
				Name:      "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := p.Fetch(ctx, tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Fetch() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Fetch() unexpected error: %v", err)
				return
			}

			if tt.wantTitle != "" && meta.Core.Title != tt.wantTitle {
				t.Errorf("Fetch() title = %v, want %v", meta.Core.Title, tt.wantTitle)
			}
			if tt.wantYear != "" && meta.Core.Year != tt.wantYear {
				t.Errorf("Fetch() year = %v, want %v", meta.Core.Year, tt.wantYear)
			}
			if tt.wantSeason > 0 && meta.Core.SeasonNum != tt.wantSeason {
				t.Errorf("Fetch() season = %v, want %v", meta.Core.SeasonNum, tt.wantSeason)
			}
			if tt.wantEpisode > 0 && meta.Core.EpisodeNum != tt.wantEpisode {
				t.Errorf("Fetch() episode = %v, want %v", meta.Core.EpisodeNum, tt.wantEpisode)
			}
		})
	}
}

func TestProviderCapabilities(t *testing.T) {
	p := New()
	caps := p.Capabilities()

	// Should support all media types
	expectedTypes := []provider.MediaType{
		provider.MediaTypeMovie,
		provider.MediaTypeShow,
		provider.MediaTypeSeason,
		provider.MediaTypeEpisode,
	}

	if diff := cmp.Diff(expectedTypes, caps.MediaTypes); diff != "" {
		t.Errorf("Capabilities() MediaTypes mismatch (-want +got):\n%s", diff)
	}

	if caps.RequiresAuth {
		t.Error("Capabilities() RequiresAuth = true, want false")
	}

	if caps.Priority != 0 {
		t.Errorf("Capabilities() Priority = %v, want 0", caps.Priority)
	}
}

func TestDetectorDetect(t *testing.T) {
	detector := NewDetector()

	episodeFile := newTestNode("Breaking.Bad.S01E01.mkv", false)
	movieFile := newTestNode("Inception.2010.mkv", false)
	seasonDir := newTestNode("Season 01", true)
	showDir := newTestNode("Breaking Bad", true)
	showSeason := newTestNode("Season 01", true)
	showDir.AddChild(showSeason)
	defaultDir := newTestNode("Extras", true)
	unsupportedFile := newTestNode("readme.txt", false)

	tests := []struct {
		name    string
		node    *treeview.Node[treeview.FileInfo]
		want    provider.MediaType
		wantErr bool
	}{
		{
			name: "EpisodeFile",
			node: episodeFile,
			want: provider.MediaTypeEpisode,
		},
		{
			name: "MovieFile",
			node: movieFile,
			want: provider.MediaTypeMovie,
		},
		{
			name: "SeasonDirectory",
			node: seasonDir,
			want: provider.MediaTypeSeason,
		},
		{
			name: "ShowDirectory",
			node: showDir,
			want: provider.MediaTypeShow,
		},
		{
			name: "DefaultDirectory",
			node: defaultDir,
			want: provider.MediaTypeMovie,
		},
		{
			name:    "UnsupportedFile",
			node:    unsupportedFile,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewParseContext(tt.node.Name(), tt.node)
			got, err := detector.Detect(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Detect(%q) error = nil, want error", tt.node.Name())
				}
				return
			}

			if err != nil {
				t.Fatalf("Detect(%q) unexpected error: %v", tt.node.Name(), err)
			}

			if got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.node.Name(), got, tt.want)
			}
		})
	}
}
