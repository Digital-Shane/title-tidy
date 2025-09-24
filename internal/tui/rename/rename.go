package rename

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbletea"
	"iter"
)

// RenameCompleteMsg is emitted once all queued operations finish running.
type RenameCompleteMsg struct{ successCount, errorCount int }

// SuccessCount returns the number of successful operations.
func (r RenameCompleteMsg) SuccessCount() int { return r.successCount }

// ErrorCount returns the number of failed operations.
func (r RenameCompleteMsg) ErrorCount() int { return r.errorCount }

// OperationProgressMsg reports incremental progress while the operation engine
// processes queued actions.
type OperationProgressMsg struct {
	Progress OperationProgress
}

// OperationProgress captures aggregate progress information for the current
// batch run. It is intentionally value-based so the UI can cache the snapshot.
type OperationProgress struct {
	OverallCompleted int
	OverallTotal     int
	SuccessCount     int
	ErrorCount       int
}

// OperationKind classifies the concrete action being executed.
type OperationKind string

const (
	OperationVirtualDir OperationKind = "virtual-dir"
	OperationDelete     OperationKind = "delete"
	OperationRename     OperationKind = "rename"
	OperationEnsureDir  OperationKind = "ensure-dir"
	OperationLink       OperationKind = "link"
)

// OperationFunctions bundles the filesystem callbacks used by the engine. Tests
// and dry-run executions can override any subset of these handlers.
type OperationFunctions struct {
	CreateVirtualDir     func(*treeview.Node[treeview.FileInfo], *core.MediaMeta) (int, []error)
	LinkVirtualDir       func(*treeview.Node[treeview.FileInfo], *core.MediaMeta, string) (int, []error)
	DeleteMarkedNode     func(*treeview.Node[treeview.FileInfo], *core.MediaMeta) error
	RenameRegular        func(*treeview.Node[treeview.FileInfo], *core.MediaMeta) (bool, error)
	LinkRegular          func(*treeview.Node[treeview.FileInfo], *core.MediaMeta) (bool, error)
	EnsureDestinationDir func(string, *core.MediaMeta) error
	StartSession         func(string, []string) error
	EndSession           func() error
}

func (f OperationFunctions) withDefaults() OperationFunctions {
	if f.CreateVirtualDir == nil {
		f.CreateVirtualDir = core.CreateVirtualDir
	}
	if f.LinkVirtualDir == nil {
		f.LinkVirtualDir = core.LinkVirtualDir
	}
	if f.DeleteMarkedNode == nil {
		f.DeleteMarkedNode = core.DeleteMarkedNode
	}
	if f.RenameRegular == nil {
		f.RenameRegular = core.RenameRegular
	}
	if f.LinkRegular == nil {
		f.LinkRegular = core.LinkRegular
	}
	if f.EnsureDestinationDir == nil {
		f.EnsureDestinationDir = core.EnsureDestinationDir
	}
	if f.StartSession == nil {
		f.StartSession = log.StartSession
	}
	if f.EndSession == nil {
		f.EndSession = log.EndSession
	}
	return f
}

// OperationConfig configures the behaviour of the operation engine.
type OperationConfig struct {
	Tree        *treeview.Tree[treeview.FileInfo]
	IsLinkMode  bool
	LinkPath    string
	Command     string
	CommandArgs []string
	Functions   OperationFunctions
	Stderr      io.Writer
}

type operation struct {
	kind OperationKind
	node *treeview.Node[treeview.FileInfo]
	meta *core.MediaMeta
}

// OperationEngine walks the annotated tree, queuing and executing rename/link
// operations one at a time so the TUI can remain responsive.
type OperationEngine struct {
	cfg            OperationConfig
	fns            OperationFunctions
	ops            []operation
	idx            int
	successes      int
	failures       int
	startedLogging bool
	finished       bool
	stderr         io.Writer
}

// NewOperationEngine builds a queue-driven executor for rename/link operations.
func NewOperationEngine(cfg OperationConfig) *OperationEngine {
	cfg.Functions = cfg.Functions.withDefaults()
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	engine := &OperationEngine{
		cfg:    cfg,
		fns:    cfg.Functions,
		stderr: cfg.Stderr,
	}
	engine.collectOperations()
	return engine
}

// TotalOperations returns the total number of queued operations.
func (e *OperationEngine) TotalOperations() int { return len(e.ops) }

// ProcessNext runs the next queued operation and returns a Bubble Tea message.
func (e *OperationEngine) ProcessNext() tea.Msg {
	if e.finished {
		return RenameCompleteMsg{successCount: e.successes, errorCount: e.failures}
	}
	e.ensureLoggingStarted()

	if e.idx >= len(e.ops) {
		e.finishLogging()
		e.finished = true
		return RenameCompleteMsg{successCount: e.successes, errorCount: e.failures}
	}

	op := e.ops[e.idx]
	success, failure := e.run(op)
	e.successes += success
	e.failures += failure
	e.idx++

	progress := OperationProgress{
		OverallCompleted: e.idx,
		OverallTotal:     len(e.ops),
		SuccessCount:     e.successes,
		ErrorCount:       e.failures,
	}

	// If that was the final operation, the caller will invoke ProcessNext again
	// to receive the completion message. Avoid closing the log session here so
	// the progress snapshot reaches the UI first.
	return OperationProgressMsg{Progress: progress}
}

