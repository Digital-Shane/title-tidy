package local

import (
	"strings"
	"unicode"
)

// ExtractNameAndYear cleans a filename and extracts the name and year components.
// This is the primary function for cleaning media names and extracting metadata.
func ExtractNameAndYear(name string) (string, string) {
	if name == "" {
		return name, ""
	}

	formatted := name
	year := ""

	// First, look for a year or year range in the name
	yearMatches := yearRangeRe.FindStringSubmatch(formatted)

	if len(yearMatches) > 1 {
		// Extract just the first year from the match
		year = yearMatches[1]

		// Find the position of the actual year within the formatted string
		yearIndex := strings.Index(formatted, year)
		if yearIndex != -1 {
			// Keep only the part before the year
			formatted = formatted[:yearIndex]
			formatted = strings.TrimRight(formatted, " ([{-_")
		}
	}

	// Replace separators with spaces
	formatted = strings.ReplaceAll(formatted, ".", " ")
	formatted = strings.ReplaceAll(formatted, "-", " ")
	formatted = strings.ReplaceAll(formatted, "_", " ")

	// Remove common encoding tags
	formatted = encodingTagsRe.ReplaceAllString(formatted, "")

	// Clean up extra spaces
	formatted = strings.TrimSpace(strings.Join(strings.Fields(formatted), " "))

	return formatted, year
}

// CleanName performs basic cleaning on a media name
func CleanName(name string) string {
	if name == "" {
		return ""
	}

	result := name

	// Remove empty brackets
	result = emptyBracketsRe.ReplaceAllString(result, "")

	// Remove multiple spaces
	result = strings.Join(strings.Fields(result), " ")

	// Trim spaces
	result = strings.TrimSpace(result)

	// Drop leading/trailing separator characters that look odd when metadata is missing
	result = strings.Trim(result, "-_–—|: ")

	// Final whitespace trim after separator removal
	result = strings.TrimSpace(result)

	return result
}

// ExtractShowNameFromPath attempts to extract a show name from a file/folder path
// by looking for patterns that indicate where the show name ends
func ExtractShowNameFromPath(path string, isFile bool) (showName, year string) {
	workingPath := path

	// For files, remove extension first
	if isFile {
		ext := ExtractExtension(path)
		if ext != "" {
			workingPath = path[:len(path)-len(ext)]
		}
	}

	// Find where season/episode info starts
	idx := FindSeasonEpisodeIndex(workingPath)
	if idx > 0 {
		// Extract everything before the pattern as potential show name
		showPart := workingPath[:idx]

		// Trim trailing separators
		showPart = strings.TrimRight(showPart, ".-_ ")

		showName, year = ExtractNameAndYear(showPart)
		if showName != "" {
			return showName, year
		}
	}

	// Check if this is a season folder
	if _, isSeasonFolder := ExtractSeasonNumber(workingPath); isSeasonFolder {
		// Look for show name before "Season" or "S" pattern
		if idx := findSeasonPatternIndex(workingPath); idx > 0 {
			showPart := workingPath[:idx]
			showPart = strings.TrimRight(showPart, ".-_ ")
			showName, year = ExtractNameAndYear(showPart)
			if showName != "" {
				return showName, year
			}
		}
		// If it's a season folder but has no show name before "Season", return empty
		return "", ""
	}

	// Fallback: extract from the whole name
	return ExtractNameAndYear(workingPath)
}

// findSeasonPatternIndex finds where a season pattern starts in the string
func findSeasonPatternIndex(s string) int {
	seasonPatterns := []string{"Season", "season", "SEASON", "S", "s"}
	earliestIdx := -1

	for _, pattern := range seasonPatterns {
		idx := strings.Index(s, pattern)
		if idx > 0 && (earliestIdx == -1 || idx < earliestIdx) {
			// Verify this is actually a season pattern
			if idx == 0 || !unicode.IsLetter(rune(s[idx-1])) {
				afterPattern := s[idx+len(pattern):]
				if len(afterPattern) > 0 {
					firstChar := rune(afterPattern[0])
					if unicode.IsDigit(firstChar) || unicode.IsSpace(firstChar) {
						earliestIdx = idx
					}
				}
			}
		}
	}

	return earliestIdx
}
