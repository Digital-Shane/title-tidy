package undo

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/google/go-cmp/cmp"
)

func sendKey(tm *teatest.TestModel, key tea.KeyType) {
	tm.Send(tea.KeyMsg{Type: key})
}

func sendRune(tm *teatest.TestModel, r rune) {
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func newUndoOperation(id, basePath string, idx int, success bool) log.OperationLog {
	return log.OperationLog{
		ID:         fmt.Sprintf("%s-op-%02d", id, idx),
		Timestamp:  time.Date(2024, time.January, 1, 12, idx, 0, 0, time.UTC),
		Type:       log.OpRename,
		SourcePath: fmt.Sprintf("%s/original-%02d.mkv", basePath, idx),
		DestPath:   fmt.Sprintf("%s/renamed-%02d.mkv", basePath, idx),
		Success:    success,
		Error: func() string {
			if success {
				return ""
			}
			return fmt.Sprintf("failure-%02d", idx)
		}(),
	}
}

func countSuccessfulOps(ops []log.OperationLog) int {
	count := 0
	for _, op := range ops {
		if op.Success {
			count++
		}
	}
	return count
}

func newUndoSummary(id string, opCount int) log.SessionSummary {
	basePath := fmt.Sprintf("/tmp/%s", id)
	ops := make([]log.OperationLog, opCount)
	for i := 0; i < opCount; i++ {
		ops[i] = newUndoOperation(id, basePath, i, i%2 == 0)
	}
	meta := log.SessionMetadata{
		CommandArgs:   []string{"undo", id},
		WorkingDir:    basePath,
		Timestamp:     time.Date(2024, time.January, 1, 10, 0, 0, 0, time.UTC).Add(time.Duration(len(id)) * time.Minute),
		SessionID:     id,
		TotalOps:      len(ops),
		SuccessfulOps: countSuccessfulOps(ops),
		FailedOps:     len(ops) - countSuccessfulOps(ops),
	}
	session := &log.LogSession{Metadata: meta, Operations: ops}
	return log.SessionSummary{
		Session:      session,
		FilePath:     fmt.Sprintf("/logs/%s.json", id),
		RelativeTime: "just now",
		Icon:         "📝",
	}
}

func newUndoTree(t *testing.T, summaries ...log.SessionSummary) (*treeview.Tree[log.SessionSummary], []*treeview.Node[log.SessionSummary]) {
	t.Helper()
	nodes := make([]*treeview.Node[log.SessionSummary], len(summaries))
	for i, summary := range summaries {
		nodes[i] = treeview.NewNode(fmt.Sprintf("session-%s", summary.Session.Metadata.SessionID), summary.Session.Metadata.SessionID, summary)
	}
	tree := treeview.NewTree(nodes)
	if len(nodes) > 0 {
		focusUndoNode(t, tree, nodes[0].ID())
	}
	return tree, nodes
}

func focusUndoNode(t *testing.T, tree *treeview.Tree[log.SessionSummary], id string) {
	t.Helper()
	if _, err := tree.SetFocusedID(context.Background(), id); err != nil {
		t.Fatalf("SetFocusedID(%q) error = %v", id, err)
	}
}

func startUndoTestModel(t *testing.T, model *UndoModel, opts ...teatest.TestOption) *teatest.TestModel {
	t.Helper()
	options := append([]teatest.TestOption{teatest.WithInitialTermSize(80, 16)}, opts...)
	tm := teatest.NewTestModel(t, model, options...)
	t.Cleanup(func() {
		_ = tm.Quit()
	})
	return tm
}

func finalUndoModel(t *testing.T, tm *teatest.TestModel) *UndoModel {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	model, ok := final.(*UndoModel)
	if !ok {
		t.Fatalf("Final model type = %T, want *UndoModel", final)
	}
	return model
}

func waitForUndoOutput(t *testing.T, tm *teatest.TestModel, contains string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(contains))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}

func withUndoSessionStub(t *testing.T, fn func(*log.LogSession) (int, int, []error)) {
	t.Helper()
	original := undoSessionFn
	undoSessionFn = fn
	t.Cleanup(func() {
		undoSessionFn = original
	})
}

func TestUndoModelQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyType
	}{
		{name: "esc", key: tea.KeyEsc},
		{name: "ctrl_c", key: tea.KeyCtrlC},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree, _ := newUndoTree(t, newUndoSummary("quit", 4))
			model := NewUndoModel(tree)
			tm := startUndoTestModel(t, model, teatest.WithInitialTermSize(80, 14))

			tm.Send(tea.WindowSizeMsg{Width: 80, Height: 14})
			tm.Send(tea.KeyMsg{Type: tc.key})
			tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

			final := finalUndoModel(t, tm)
			if final.undoInProgress {
				t.Error("undoInProgress = true, want false after quitting")
			}
			if final.confirmingUndo {
				t.Error("confirmingUndo = true, want false after quitting")
			}
		})
	}
}

func TestUndoModelTabFocusAndScrolling(t *testing.T) {
	tree, _ := newUndoTree(t, newUndoSummary("scroll", 18))
	model := NewUndoModel(tree)
	tm := startUndoTestModel(t, model, teatest.WithInitialTermSize(90, 12))

	tm.Send(tea.WindowSizeMsg{Width: 90, Height: 12})
	if model.detailsFocused {
		t.Fatal("detailsFocused = true, want false before toggling")
	}

	sendKey(tm, tea.KeyTab)
	if !model.detailsFocused {
		t.Fatal("detailsFocused = false, want true after Tab")
	}

	sendKey(tm, tea.KeyPgDown)
	if model.detailsViewport.YOffset == 0 {
		t.Fatal("detailsViewport.YOffset = 0, want >0 after PgDown")
	}
	downOffset := model.detailsViewport.YOffset

	sendKey(tm, tea.KeyPgUp)
	if model.detailsViewport.YOffset >= downOffset {
		t.Fatalf("detailsViewport.YOffset = %d, want < %d after PgUp", model.detailsViewport.YOffset, downOffset)
	}

	sendKey(tm, tea.KeyDown)
	if model.detailsViewport.YOffset == 0 {
		t.Fatal("detailsViewport.YOffset = 0, want >0 after Down")
	}

	sendKey(tm, tea.KeyUp)
	if model.detailsViewport.YOffset != 0 {
		t.Fatalf("detailsViewport.YOffset = %d, want 0 after Up", model.detailsViewport.YOffset)
	}

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	final := finalUndoModel(t, tm)
	if !final.detailsFocused {
		t.Error("detailsFocused = false, want true at exit")
	}
}

