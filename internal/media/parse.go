package media

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

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

	// dottedSeasonEpisodeRe matches compact dotted forms: 1.04, 01.4, 10.12
	// We look for these patterns with word boundaries to avoid false positives
	// and cap the season to two digits to avoid capturing a leading year like 2024.05.
	dottedSeasonEpisodeRe = regexp.MustCompile(`(?i)(?:^|[\s_\-\.])([0-9]{1,2})[\. _-]([0-9]{1,2})(?:[^0-9]|$)`)

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
		regexp.MustCompile(`\b\d{1,2}[\. _-]\d{1,2}\b`),    // Dotted format: 1.04, 01.4
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

// ExtractShowInfo is the single entry point for extracting show name and year
// from any media file or folder (episodes, seasons, or shows).
// It first attempts to extract from the current path/filename, then falls back
// to searching parent directories if needed.
func ExtractShowInfo(node *treeview.Node[treeview.FileInfo], isFile bool) (showName, year string) {
	if node == nil {
		return "", ""
	}

	name := node.Name()
	workingPath := name

	// For files, remove extension before processing
	if isFile {
		ext := ExtractExtension(name)
		if ext != "" {
			workingPath = name[:len(name)-len(ext)]
		}
	}

	// First, try to extract show name from the current path/filename
	// Check if there's a season/episode pattern
	idx := FindSeasonEpisodeIndex(workingPath)
	if idx > 0 {
		// Extract everything before the pattern as potential show name
		showPart := workingPath[:idx]

		// Trim any trailing separator characters (dots, underscores, dashes, spaces)
		// This handles cases like "Better.Call.Saul." where the trailing dot should be removed
		showPart = strings.TrimRight(showPart, ".-_ ")

		showName, year = config.ExtractNameAndYear(showPart)

		// If we found a valid show name, return it
		if showName != "" {
			return showName, year
		}
	}

	// If the pattern starts at the beginning (idx == 0), there's no show name in this file
	// Go straight to parent search
	if idx == 0 {
		// No show name in current file, search parents
		if parent := node.Parent(); parent != nil {
			return ExtractShowInfo(parent, false)
		}
		return "", ""
	}

	// Check if this is a season folder - it might have show name before "Season"
	if _, isSeasonFolder := ExtractSeasonNumber(workingPath); isSeasonFolder {
		// Check if there's text before "Season" or "S" pattern
		seasonPatterns := []string{"Season", "season", "SEASON", "S", "s"}
		earliestSeasonIdx := -1

		for _, pattern := range seasonPatterns {
			idx := strings.Index(workingPath, pattern)
			if idx > 0 && (earliestSeasonIdx == -1 || idx < earliestSeasonIdx) {
				// Make sure this is actually the season pattern, not part of a word
				if idx == 0 || !unicode.IsLetter(rune(workingPath[idx-1])) {
					// Also verify the pattern is followed by a number or space+number
					afterPattern := workingPath[idx+len(pattern):]
					if len(afterPattern) > 0 {
						firstChar := rune(afterPattern[0])
						if unicode.IsDigit(firstChar) || unicode.IsSpace(firstChar) {
							earliestSeasonIdx = idx
						}
					}
				}
			}
		}

		if earliestSeasonIdx > 0 {
			// Extract everything before the season pattern
			showPart := workingPath[:earliestSeasonIdx]
			// Trim any trailing separator characters and spaces
			showPart = strings.TrimRight(showPart, ".-_ ")
			showName, year = config.ExtractNameAndYear(showPart)
			if showName != "" {
				return showName, year
			}
		}

		// No show name in this season folder, look to parent
		if parent := node.Parent(); parent != nil {
			return ExtractShowInfo(parent, false)
		}
		return "", ""
	}

	// If no show name found in current path, try to extract from the whole name
	// (useful for show folders that don't have season/episode patterns)
	showName, year = config.ExtractNameAndYear(workingPath)
	if showName != "" {
		return showName, year
	}

	// Fallback: search up the directory hierarchy
	parent := node.Parent()
	searchDepth := 0
	maxSearchDepth := 3 // Prevent infinite loops, search up to 3 levels

	for parent != nil && searchDepth < maxSearchDepth {
		parentName := parent.Name()

		// Try to extract show name from parent
		showName, year = config.ExtractNameAndYear(parentName)
		if showName != "" {
			return showName, year
		}

		parent = parent.Parent()
		searchDepth++
	}

	return "", ""
}

// ProcessEpisodeNode processes an episode node and extracts all relevant information.
// This centralizes episode processing logic for use across all commands.
func ProcessEpisodeNode(node *treeview.Node[treeview.FileInfo]) (showName, year string, season, episode int, found bool) {
	if node == nil || node.Data().IsDir() {
		return "", "", 0, 0, false
	}

	// Parse season and episode numbers
	season, episode, found = ParseSeasonEpisode(node.Name(), node)
	if !found {
		return "", "", 0, 0, false
	}

	// Extract show information using the centralized function
	showName, year = ExtractShowInfo(node, true)

	return showName, year, season, episode, true
}
