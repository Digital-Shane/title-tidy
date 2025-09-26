package rename

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/treeview"
)

// fsTestNode creates a node representing a filesystem entry; path provided explicitly.
func fsTestNode(name string, isDir bool, path string) *treeview.Node[treeview.FileInfo] {
	fi := core.NewSimpleFileInfo(name, isDir)
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: fi, Path: path})
}

func TestRenameRegular_NoChange(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "same.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write same.txt: %v", err)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	n := fsTestNode("same.txt", false, "same.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "same.txt" // identical
	renamed, err := core.RenameRegular(n, mm)
	if err != nil || renamed {
		t.Errorf("RenameRegular(no change) = (%v,%v), want (false,<nil>)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusNone {
		t.Errorf("RenameRegular(no change) status = %v, want %v", mm.RenameStatus, core.RenameStatusNone)
	}
}

func TestRenameRegular_DestinationExists(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("src.txt", []byte("src"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile("dest.txt", []byte("dest"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	n := fsTestNode("src.txt", false, "src.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "dest.txt"
	renamed, err := core.RenameRegular(n, mm)
	if err == nil || renamed {
		t.Errorf("RenameRegular(dest exists) = (%v,%v), want (false,error)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusError || mm.RenameError == "" {
		t.Errorf("RenameRegular(dest exists) meta = %+v, want error status with message", mm)
	}
	if n.Data().Path != "src.txt" {
		t.Errorf("RenameRegular(dest exists) path = %s, want src.txt", n.Data().Path)
	}
}

func TestRenameRegular_SourceMissingCausesError(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	n := fsTestNode("src.txt", false, "src.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "renamed.txt"
	renamed, err := core.RenameRegular(n, mm)
	if err == nil || renamed {
		t.Errorf("RenameRegular(missing source) = (%v,%v), want (false,error)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusError {
		t.Errorf("RenameRegular(missing source) status = %v, want %v", mm.RenameStatus, core.RenameStatusError)
	}
}

func TestRenameRegular_Success(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("orig.txt", []byte("content"), 0o644); err != nil {
		t.Fatalf("write orig: %v", err)
	}
	n := fsTestNode("orig.txt", false, "orig.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "new.txt"
	renamed, err := core.RenameRegular(n, mm)
	if err != nil || !renamed {
		t.Errorf("RenameRegular(success) = (%v,%v), want (true,<nil>)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("RenameRegular(success) status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
	if n.Data().Path != "new.txt" {
		t.Errorf("RenameRegular(success) path = %s, want new.txt", n.Data().Path)
	}
	if _, err := os.Stat("new.txt"); err != nil {
		t.Errorf("RenameRegular(success) new file stat error = %v", err)
	}
}

func TestCreateVirtualDir_MkdirFails(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.Mkdir("Already", 0o755); err != nil {
		t.Fatalf("mkdir existing: %v", err)
	}
	n := fsTestNode("virtual", true, "virtual")
	mm := core.EnsureMeta(n)
	mm.NewName = "Already"
	mm.IsVirtual = true
	mm.NeedsDirectory = true
	successes, errs := core.CreateVirtualDir(n, mm)
	if successes != 0 || len(errs) != 1 {
		t.Errorf("CreateVirtualDir(mkdir fail) = (%d success,%d errs), want (0,1)", successes, len(errs))
	}
	if mm.RenameStatus != core.RenameStatusError {
		t.Errorf("CreateVirtualDir(mkdir fail) status = %v, want %v", mm.RenameStatus, core.RenameStatusError)
	}
	if n.Data().Path == "./Already" {
		t.Errorf("CreateVirtualDir(mkdir fail) path updated unexpectedly")
	}
}

func TestCreateVirtualDir_SuccessChildrenMixed(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("child1.mkv", []byte("a"), 0o644); err != nil {
		t.Fatalf("write child1: %v", err)
	}
	if err := os.WriteFile("child2.mkv", []byte("b"), 0o644); err != nil {
		t.Fatalf("write child2: %v", err)
	}
	if err := os.WriteFile("childSkip.mkv", []byte("c"), 0o644); err != nil {
		t.Fatalf("write childSkip: %v", err)
	}
	if err := os.Remove("child2.mkv"); err != nil {
		t.Fatalf("remove child2: %v", err)
	}
	vdir := fsTestNode("virt-old", true, "virt-old")
	mm := core.EnsureMeta(vdir)
	mm.NewName = "Movie Name"
	mm.IsVirtual = true
	mm.NeedsDirectory = true
	c1 := fsTestNode("child1.mkv", false, "child1.mkv")
	cm1 := core.EnsureMeta(c1)
	cm1.NewName = "Renamed1.mkv"
	c2 := fsTestNode("child2.mkv", false, "child2.mkv")
	cm2 := core.EnsureMeta(c2)
	cm2.NewName = "Renamed2.mkv"
	c3 := fsTestNode("childSkip.mkv", false, "childSkip.mkv")
	vdir.AddChild(c1)
	vdir.AddChild(c2)
	vdir.AddChild(c3)
	successes, errs := core.CreateVirtualDir(vdir, mm)
	if successes != 2 || len(errs) != 1 {
		t.Errorf("CreateVirtualDir(mixed) = (%d success,%d errs), want (2,1)", successes, len(errs))
	}
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("CreateVirtualDir(mixed) dir status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
	if cm1.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("CreateVirtualDir(mixed) child1 status = %v, want %v", cm1.RenameStatus, core.RenameStatusSuccess)
	}
	if cm2.RenameStatus != core.RenameStatusError {
		t.Errorf("CreateVirtualDir(mixed) child2 status = %v, want %v", cm2.RenameStatus, core.RenameStatusError)
	}
	if c1.Data().Path != "Movie Name/Renamed1.mkv" {
		t.Errorf("CreateVirtualDir(mixed) child1 path = %s, want Movie Name/Renamed1.mkv", c1.Data().Path)
	}
	if c2.Data().Path != "child2.mkv" {
		t.Errorf("CreateVirtualDir(mixed) child2 path = %s, want child2.mkv", c2.Data().Path)
	}
	if vdir.Data().Path != "Movie Name" {
		t.Errorf("CreateVirtualDir(mixed) dir path = %s, want Movie Name", vdir.Data().Path)
	}
}

func TestRenameCompleteMsg(t *testing.T) {
	msg := RenameCompleteMsg{successCount: 5, errorCount: 2}
	if got := msg.SuccessCount(); got != 5 {
		t.Errorf("RenameCompleteMsg.SuccessCount() = %d, want 5", got)
	}
	if got := msg.ErrorCount(); got != 2 {
		t.Errorf("RenameCompleteMsg.ErrorCount() = %d, want 2", got)
	}
}

func TestOperationEngineDeletionPhase(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	mustWrite := func(name string) {
		t.Helper()
		if err := os.WriteFile(name, []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("delete1.nfo")
	mustWrite("delete2.jpg")
	mustWrite("keep.mkv")

	deleteFile1 := fsTestNode("delete1.nfo", false, "delete1.nfo")
	deleteFile1Meta := core.EnsureMeta(deleteFile1)
	deleteFile1Meta.MarkedForDeletion = true

	deleteFile2 := fsTestNode("delete2.jpg", false, "delete2.jpg")
	deleteFile2Meta := core.EnsureMeta(deleteFile2)
	deleteFile2Meta.MarkedForDeletion = true

	keepFile := fsTestNode("keep.mkv", false, "keep.mkv")
	core.EnsureMeta(keepFile)

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{deleteFile1, deleteFile2, keepFile},
		treeview.WithProvider(components.CreateRenameProvider()))

	engine := NewOperationEngine(OperationConfig{Tree: tree})
	result := engine.RunToCompletion()

	if result.SuccessCount() != 2 || result.ErrorCount() != 0 {
		t.Fatalf("RunToCompletion success/error = (%d,%d), want (2,0)", result.SuccessCount(), result.ErrorCount())
	}

	if _, err := os.Stat("delete1.nfo"); err == nil {
		t.Errorf("os.Stat(delete1.nfo) error = nil, want error")
	}
	if _, err := os.Stat("delete2.jpg"); err == nil {
		t.Errorf("os.Stat(delete2.jpg) error = nil, want error")
	}
	if _, err := os.Stat("keep.mkv"); err != nil {
		t.Errorf("os.Stat(keep.mkv) error = %v, want nil", err)
	}
	if deleteFile1Meta.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("delete1 status = %v, want %v", deleteFile1Meta.RenameStatus, core.RenameStatusSuccess)
	}
	if deleteFile2Meta.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("delete2 status = %v, want %v", deleteFile2Meta.RenameStatus, core.RenameStatusSuccess)
	}
}

func TestOperationEngineDeletionError(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	deleteFile := fsTestNode("missing.nfo", false, "missing.nfo")
	deleteMeta := core.EnsureMeta(deleteFile)
	deleteMeta.MarkedForDeletion = true

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{deleteFile},
		treeview.WithProvider(components.CreateRenameProvider()))

	engine := NewOperationEngine(OperationConfig{Tree: tree})
	result := engine.RunToCompletion()

	if result.SuccessCount() != 0 || result.ErrorCount() != 1 {
		t.Fatalf("RunToCompletion success/error = (%d,%d), want (0,1)", result.SuccessCount(), result.ErrorCount())
	}
	if deleteMeta.RenameStatus != core.RenameStatusError {
		t.Errorf("delete meta status = %v, want %v", deleteMeta.RenameStatus, core.RenameStatusError)
	}
}

func TestOperationEngineProgressSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	if err := os.WriteFile("episode.mkv", []byte("x"), 0o644); err != nil {
		t.Fatalf("write episode: %v", err)
	}

	node := fsTestNode("episode.mkv", false, "episode.mkv")
	meta := core.EnsureMeta(node)
	meta.NewName = "renamed.mkv"

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node})
	engine := NewOperationEngine(OperationConfig{Tree: tree})

	msg := engine.ProcessNext()
	progressMsg, ok := msg.(OperationProgressMsg)
	if !ok {
		t.Fatalf("ProcessNext type = %T, want OperationProgressMsg", msg)
	}
	progress := progressMsg.Progress
	if progress.OverallTotal != 1 || progress.OverallCompleted != 1 {
		t.Errorf("progress overall = (%d/%d), want (1/1)", progress.OverallCompleted, progress.OverallTotal)
	}
	if progress.SuccessCount != 1 {
		t.Errorf("OperationProgress.SuccessCount = %d, want 1", progress.SuccessCount)
	}
	if progress.ErrorCount != 0 {
		t.Errorf("OperationProgress.ErrorCount = %d, want 0", progress.ErrorCount)
	}

	complete := engine.RunToCompletion()
	if complete.ErrorCount() != 0 {
		t.Fatalf("RunToCompletion after progress error count = %d, want 0", complete.ErrorCount())
	}
}

func TestOperationEngineUsesInjectedFunctions(t *testing.T) {
	called := false
	node := fsTestNode("file.txt", false, "file.txt")
	meta := core.EnsureMeta(node)
	meta.NewName = "new.txt"

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{node})
	engine := NewOperationEngine(OperationConfig{
		Tree: tree,
		Functions: OperationFunctions{
			RenameRegular: func(n *treeview.Node[treeview.FileInfo], mm *core.MediaMeta) (bool, error) {
				called = true
				mm.Success()
				n.Data().Path = "new.txt"
				return true, nil
			},
			StartSession: func(string, []string) error { return nil },
			EndSession:   func() error { return nil },
		},
	})

	result := engine.RunToCompletion()
	if !called {
		t.Fatalf("custom RenameRegular was not invoked")
	}
	if result.SuccessCount() != 1 || result.ErrorCount() != 0 {
		t.Fatalf("RunToCompletion success/error = (%d,%d), want (1,0)", result.SuccessCount(), result.ErrorCount())
	}
	if node.Data().Path != "new.txt" {
		t.Errorf("custom rename path = %s, want new.txt", node.Data().Path)
	}
}
