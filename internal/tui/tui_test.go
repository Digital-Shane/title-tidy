package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/google/go-cmp/cmp"

	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
)

// tuiTestNode creates a node with path == name for simplicity (duplicated to keep test self-contained)
func tuiTestNode(name string, isDir bool) *treeview.Node[treeview.FileInfo] {
	fi := core.NewSimpleFileInfo(name, isDir)
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: fi, Path: name})
}

func buildTVTestTree() *treeview.Tree[treeview.FileInfo] {
	show := tuiTestNode("My Show", true)
	sm := core.EnsureMeta(show)
	sm.Type = core.MediaShow
	sm.NewName = "My Show (2024)"

	season := tuiTestNode("Season 1", true)
	sem := core.EnsureMeta(season)
	sem.Type = core.MediaSeason
	sem.NewName = "Season 01"
	show.AddChild(season)

	epVideo := tuiTestNode("Episode1.mkv", false)
	evm := core.EnsureMeta(epVideo)
	evm.Type = core.MediaEpisode
	evm.NewName = "My Show - S01E01.mkv" // renamed
	season.AddChild(epVideo)

	epSub := tuiTestNode("Episode1.srt", false)
	esm := core.EnsureMeta(epSub)
	esm.Type = core.MediaEpisode
	esm.NewName = epSub.Name() // identical => noChange
	season.AddChild(epSub)

	return treeview.NewTree([]*treeview.Node[treeview.FileInfo]{show},
		treeview.WithExpandAll[treeview.FileInfo](),
		treeview.WithProvider(CreateRenameProvider()),
	)
}

func buildMovieTestTree() *treeview.Tree[treeview.FileInfo] {
	movieDir := tuiTestNode("MovieDir", true)
	mdm := core.EnsureMeta(movieDir)
	mdm.Type = core.MediaMovie
	mdm.NewName = "Movie Dir (2024)"
	mdm.IsVirtual = true
	mdm.NeedsDirectory = true

	vid := tuiTestNode("moviefile.mkv", false)
	vmm := core.EnsureMeta(vid)
	vmm.Type = core.MediaMovieFile
	vmm.NewName = "Movie Dir (2024).mkv"
	movieDir.AddChild(vid)

	sub := tuiTestNode("moviefile.srt", false)
	smm := core.EnsureMeta(sub)
	smm.Type = core.MediaMovieFile
	smm.NewName = sub.Name() // unchanged
	movieDir.AddChild(sub)

	return treeview.NewTree([]*treeview.Node[treeview.FileInfo]{movieDir},
		treeview.WithExpandAll[treeview.FileInfo](),
		treeview.WithProvider(CreateRenameProvider()),
	)
}

func TestNewRenameModelInitialization(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	if m.width != 80 || m.height != 24 || m.treeWidth != 48 || m.treeHeight != 20 || m.statsWidth != 32 || m.statsHeight != 20 {
		t.Errorf("NewRenameModel defaults = (w=%d h=%d tw=%d th=%d sw=%d sh=%d), want (80 24 48 20 32 20)", m.width, m.height, m.treeWidth, m.treeHeight, m.statsWidth, m.statsHeight)
	}
}

func TestRecalcLayoutSmallSizes(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	m.width = 5
	m.height = 1
	m.CalculateLayout()
	if m.treeHeight < 5 {
		t.Errorf("recalcLayout small treeHeight = %d, want >=5", m.treeHeight)
	}
	if m.statsHeight < 1 {
		t.Errorf("recalcLayout small statsHeight = %d, want >=1", m.statsHeight)
	}
}

func TestWindowResizeUpdatesTreeModel(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	newW, newH := 120, 40
	updated, _ := m.Update(tea.WindowSizeMsg{Width: newW, Height: newH})
	rm := updated.(*RenameModel)
	if rm.width != newW || rm.height != newH {
		t.Errorf("Update(WindowSize).size = %dx%d, want %dx%d", rm.width, rm.height, newW, newH)
	}
	if rm.treeWidth != newW*6/10 {
		t.Errorf("Update(WindowSize).treeWidth = %d, want %d", rm.treeWidth, newW*6/10)
	}
	view := rm.View()
	if !strings.Contains(view, "TV Show Rename") {
		t.Errorf("Update(WindowSize) view missing header substring")
	}
}

