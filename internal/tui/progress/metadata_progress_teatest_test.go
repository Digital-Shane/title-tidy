package progress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/google/go-cmp/cmp"
)

type metadataFakeFileInfo struct {
	name string
	dir  bool
}

func (fi metadataFakeFileInfo) Name() string { return fi.name }
func (fi metadataFakeFileInfo) Size() int64  { return 0 }
func (fi metadataFakeFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (fi metadataFakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi metadataFakeFileInfo) IsDir() bool        { return fi.dir }
func (fi metadataFakeFileInfo) Sys() any           { return nil }

func newMetadataFileNode(id, name, path string, dir bool) *treeview.Node[treeview.FileInfo] {
	data := treeview.FileInfo{
		FileInfo: metadataFakeFileInfo{name: name, dir: dir},
		Path:     path,
	}
	return treeview.NewNode(id, name, data)
}

func newMetadataTestTree() *treeview.Tree[treeview.FileInfo] {
	movie := newMetadataFileNode("movie", "Sample Movie (2021).mkv", "/library/Sample Movie (2021).mkv", false)
	show := newMetadataFileNode("show", "Test Show (2020)", "/library/Test Show (2020)", true)
	season := newMetadataFileNode("season", "Season 01", "/library/Test Show (2020)/Season 01", true)
	episode := newMetadataFileNode("episode", "Test Show (2020) - S01E01 - Pilot.mkv", "/library/Test Show (2020)/Season 01/Test Show (2020) - S01E01 - Pilot.mkv", false)

	season.SetChildren([]*treeview.Node[treeview.FileInfo]{episode})
	show.SetChildren([]*treeview.Node[treeview.FileInfo]{season})

	tree := &treeview.Tree[treeview.FileInfo]{}
	tree.SetNodes([]*treeview.Node[treeview.FileInfo]{movie, show})

	return tree
}

func newSingleMovieTree() *treeview.Tree[treeview.FileInfo] {
	movie := newMetadataFileNode("movie", "Manual Movie (2022).mkv", "/library/Manual Movie (2022).mkv", false)
	tree := &treeview.Tree[treeview.FileInfo]{}
	tree.SetNodes([]*treeview.Node[treeview.FileInfo]{movie})
	return tree
}

type metadataFakeProvider struct {
	name      string
	fetchFunc func(provider.FetchRequest) (*provider.Metadata, error)
}

func newMetadataFakeProvider(name string, fetch func(provider.FetchRequest) (*provider.Metadata, error)) *metadataFakeProvider {
	return &metadataFakeProvider{name: name, fetchFunc: fetch}
}

func configureTestEngine(model *MetadataProgressModel, tree *treeview.Tree[treeview.FileInfo], prov provider.Provider, workerCount int) {
	cache := true
	cfg := core.MetadataEngineConfig{
		Tree:        tree,
		WorkerCount: workerCount,
		Providers: core.MetadataProvidersConfig{
			TMDB: core.TMDBProviderConfig{
				Enabled:      true,
				APIKey:       "test-key",
				Language:     "en-US",
				CacheEnabled: &cache,
				Provider:     prov,
			},
		},
	}
	engine := core.NewMetadataEngine(cfg)
	model.engine = engine
	model.summary = engine.SummarySnapshot()
	model.shouldRun = true
}

func (p *metadataFakeProvider) Name() string { return p.name }

func (p *metadataFakeProvider) Description() string { return p.name }

func (p *metadataFakeProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{
		MediaTypes: []provider.MediaType{
			provider.MediaTypeMovie,
			provider.MediaTypeShow,
			provider.MediaTypeSeason,
			provider.MediaTypeEpisode,
		},
	}
}

func (p *metadataFakeProvider) SupportedVariables() []provider.TemplateVariable { return nil }

func (p *metadataFakeProvider) Configure(map[string]interface{}) error { return nil }

func (p *metadataFakeProvider) ConfigSchema() provider.ConfigSchema { return provider.ConfigSchema{} }

func (p *metadataFakeProvider) Fetch(ctx context.Context, req provider.FetchRequest) (*provider.Metadata, error) {
	if p.fetchFunc == nil {
		return nil, nil
	}
	return p.fetchFunc(req)
}

func newMetadataProgressTestModel(t *testing.T, model *MetadataProgressModel, opts ...teatest.TestOption) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, model, opts...)
	t.Cleanup(func() {
		_ = tm.Quit()
	})
	return tm
}