func TestUndoModelTreeNavigationRespectsFocus(t *testing.T) {
	tree, nodes := newUndoTree(t, newUndoSummary("first", 6), newUndoSummary("second", 4))
	model := NewUndoModel(tree)
	tm := startUndoTestModel(t, model, teatest.WithInitialTermSize(80, 14))

	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 14})

	sendKey(tm, tea.KeyDown)
	if got := model.TuiTreeModel.Tree.GetFocusedNode(); got == nil || got.ID() != nodes[1].ID() {
		id := "<nil>"
		if got != nil {
			id = got.ID()
		}
		t.Fatalf("tree focus after Down = %s, want %s", id, nodes[1].ID())
	}

	sendKey(tm, tea.KeyTab)
	if !model.detailsFocused {
		t.Fatal("detailsFocused = false, want true after Tab")
	}
	sendKey(tm, tea.KeyDown)
	if got := model.TuiTreeModel.Tree.GetFocusedNode(); got == nil || got.ID() != nodes[1].ID() {
		id := "<nil>"
		if got != nil {
			id = got.ID()
		}
		t.Fatalf("tree focus changed to %s while details focused, want %s", id, nodes[1].ID())
	}

	if model.detailsViewport.YOffset == 0 {
		t.Fatal("detailsViewport.YOffset = 0, want >0 after Down when details focused")
	}

	sendKey(tm, tea.KeyTab)
	if model.detailsFocused {
		t.Fatal("detailsFocused = true, want false after second Tab")
	}
	sendKey(tm, tea.KeyUp)
	if got := model.TuiTreeModel.Tree.GetFocusedNode(); got == nil || got.ID() != nodes[0].ID() {
		id := "<nil>"
		if got != nil {
			id = got.ID()
		}
		t.Fatalf("tree focus after Up = %s, want %s", id, nodes[0].ID())
	}

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestUndoModelCancelConfirmation(t *testing.T) {
	tree, _ := newUndoTree(t, newUndoSummary("cancel", 5))
	model := NewUndoModel(tree)
	tm := startUndoTestModel(t, model, teatest.WithInitialTermSize(80, 14))

	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 14})

	sendKey(tm, tea.KeyEnter)
	if !model.confirmingUndo {
		t.Fatal("confirmingUndo = false, want true after Enter")
	}

	tests := []rune{'n', 'N'}
	for _, key := range tests {
		sendRune(tm, key)
		if model.confirmingUndo {
			t.Fatalf("confirmingUndo = true, want false after key %q", string(key))
		}
		sendKey(tm, tea.KeyEnter)
		if !model.confirmingUndo {
			t.Fatalf("confirmingUndo = false, want true after Enter following %q", string(key))
		}
	}

	sendRune(tm, 'n')
	if model.confirmingUndo {
		t.Fatal("confirmingUndo = true, want false after final cancel")
	}

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestUndoModelPerformUndoFlow(t *testing.T) {
	tree, _ := newUndoTree(t, newUndoSummary("perform", 7))
	model := NewUndoModel(tree)
	called := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
	})
	withUndoSessionStub(t, func(session *log.LogSession) (int, int, []error) {
		if session == nil {
			t.Fatal("UndoSession called with nil session")
		}
		close(called)
		<-release
		return 3, 1, nil
	})

	tm := startUndoTestModel(t, model, teatest.WithInitialTermSize(90, 16))
	tm.Send(tea.WindowSizeMsg{Width: 90, Height: 16})

	sendKey(tm, tea.KeyEnter)
	if !model.confirmingUndo {
		t.Fatal("confirmingUndo = false, want true after initial Enter")
	}

	sendKey(tm, tea.KeyEnter)
	if model.confirmingUndo {
		t.Fatal("confirmingUndo = true, want false while undo in progress")
	}
	if !model.undoInProgress {
		t.Fatal("undoInProgress = false, want true immediately after confirm")
	}

	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for UndoSession to be invoked")
	}

	waitForUndoOutput(t, tm, "Undoing operations...")
	releaseOnce.Do(func() { close(release) })

	waitForUndoOutput(t, tm, "Undo completed: 3 success, 1 failed")

	if model.undoInProgress {
		t.Error("undoInProgress = true, want false after completion")
	}
	if !model.undoComplete {
		t.Error("undoComplete = false, want true after completion")
	}
	if diff := cmp.Diff(3, model.undoSuccess); diff != "" {
		t.Errorf("undoSuccess diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, model.undoFailed); diff != "" {
		t.Errorf("undoFailed diff (-want +got):\n%s", diff)
	}

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	final := finalUndoModel(t, tm)
	if !final.undoComplete {
		t.Error("undoComplete = false in final model, want true")
	}
	if final.undoInProgress {
		t.Error("undoInProgress = true in final model, want false")
	}
	if diff := cmp.Diff(3, final.undoSuccess); diff != "" {
		t.Errorf("final undoSuccess diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, final.undoFailed); diff != "" {
		t.Errorf("final undoFailed diff (-want +got):\n%s", diff)
	}
}

func TestUndoModelWindowResizeUpdatesLayout(t *testing.T) {
	tree, _ := newUndoTree(t, newUndoSummary("resize", 8))
	model := NewUndoModel(tree)
	tm := startUndoTestModel(t, model)

	first := tea.WindowSizeMsg{Width: 120, Height: 40}
	tm.Send(first)
	if diff := cmp.Diff(120, model.width); diff != "" {
		t.Errorf("first width diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(40, model.height); diff != "" {
		t.Errorf("first height diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(56, model.detailsViewport.Width); diff != "" {
		t.Errorf("first viewport width diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(32, model.detailsViewport.Height); diff != "" {
		t.Errorf("first viewport height diff (-want +got):\n%s", diff)
	}

	second := tea.WindowSizeMsg{Width: 60, Height: 20}
	tm.Send(second)
	if diff := cmp.Diff(60, model.width); diff != "" {
		t.Errorf("second width diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(20, model.height); diff != "" {
		t.Errorf("second height diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(26, model.detailsViewport.Width); diff != "" {
		t.Errorf("second viewport width diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(12, model.detailsViewport.Height); diff != "" {
		t.Errorf("second viewport height diff (-want +got):\n%s", diff)
	}

	sendKey(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