func TestCalculateStatsTVMode(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	stats := m.calculateStats()
	if stats.showCount != 1 || stats.seasonCount != 1 || stats.episodeCount != 2 || stats.subtitleCount != 1 || stats.needRenameCount != 3 || stats.noChangeCount != 1 {
		t.Errorf("calculateStats(tv) counts = (%d %d %d %d %d %d) want (1 1 2 1 3 1)", stats.showCount, stats.seasonCount, stats.episodeCount, stats.subtitleCount, stats.needRenameCount, stats.noChangeCount)
	}
	m.successCount = 7
	stats2 := m.calculateStats()
	if stats2.successCount != 7 {
		t.Errorf("calculateStats(tv cached) successCount = %d, want 7", stats2.successCount)
	}
}

func TestCalculateStatsMovieMode(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildMovieTestTree())
	m.IsMovieMode = true
	stats := m.calculateStats()
	if stats.movieCount != 1 || stats.movieFileCount != 2 || stats.subtitleCount != 1 || stats.needRenameCount != 2 || stats.noChangeCount != 1 {
		t.Errorf("calculateStats(movie) counts = (%d %d %d %d %d) want (1 2 1 2 1)", stats.movieCount, stats.movieFileCount, stats.subtitleCount, stats.needRenameCount, stats.noChangeCount)
	}
}

func TestKeyRenameFlow(t *testing.T) {
	t.Parallel()
	n := tuiTestNode("file.txt", false)
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{n}, treeview.WithProvider(CreateRenameProvider()))
	m := NewRenameModel(tree)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !m.renameInProgress {
		t.Errorf("Update('r') renameInProgress = false, want true")
	}
	if cmd == nil {
		t.Fatalf("Update('r') cmd = nil, want non-nil")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	rm := updated.(*RenameModel)
	if !rm.renameComplete || rm.renameInProgress {
		t.Errorf("Update(renameComplete) flags = (inProgress=%v,complete=%v) want (false,true)", rm.renameInProgress, rm.renameComplete)
	}
}

func TestViewComponents(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	view := m.View()
	if !strings.Contains(view, "📺 TV Show Rename") {
		t.Errorf("View(tv) missing TV header")
	}
	if !strings.Contains(view, ": Navigate") {
		t.Errorf("View(tv) missing navigation hints")
	}
	if !strings.Contains(view, "📊 Statistics") {
		t.Errorf("View(tv) missing statistics panel")
	}
	m.IsMovieMode = true
	header := m.renderHeader()
	if !strings.Contains(header, "🎬 Movie Rename") {
		t.Errorf("renderHeader(movie) missing movie header")
	}
}

func TestRenderStatsPanelLastOperationAndPercentages(t *testing.T) {
	m := NewRenameModel(buildTVTestTree())
	m.successCount = 2
	m.errorCount = 1
	m.statsDirty = true
	panel := m.renderStatsPanel()
	checks := []string{"Last Operation:", "✅ Success:     2", "❌ Errors:      1", "Need rename:"}
	for _, c := range checks {
		if !strings.Contains(panel, c) {
			t.Errorf("renderStatsPanel(tv) missing %q", c)
		}
	}
}

func TestRenderStatsPanelMovieModeLabels(t *testing.T) {
	m := NewRenameModel(buildMovieTestTree())
	m.IsMovieMode = true
	panel := m.renderStatsPanel()
	if !strings.Contains(panel, "🎬 Movies:") || !strings.Contains(panel, "🎥 Video Files:") {
		t.Errorf("renderStatsPanel(movie) missing movie mode labels")
	}
}

func TestInitEmitsCmds(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init() cmd = nil, want non-nil")
	}
	msg := cmd()
	if msg == nil {
		t.Errorf("Init() produced nil message")
	}
}

