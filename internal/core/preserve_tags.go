package core

import (
	"regexp"
	"strings"
)

var bracketTagPattern = regexp.MustCompile(`\[[^\[\]]+\]`)

func PreserveExistingBracketTags(generatedBaseName string, sourceBaseName string, enabled bool) string {
	if !enabled || sourceBaseName == "" {
		return generatedBaseName
	}

	sourceTags := bracketTagPattern.FindAllString(sourceBaseName, -1)
	if len(sourceTags) == 0 {
		return generatedBaseName
	}

	existingTags := bracketTagPattern.FindAllString(generatedBaseName, -1)
	seen := make(map[string]struct{}, len(existingTags))
	for _, tag := range existingTags {
		norm := normalizeBracketTag(tag)
		if norm == "" {
			continue
		}
		seen[norm] = struct{}{}
	}

	result := generatedBaseName
	for _, tag := range sourceTags {
		norm := normalizeBracketTag(tag)
		if norm == "" {
			continue
		}
		if _, exists := seen[norm]; exists {
			continue
		}
		result += tag
		seen[norm] = struct{}{}
	}

	return result
}

func normalizeBracketTag(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	return strings.ToLower(strings.TrimSpace(trimmed))
}
