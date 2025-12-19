package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

const invalidFilenameChars = "<>:\"/\\|?*"

func sanitizeFilename(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is empty after sanitization")
	}

	var b strings.Builder
	b.Grow(len(name))

	lastSpace := false
	for _, r := range name {
		if r < 32 || r == 127 || strings.ContainsRune(invalidFilenameChars, r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if r == ' ' {
			if lastSpace {
				continue
			}
			lastSpace = true
			b.WriteRune(' ')
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}

	result := strings.TrimSpace(b.String())
	if result == "" {
		return "", fmt.Errorf("name is empty after sanitization")
	}
	return result, nil
}

func sanitizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty after sanitization")
	}

	volume := filepath.VolumeName(path)
	rest := path[len(volume):]
	sep := string(filepath.Separator)
	isAbs := strings.HasPrefix(rest, sep)

	parts := strings.Split(rest, sep)
	sanitizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		sanitized, err := sanitizeFilename(part)
		if err != nil {
			return "", err
		}
		sanitizedParts = append(sanitizedParts, sanitized)
	}
	if len(sanitizedParts) == 0 {
		return "", fmt.Errorf("path is empty after sanitization")
	}

	sanitizedPath := filepath.Join(sanitizedParts...)
	if isAbs {
		sanitizedPath = sep + sanitizedPath
	}

	return volume + sanitizedPath, nil
}