func TestPerformRenames_Integration(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	os.WriteFile("childA.txt", []byte("a"), 0644)
	os.WriteFile("childB.txt", []byte("b"), 0644)
	os.WriteFile("vchild1.mkv", []byte("c"), 0644)
	os.WriteFile("vchild2.mkv", []byte("d"), 0644)
	os.Remove("vchild2.mkv") // force failure for second virtual child
	childA := fsTestNode("childA.txt", false, "childA.txt")
	mma := core.EnsureMeta(childA)
	mma.NewName = "childA-renamed.txt"
	childB := fsTestNode("childB.txt", false, "childB.txt")
	mmb := core.EnsureMeta(childB)
	mmb.NewName = "childB.txt" // no change
	vdir := fsTestNode("virt-old", true, "virt-old")
	mv := core.EnsureMeta(vdir)
	mv.NewName = "NewMovieDir"
	mv.IsVirtual = true
	mv.NeedsDirectory = true
	v1 := fsTestNode("vchild1.mkv", false, "vchild1.mkv")
	mv1 := core.EnsureMeta(v1)
	mv1.NewName = "Movie1.mkv"
	v2 := fsTestNode("vchild2.mkv", false, "vchild2.mkv")
	mv2 := core.EnsureMeta(v2)
	mv2.NewName = "Movie2.mkv"
	vdir.AddChild(v1)
	vdir.AddChild(v2)
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{childA, childB, vdir}, treeview.WithProvider(CreateRenameProvider()))
	model := &RenameModel{TuiTreeModel: treeview.NewTuiTreeModel(tree)}
	model.prepareRenameProgress()

	// Process all operations until completion for test
	var rc RenameCompleteMsg
	for {
		msg := model.PerformRenames()()
		if completeMsg, ok := msg.(RenameCompleteMsg); ok {
			rc = completeMsg
			break
		}
		// Continue processing if we got a progress message
	}
	if rc.successCount != 3 || rc.errorCount != 1 {
		t.Errorf("performRenames(integration) counts = (success=%d,error=%d), want (3,1)", rc.successCount, rc.errorCount)
	}
	if mma.RenameStatus != core.RenameStatusSuccess {
		t.Errorf("performRenames childA status = %v, want %v", mma.RenameStatus, core.RenameStatusSuccess)
	}
	if mmb.RenameStatus != core.RenameStatusNone {
		t.Errorf("performRenames childB status = %v, want %v", mmb.RenameStatus, core.RenameStatusNone)
	}
	if mv.RenameStatus != core.RenameStatusSuccess || mv1.RenameStatus != core.RenameStatusSuccess || mv2.RenameStatus != core.RenameStatusError {
		t.Errorf("performRenames virtual statuses = (%v,%v,%v) want (Success,Success,Error)", mv.RenameStatus, mv1.RenameStatus, mv2.RenameStatus)
	}
	if v1.Data().Path != "NewMovieDir/Movie1.mkv" {
		t.Errorf("performRenames vchild1 path = %s, want NewMovieDir/Movie1.mkv", v1.Data().Path)
	}
	if v2.Data().Path != "vchild2.mkv" {
		t.Errorf("performRenames vchild2 path = %s, want vchild2.mkv", v2.Data().Path)
	}
	if _, err := os.Stat("childA-renamed.txt"); err != nil {
		t.Errorf("performRenames renamed file stat error = %v", err)
	}
}

func TestPerformRenames_NoOps(t *testing.T) {
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{}, treeview.WithProvider(CreateRenameProvider()))
	m := &RenameModel{TuiTreeModel: treeview.NewTuiTreeModel(tree)}
	m.prepareRenameProgress()
	rc := m.PerformRenames()().(RenameCompleteMsg)
	if rc.successCount != 0 || rc.errorCount != 0 {
		t.Errorf("performRenames(noNodes) counts = %+v, want 0/0", rc)
	}
	n := fsTestNode("file.txt", false, "file.txt")
	tree2 := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{n}, treeview.WithProvider(CreateRenameProvider()))
	m2 := &RenameModel{TuiTreeModel: treeview.NewTuiTreeModel(tree2)}
	m2.prepareRenameProgress()
	rc2 := m2.PerformRenames()().(RenameCompleteMsg)
	if rc2.successCount != 0 || rc2.errorCount != 0 {
		t.Errorf("performRenames(oneNoMeta) counts = %+v, want 0/0", rc2)
	}
}