func finalMetadataProgressModel(t *testing.T, tm *teatest.TestModel) *MetadataProgressModel {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	model, ok := final.(*MetadataProgressModel)
	if !ok {
		t.Fatalf("Final model type = %T, want *MetadataProgressModel", final)
	}
	return model
}

func sortedKeys(m map[string]*provider.Metadata) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestMetadataProgressCompletesAndStoresResults(t *testing.T) {
	tree := newMetadataTestTree()

	cfg := &config.FormatConfig{
		EnableTMDBLookup: false,
		EnableOMDBLookup: false,
		EnableFFProbe:    false,
		TMDBWorkerCount:  2,
	}

	model := NewMetadataProgressModel(tree, cfg, theme.Default())
	configureTestEngine(model, tree, newMetadataFakeProvider("fakeTMDB", func(req provider.FetchRequest) (*provider.Metadata, error) {
		meta := &provider.Metadata{
			Core: provider.CoreMetadata{
				Title:      req.Name,
				Year:       req.Year,
				MediaType:  req.MediaType,
				SeasonNum:  req.Season,
				EpisodeNum: req.Episode,
			},
			Extended: make(map[string]interface{}),
			Sources:  map[string]string{"provider": "fakeTMDB"},
			IDs:      map[string]string{"kind": string(req.MediaType)},
		}
		return meta, nil
	}), 2)
	tm := newMetadataProgressTestModel(t, model, teatest.WithInitialTermSize(100, 24))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	finalModel := finalMetadataProgressModel(t, tm)
	output := finalOutput(t, tm)

	if diff := cmp.Diff(finalModel.summary.TotalItems, finalModel.summary.ProcessedItems); diff != "" {
		t.Errorf("processedItems mismatch (-want +got):\n%s", diff)
	}

	if !finalModel.done {
		t.Error("done = false, want true after completing all phases")
	}

	metadata := finalModel.Metadata()
	wantKeys := []string{
		provider.GenerateMetadataKey("movie", "Sample Movie", "2021", 0, 0),
		provider.GenerateMetadataKey("show", "Test Show", "2020", 0, 0),
		provider.GenerateMetadataKey("season", "Test Show", "2020", 1, 0),
		provider.GenerateMetadataKey("episode", "Test Show", "2020", 1, 1),
	}
	sortedWant := append([]string(nil), wantKeys...)
	sort.Strings(sortedWant)
	gotKeys := sortedKeys(metadata)
	if diff := cmp.Diff(sortedWant, gotKeys); diff != "" {
		t.Errorf("Metadata keys mismatch (-want +got):\n%s", diff)
	}

	for _, key := range wantKeys {
		item, ok := metadata[key]
		if !ok {
			t.Fatalf("Metadata() missing key %q", key)
		}
		if item == nil {
			t.Fatalf("Metadata()[%q] = nil, want metadata", key)
		}
		if item.Core.Title == "" {
			t.Errorf("Metadata()[%q].Core.Title empty, want populated", key)
		}
	}

	if finalModel.Err() != nil {
		t.Errorf("Err() = %v, want nil", finalModel.Err())
	}

	for _, want := range []string{
		"Fetching Metadata (fakeTMDB)",
		"Total Items: 4",
		"Processed: 4",
		"Progress: 100%",
	} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("final output missing %q; output = %q", want, output)
		}
	}
}

func TestMetadataProgressQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyType
	}{
		{name: "ctrl_c", key: tea.KeyCtrlC},
		{name: "esc", key: tea.KeyEsc},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMetadataTestTree()

			cfg := &config.FormatConfig{TMDBWorkerCount: 1}
			model := NewMetadataProgressModel(tree, cfg, theme.Default())

			ready := make(chan struct{})
			release := make(chan struct{})
			var readyOnce sync.Once
			var releaseOnce sync.Once

			configureTestEngine(model, tree, newMetadataFakeProvider("fakeTMDB", func(provider.FetchRequest) (*provider.Metadata, error) {
				readyOnce.Do(func() { close(ready) })
				<-release
				return nil, context.Canceled
			}), 1)
			releaseClose := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseClose)

			tm := newMetadataProgressTestModel(t, model)
			<-ready
			tm.Send(tea.KeyMsg{Type: tc.key})
			releaseClose()

			tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
			finalModel := finalMetadataProgressModel(t, tm)

			if finalModel.done {
				t.Error("done = true, want false when exiting via quit key")
			}

			if finalModel.Err() != nil {
				t.Errorf("Err() = %v, want nil on quit", finalModel.Err())
			}

			if finalModel.summary.ProcessedItems > finalModel.summary.TotalItems {
				t.Errorf("processedItems = %d exceeds totalItems = %d", finalModel.summary.ProcessedItems, finalModel.summary.TotalItems)
			}
		})
	}
}

func TestMetadataProgressWindowResize(t *testing.T) {
	tree := newMetadataTestTree()

	cfg := &config.FormatConfig{TMDBWorkerCount: 1}
	model := NewMetadataProgressModel(tree, cfg, theme.Default())

	ready := make(chan struct{})
	release := make(chan struct{})
	var readyOnce sync.Once
	var releaseOnce sync.Once

	configureTestEngine(model, tree, newMetadataFakeProvider("fakeTMDB", func(req provider.FetchRequest) (*provider.Metadata, error) {
		readyOnce.Do(func() { close(ready) })
		<-release
		meta := &provider.Metadata{
			Core: provider.CoreMetadata{
				Title:      req.Name,
				Year:       req.Year,
				MediaType:  req.MediaType,
				SeasonNum:  req.Season,
				EpisodeNum: req.Episode,
			},
		}
		return meta, nil
	}), 1)
	releaseClose := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseClose)

	tm := newMetadataProgressTestModel(t, model, teatest.WithInitialTermSize(80, 20))
	<-ready

	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	releaseClose()

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	finalModel := finalMetadataProgressModel(t, tm)

	if diff := cmp.Diff(120, finalModel.width); diff != "" {
		t.Errorf("width mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(40, finalModel.height); diff != "" {
		t.Errorf("height mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(116, finalModel.progress.Width); diff != "" {
		t.Errorf("progress width mismatch (-want +got):\n%s", diff)
	}
}

func TestMetadataProgressDisplaysErrorsAndExposesErr(t *testing.T) {
	tree := newMetadataTestTree()

	cfg := &config.FormatConfig{TMDBWorkerCount: 1}
	model := NewMetadataProgressModel(tree, cfg, theme.Default())
	configureTestEngine(model, tree, newMetadataFakeProvider("fakeTMDB", func(provider.FetchRequest) (*provider.Metadata, error) {
		return nil, &provider.ProviderError{Provider: "fakeTMDB", Code: "AUTH_FAILED", Message: "bad key", Retry: false}
	}), 1)
	tm := newMetadataProgressTestModel(t, model, teatest.WithInitialTermSize(90, 20))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	finalModel := finalMetadataProgressModel(t, tm)
	output := finalOutput(t, tm)

	wantErrors := finalModel.summary.TotalItems
	if diff := cmp.Diff(wantErrors, len(finalModel.errors)); diff != "" {
		t.Errorf("errors length mismatch (-want +got):\n%s", diff)
	}

	var providerErr *provider.ProviderError
	if !errors.As(finalModel.Err(), &providerErr) {
		t.Fatalf("Err() = %v, want *provider.ProviderError", finalModel.Err())
	}
	if diff := cmp.Diff("AUTH_FAILED", providerErr.Code); diff != "" {
		t.Errorf("Provider error code mismatch (-want +got):\n%s", diff)
	}

	metadata := finalModel.Metadata()
	if diff := cmp.Diff(0, len(metadata)); diff != "" {
		t.Errorf("metadata length mismatch (-want +got):\n%s", diff)
	}

	for _, want := range []string{
		fmt.Sprintf("Errors: %d", wantErrors),
		"Sample Movie: bad key",
	} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("final output missing %q; output = %q", want, output)
		}
	}
}

