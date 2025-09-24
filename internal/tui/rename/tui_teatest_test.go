package rename

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/google/go-cmp/cmp"
)

func newRenameTestNode(id, name string, isDir bool, path string) *treeview.Node[treeview.FileInfo] {
	info := treeview.FileInfo{
		FileInfo: core.NewSimpleFileInfo(name, isDir),
		Path:     path,
		Extra:    map[string]any{},
	}
	node := treeview.NewNode(id, name, info)
	if isDir {
		node.SetExpanded(true)
	}
	return node
}

func focusNode(t *testing.T, tree *treeview.Tree[treeview.FileInfo], id string) {
	t.Helper()
	if _, err := tree.SetFocusedID(context.Background(), id); err != nil {
		t.Fatalf("SetFocusedID(%q) error = %v", id, err)
	}
}

func newBasicRenameTree(t *testing.T) *treeview.Tree[treeview.FileInfo] {
	t.Helper()

	root := newRenameTestNode("root", "Root", true, ".")
	child := newRenameTestNode("file", "file.txt", false, "file.txt")
	root.AddChild(child)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{root})
	focusNode(t, tree, child.ID())
	return tree
}

func newStatsTree(t *testing.T) *treeview.Tree[treeview.FileInfo] {
	t.Helper()

	show := newRenameTestNode("show", "Show", true, "Show")
	showMeta := core.EnsureMeta(show)
	showMeta.Type = core.MediaShow

	season := newRenameTestNode("season", "Season 01", true, filepath.Join("Show", "Season 01"))
	seasonMeta := core.EnsureMeta(season)
	seasonMeta.Type = core.MediaSeason

	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("ep-%02d", i)
		name := fmt.Sprintf("Show - S01E%02d.mkv", i)
		node := newRenameTestNode(id, name, false, filepath.Join("Show", "Season 01", name))
		meta := core.EnsureMeta(node)
		meta.Type = core.MediaEpisode
		if i%2 == 1 {
			meta.NewName = fmt.Sprintf("Renamed-%02d.mkv", i)
		} else {
			meta.NewName = name
		}
		if i == 6 {
			meta.MarkedForDeletion = true
		}
		if i%3 == 0 {
			// count a subtitle entry to exercise stats
			subtitle := newRenameTestNode(fmt.Sprintf("sub-%02d", i), fmt.Sprintf("%s.srt", name), false, filepath.Join("Show", "Season 01", fmt.Sprintf("%s.srt", name)))
			subMeta := core.EnsureMeta(subtitle)
			subMeta.Type = core.MediaEpisode
			season.AddChild(subtitle)
		}
		season.AddChild(node)
	}

	show.AddChild(season)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{show})
	focusNode(t, tree, season.Children()[0].ID())
	return tree
}

func newPagedTree(t *testing.T, count int) (*treeview.Tree[treeview.FileInfo], []*treeview.Node[treeview.FileInfo]) {
	t.Helper()

	nodes := make([]*treeview.Node[treeview.FileInfo], 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("node-%02d", i)
		node := newRenameTestNode(id, fmt.Sprintf("Node %02d", i), false, fmt.Sprintf("node-%02d", i))
		nodes = append(nodes, node)
	}

	tree := treeview.NewTree(nodes)
	focusNode(t, tree, nodes[0].ID())
	return tree, nodes
}

func newRenameFlowTree(t *testing.T) (*treeview.Tree[treeview.FileInfo], string, string) {
	t.Helper()

	oldName := "original.txt"
	newName := "renamed.txt"
	if err := os.WriteFile(oldName, []byte("content"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	node := newRenameTestNode("rename-target", oldName, false, oldName)
	meta := core.EnsureMeta(node)
	meta.Type = core.MediaEpisode
	meta.NewName = newName

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node})
	focusNode(t, tree, node.ID())
	return tree, oldName, newName
}

func newDeleteTree(t *testing.T) (*treeview.Tree[treeview.FileInfo], []*treeview.Node[treeview.FileInfo]) {
	t.Helper()

	first := newRenameTestNode("first", "first.txt", false, "first.txt")
	second := newRenameTestNode("second", "second.txt", false, "second.txt")

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{first, second})
	focusNode(t, tree, first.ID())
	return tree, []*treeview.Node[treeview.FileInfo]{first, second}
}

func startRenameTestModel(t *testing.T, model *RenameModel, opts ...teatest.TestOption) *teatest.TestModel {
	t.Helper()
	options := append([]teatest.TestOption{teatest.WithInitialTermSize(100, 28)}, opts...)
	tm := teatest.NewTestModel(t, model, options...)
	t.Cleanup(func() {
		_ = tm.Quit()
	})
	return tm
}