// Benchmark: trivial tree to ensure performance stability on no-op case.
func BenchmarkPerformRenames_NoWork(b *testing.B) {
	nodes := make([]*treeview.Node[treeview.FileInfo], 0, 100)
	for i := 0; i < 100; i++ {
		n := fsTestNode("file"+strconv.Itoa(i), false, "file"+strconv.Itoa(i))
		nodes = append(nodes, n)
	}
	tree := treeview.NewTree(nodes, treeview.WithProvider(CreateRenameProvider()))
	m := &RenameModel{TuiTreeModel: treeview.NewTuiTreeModel(tree)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.prepareRenameProgress()
		_ = m.PerformRenames()().(RenameCompleteMsg)
	}
}

func TestPerformRenames_BottomUpOrder(t *testing.T) {
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	if err := os.Mkdir("parentDir", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("parentDir/child.txt", []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	parent := fsTestNode("parentDir", true, "parentDir")
	mp := core.EnsureMeta(parent)
	mp.NewName = "parentDirRenamed"
	child := fsTestNode("child.txt", false, "parentDir/child.txt")
	mc := core.EnsureMeta(child)
	mc.NewName = "child-renamed.txt"
	parent.AddChild(child)
	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{parent}, treeview.WithProvider(CreateRenameProvider()))
	m := &RenameModel{TuiTreeModel: treeview.NewTuiTreeModel(tree)}
	m.prepareRenameProgress()

	// Process all operations until completion for test
	var rc RenameCompleteMsg
	for {
		msg := m.PerformRenames()()
		if completeMsg, ok := msg.(RenameCompleteMsg); ok {
			rc = completeMsg
			break
		}
		// Continue processing if we got a progress message
	}
	if rc.successCount != 2 || rc.errorCount != 0 {
		t.Errorf("performRenames(bottomUp) counts = %+v, want success=2 error=0", rc)
	}
	if child.Data().Path != filepath.Join("parentDir", "child-renamed.txt") {
		t.Errorf("performRenames(bottomUp) child path = %s, want parentDir/child-renamed.txt", child.Data().Path)
	}
	if parent.Data().Path != "parentDirRenamed" {
		t.Errorf("performRenames(bottomUp) parent path = %s, want parentDirRenamed", parent.Data().Path)
	}
}

// Helper to create a test node for removal tests
func testRemovalNode(t *testing.T, name string, isDir bool) *treeview.Node[treeview.FileInfo] {
	t.Helper()
	fi := core.NewSimpleFileInfo(name, isDir)
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: fi, Path: name})
}

// Helper to create a simple tree for testing
func createTestTree(t *testing.T, nodes ...*treeview.Node[treeview.FileInfo]) *treeview.Tree[treeview.FileInfo] {
	t.Helper()
	return treeview.NewTree(nodes,
		treeview.WithExpandAll[treeview.FileInfo](),
		treeview.WithProvider(CreateRenameProvider()),
	)
}

// Helper to get node names from a slice
func nodeNames(t *testing.T, nodes []*treeview.Node[treeview.FileInfo]) []string {
	t.Helper()
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name()
	}
	return names
}

