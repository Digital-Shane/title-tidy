package local

import (
	"strconv"
	"strings"
)

const (
	maxSeasonNumber  = 100
	maxEpisodeNumber = 300
)

// seasonEpisodeFromString extracts season and episode numbers from a name-only string.
// It mirrors the heuristics the legacy EpisodeParser used so the behavior stays stable
// while allowing other callers to share the same logic.
func seasonEpisodeFromString(input string) (int, int, bool) {
	if m := seasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 && matchOutsideTagBlock(input, seasonEpisodeRe.FindStringIndex(input)) {
		seasonText, episodeText := m[1], m[2]
		if seasonText == "" && len(m) >= 5 {
			seasonText, episodeText = m[3], m[4]
		}

		season, err1 := strconv.Atoi(seasonText)
		episode, err2 := strconv.Atoi(episodeText)
		if err1 == nil && err2 == nil && validSeasonEpisode(season, episode) {
			return season, episode, true
		}
	}

	if m := dottedSeasonEpisodeRe.FindStringSubmatch(input); len(m) >= 3 && matchOutsideTagBlock(input, dottedSeasonEpisodeRe.FindStringIndex(input)) {
		season, err1 := strconv.Atoi(m[1])
		episode, err2 := strconv.Atoi(m[2])
		if err1 == nil && err2 == nil && validSeasonEpisode(season, episode) {
			return season, episode, true
		}
	}

	return 0, 0, false
}

func validSeasonEpisode(season, episode int) bool {
	return season >= 0 && season <= maxSeasonNumber && episode >= 0 && episode <= maxEpisodeNumber
}

// SeasonEpisodeFromContext extracts season and episode numbers using the parse context.
// It falls back to episode-only matches and resolves season numbers from parent folders.
func SeasonEpisodeFromContext(ctx ParseContext) (int, int, bool) {
	return seasonEpisodeFromContext(ctx, false)
}

func seasonEpisodeFromContext(ctx ParseContext, allowBareSeparatorEpisode bool) (int, int, bool) {
	candidates := contextNameCandidates(ctx)

	for _, candidate := range candidates {
		if season, episode, ok := seasonEpisodeFromString(candidate); ok {
			return season, episode, true
		}
	}

	for _, candidate := range candidates {
		episode, fromSeparator, explicitEpisode, ok := episodeNumberFromString(candidate)
		if !ok {
			continue
		}

		// Try to resolve season from parent folders.
		if season, found := seasonFromParents(ctx); found {
			return season, episode, true
		}

		if episodeOnlyAllowed(fromSeparator, explicitEpisode, allowBareSeparatorEpisode) {
			return 0, episode, true
		}
	}

	return 0, 0, false
}

func contextNameCandidates(ctx ParseContext) []string {
	workingName := strings.TrimSpace(ctx.WorkingName())
	rawName := strings.TrimSpace(ctx.Name)

	if workingName == "" {
		if rawName == "" {
			return nil
		}
		return []string{rawName}
	}
	if rawName == "" || rawName == workingName {
		return []string{workingName}
	}

	return []string{workingName, rawName}
}

func episodeNumberFromString(input string) (episode int, fromSeparator bool, explicitEpisode bool, ok bool) {
	if episode, _, ok := episodeFromMatches(input, separatorEpisodeRe.FindStringSubmatchIndex(input)); ok {
		return episode, true, false, true
	}

	if episode, numberStart, ok := episodeFromMatches(input, episodeNumberRe.FindStringSubmatchIndex(input)); ok {
		return episode, false, hasExplicitEpisodePrefix(input, numberStart), true
	}

	return 0, false, false, false
}

func episodeFromMatches(input string, matches []int) (episode int, numberStart int, ok bool) {
	if len(matches) < 4 || !matchOutsideTagBlock(input, matches[:2]) {
		return 0, 0, false
	}

	for i := 2; i < len(matches); i += 2 {
		if matches[i] < 0 || matches[i+1] < 0 {
			continue
		}

		episode, err := strconv.Atoi(input[matches[i]:matches[i+1]])
		if err == nil && episode >= 0 && episode <= maxEpisodeNumber {
			return episode, matches[i], true
		}
	}

	return 0, 0, false
}

func matchOutsideTagBlock(input string, match []int) bool {
	if len(match) < 2 {
		return false
	}

	for _, tag := range tagBlockRe.FindAllStringIndex(input, -1) {
		if rangesOverlap(match[0], match[1], tag[0], tag[1]) {
			return false
		}
	}

	return true
}

func rangesOverlap(startA, endA, startB, endB int) bool {
	return startA < endB && startB < endA
}

func episodeOnlyAllowed(fromSeparator bool, explicitEpisode bool, allowBareSeparatorEpisode bool) bool {
	if fromSeparator {
		return allowBareSeparatorEpisode
	}

	return explicitEpisode
}

func hasExplicitEpisodePrefix(input string, numberStart int) bool {
	prefix := strings.TrimRight(input[:numberStart], " \t\r\n._-–—")

	return hasTokenSuffix(prefix, "episode") || hasTokenSuffix(prefix, "e")
}

func hasTokenSuffix(input string, token string) bool {
	start := len(input) - len(token)
	if start < 0 || !strings.EqualFold(input[start:], token) {
		return false
	}
	if start == 0 {
		return true
	}

	return !isASCIIAlphaNumeric(input[start-1])
}

func isASCIIAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
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