// ProcessNextCmd returns a Bubble Tea command that advances the operation
// engine by one step.
func (e *OperationEngine) ProcessNextCmd() tea.Cmd {
	return func() tea.Msg {
		return e.ProcessNext()
	}
}

// RunToCompletion executes every queued operation synchronously.
func (e *OperationEngine) RunToCompletion() RenameCompleteMsg {
	for {
		msg := e.ProcessNext()
		if complete, ok := msg.(RenameCompleteMsg); ok {
			return complete
		}
	}
}

func (e *OperationEngine) ensureLoggingStarted() {
	if e.startedLogging {
		return
	}
	e.startedLogging = true
	if err := e.fns.StartSession(e.cfg.Command, e.cfg.CommandArgs); err != nil {
		fmt.Fprintf(e.stderr, "Warning: Failed to start operation log: %v\n", err)
	}
}

func (e *OperationEngine) finishLogging() {
	if !e.startedLogging || e.finished {
		return
	}
	if err := e.fns.EndSession(); err != nil {
		fmt.Fprintf(e.stderr, "Warning: Failed to save operation log: %v\n", err)
	}
	e.finished = true
}

func (e *OperationEngine) run(op operation) (successes, failures int) {
	switch op.kind {
	case OperationVirtualDir:
		if e.cfg.IsLinkMode {
			s, errs := e.fns.LinkVirtualDir(op.node, op.meta, e.cfg.LinkPath)
			return s, len(errs)
		}
		s, errs := e.fns.CreateVirtualDir(op.node, op.meta)
		return s, len(errs)

	case OperationDelete:
		if err := e.fns.DeleteMarkedNode(op.node, op.meta); err != nil {
			return 0, 1
		}
		return 1, 0

	case OperationRename:
		renamed, err := e.fns.RenameRegular(op.node, op.meta)
		if err != nil {
			return 0, 1
		}
		if renamed {
			return 1, 0
		}
		return 0, 0

	case OperationEnsureDir:
		if err := e.fns.EnsureDestinationDir(op.meta.DestinationPath, op.meta); err != nil {
			return 0, 1
		}
		return 1, 0

	case OperationLink:
		linked, err := e.fns.LinkRegular(op.node, op.meta)
		if err != nil {
			return 0, 1
		}
		if linked {
			return 1, 0
		}
		return 0, 0
	}
	return 0, 0
}

func (e *OperationEngine) collectOperations() {
	if e.cfg.Tree == nil {
		return
	}

	ctx := context.Background()

	// Phase 1: virtual directories
	for info := range e.cfg.Tree.All(ctx) {
		node := info.Node
		meta := core.GetMeta(node)
		if meta == nil {
			continue
		}
		if meta.NeedsDirectory && meta.IsVirtual {
			e.appendOperation(operation{
				kind: OperationVirtualDir,
				node: node,
				meta: meta,
			})
		}
	}

	// Phase 2: deletions (rename mode only)
	if !e.cfg.IsLinkMode {
		for info := range e.cfg.Tree.All(ctx) {
			node := info.Node
			meta := core.GetMeta(node)
			if meta == nil || !meta.MarkedForDeletion {
				continue
			}
			e.appendOperation(operation{
				kind: OperationDelete,
				node: node,
				meta: meta,
			})
		}
	}

	// Phase 3: standard rename/link operations
	var iterator iter.Seq2[treeview.NodeInfo[treeview.FileInfo], error]
	if e.cfg.IsLinkMode {
		iterator = e.cfg.Tree.BreadthFirst(ctx)
	} else {
		iterator = e.cfg.Tree.AllBottomUp(ctx)
	}

	for info := range iterator {
		node := info.Node
		meta := core.GetMeta(node)
		if meta == nil {
			continue
		}
		if meta.MarkedForDeletion || (meta.NeedsDirectory && meta.IsVirtual) {
			continue
		}
		if parent := node.Parent(); parent != nil {
			if pm := core.GetMeta(parent); pm != nil && pm.IsVirtual {
				continue
			}
		}

		if e.cfg.IsLinkMode {
			if meta.DestinationPath == "" {
				continue
			}
			kind := OperationLink
			if node.Data().IsDir() {
				kind = OperationEnsureDir
			}
			e.appendOperation(operation{
				kind: kind,
				node: node,
				meta: meta,
			})
			continue
		}

		if meta.NewName == "" || meta.NewName == node.Name() {
			continue
		}
		e.appendOperation(operation{
			kind: OperationRename,
			node: node,
			meta: meta,
		})
	}
}

func (e *OperationEngine) appendOperation(op operation) {
	e.ops = append(e.ops, op)
}
