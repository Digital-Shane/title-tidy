package rename

import (
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"os"
	"path/filepath"
	"testing"

	"github.com/Digital-Shane/treeview"
)

// fsTestNode creates a node representing a filesystem entry; path provided explicitly.
func fsTestNode(name string, isDir bool, path string) *treeview.Node[treeview.FileInfo] {
	fi := core.NewSimpleFileInfo(name, isDir)
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: fi, Path: path})
}

func TestRenameRegular_NoChange(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "same.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("prep: %v", err)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	n := fsTestNode("same.txt", false, "same.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "same.txt" // identical
	renamed, err := core.RenameRegular(n, mm)
	if err != nil || renamed {
		t.Errorf("renameRegular(identical) = (%v,%v), want (false,<nil>)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusNone {
		t.Errorf("renameRegular(identical) status = %v, want %v", mm.RenameStatus, core.RenameStatusNone)
	}
}

func TestRenameRegular_DestinationExists(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("src.txt", []byte("src"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("dest.txt", []byte("dest"), 0644); err != nil {
		t.Fatal(err)
	}
	n := fsTestNode("src.txt", false, "src.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "dest.txt"
	renamed, err := core.RenameRegular(n, mm)
	if err == nil || renamed {
		t.Errorf("renameRegular(destExists) = (%v,%v), want (false,error)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusError || mm.RenameError == "" {
		t.Errorf("renameRegular(destExists) meta = %+v, want error status with message", mm)
	}
	if n.Data().Path != "src.txt" {
		t.Errorf("renameRegular(destExists) path = %s, want src.txt", n.Data().Path)
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
		t.Errorf("renameRegular(missingSource) = (%v,%v), want (false,error)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusError {
		t.Errorf("renameRegular(missingSource) status = %v, want %v", mm.RenameStatus, core.RenameStatusError)
	}
}

func TestRenameRegular_Success(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("orig.txt", []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	n := fsTestNode("orig.txt", false, "orig.txt")
	mm := core.EnsureMeta(n)
	mm.NewName = "new.txt"
	renamed, err := core.RenameRegular(n, mm)
	if err != nil || !renamed {
		t.Errorf("renameRegular(success) = (%v,%v), want (true,<nil>)", renamed, err)
	}
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("renameRegular(success) status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
	if n.Data().Path != "new.txt" {
		t.Errorf("renameRegular(success) path = %s, want new.txt", n.Data().Path)
	}
	if _, err := os.Stat("new.txt"); err != nil {
		t.Errorf("renameRegular(success) new file stat error = %v", err)
	}
}

func TestCreateVirtualDir_MkdirFails(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.Mkdir("Already", 0755); err != nil {
		t.Fatal(err)
	}
	n := fsTestNode("virtual", true, "virtual")
	mm := core.EnsureMeta(n)
	mm.NewName = "Already"
	mm.IsVirtual = true
	mm.NeedsDirectory = true
	successes, errs := core.CreateVirtualDir(n, mm)
	if successes != 0 || len(errs) != 1 {
		t.Errorf("createVirtualDir(mkdirFail) = (%d,%d errs), want (0,1)", successes, len(errs))
	}
	if mm.RenameStatus != core.RenameStatusError {
		t.Errorf("createVirtualDir(mkdirFail) status = %v, want %v", mm.RenameStatus, core.RenameStatusError)
	}
	if n.Data().Path == "./Already" {
		t.Errorf("createVirtualDir(mkdirFail) path updated unexpectedly")
	}
}

func TestCreateVirtualDir_SuccessChildrenMixed(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.WriteFile("child1.mkv", []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("child2.mkv", []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("childSkip.mkv", []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove("child2.mkv"); err != nil {
		t.Fatal(err)
	} // remove to force failure
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
	cm2.NewName = "Renamed2.mkv"                              // will fail
	c3 := fsTestNode("childSkip.mkv", false, "childSkip.mkv") // no metadata new name => skipped
	vdir.AddChild(c1)
	vdir.AddChild(c2)
	vdir.AddChild(c3)
	successes, errs := core.CreateVirtualDir(vdir, mm)
	if successes != 2 || len(errs) != 1 {
		t.Errorf("createVirtualDir(mixed) counts = (%d successes,%d errs), want (2,1)", successes, len(errs))
	}
	if mm.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("createVirtualDir(mixed) dir status = %v, want %v", mm.RenameStatus, core.RenameStatusSuccess)
	}
	if cm1.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("createVirtualDir(mixed) child1 status = %v, want %v", cm1.RenameStatus, core.RenameStatusSuccess)
	}
	if cm2.RenameStatus != core.RenameStatusError {
		t.Errorf("createVirtualDir(mixed) child2 status = %v, want %v", cm2.RenameStatus, core.RenameStatusError)
	}
	if c1.Data().Path != "Movie Name/Renamed1.mkv" {
		t.Errorf("createVirtualDir(mixed) child1 path = %s, want Movie Name/Renamed1.mkv", c1.Data().Path)
	}
	if c2.Data().Path != "child2.mkv" {
		t.Errorf("createVirtualDir(mixed) child2 path = %s, want child2.mkv", c2.Data().Path)
	}
	if vdir.Data().Path != "Movie Name" {
		t.Errorf("createVirtualDir(mixed) dir path = %s, want Movie Name", vdir.Data().Path)
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

func TestPerformRenames_DeletionPhase(t *testing.T) {
	// Test the deletion phase (lines 158-179)
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)

	// Create files to delete
	os.WriteFile("delete1.nfo", []byte("nfo1"), 0644)
	os.WriteFile("delete2.jpg", []byte("img1"), 0644)
	os.WriteFile("keep.mkv", []byte("video"), 0644)

	// Create nodes
	deleteFile1 := fsTestNode("delete1.nfo", false, "delete1.nfo")
	deleteFile1Meta := core.EnsureMeta(deleteFile1)
	deleteFile1Meta.MarkedForDeletion = true

	deleteFile2 := fsTestNode("delete2.jpg", false, "delete2.jpg")
	deleteFile2Meta := core.EnsureMeta(deleteFile2)
	deleteFile2Meta.MarkedForDeletion = true

	keepFile := fsTestNode("keep.mkv", false, "keep.mkv")
	keepFileMeta := core.EnsureMeta(keepFile)
	keepFileMeta.NewName = "keep.mkv" // No rename needed

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{
		deleteFile1, deleteFile2, keepFile,
	}, treeview.WithProvider(components.CreateRenameProvider()))

	model := NewRenameModel(tree)
	model.prepareRenameProgress()

	// Process all operations
	var rc RenameCompleteMsg
	for {
		msg := model.PerformRenames()()
		if completeMsg, ok := msg.(RenameCompleteMsg); ok {
			rc = completeMsg
			break
		}
	}

	// Should have deleted 2 files
	if rc.successCount != 2 || rc.errorCount != 0 {
		t.Errorf("PerformRenames deletion phase: success=%d, errors=%d, want success=2, errors=0", rc.successCount, rc.errorCount)
	}

	// Files should be deleted
	if _, err := os.Stat("delete1.nfo"); err == nil {
		t.Errorf("delete1.nfo should have been deleted")
	}
	if _, err := os.Stat("delete2.jpg"); err == nil {
		t.Errorf("delete2.jpg should have been deleted")
	}
	if _, err := os.Stat("keep.mkv"); err != nil {
		t.Errorf("keep.mkv should still exist: %v", err)
	}
}

func TestPerformRenames_DeletionError(t *testing.T) {
	// Test deletion error handling (lines 170-172)
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)

	// Create node for non-existent file
	deleteFile := fsTestNode("nonexistent.nfo", false, "nonexistent.nfo")
	deleteFileMeta := core.EnsureMeta(deleteFile)
	deleteFileMeta.MarkedForDeletion = true

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{deleteFile},
		treeview.WithProvider(components.CreateRenameProvider()))

	model := NewRenameModel(tree)
	model.prepareRenameProgress()

	// Process deletion
	var rc RenameCompleteMsg
	for {
		msg := model.PerformRenames()()
		if completeMsg, ok := msg.(RenameCompleteMsg); ok {
			rc = completeMsg
			break
		}
	}

	// Should have error
	if rc.errorCount != 1 || rc.successCount != 0 {
		t.Errorf("PerformRenames deletion error: success=%d, errors=%d, want success=0, errors=1", rc.successCount, rc.errorCount)
	}

	// Check error status
	if deleteFileMeta.RenameStatus != core.RenameStatusError {
		t.Errorf("Deletion failure should set RenameStatusError, got %v", deleteFileMeta.RenameStatus)
	}
}
