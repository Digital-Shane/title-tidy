package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/media"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/tui"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// CommandConfig defines the configuration for a media command
type CommandConfig struct {
	CommandName    string
	MaxDepth       int
	IncludeDirs    bool
	IsMovieMode    bool
	TreeAnnotator  func(*treeview.Tree[treeview.FileInfo], *config.FormatConfig, map[string]*provider.EnrichedMetadata)
	TreePreprocess func([]*treeview.Node[treeview.FileInfo], *config.FormatConfig) []*treeview.Node[treeview.FileInfo]
}

// RunMediaCommand executes the common logic for all media commands
func RunMediaCommand(cmd *cobra.Command, cmdConfig CommandConfig) error {
	formatConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateLinkDestination(linkPath); err != nil {
		return err
	}

	log.Initialize(formatConfig.EnableLogging, formatConfig.LogRetentionDays)

	// Index files
	t, err := indexFiles(formatConfig, cmdConfig)
	if err != nil {
		return err
	}

	// Fetch metadata if enabled
	metadata := fetchMetadataIfEnabled(t, formatConfig)

	// Annotate tree with rename information
	cmdConfig.TreeAnnotator(t, formatConfig, metadata)
	markFilesForDeletion(t)

	// Create and configure the rename model
	model := tui.NewRenameModel(t)
	model.IsMovieMode = cmdConfig.IsMovieMode
	model.LinkPath = linkPath
	model.IsLinkMode = linkPath != ""
	model.Command = cmdConfig.CommandName
	model.CommandArgs = os.Args[1:]

	// Execute in instant or interactive mode
	if instant {
		return executeInstantMode(model, cmdConfig.CommandName, os.Args[1:])
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// validateLinkDestination checks if the link destination is valid
func validateLinkDestination(linkPath string) error {
	if linkPath == "" {
		return nil
	}

	info, err := os.Stat(linkPath)
	if err != nil {
		return fmt.Errorf("link destination does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("link destination must be a directory")
	}
	return nil
}

// indexFiles performs file indexing and tree creation
func indexFiles(formatConfig *config.FormatConfig, cmdConfig CommandConfig) (*treeview.Tree[treeview.FileInfo], error) {
	idxModel := tui.NewIndexProgressModel(".", tui.IndexConfig{
		MaxDepth:    cmdConfig.MaxDepth,
		IncludeDirs: cmdConfig.IncludeDirs,
		Filter:      createMediaFilter(cmdConfig.IncludeDirs),
	})

	finalModel, err := tea.NewProgram(idxModel, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}

	im, ok := finalModel.(*tui.IndexProgressModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type %T after indexing", finalModel)
	}

	t := im.Tree()
	if t == nil {
		return nil, fmt.Errorf("indexing produced no tree")
	}

	nodes := unwrapRoot(t)

	// Apply preprocessing if provided
	if cmdConfig.TreePreprocess != nil {
		nodes = cmdConfig.TreePreprocess(nodes, formatConfig)
	}

	t = treeview.NewTree(nodes,
		treeview.WithExpandAll[treeview.FileInfo](),
		treeview.WithProvider(tui.CreateRenameProvider()),
	)

	return t, nil
}

// fetchMetadataIfEnabled fetches TMDB metadata if configured
func fetchMetadataIfEnabled(t *treeview.Tree[treeview.FileInfo], formatConfig *config.FormatConfig) map[string]*provider.EnrichedMetadata {
	if !formatConfig.EnableTMDBLookup || formatConfig.TMDBAPIKey == "" {
		return nil
	}

	metaModel := tui.NewMetadataProgressModel(t, formatConfig)
	finalMetaModel, err := tea.NewProgram(metaModel, tea.WithAltScreen()).Run()
	if err != nil {
		return nil
	}

	if mm, ok := finalMetaModel.(*tui.MetadataProgressModel); ok {
		return mm.Metadata()
	}
	return nil
}

// executeInstantMode runs the rename operation in non-interactive mode
func executeInstantMode(model *tui.RenameModel, commandName string, commandArgs []string) error {
	if err := log.StartSession(commandName, commandArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start operation log: %v\n", err)
	}

	cmd := model.PerformRenames()
	if cmd != nil {
		msg := cmd()
		if result, ok := msg.(tui.RenameCompleteMsg); ok && result.ErrorCount() > 0 {
			log.EndSession()
			return fmt.Errorf("%d errors occurred during renaming", result.ErrorCount())
		}
	}

	if err := log.EndSession(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save operation log: %v\n", err)
	}
	return nil
}

func createMediaFilter(includeDirectories bool) func(info treeview.FileInfo) bool {
	return func(info treeview.FileInfo) bool {
		if len(info.Name()) > 0 && info.Name()[0] == '.' {
			return false
		}
		if includeDirectories && info.IsDir() {
			return true
		}
		return media.IsSubtitle(info.Name()) || media.IsVideo(info.Name()) || media.IsNFO(info.Name()) || media.IsImage(info.Name())
	}
}

func unwrapRoot(t *treeview.Tree[treeview.FileInfo]) []*treeview.Node[treeview.FileInfo] {
	ns := t.Nodes()
	if len(ns) == 1 && ns[0].Data().IsDir() {
		children := ns[0].Children()
		cloned := make([]*treeview.Node[treeview.FileInfo], len(children))
		for i, child := range children {
			clone := treeview.NewNodeClone(child)
			clone.SetChildren(child.Children())
			cloned[i] = clone
		}
		return cloned
	}
	return ns
}

func markFilesForDeletion(t *treeview.Tree[treeview.FileInfo]) {
	if !noNfo && !noImg && !noSample {
		return
	}

	for ni := range t.All(context.Background()) {
		name := ni.Node.Name()
		shouldDelete := false

		if noSample && media.IsSample(name) {
			shouldDelete = true
		}

		if !ni.Node.Data().IsDir() {
			if noNfo && media.IsNFO(name) {
				shouldDelete = true
			}

			if noImg && media.IsImage(name) {
				shouldDelete = true
			}
		}

		if shouldDelete {
			meta := core.EnsureMeta(ni.Node)
			meta.MarkedForDeletion = true
		}
	}
}

func createFormatContext(cfg *config.FormatConfig, showName, movieName string, year string, season, episode int, metadata *provider.EnrichedMetadata) *config.FormatContext {
	return &config.FormatContext{
		ShowName:  showName,
		MovieName: movieName,
		Year:      year,
		Season:    season,
		Episode:   episode,
		Metadata:  metadata,
		Config:    cfg,
	}
}
