package local

import (
	"slices"
	"strings"
	"unicode"
)

// ExtractNameAndYear cleans a filename and extracts the name and year components.
// This is the primary function for cleaning media names and extracting metadata.
func ExtractNameAndYear(name string) (string, string) {
	if name == "" {
		return name, ""
	}

	for _, match := range yearRe.FindAllStringSubmatchIndex(name, -1) {
		start, end := match[2], match[3]
		if end < len(name) && name[end] >= '0' && name[end] <= '9' {
			continue
		}

		// A year cannot be the release year if it leaves no title behind.
		// Use match offsets so titles such as "1984 (1984)" stay intact.
		title := cleanMediaName(strings.TrimRight(name[:start], " .([{<-_"))
		if title == "" {
			continue
		}

		// Release years end the title. Words after a year belong to the
		// title unless they are bracketed tags or follow an encoding tag.
		suffix := yearRangeEndRe.ReplaceAllString(name[end:], "")
		suffix = stripTagBlocks(suffix)
		suffix = strings.ReplaceAll(suffix, "_", " ")
		if tag := encodingTagsRe.FindStringIndex(suffix); tag != nil {
			suffix = suffix[:tag[0]]
		}
		if strings.Trim(suffix, " .-_–—()[]{}<>") != "" {
			continue
		}

		return title, name[start:end]
	}

	return cleanMediaName(name), ""
}

func cleanMediaName(name string) string {
	formatted := stripTagBlocksIfUseful(name)

	// Replace separators with spaces
	formatted = strings.ReplaceAll(formatted, ".", " ")
	formatted = strings.ReplaceAll(formatted, "-", " ")
	formatted = strings.ReplaceAll(formatted, "_", " ")

	// Remove common encoding tags
	formatted = encodingTagsRe.ReplaceAllString(formatted, "")

	// Clean up extra spaces
	formatted = strings.TrimSpace(strings.Join(strings.Fields(formatted), " "))

	return formatted
}

func tagAwareCandidates(input string) []string {
	candidates := make([]string, 0, 2)
	addCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if slices.Contains(candidates, candidate) {
			return
		}
		candidates = append(candidates, candidate)
	}

	addCandidate(stripTagBlocks(input))
	addCandidate(input)

	return candidates
}

func stripTagBlocksIfUseful(input string) string {
	stripped := stripTagBlocks(input)
	if stripped == "" {
		return input
	}
	return stripped
}

func stripTagBlocks(input string) string {
	stripped := tagBlockRe.ReplaceAllString(input, " ")
	return CleanName(stripped)
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

	for _, candidate := range tagAwareCandidates(workingPath) {
		if showName, year, found := extractShowNameFromCandidate(candidate); found {
			return showName, year
		}
	}

	// Fallback: extract from the whole original name.
	return ExtractNameAndYear(workingPath)
}

func extractShowNameFromCandidate(workingPath string) (showName, year string, found bool) {
	// Find where season/episode info starts
	idx := FindSeasonEpisodeIndex(workingPath)
	if idx > 0 {
		// Extract everything before the pattern as potential show name
		showPart := workingPath[:idx]

		// Trim trailing separators
		showPart = strings.TrimRight(showPart, ".-_ ")

		showName, year = ExtractNameAndYear(showPart)
		if showName != "" {
			return showName, year, true
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
				return showName, year, true
			}
		}
		// If it's a season folder but has no show name before "Season", return empty
		return "", "", true
	}

	return "", "", false
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
