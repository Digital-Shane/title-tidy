package media

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/treeview"
)

// Filename parsing & formatting utilities.
//
// This file consolidates regular expressions and helpers used to detect and
// normalize show / season / episode / movie naming patterns. Parsing is kept
// deliberately tolerant: we accept multiple community naming conventions and
// derive structured data (season, episode, year, language suffix) to drive
// consistent canonical output names.
var (
	// seasonRe matches canonical season tokens like "Season 01", "S01", "s1".
	seasonRe = regexp.MustCompile(`(?i)\b(?:s|season)\.? *(\d+)\b`)

	// seasonAltRe matches alternative season tokens with separators: _Season_01_, season-1.
	seasonAltRe = regexp.MustCompile(`(?i)(?:^|[\s\.\-_])(?:s|season)[\s\.\-_]+(\d+)`)

	// seasonEpisodeRe matches combined season/episode forms: S01E02, 1x02, s1e2.
	seasonEpisodeRe = regexp.MustCompile(`(?i)[sx]?(\d+)[ex](\d+)`)

	// dottedSeasonEpisodeRe matches compact dotted forms at start of filename: 1.04, 01.4, 10.12
	// We purposefully anchor at start (^|separator) to avoid misinterpreting years appearing later
	// and cap the season to two digits to avoid capturing a leading year like 2024.05.
	dottedSeasonEpisodeRe = regexp.MustCompile(`(?i)^(?:|[\s_\-\.])([0-9]{1,2})[\. _-]([0-9]{1,2})(?:[^0-9]|$)`)

	// videoRe matches video file extensions used to include media files.
	videoRe = regexp.MustCompile(`(?i)\.(mp4|mkv|avi|mov|wmv|flv|webm|mpeg|mpg|m4v|3gp|vob|ts|mts|m2ts|rmvb|divx)$`)

	// subtitleRe matches subtitle file extensions (case‑insensitive).
	subtitleRe = regexp.MustCompile(`(?i)\.(srt|sub|idx|ass|ssa|smi|vtt|sbv|sami|usf|stl|dks|pjs|jss|psb|rt|scc|cap|sup|dfxp|ttml)$`)

	// yearRangeRe extracts a year or year range; only the first year is used in output.
	yearRangeRe = regexp.MustCompile(`\b((19|20)\d{2})(?:[\s\-–—]+(?:19|20)\d{2})?\b`)

	// episodeNumberRe captures a loose episode number when SxxExx not present.
	episodeNumberRe = regexp.MustCompile(`(?:^|[\s\.\-_]|[Ee])(\d+)(?:[\s\.\-_]|$)`)

	// encodingTagsRe removes codec/resolution/source tags to isolate the series title.
	encodingTagsRe = regexp.MustCompile(`(?i)\b(?:HD|HDR|DV|x265|x264|H\.?264|H\.?265|HEVC|AVC|AAC|AC3|DD|DTS|FLAC|MP3|WEB-?DL|BluRay|BDRip|DVDRip|HDTV|720p|1080p|2160p|4K|UHD|SDR|10bit|8bit|PROPER|REPACK|iNTERNAL|LiMiTED|UNRATED|EXTENDED|DiRECTORS?\.?CUT|THEATRICAL|COMPLETE|SEASON|SERIES|MULTI|DUAL|DUBBED|SUBBED|SUB|RETAIL|WS|FS|NTSC|PAL|R[1-6]|UNCUT|UNCENSORED)\b`)

	// langPattern matches trailing language codes before subtitle extension: .en, .eng, .en-US.
	langPattern = regexp.MustCompile(`(\.[a-zA-Z]{2,3}(?:[-_][a-zA-Z]{2,4})?)$`)

	// simpleNumberRe matches a standalone number that might represent a season.
	simpleNumberRe = regexp.MustCompile(`^(\d+)|[\s\.\-_](\d+)(?:[\s\.\-_]|$)`)

	// nfoRe matches NFO info file extensions.
	nfoRe = regexp.MustCompile(`(?i)\.nfo$`)

	// imageRe matches common image file extensions.
	imageRe = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|bmp|webp|tiff?|ico|svg)$`)

	// seasonEpisodePatterns are used to find where season/episode info starts in a filename
	seasonEpisodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[sx]?\d+[ex]\d+`),          // S01E01, 1x01, s1e1
		regexp.MustCompile(`(?i)\b(?:s|season)\.? *\d+\b`), // Season 01, S01
	}
)

// IsVideo reports whether filename has a recognized video extension.
func IsVideo(filename string) bool {
	return videoRe.MatchString(filename)
}

// IsSubtitle reports whether filename has a recognized subtitle extension.
func IsSubtitle(filename string) bool {
	return subtitleRe.MatchString(filename)
}

// IsNFO reports whether filename has an NFO extension.
func IsNFO(filename string) bool {
	return nfoRe.MatchString(filename)
}

