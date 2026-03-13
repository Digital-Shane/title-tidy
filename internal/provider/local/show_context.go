package local

import "strings"

// ResolveShowInfo attempts to determine the show title and year from the parse context.
// It first inspects the current name, then walks up the parent hierarchy for clues.
func ResolveShowInfo(ctx ParseContext) (string, string) {
	if show, year, ok := showInfoFromName(ctx); ok {
		if year == "" {
			if parentShow, parentYear := showInfoFromParents(ctx, 3); parentYear != "" && sameShowTitle(show, parentShow) {
				return show, parentYear
			}
		}
		return show, year
	}

	return showInfoFromParents(ctx, 3)
}

func sameShowTitle(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	return strings.EqualFold(a, b)
}

func showInfoFromName(ctx ParseContext) (string, string, bool) {
	show, year := ExtractShowNameFromPath(ctx.Name, ctx.IsFile)
	if show == "" {
		return "", "", false
	}

	if ctx.IsFile {
		if idx := FindSeasonEpisodeIndex(ctx.WorkingName()); idx <= 0 {
			return "", "", false
		}
	}

	return show, year, true
}

func showInfoFromParents(ctx ParseContext, maxDepth int) (string, string) {
	if ctx.Node == nil {
		return "", ""
	}

	parent := ctx.Node.Parent()
	depth := 0
	for parent != nil && depth < maxDepth {
		parentName := parent.Name()

		if show, year := ExtractShowNameFromPath(parentName, false); show != "" {
			return show, year
		}

		if _, isSeason := ExtractSeasonNumber(parentName); !isSeason {
			if show, year := ExtractNameAndYear(parentName); show != "" {
				return show, year
			}
		}

		parent = parent.Parent()
		depth++
	}

	return "", ""
}
