package local

import (
	"regexp"
	"strconv"
	"strings"
)

// Pattern compilation for media file parsing
var (
	// Season patterns
	seasonRe    = regexp.MustCompile(`(?i)\b(?:s|season)\.? *(\d+)\b`)
	seasonAltRe = regexp.MustCompile(`(?i)(?:^|[\s\.\-_])(?:s|season)[\s\.\-_]+(\d+)`)

	// Episode patterns
	seasonEpisodeRe       = regexp.MustCompile(`(?i)[sx]?(\d+)[ex](\d+)`)
	dottedSeasonEpisodeRe = regexp.MustCompile(`(?i)(?:^|[\s_\-\.])([0-9]{1,2})[\. _-]([0-9]{1,2})(?:[^0-9]|$)`)
	episodeNumberRe       = regexp.MustCompile(`(?:^|[\s\.\-_]|[Ee])(\d+)(?:[\s\.\-_]|$)`)

	// File type patterns
	videoRe    = regexp.MustCompile(`(?i)\.(mp4|mkv|avi|mov|wmv|flv|webm|mpeg|mpg|m4v|3gp|vob|ts|mts|m2ts|rmvb|divx)$`)
	subtitleRe = regexp.MustCompile(`(?i)\.(srt|sub|idx|ass|ssa|smi|vtt|sbv|sami|usf|stl|dks|pjs|jss|psb|rt|scc|cap|sup|dfxp|ttml)$`)
	nfoRe      = regexp.MustCompile(`(?i)\.nfo$`)
	imageRe    = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|bmp|webp|tiff?|ico|svg)$`)

	// Language pattern for subtitles
	langPattern = regexp.MustCompile(`(\.[a-zA-Z]{2,3}(?:[-_][a-zA-Z]{2,4})?)$`)

	// Year extraction
	yearRangeRe = regexp.MustCompile(`(?:^|[^\d])((19|20)\d{2})(?:[\s\-–—]+(?:19|20)\d{2})?(?:[^\d]|$)`)

	// Encoding tags to remove
	encodingTagsRe = regexp.MustCompile(`(?i)\b(?:HD|HDR|DV|x265|x264|H\.?264|H\.?265|HEVC|AVC|AAC|AC3|DD|DTS|FLAC|MP3|WEB-?DL|BluRay|BDRip|DVDRip|HDTV|720p|1080p|2160p|4K|UHD|SDR|10bit|8bit|PROPER|REPACK|iNTERNAL|LiMiTED|UNRATED|EXTENDED|DiRECTORS?\.?CUT|THEATRICAL|COMPLETE|SEASON|SERIES|MULTI|DUAL|DUBBED|SUBBED|SUB|RETAIL|WS|FS|NTSC|PAL|R[1-6]|UNCUT|UNCENSORED)\b`)

	// Empty brackets
	emptyBracketsRe = regexp.MustCompile(`\s*[\(\[\{<]\s*[\)\]\}>]`)

	// Simple number pattern (for fallback season detection)
	simpleNumberRe = regexp.MustCompile(`^(\d+)|[\s\.\-_](\d+)(?:[\s\.\-_]|$)`)

	// Patterns to find where season/episode info starts
	seasonEpisodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[sx]?\d+[ex]\d+`),                // S01E01, 1x01, s1e1
		regexp.MustCompile(`(?i)[\s._-](?:s|season)[\s._-]*\d+`), // _Season_02, .Season.02
		regexp.MustCompile(`(?i)^(?:s|season)[\s._-]*\d+`),       // Season at start
		regexp.MustCompile(`\b\d{1,2}[\. _-]\d{1,2}\b`),          // Dotted format: 1.04
		regexp.MustCompile(`(?i)^[eE]\d+`),                       // Episode at start: E01
		regexp.MustCompile(`(?i)^Episode[\s._-]*\d+`),            // Episode at start
	}
)

// IsVideo checks if the filename has a video extension
func IsVideo(filename string) bool {
	return videoRe.MatchString(filename)
}

// IsSubtitle checks if the filename has a subtitle extension
func IsSubtitle(filename string) bool {
	return subtitleRe.MatchString(filename)
}

// IsNFO checks if the filename has an NFO extension
func IsNFO(filename string) bool {
	return nfoRe.MatchString(filename)
}

// IsImage checks if the filename has an image extension
func IsImage(filename string) bool {
	return imageRe.MatchString(filename)
}

// IsSample checks if the filename contains "sample"
func IsSample(name string) bool {
	return strings.Contains(strings.ToLower(name), "sample")
}

// ExtractExtension extracts the file extension (handles both regular and subtitle files)
func ExtractExtension(filename string) string {
	if IsSubtitle(filename) {
		return extractSubtitleSuffix(filename)
	}
	return extractSimpleExtension(filename)
}

// extractSimpleExtension extracts regular file extensions
func extractSimpleExtension(filename string) string {
	if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
		return filename[dotIndex:]
	}
	return ""
}

// extractSubtitleSuffix extracts subtitle suffix including language codes
func extractSubtitleSuffix(filename string) string {
	if !IsSubtitle(filename) {
		return ""
	}

	// Find the subtitle extension
	subtitleMatch := subtitleRe.FindStringIndex(filename)
	if len(subtitleMatch) == 0 {
		return ""
	}

	// Look for language codes before the subtitle extension
	beforeExt := filename[:subtitleMatch[0]]
	langMatch := langPattern.FindString(beforeExt)

	// Return language code + subtitle extension
	return langMatch + filename[subtitleMatch[0]:]
}

// ExtractSeasonNumber extracts a season number from a string
func ExtractSeasonNumber(input string) (int, bool) {
	// Try explicit season patterns first
	if num, found := firstIntFromRegexps(input, seasonRe, seasonAltRe); found {
		// Season 0 is valid (used for specials)
		if num >= 0 {
			return num, found
		}
	}

	// For simple number patterns, be more restrictive
	if matches := simpleNumberRe.FindStringSubmatch(input); len(matches) > 0 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				if num, err := strconv.Atoi(matches[i]); err == nil {
					// Reject 4-digit numbers (likely years)
					if num >= 1900 && num <= 2100 {
						continue
					}
					if num >= 0 && num <= 100 {
						// If this is the whole string and it's just a number, it's probably a season folder
						trimmed := strings.TrimSpace(input)
						if trimmed == matches[i] {
							return num, true
						}
					}
				}
			}
		}
	}

	return 0, false
}

// FindSeasonEpisodeIndex finds where season/episode information starts in a filename
func FindSeasonEpisodeIndex(filename string) int {
	earliestIndex := -1

	for _, pattern := range seasonEpisodePatterns {
		if matches := pattern.FindStringIndex(filename); matches != nil {
			if earliestIndex == -1 || matches[0] < earliestIndex {
				earliestIndex = matches[0]
			}
		}
	}

	return earliestIndex
}

// Helper function to extract first integer from multiple regexps
func firstIntFromRegexps(input string, regexps ...*regexp.Regexp) (int, bool) {
	for _, re := range regexps {
		m := re.FindStringSubmatch(input)
		if len(m) >= 2 {
			for i := 1; i < len(m); i++ {
				if m[i] == "" {
					continue
				}
				if n, err := strconv.Atoi(m[i]); err == nil {
					return n, true
				}
			}
		}
	}
	return 0, false
}
