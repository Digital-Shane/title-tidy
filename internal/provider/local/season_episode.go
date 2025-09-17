package local

import (
	"strconv"
	"strings"
)

// seasonEpisodeFromString extracts season and episode numbers from a name-only string.
// It mirrors the heuristics the legacy EpisodeParser used so the behavior stays stable
// while allowing other callers to share the same logic.
func seasonEpisodeFromString(input string) (int, int, bool) {
	if m := dottedSeasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 {
		season, err1 := strconv.Atoi(m[1])
		episode, err2 := strconv.Atoi(m[2])
		if err1 == nil && err2 == nil && season > 0 && season <= 100 && episode > 0 && episode <= 300 {
			return season, episode, true
		}
	}

	if m := seasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 {
		season, err1 := strconv.Atoi(m[1])
		episode, err2 := strconv.Atoi(m[2])
		if err1 == nil && err2 == nil {
			return season, episode, true
		}
	}

	return 0, 0, false
}

// SeasonEpisodeFromContext extracts season and episode numbers using the parse context.
// It falls back to episode-only matches and resolves season numbers from parent folders.
func SeasonEpisodeFromContext(ctx ParseContext) (int, int, bool) {
	workingName := ctx.WorkingName()

	if season, episode, ok := seasonEpisodeFromString(workingName); ok {
		return season, episode, true
	}

	// Fall back to checking the raw name (handles cases where extension removal changed the string).
	if workingName != ctx.Name {
		if season, episode, ok := seasonEpisodeFromString(ctx.Name); ok {
			return season, episode, true
		}
	}

	episode, ok := firstIntFromRegexps(workingName, episodeNumberRe)
	if !ok && workingName != ctx.Name {
		episode, ok = firstIntFromRegexps(ctx.Name, episodeNumberRe)
	}
	if !ok {
		return 0, 0, false
	}

	// Try to resolve season from parent folders.
	if season, found := seasonFromParents(ctx); found {
		return season, episode, true
	}

	lower := strings.ToLower(workingName)
	if episode > 0 && (strings.HasPrefix(lower, "e") || strings.Contains(lower, "episode")) {
		return 0, episode, true
	}

	return 0, 0, false
}

// seasonFromParents inspects ancestor folder names for a season indicator.
func seasonFromParents(ctx ParseContext) (int, bool) {
	parents := ctx.ParentNames(3)
	for _, name := range parents {
		if season, found := ExtractSeasonNumber(name); found {
			return season, true
		}
	}
	return 0, false
}
