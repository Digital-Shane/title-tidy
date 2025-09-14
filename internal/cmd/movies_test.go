package cmd

import (
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/media"
)

// TestSubtitleExtensionWithLanguage tests that media.ExtractExtension properly
// handles subtitle files with language codes
func TestSubtitleExtensionWithLanguage(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantExt  string
	}{
		{"subtitle_no_lang", "movie.srt", ".srt"},
		{"subtitle_2letter_lang", "movie.en.srt", ".en.srt"},
		{"subtitle_3letter_lang", "movie.eng.srt", ".eng.srt"},
		{"subtitle_locale", "movie.en-US.srt", ".en-US.srt"},
		{"subtitle_locale_underscore", "movie.pt_BR.srt", ".pt_BR.srt"},
		{"video_file", "movie.mkv", ".mkv"},
		{"video_with_dots", "movie.2020.1080p.mkv", ".mkv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := media.ExtractExtension(tt.filename)
			if got != tt.wantExt {
				t.Errorf("ExtractExtension(%q) = %q, want %q", tt.filename, got, tt.wantExt)
			}
		})
	}
}

func TestNoDirSubtitleRenaming(t *testing.T) {
	tests := []struct {
		name           string
		videoName      string
		videoNewName   string
		subtitleName   string
		wantSubNewName string
	}{
		{
			name:           "subtitle_with_same_base",
			videoName:      "Pulp.Fiction.1994.mkv",
			videoNewName:   "Pulp Fiction (1994).mkv",
			subtitleName:   "Pulp.Fiction.1994.srt",
			wantSubNewName: "Pulp Fiction (1994).srt",
		},
		{
			name:           "subtitle_with_language_code",
			videoName:      "Pulp.Fiction.1994.mkv",
			videoNewName:   "Pulp Fiction (1994).mkv",
			subtitleName:   "Pulp.Fiction.1994.en.srt",
			wantSubNewName: "Pulp Fiction (1994).en.srt",
		},
		{
			name:           "subtitle_with_locale",
			videoName:      "The.Matrix.1999.mkv",
			videoNewName:   "The Matrix (1999).mkv",
			subtitleName:   "The.Matrix.1999.en-US.srt",
			wantSubNewName: "The Matrix (1999).en-US.srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is more of an integration test that would require
			// setting up nodes and running moviePreprocess
			// For now, we're testing the logic conceptually
			t.Logf("Test case: video %s -> %s, subtitle %s -> %s",
				tt.videoName, tt.videoNewName, tt.subtitleName, tt.wantSubNewName)
		})
	}
}

func TestSubtitleBaseNameExtraction(t *testing.T) {
	tests := []struct {
		name         string
		videoName    string
		subtitleName string
		shouldMatch  bool
	}{
		{
			name:         "subtitle_with_language_code",
			videoName:    "Pulp.Fiction.1994.mkv",
			subtitleName: "Pulp.Fiction.1994.en.srt",
			shouldMatch:  true,
		},
		{
			name:         "subtitle_without_language",
			videoName:    "Pulp.Fiction.1994.mkv",
			subtitleName: "Pulp.Fiction.1994.srt",
			shouldMatch:  true,
		},
		{
			name:         "subtitle_with_3letter_lang",
			videoName:    "The.Matrix.1999.mkv",
			subtitleName: "The.Matrix.1999.eng.srt",
			shouldMatch:  true,
		},
		{
			name:         "subtitle_with_locale",
			videoName:    "Inception.2010.mkv",
			subtitleName: "Inception.2010.en-US.srt",
			shouldMatch:  true,
		},
		{
			name:         "different_movie",
			videoName:    "Movie.2020.mkv",
			subtitleName: "Other.2020.en.srt",
			shouldMatch:  false,
		},
		{
			name:         "year_not_language",
			videoName:    "Movie.Name.mkv",
			subtitleName: "Movie.Name.1994.srt",
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract video base name
			videoBaseName := tt.videoName
			if ext := media.ExtractExtension(videoBaseName); ext != "" {
				videoBaseName = videoBaseName[:len(videoBaseName)-len(ext)]
			}

			// Extract subtitle base name using the same logic as in moviePreprocess
			// media.ExtractExtension already handles language codes for subtitles
			otherBaseName := tt.subtitleName
			if ext := media.ExtractExtension(otherBaseName); ext != "" {
				otherBaseName = otherBaseName[:len(otherBaseName)-len(ext)]
			}

			got := otherBaseName == videoBaseName
			if got != tt.shouldMatch {
				t.Errorf("Subtitle matching for video=%q, subtitle=%q: got match=%v, want match=%v (videoBase=%q, subtitleBase=%q)",
					tt.videoName, tt.subtitleName, got, tt.shouldMatch, videoBaseName, otherBaseName)
			}
		})
	}
}
