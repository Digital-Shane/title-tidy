package local

import (
	"regexp"
	"slices"
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
	for _, candidate := range contextNameCandidates(ctx) {
		if season, episode, ok := seasonEpisodeFromString(candidate); ok {
			return season, episode, true
		}
	}

	for _, candidate := range contextNameCandidates(ctx) {
		episode, fromSeparator, ok := episodeNumberFromString(candidate)
		if !ok {
			continue
		}

		// Try to resolve season from parent folders.
		if season, found := seasonFromParents(ctx); found {
			return season, episode, true
		}

		if episodeOnlyAllowed(candidate, fromSeparator, allowBareSeparatorEpisode) {
			return 0, episode, true
		}
	}

	return 0, 0, false
}

func contextNameCandidates(ctx ParseContext) []string {
	workingName := ctx.WorkingName()
	inputs := []string{workingName}
	if workingName != ctx.Name {
		inputs = append(inputs, ctx.Name)
	}

	candidates := make([]string, 0, len(inputs)*2)
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

	for _, input := range inputs {
		addCandidate(stripTagBlocks(input))
	}
	for _, input := range inputs {
		addCandidate(input)
	}

	return candidates
}

func episodeNumberFromString(input string) (episode int, fromSeparator bool, ok bool) {
	if episode, ok := firstEpisodeIntFromRegexp(input, separatorEpisodeRe); ok {
		return episode, true, true
	}
	if episode, ok := firstEpisodeIntFromRegexp(input, episodeNumberRe); ok {
		return episode, false, true
	}
	return 0, false, false
}

func firstEpisodeIntFromRegexp(input string, re *regexp.Regexp) (int, bool) {
	if !matchOutsideTagBlock(input, re.FindStringIndex(input)) {
		return 0, false
	}

	m := re.FindStringSubmatch(input)
	if len(m) < 2 {
		return 0, false
	}
	for i := 1; i < len(m); i++ {
		if m[i] == "" {
			continue
		}
		episode, err := strconv.Atoi(m[i])
		if err == nil && episode >= 0 && episode <= maxEpisodeNumber {
			return episode, true
		}
	}
	return 0, false
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

func episodeOnlyAllowed(input string, fromSeparator bool, allowBareSeparatorEpisode bool) bool {
	if fromSeparator {
		return allowBareSeparatorEpisode
	}

	if matchOutsideTagBlock(input, explicitEpisodeRe.FindStringIndex(input)) {
		return true
	}

	return false
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