func finalRenameModel(t *testing.T, tm *teatest.TestModel) *RenameModel {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	model, ok := final.(*RenameModel)
	if !ok {
		t.Fatalf("Final model type = %T, want *RenameModel", final)
	}
	return model
}

func waitForRenameOutput(t *testing.T, tm *teatest.TestModel, contains string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(contains))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}

func sendRune(tm *teatest.TestModel, r rune) {
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func sendKey(tm *teatest.TestModel, key tea.KeyType) {
	tm.Send(tea.KeyMsg{Type: key})
}

func TestRenameTUIQuitKeys(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "Esc", msg: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "CtrlC", msg: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tree := newBasicRenameTree(t)
			model := NewRenameModel(tree)
			tm := startRenameTestModel(t, model, teatest.WithInitialTermSize(100, 12))
			tm.Send(tea.WindowSizeMsg{Width: 100, Height: 12})

			tm.Send(tc.msg)
			tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
			final := finalRenameModel(t, tm)
			if final.renameInProgress {
				t.Error("renameInProgress = true, want false after quit")
			}
		})
	}
}

func TestRenameTUIStatsFocusAndScroll(t *testing.T) {
	tree := newStatsTree(t)
	model := NewRenameModel(tree)
	tm := startRenameTestModel(t, model)

	waitForRenameOutput(t, tm, "TV Shows:")

	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 12})

	sendKey(tm, tea.KeyTab)
	waitForRenameOutput(t, tm, "Tab: Tree Focus")

	sendKey(tm, tea.KeyDown)

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := finalRenameModel(t, tm)
	if !final.statsFocused {
		t.Error("statsFocused = false, want true after Tab")
	}
	if final.statsViewport.YOffset == 0 {
		t.Fatalf("statsViewport.YOffset = 0, height=%d, totalLines=%d", final.statsViewport.Height, final.statsViewport.TotalLineCount())
	}
}

func TestRenameTUITreePageNavigation(t *testing.T) {
	tree, nodes := newPagedTree(t, 25)
	model := NewRenameModel(tree)

	t.Run("PageDownMovesToEnd", func(t *testing.T) {
		tm := startRenameTestModel(t, model)
		sendKey(tm, tea.KeyPgDown)
		sendKey(tm, tea.KeyCtrlC)
		tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
		final := finalRenameModel(t, tm)
		focused := final.TuiTreeModel.Tree.GetFocusedNode()
		if focused == nil || focused.ID() != nodes[len(nodes)-1].ID() {
			t.Fatalf("focused ID = %v, want %v", nodeID(focused), nodes[len(nodes)-1].ID())
		}
	})

	tree, nodes = newPagedTree(t, 25)
	model = NewRenameModel(tree)

	t.Run("PageUpReturnsToStart", func(t *testing.T) {
		tm := startRenameTestModel(t, model)
		sendKey(tm, tea.KeyPgDown)
		sendKey(tm, tea.KeyPgUp)
		sendKey(tm, tea.KeyCtrlC)
		tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
		final := finalRenameModel(t, tm)
		focused := final.TuiTreeModel.Tree.GetFocusedNode()
		if focused == nil || focused.ID() != nodes[0].ID() {
			t.Fatalf("focused ID = %v, want %v", nodeID(focused), nodes[0].ID())
		}
	})
}

func nodeID(node *treeview.Node[treeview.FileInfo]) string {
	if node == nil {
		return ""
	}
	return node.ID()
}

func TestRenameTUIDeleteKeysRemoveNodes(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "DeleteKey", msg: tea.KeyMsg{Type: tea.KeyDelete}},
		{name: "RuneD", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tree, nodes := newDeleteTree(t)
			model := NewRenameModel(tree)
			tm := startRenameTestModel(t, model)

			sendKey(tm, tea.KeyDown)
			tm.Send(tc.msg)
			sendKey(tm, tea.KeyCtrlC)
			tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

			final := finalRenameModel(t, tm)
			remaining := final.TuiTreeModel.Tree.Nodes()
			gotIDs := []string{}
			for _, n := range remaining {
				gotIDs = append(gotIDs, n.ID())
			}
			want := []string{nodes[0].ID()}
			if diff := cmp.Diff(want, gotIDs); diff != "" {
				t.Errorf("remaining node IDs diff (-want +got):\n%s", diff)
			}
			focused := final.TuiTreeModel.Tree.GetFocusedNode()
			if focused == nil || focused.ID() != nodes[0].ID() {
				t.Errorf("focused ID = %v, want %v", nodeID(focused), nodes[0].ID())
			}
		})
	}
}

func TestRenameTUIRenameFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".title-tidy"), 0o755); err != nil {
		t.Fatalf("mkdir home config: %v", err)
	}

	log.Initialize(true, 7)

	base := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	tree, oldName, newName := newRenameFlowTree(t)
	model := NewRenameModel(tree)
	tm := startRenameTestModel(t, model)

	waitForRenameOutput(t, tm, "r: Rename")
	sendRune(tm, 'r')

	waitForRenameOutput(t, tm, "u: Undo")

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	final := finalRenameModel(t, tm)
	if !final.renameComplete {
		t.Error("renameComplete = false, want true after rename")
	}
	if final.successCount != 1 {
		t.Errorf("successCount = %d, want 1", final.successCount)
	}
	if final.errorCount != 0 {
		t.Errorf("errorCount = %d, want 0", final.errorCount)
	}
	if !final.undoAvailable {
		t.Error("undoAvailable = false, want true after successful rename")
	}
	if _, err := os.Stat(newName); err != nil {
		t.Fatalf("stat %s after rename = %v, want nil", newName, err)
	}
	if _, err := os.Stat(oldName); !os.IsNotExist(err) {
		t.Fatalf("stat %s after rename = %v, want not exists", oldName, err)
	}
}

func TestRenameTUIUndoFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".title-tidy"), 0o755); err != nil {
		t.Fatalf("mkdir home config: %v", err)
	}

	log.Initialize(true, 7)

	base := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	tree, oldName, newName := newRenameFlowTree(t)
	model := NewRenameModel(tree)
	tm := startRenameTestModel(t, model)

	waitForRenameOutput(t, tm, "r: Rename")
	sendRune(tm, 'r')
	waitForRenameOutput(t, tm, "u: Undo")

	sendRune(tm, 'u')
	waitForRenameOutput(t, tm, "Undo:")

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	final := finalRenameModel(t, tm)
	if !final.undoComplete {
		t.Error("undoComplete = false, want true")
	}
	if final.undoSuccess != 1 {
		t.Errorf("undoSuccess = %d, want 1", final.undoSuccess)
	}
	if final.undoFailed != 0 {
		t.Errorf("undoFailed = %d, want 0", final.undoFailed)
	}
	if final.undoAvailable {
		t.Error("undoAvailable = true, want false after undo completes")
	}
	if _, err := os.Stat(oldName); err != nil {
		t.Fatalf("stat %s after undo = %v, want nil", oldName, err)
	}
	if _, err := os.Stat(newName); !os.IsNotExist(err) {
		t.Fatalf("stat %s after undo = %v, want not exists", newName, err)
	}
}

func TestRenameTUIMetadataStatus(t *testing.T) {
	tree := newBasicRenameTree(t)
	model := NewRenameModel(tree)
	tm := startRenameTestModel(t, model)

	waitForRenameOutput(t, tm, "TV Shows:")

	progress := MetadataProgressMsg{Total: 5, Completed: 2}
	tm.Send(progress)
	waitForRenameOutput(t, tm, "Fetching metadata")

	complete := MetadataCompleteMsg{Errors: 1}
	tm.Send(complete)

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := finalRenameModel(t, tm)
	if final.metadataFetching {
		t.Error("metadataFetching = true, want false after completion")
	}
	if final.metadataStatus != "Metadata fetching complete (1 errors)" {
		t.Errorf("metadataStatus = %q, want %q", final.metadataStatus, "Metadata fetching complete (1 errors)")
	}
	if final.metadataCompleted != progress.Completed {
		t.Errorf("metadataCompleted = %d, want %d", final.metadataCompleted, progress.Completed)
	}
}

func TestRenameTUIMouseScroll(t *testing.T) {
	t.Run("TreeScroll", func(t *testing.T) {
		tree, nodes := newPagedTree(t, 5)
		model := NewRenameModel(tree)
		tm := startRenameTestModel(t, model)

		tm.Send(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButton(5)})
		sendKey(tm, tea.KeyCtrlC)
		tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

		final := finalRenameModel(t, tm)
		focused := final.TuiTreeModel.Tree.GetFocusedNode()
		if focused == nil || focused.ID() != nodes[1].ID() {
			t.Fatalf("focused ID = %v, want %v", nodeID(focused), nodes[1].ID())
		}
	})

	t.Run("StatsScroll", func(t *testing.T) {
		tree := newStatsTree(t)
		model := NewRenameModel(tree)
		tm := startRenameTestModel(t, model, teatest.WithInitialTermSize(100, 12))

		sendKey(tm, tea.KeyTab)
		tm.Send(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButton(5)})
		sendKey(tm, tea.KeyCtrlC)
		tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

		final := finalRenameModel(t, tm)
		if final.statsViewport.YOffset == 0 {
			t.Fatal("statsViewport.YOffset = 0, want >0 after mouse scroll")
		}
	})
}