func TestMetadataProgressManualRetryResolvesFailure(t *testing.T) {
	tree := newSingleMovieTree()

	cfg := &config.FormatConfig{TMDBWorkerCount: 1}
	model := NewMetadataProgressModel(tree, cfg, theme.Default())
	provider := newMetadataFakeProvider("fakeTMDB", func(req provider.FetchRequest) (*provider.Metadata, error) {
		if req.Name == "Manual k Success" {
			return &provider.Metadata{Core: provider.CoreMetadata{Title: req.Name, MediaType: req.MediaType}}, nil
		}
		return nil, &provider.ProviderError{Provider: "fakeTMDB", Code: "NOT_FOUND", Message: fmt.Sprintf("no results for %s", req.Name), Retry: false}
	})
	configureTestEngine(model, tree, provider, 1)

	tm := newMetadataProgressTestModel(t, model, teatest.WithInitialTermSize(90, 20))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Resolve Metadata Search"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	tm.Type("Manual k Success")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	finalModel := finalMetadataProgressModel(t, tm)

	if !finalModel.done {
		t.Error("done = false, want true after resolving failures")
	}
	if finalModel.manualActive {
		t.Error("manualActive = true, want false after resolution")
	}
	if diff := cmp.Diff(0, len(finalModel.failures)); diff != "" {
		t.Errorf("failures length mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(0, finalModel.summary.ErrorCount); diff != "" {
		t.Errorf("summary.ErrorCount mismatch (-want +got):\n%s", diff)
	}

	metadata := finalModel.Metadata()
	if len(metadata) == 0 {
		t.Fatal("Metadata() returned no entries, want manual result")
	}
	found := false
	for _, meta := range metadata {
		if meta != nil && meta.Core.Title == "Manual k Success" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Metadata() missing manual title 'Manual k Success'; metadata = %+v", metadata)
	}
}

func TestMetadataProgressManualSkipAllowsContinue(t *testing.T) {
	tree := newSingleMovieTree()

	cfg := &config.FormatConfig{TMDBWorkerCount: 1}
	model := NewMetadataProgressModel(tree, cfg, theme.Default())
	provider := newMetadataFakeProvider("fakeTMDB", func(req provider.FetchRequest) (*provider.Metadata, error) {
		return nil, &provider.ProviderError{Provider: "fakeTMDB", Code: "NOT_FOUND", Message: fmt.Sprintf("no results for %s", req.Name), Retry: false}
	})
	configureTestEngine(model, tree, provider, 1)

	tm := newMetadataProgressTestModel(t, model, teatest.WithInitialTermSize(90, 20))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Resolve Metadata Search"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	finalModel := finalMetadataProgressModel(t, tm)

	if !finalModel.manualSkipped {
		t.Error("manualSkipped = false, want true after ctrl+s")
	}
	if finalModel.manualActive {
		t.Error("manualActive = true, want false after skip")
	}
	if diff := cmp.Diff(finalModel.summary.ErrorCount, len(finalModel.failures)); diff != "" {
		t.Errorf("error count mismatch (-want +got):\n%s", diff)
	}
	if finalModel.Err() != nil {
		t.Errorf("Err() = %v, want nil when skipping", finalModel.Err())
	}
}
