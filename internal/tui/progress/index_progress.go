package progress

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/tui/theme"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Digital-Shane/treeview/v2"
)

// IndexProgressModel is a dedicated Bubble Tea model that displays a full-screen
// progress UI while the filesystem is being indexed into a tree. Once complete
// the caller can extract the constructed tree and transition to the main UI.
type IndexProgressModel struct {
	// config
	path       string
	cfg        IndexConfig
	totalRoots int

	// indexing progress
	processedRoots int
	filesIndexed   int
	indexingDone   bool

	// layout
	width  int
	height int

	// tree building + error
	tree *treeview.Tree[treeview.FileInfo]
	err  error

	// progress components
	progress progress.Model
	msgCh    chan tea.Msg
	rootPath string
	seen     map[string]struct{}

	theme theme.Theme
}

// indexProgressMsg updates counters.
type indexProgressMsg struct{}

// indexCompleteMsg signals completion.
type indexCompleteMsg struct{}

// IndexConfig carries the knobs required to build and annotate the tree.
type IndexConfig struct {
	MaxDepth    int
	IncludeDirs bool
	Filter      func(treeview.FileInfo) bool
}

// indexBuildParams are the resolved knobs that IndexProgressModel hands to a
// treeBuilderFunc. Exposing a struct instead of treeview.Option values keeps
// treeview's internal config private (v2 sealed those APIs) while still letting
// tests substitute a fake builder that can observe the filter and progress
// callback directly.
type indexBuildParams struct {
	MaxDepth     int
	TraversalCap int
	Filter       func(treeview.FileInfo) bool
	Progress     func(n int, node *treeview.Node[treeview.FileInfo])
}

type treeBuilderFunc func(ctx context.Context, path string, params indexBuildParams) (*treeview.Tree[treeview.FileInfo], error)

var indexProgressTreeBuilder treeBuilderFunc = defaultIndexProgressBuilder

func defaultIndexProgressBuilder(ctx context.Context, path string, params indexBuildParams) (*treeview.Tree[treeview.FileInfo], error) {
	opts := []treeview.Option[treeview.FileInfo]{
		treeview.WithMaxDepth[treeview.FileInfo](params.MaxDepth),
		treeview.WithTraversalCap[treeview.FileInfo](params.TraversalCap),
	}
	if params.Filter != nil {
		opts = append(opts, treeview.WithFilterFunc(params.Filter))
	}
	if params.Progress != nil {
		opts = append(opts, treeview.WithProgressCallback[treeview.FileInfo](params.Progress))
	}
	return treeview.NewTreeFromFileSystem(ctx, path, false, opts...)
}

// NewIndexProgressModel creates a model and pre computes root entry count.
func NewIndexProgressModel(path string, cfg IndexConfig, th theme.Theme) *IndexProgressModel {
	entries, _ := os.ReadDir(path)
	total := max(len(entries), 1)
	gradient := th.ProgressGradient()

	var p progress.Model
	if len(gradient) >= 2 {
		p = progress.New(progress.WithColors(lipgloss.Color(gradient[0]), lipgloss.Color(gradient[1])))
	} else {
		colors := th.Colors()
		p = progress.New(progress.WithColors(colors.Primary, colors.Accent))
	}
	p.SetWidth(50)

	rootPath, _ := filepath.Abs(path)
	return &IndexProgressModel{
		path:       path,
		cfg:        cfg,
		totalRoots: total,
		width:      80,
		height:     12,
		progress:   p,
		msgCh:      make(chan tea.Msg, 64),
		rootPath:   rootPath,
		seen:       make(map[string]struct{}),
		theme:      th,
	}
}

// Init kicks off asynchronous tree building.
func (m *IndexProgressModel) Init() tea.Cmd {
	go m.buildTreeAsync()
	return m.waitForMsg()
}

func (m *IndexProgressModel) waitForMsg() tea.Cmd { return func() tea.Msg { return <-m.msgCh } }

func (m *IndexProgressModel) buildTreeAsync() {
	filter := m.cfg.Filter
	if filter == nil {
		filter = m.defaultFilter
	}
	t, err := indexProgressTreeBuilder(context.Background(), m.path, indexBuildParams{
		MaxDepth:     m.cfg.MaxDepth,
		TraversalCap: 2000000,
		Filter:       filter,
		Progress:     m.onProgress,
	})
	m.tree = t
	m.err = err
	m.indexingDone = true
	m.msgCh <- indexCompleteMsg{}
}

func (m *IndexProgressModel) defaultFilter(fi treeview.FileInfo) bool {
	if fi.Name() == ".DS_Store" || strings.HasPrefix(fi.Name(), "._") {
		return false
	}
	if m.cfg.IncludeDirs {
		return fi.IsDir() || fi.FileInfo.Mode().IsRegular()
	}
	return fi.FileInfo.Mode().IsRegular()
}

func (m *IndexProgressModel) onProgress(_ int, n *treeview.Node[treeview.FileInfo]) {
	parent := filepath.Dir(n.Data().Path)
	if parent == m.rootPath {
		name := n.Data().Name()
		if _, ok := m.seen[name]; !ok {
			m.seen[name] = struct{}{}
			m.processedRoots++
		}
	}
	if !n.Data().IsDir() {
		m.filesIndexed++
	}
	select {
	case m.msgCh <- indexProgressMsg{}:
	default:
	}
}

// Update processes Bubble Tea messages.
func (m *IndexProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.SetWidth(msg.Width - 4)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case indexProgressMsg:
		ratio := math.Min(float64(m.processedRoots)/float64(m.totalRoots), 1)
		cmd := m.progress.SetPercent(ratio)
		// Always continue waiting so we can receive indexCompleteMsg.
		return m, tea.Batch(cmd, m.waitForMsg())
	case indexCompleteMsg:
		return m, tea.Quit
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the progress UI.
func (m *IndexProgressModel) View() tea.View {
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("Error: %v\n", m.err))
	}
	percent := 0
	if m.totalRoots > 0 {
		percent = 100 * m.processedRoots / m.totalRoots
	}

	infoLines := []string{fmt.Sprintf("Roots processed: %d/%d  Files indexed: %d", m.processedRoots, m.totalRoots, m.filesIndexed)}
	statsLines := []string{
		fmt.Sprintf("Root Directories: %d", m.totalRoots),
		fmt.Sprintf("Processed Roots: %d", m.processedRoots),
		fmt.Sprintf("Files Indexed: %d", m.filesIndexed),
		fmt.Sprintf("Progress: %d%%", percent),
	}

	sections := []string{
		m.theme.HeaderStyle().Width(m.width).Render("Indexing Media Library"),
		m.progress.View(),
	}

	if len(infoLines) > 0 {
		sections = append(sections, strings.Join(infoLines, "\n"))
	}

	if len(statsLines) > 0 {
		panel := m.theme.PanelStyle()
		panelWidth := m.width - panel.GetHorizontalFrameSize()
		if panelWidth < 0 {
			panelWidth = 0
		}
		sections = append(sections, panel.Width(panelWidth).Render(strings.Join(statsLines, "\n")))
	}

	status := m.theme.StatusBarStyle().Width(m.width).Render("Indexing... please wait")
	sections = append(sections, status)

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, sections...))
	v.AltScreen = true
	return v
}

// Tree returns the constructed tree.
func (m *IndexProgressModel) Tree() *treeview.Tree[treeview.FileInfo] { return m.tree }

// Err returns any build error.
func (m *IndexProgressModel) Err() error { return m.err }