func TestRemoveNodeFromTree(t *testing.T) {
	tests := []struct {
		name         string
		setupTree    func() (*RenameModel, *treeview.Node[treeview.FileInfo])
		verifyResult func(t *testing.T, model *RenameModel)
	}{
		{
			name: "remove_nil_node",
			setupTree: func() (*RenameModel, *treeview.Node[treeview.FileInfo]) {
				root := testRemovalNode(t, "root", true)
				tree := createTestTree(t, root)
				model := NewRenameModel(tree)
				return model, nil
			},
			verifyResult: func(t *testing.T, model *RenameModel) {
				// Should not panic, tree should be unchanged
				if len(model.TuiTreeModel.Tree.Nodes()) != 1 {
					t.Errorf("removeNodeFromTree(nil) changed tree, nodes = %d, want 1", len(model.TuiTreeModel.Tree.Nodes()))
				}
			},
		},
		{
			name: "remove_root_node_no_parent",
			setupTree: func() (*RenameModel, *treeview.Node[treeview.FileInfo]) {
				root1 := testRemovalNode(t, "root1", true)
				root2 := testRemovalNode(t, "root2", true)
				tree := createTestTree(t, root1, root2)
				model := NewRenameModel(tree)
				return model, root1
			},
			verifyResult: func(t *testing.T, model *RenameModel) {
				got := nodeNames(t, model.TuiTreeModel.Tree.Nodes())
				want := []string{"root2"}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("removeNodeFromTree(root) remaining nodes mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "remove_child_node",
			setupTree: func() (*RenameModel, *treeview.Node[treeview.FileInfo]) {
				root := testRemovalNode(t, "root", true)
				child1 := testRemovalNode(t, "child1", false)
				child2 := testRemovalNode(t, "child2", false)
				child3 := testRemovalNode(t, "child3", false)
				root.SetChildren([]*treeview.Node[treeview.FileInfo]{child1, child2, child3})
				tree := createTestTree(t, root)
				model := NewRenameModel(tree)
				return model, child2
			},
			verifyResult: func(t *testing.T, model *RenameModel) {
				root := model.TuiTreeModel.Tree.Nodes()[0]
				got := nodeNames(t, root.Children())
				want := []string{"child1", "child3"}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("removeNodeFromTree(child2) parent's children mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, nodeToRemove := tt.setupTree()
			model.removeNodeFromTree(nodeToRemove)
			tt.verifyResult(t, model)
		})
	}
}

func TestKeyHandling(t *testing.T) {
	t.Parallel()
	m := NewRenameModel(buildTVTestTree())

	// Test 'd' key (delete)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Errorf("Update('d') cmd = %v, want nil", cmd)
	}

	// Should still return the model
	if rm, ok := updated.(*RenameModel); !ok {
		t.Errorf("Update('d') returned %T, want *RenameModel", updated)
	} else if rm != m {
		t.Errorf("Update('d') returned different model")
	}
}

func TestDeleteFilesMode(t *testing.T) {
	t.Parallel()
	n := tuiTestNode("delete.nfo", false)
	nm := core.EnsureMeta(n)
	nm.MarkedForDeletion = true

	tree := treeview.NewTree([]*treeview.Node[treeview.FileInfo]{n},
		treeview.WithProvider(CreateRenameProvider()))
	m := NewRenameModel(tree)
	m.DeleteNFO = true
	m.DeleteImages = true

	// Should not affect other counts since files marked for deletion
	// are handled differently in statistics
	stats := m.calculateStats()
	if stats.needRenameCount != 0 && stats.noChangeCount != 0 {
		t.Errorf("calculateStats() with marked for deletion affects rename counts")
	}
}

func TestPageKeysMinimalHeight(t *testing.T) {
	t.Parallel()
	// Test page keys with minimal tree height to trigger max() logic
	m := NewRenameModel(buildTVTestTree())
	m.treeHeight = 5 // Less than 10, should use 10 as pageSize

	// Test that page keys work with small tree height
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgup")})
	if cmd != nil {
		t.Errorf("Update(pgup small height) cmd = %v, want nil", cmd)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgdown")})
	if cmd != nil {
		t.Errorf("Update(pgdown small height) cmd = %v, want nil", cmd)
	}
}

func TestMouseWheelScrolling(t *testing.T) {
	t.Parallel()
	// Test mouse wheel handling (lines 178-188)
	m := NewRenameModel(buildTVTestTree())

	// Test Mouse Wheel Up
	updated, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButton(4)})
	if cmd != nil {
		t.Errorf("Update(mouse wheel up) cmd = %v, want nil", cmd)
	}
	if _, ok := updated.(*RenameModel); !ok {
		t.Errorf("Update(mouse wheel up) returned %T, want *RenameModel", updated)
	}

	// Test Mouse Wheel Down
	updated, cmd = m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButton(5)})
	if cmd != nil {
		t.Errorf("Update(mouse wheel down) cmd = %v, want nil", cmd)
	}
	if _, ok := updated.(*RenameModel); !ok {
		t.Errorf("Update(mouse wheel down) returned %T, want *RenameModel", updated)
	}
}

func TestProgressMessages(t *testing.T) {
	t.Parallel()
	// Test progress message handling
	m := NewRenameModel(buildTVTestTree())

	// Test rename progress message
	updated, cmd := m.Update(renameProgressMsg{})
	if cmd == nil {
		t.Errorf("Update(renameProgressMsg) cmd = nil, want non-nil")
	}

	rm := updated.(*RenameModel)
	if rm == nil {
		t.Errorf("Update(renameProgressMsg) returned nil model")
	}
}
