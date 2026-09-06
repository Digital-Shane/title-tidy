package local

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParentNamesUsesSourcePathBeyondTree(t *testing.T) {
	showPath := filepath.Join(string(filepath.Separator), "library", "Example Show (2020)")
	seasonPath := filepath.Join(showPath, "Season 02")
	episode := newTestNode("E01.mkv", false)
	episode.Data().Path = filepath.Join(seasonPath, episode.Name())
	season := newTestNode("Season 02", true)
	season.Data().Path = seasonPath
	season.AddChild(episode)

	ctx := NewParseContext(episode.Name(), episode)
	want := []string{"Season 02", "Example Show (2020)", "library"}
	if diff := cmp.Diff(want, ctx.ParentNames(10)); diff != "" {
		t.Errorf("parent names mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want[:1], ctx.ParentNames(1)); diff != "" {
		t.Errorf("parent depth limit mismatch (-want +got):\n%s", diff)
	}

	// A tree can carry a corrected name that differs from its source path.
	show := newTestNode("Canonical Show (2020)", true)
	show.Data().Path = showPath
	show.AddChild(season)
	want[1] = show.Name()
	if diff := cmp.Diff(want, ctx.ParentNames(3)); diff != "" {
		t.Errorf("tree names should take precedence (-want +got):\n%s", diff)
	}
}

func TestParentNamesDoesNotInferWorkingDirectory(t *testing.T) {
	for _, path := range []string{"", "S02E01.mkv", filepath.Join("..", "S02E01.mkv")} {
		t.Run(path, func(t *testing.T) {
			episode := newTestNode("S02E01.mkv", false)
			episode.Data().Path = path
			ctx := NewParseContext(episode.Name(), episode)
			if names := ctx.ParentNames(3); len(names) != 0 {
				t.Errorf("parent names for %q = %v, want none", path, names)
			}
		})
	}
}
