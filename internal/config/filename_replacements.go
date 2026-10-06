package config

import (
	"fmt"
	"sort"
	"strings"
)

// ValidateFilenameReplacements checks literal replacement rules before they are
// used to build filesystem paths. Empty values are allowed and delete matches.
func ValidateFilenameReplacements(replacements map[string]string) error {
	for _, key := range sortedReplacementKeys(replacements) {
		if key == "" {
			return fmt.Errorf("filename_replacements: search text must not be empty")
		}
		for _, r := range replacements[key] {
			if r < 32 || r == 127 || strings.ContainsRune(`<>:"/\|?*`, r) {
				return fmt.Errorf("filename_replacements[%q]: replacement contains invalid filename character %q", key, r)
			}
		}
	}
	return nil
}

// ApplyFilenameReplacements applies case-sensitive literal rules to a generated
// base name. Longer matches take priority; replacement text is never reprocessed.
// Call after preserving source tags and before appending the file extension.
func (cfg *FormatConfig) ApplyFilenameReplacements(name string) string {
	if len(cfg.FilenameReplacements) == 0 {
		return name
	}
	keys := sortedReplacementKeys(cfg.FilenameReplacements)
	pairs := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		// Load and Save reject empty keys. Ignore them for configs built in code.
		if key != "" {
			pairs = append(pairs, key, cfg.FilenameReplacements[key])
		}
	}
	result := strings.NewReplacer(pairs...).Replace(name)
	// A spaced replacement, such as ":" -> " - ", must not double spaces
	// already surrounding the source punctuation.
	return strings.Join(strings.Fields(result), " ")
}

func sortedReplacementKeys(replacements map[string]string) []string {
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}