// IsImage reports whether filename has a recognized image extension.
func IsImage(filename string) bool {
	return imageRe.MatchString(filename)
}

// IsSample reports whether filename or folder name contains "sample".
func IsSample(name string) bool {
	return strings.Contains(strings.ToLower(name), "sample")
}

// extractSubtitleSuffix extracts the language code and extension from subtitle files.
// For example: "movie.en.srt" returns ".en.srt", "movie.srt" returns ".srt"
// Also handles cases like "movie.eng.srt", "movie.en-US.srt", etc.
func extractSubtitleSuffix(filename string) string {
	if !IsSubtitle(filename) {
		return ""
	}

	// Find the subtitle extension first
	subtitleMatch := subtitleRe.FindStringIndex(filename)
	if len(subtitleMatch) == 0 {
		return ""
	}

	// Look for language codes before the subtitle extension
	// Matches patterns like .en, .eng, .en-US, .en_US, etc.
	beforeExt := filename[:subtitleMatch[0]]
	langMatch := langPattern.FindString(beforeExt)

	// Return language code + subtitle extension
	return langMatch + filename[subtitleMatch[0]:]
}

// ExtractSeasonNumber attempts to extract a season number from a string.
// Returns the season number and true if found, or 0 and false if not found.
func ExtractSeasonNumber(input string) (int, bool) {
	// Table-driven: check patterns in order and return the first parsed integer.
	return firstIntFromRegexps(input, seasonRe, seasonAltRe, simpleNumberRe)
}

// ParseSeasonEpisode extracts season and episode numbers from a filename using node context.
// Returns season, episode, and true if both are found, or 0, 0, false if not.
func ParseSeasonEpisode(input string, node *treeview.Node[treeview.FileInfo]) (int, int, bool) {
	// Attempt dotted pattern first because it's otherwise ambiguous with fallback episode extraction.
	if m := dottedSeasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 {
		season, err1 := strconv.Atoi(m[1])
		episode, err2 := strconv.Atoi(m[2])
		// Guard against false positives like a leading year (e.g. 2024.05)
		if err1 == nil && err2 == nil && season > 0 && season <= 100 && episode > 0 && episode <= 300 {
			return season, episode, true
		}
	}
	// Try combined season/episode pattern first (S01E02, 1x02, etc.)
	if m := seasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 {
		season, err1 := strconv.Atoi(m[1])
		episode, err2 := strconv.Atoi(m[2])
		if err1 == nil && err2 == nil {
			return season, episode, true
		}
	}
	// Fallback: episode from filename, season from parent folder context
	return tryEpisodeFromContext(input, node)
}

// ExtractExtension extracts the file extension.
func ExtractExtension(filename string) string {
	ext := ""
	if IsSubtitle(filename) {
		ext = extractSubtitleSuffix(filename)
	} else {
		ext = extractExtension(filename)
	}
	return ext
}

// extractExtension extracts the file extension including the dot.
// Returns the extension (e.g., ".mp4") or empty string if no extension found.
func extractExtension(filename string) string {
	if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
		return filename[dotIndex:]
	}
	return ""
}

// Generic, table-driven helpers to reduce duplication
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

// Context-based fallback: episode from filename, season from parent folder name
func tryEpisodeFromContext(input string, node *treeview.Node[treeview.FileInfo]) (int, int, bool) {
	// Episode number from filename
	episode, ok := firstIntFromRegexps(input, episodeNumberRe)
	if !ok {
		return 0, 0, false
	}
	if node == nil {
		return 0, 0, false
	}
	// Season number from parent folder
	parent := node.Parent()
	if parent == nil {
		return 0, 0, false
	}
	season, found := ExtractSeasonNumber(parent.Name())
	if !found {
		return 0, 0, false
	}
	return season, episode, true
}

// FindSeasonEpisodeIndex finds the index where season/episode information starts in a filename.
// Returns -1 if no pattern is found.
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

// ExtractShowNameFromPath extracts show name and year from a file or folder path
// by finding where the season/episode pattern starts and cleaning everything before it.
// This is moved here from cmd package to avoid import cycles.
func ExtractShowNameFromPath(path string, removeExtension bool) (showName, year string) {
	workingPath := path

	// Remove extension if needed (for files)
	if removeExtension {
		ext := ExtractExtension(path)
		if ext != "" {
			workingPath = path[:len(path)-len(ext)]
		}
	}

	// Find where the season/episode pattern starts
	if idx := FindSeasonEpisodeIndex(workingPath); idx > 0 {
		// Extract everything before the pattern
		showPart := workingPath[:idx]
		// ExtractNameAndYear handles all the complex parsing
		showName, year = config.ExtractNameAndYear(showPart)
	} else {
		// No pattern found, just clean the whole name
		showName, year = config.ExtractNameAndYear(workingPath)
	}

	return showName, year
}
