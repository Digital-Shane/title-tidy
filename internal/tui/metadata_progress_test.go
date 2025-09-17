package tui

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"
	csmap "github.com/mhmtszr/concurrent-swiss-map"
)

// Test helper functions
func testNewFileNode(name string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(name, false), Path: name})
}

func testNewDirNode(name string) *treeview.Node[treeview.FileInfo] {
	return treeview.NewNode(name, name, treeview.FileInfo{FileInfo: core.NewSimpleFileInfo(name, true), Path: name})
}

func testNewTree(nodes ...*treeview.Node[treeview.FileInfo]) *treeview.Tree[treeview.FileInfo] {
	return treeview.NewTree(nodes)
}

func TestMetadataProgressModel_EpisodesAtDepth0(t *testing.T) {
	// Create episode files at depth 0 (as in episodes command)
	episode1 := testNewFileNode("the.show.title.s01e01.episode.one.mkv")
	episode2 := testNewFileNode("the.show.title.s01e02.episode.two.mkv")

	tree := testNewTree(episode1, episode2)

	// Create model
	cfg := &config.FormatConfig{
		EnableTMDBLookup: false,
	}
	model := NewMetadataProgressModel(tree, cfg)

	// Collect metadata items
	items := model.collectMetadataItems()

	// Should have detected show and episodes
	expectedKeys := map[string]bool{
		"show:the show title:":        true, // Empty year has one colon
		"episode:the show title::1:1": true,
		"episode:the show title::1:2": true,
	}

	foundKeys := make(map[string]bool)
	for _, item := range items {
		foundKeys[item.Key] = true
		t.Logf("Found key: %q (IsMovie: %v)", item.Key, item.IsMovie)
	}

	for key := range expectedKeys {
		if !foundKeys[key] {
			t.Errorf("Expected metadata key %q not found", key)
			t.Logf("Available keys: %v", foundKeys)
		}
	}

	// Should NOT have movie keys
	for _, item := range items {
		if item.IsMovie {
			t.Errorf("Episode file incorrectly classified as movie: %s", item.Key)
		}
	}
}

func TestMetadataProgressModel_OrganizeItemsByPhase(t *testing.T) {
	// Create a test tree structure
	showNode := testNewDirNode("Breaking Bad (2008)")
	seasonNode := testNewDirNode("Season 01")
	episodeNode := testNewFileNode("S01E01 - Pilot.mkv")
	seasonNode.AddChild(episodeNode)
	showNode.AddChild(seasonNode)

	// Add a movie
	movieNode := testNewFileNode("The Matrix (1999).mkv")

	tree := testNewTree(showNode, movieNode)

	// Create model
	cfg := &config.FormatConfig{
		EnableTMDBLookup: false, // Don't need actual TMDB for this test
	}
	model := NewMetadataProgressModel(tree, cfg)

	// Test phase organization
	phases := model.organizeItemsByPhase()

	// Debug: Print ALL collected items
	t.Logf("All collected items:")
	for _, phase := range phases {
		for _, item := range phase {
			t.Logf("  - Key: %s, Name: %s, Year: %s, IsMovie: %v, Phase: %d",
				item.Key, item.Name, item.Year, item.IsMovie, item.Phase)
		}
	}

	// Debug: Print what's in phase 0
	t.Logf("Phase 0 items specifically:")
	for _, item := range phases[0] {
		t.Logf("  - Key: %s, Name: %s, IsMovie: %v, Phase: %d", item.Key, item.Name, item.IsMovie, item.Phase)
	}

	// Check phase 0 (shows/movies)
	if len(phases[0]) != 2 {
		t.Errorf("Phase 0 item count = %d, want 2", len(phases[0]))
	}

	// Check phase 1 (seasons)
	if len(phases[1]) != 1 {
		t.Errorf("Phase 1 item count = %d, want 1", len(phases[1]))
	}

	// Check phase 2 (episodes)
	if len(phases[2]) != 1 {
		t.Errorf("Phase 2 item count = %d, want 1", len(phases[2]))
	}
}

func TestMetadataProgressModel_GetPhaseName(t *testing.T) {
	model := &MetadataProgressModel{}

	tests := []struct {
		phase int
		want  string
	}{
		{0, "Shows/Movies"},
		{1, "Seasons"},
		{2, "Episodes"},
		{3, "Unknown"},
	}

	for _, tt := range tests {
		got := model.getPhaseName(tt.phase)
		if got != tt.want {
			t.Errorf("getPhaseName(%d) = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestMetadataProgressModel_ProcessResults(t *testing.T) {
	model := &MetadataProgressModel{
		metadata: csmap.Create[string, *provider.Metadata](),
		errors:   make([]error, 0),
		resultCh: make(chan metadataResult, 2),
	}

	// Send some results
	testMeta := &provider.Metadata{
		Core: provider.CoreMetadata{
			Title: "Test Show",
			Year:  "2024",
		},
	}

	model.resultCh <- metadataResult{
		item: MetadataItem{
			Name: "Test Show",
			Key:  "show:Test Show:2024",
		},
		meta: testMeta,
		errs: nil,
	}

	// Send an error result
	model.resultCh <- metadataResult{
		item: MetadataItem{
			Name: "Error Show",
			Key:  "show:Error Show:2024",
		},
		meta: nil,
		errs: []error{&provider.ProviderError{Code: "NOT_FOUND"}},
	}

	close(model.resultCh)

	// Process results using the channel-based method
	done := make(chan bool)
	go model.processResults(model.resultCh, done)
	<-done

	// Check metadata was stored
	if got, exists := model.Get("show:Test Show:2024"); !exists {
		t.Error("Expected metadata not stored")
	} else if !cmp.Equal(got, testMeta) {
		t.Errorf("Stored metadata differs: %v", cmp.Diff(testMeta, got))
	}

	// Check processed count
	if model.processedItems != 2 {
		t.Errorf("processedItems = %d, want 2", model.processedItems)
	}
}

func TestMetadataProgressModel_WorkerConcurrency(t *testing.T) {
	// Create a test tree
	var nodes []*treeview.Node[treeview.FileInfo]
	for i := 0; i < 10; i++ {
		nodes = append(nodes, testNewDirNode("Show"))
	}
	tree := testNewTree(nodes...)

	cfg := &config.FormatConfig{
		EnableTMDBLookup: false,
	}
	model := NewMetadataProgressModel(tree, cfg)

	// Verify worker count is set
	if model.workerCount != 20 {
		t.Errorf("workerCount = %d, want 20", model.workerCount)
	}

	// Test that activeWorkers tracking works
	model.workersMu.Lock()
	model.activeWorkers = 3
	model.workersMu.Unlock()

	model.workersMu.RLock()
	workers := model.activeWorkers
	model.workersMu.RUnlock()

	if workers != 3 {
		t.Errorf("activeWorkers = %d, want 3", workers)
	}
}

func TestMetadataProgressModel_CollectMetadataItems(t *testing.T) {
	// Create a test tree with known structure
	showNode := testNewDirNode("The Office (2005)")

	season1 := testNewDirNode("Season 01")
	ep1 := testNewFileNode("S01E01 - Pilot.mkv")
	season1.AddChild(ep1)
	ep2 := testNewFileNode("S01E02 - Diversity Day.mkv")
	season1.AddChild(ep2)
	showNode.AddChild(season1)

	season2 := testNewDirNode("Season 02")
	showNode.AddChild(season2)

	// Add a movie
	movieNode := testNewFileNode("Inception (2010).mkv")

	tree := testNewTree(showNode, movieNode)

	cfg := &config.FormatConfig{}
	model := NewMetadataProgressModel(tree, cfg)

	items := model.collectMetadataItems()

	// Count items by phase
	phaseCounts := make(map[int]int)
	for _, item := range items {
		phaseCounts[item.Phase]++
	}

	// Should have:
	// Phase 0: 2 items (1 show, 1 movie)
	// Phase 1: 2 items (2 seasons)
	// Phase 2: 2 items (2 episodes)
	if phaseCounts[0] != 2 {
		t.Errorf("Phase 0 count = %d, want 2", phaseCounts[0])
	}
	if phaseCounts[1] != 2 {
		t.Errorf("Phase 1 count = %d, want 2", phaseCounts[1])
	}
	if phaseCounts[2] != 2 {
		t.Errorf("Phase 2 count = %d, want 2", phaseCounts[2])
	}

	// Check that items have correct metadata
	var foundShow, foundMovie bool
	for _, item := range items {
		if item.Name == "The Office" && item.Year == "2005" && item.Phase == 0 {
			foundShow = true
		}
		if item.Name == "Inception" && item.Year == "2010" && item.Phase == 0 && item.IsMovie {
			foundMovie = true
		}
	}

	if !foundShow {
		t.Error("TV show not correctly identified in items")
	}
	if !foundMovie {
		t.Error("Movie not correctly identified in items")
	}
}

func TestCountMetadataItems(t *testing.T) {
	// Create a test tree
	show1 := testNewDirNode("Show1 (2020)")
	season1 := testNewDirNode("Season 01")
	show1.AddChild(season1)
	movie1 := testNewFileNode("Movie1 (2021).mkv")

	tree := testNewTree(show1, movie1)

	count := countMetadataItems(tree, local.New())

	// Should count 1 show + 1 season + 1 movie = 3
	if count != 3 {
		t.Errorf("countMetadataItems() = %d, want 3", count)
	}
}

func TestMetadataProgressModel_ErrorHandling(t *testing.T) {
	model := &MetadataProgressModel{
		errors: []error{
			&provider.ProviderError{Code: "NOT_FOUND"},
			&provider.ProviderError{Code: "AUTH_FAILED"},
		},
	}

	// Test that critical errors are returned
	err := model.Err()
	var provErr *provider.ProviderError
	if !errors.As(err, &provErr) || provErr.Code != "AUTH_FAILED" {
		t.Errorf("Err() = %v, want ErrInvalidAPIKey", err)
	}

	// Test with only non-critical errors
	model.errors = []error{
		&provider.ProviderError{Code: "NOT_FOUND"},
		&provider.ProviderError{Code: "NOT_FOUND"},
	}
	model.err = nil

	err = model.Err()
	if err != nil {
		t.Errorf("Err() = %v, want nil for non-critical errors", err)
	}
}

func TestMetadataProgressModel_Initialization(t *testing.T) {
	tree := testNewTree()
	cfg := &config.FormatConfig{
		EnableTMDBLookup: true,
		TMDBAPIKey:       "test-key",
		TMDBLanguage:     "en-US",
	}

	model := NewMetadataProgressModel(tree, cfg)

	// Check initialization
	if model.tree != tree {
		t.Error("Tree not properly initialized")
	}
	if model.cfg != cfg {
		t.Error("Config not properly initialized")
	}
	if model.workerCount != 20 {
		t.Errorf("Worker count = %d, want 20", model.workerCount)
	}
	if model.workCh == nil {
		t.Error("Work channel not initialized")
	}
	if model.resultCh == nil {
		t.Error("Result channel not initialized")
	}
	if model.metadata == nil {
		t.Error("Metadata map not initialized")
	}
}

// Test concurrent metadata worker
func TestMetadataWorker_ProcessesItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := &MetadataProgressModel{
		workCh:       make(chan MetadataItem, 2),
		resultCh:     make(chan metadataResult, 2),
		msgCh:        make(chan tea.Msg, 10),
		metadata:     csmap.Create[string, *provider.Metadata](),
		currentPhase: "Test Phase",
		ctx:          ctx,
	}

	// Send test items
	testItem := MetadataItem{
		Name:    "Test Movie",
		Year:    "2024",
		IsMovie: true,
		Key:     "movie:Test Movie:2024",
	}

	go func() {
		model.workCh <- testItem
		close(model.workCh)
	}()

	// Run worker with timeout
	// (ctx and cancel already defined above)

	go func() {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			model.metadataWorker(model.workCh, model.resultCh, 0)
		}()
		wg.Wait()
	}()

	// Should receive a result
	select {
	case result := <-model.resultCh:
		if result.item.Key != testItem.Key {
			t.Errorf("Result key = %s, want %s", result.item.Key, testItem.Key)
		}
	case <-ctx.Done():
		t.Error("Timeout waiting for worker result")
	}
}
