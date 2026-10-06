package cmd

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	"github.com/Digital-Shane/title-tidy/internal/tui/progress"
	"github.com/Digital-Shane/title-tidy/internal/tui/rename"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/Digital-Shane/treeview/v2"
	"github.com/spf13/cobra"
)

// CommandConfig defines the configuration for a media command
type CommandConfig struct {
	CommandName    string
	MaxDepth       int
	IncludeDirs    bool
	IsMovieMode    bool
	TreeAnnotator  func(*treeview.Tree[treeview.FileInfo], *config.FormatConfig, map[string]*provider.Metadata)
	TreePreprocess func([]*treeview.Node[treeview.FileInfo], *config.FormatConfig) []*treeview.Node[treeview.FileInfo]
}

// formatGeneratedName applies rename rules to the base name once, leaving file
// extensions and parent paths outside the replacement scope.
func formatGeneratedName(cfg *config.FormatConfig, generated, source string) string {
	name := core.PreserveExistingBracketTags(generated, source, cfg.PreserveExistingTags)
	return cfg.ApplyFilenameReplacements(name)
}

// RunMediaCommand executes the common logic for all media commands
func RunMediaCommand(cmd *cobra.Command, cmdConfig CommandConfig) error {
	formatConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if cmd.Flags().Changed("manual-search") {
		formatConfig.EnableManualSearch, err = cmd.Flags().GetBool("manual-search")
		if err != nil {
			return err
		}
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
	model := rename.NewRenameModel(t)
	model.IsMovieMode = cmdConfig.IsMovieMode
	model.LinkPath = linkPath
	model.IsLinkMode = linkPath != ""
	model.Command = cmdConfig.CommandName
	model.CommandArgs = os.Args[1:]

	// Execute in instant or interactive mode
	if instant {
		return executeInstantMode(model, cmdConfig.CommandName, os.Args[1:])
	}

	p := tea.NewProgram(model)
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
	indexConfig := progress.IndexConfig{
		MaxDepth:    cmdConfig.MaxDepth,
		IncludeDirs: cmdConfig.IncludeDirs,
		Filter:      createIndexFilter(),
	}

	t, err := runIndexing(indexConfig)
	if err != nil {
		return nil, err
	}

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
		treeview.WithFilterFunc(createMediaFilter(cmdConfig.IncludeDirs)),
		treeview.WithProvider(components.CreateRenameProvider()),
	)

	return t, nil
}

func runIndexing(indexConfig progress.IndexConfig) (*treeview.Tree[treeview.FileInfo], error) {
	idxModel := progress.NewIndexProgressModel(".", indexConfig, theme.Default())

	finalModel, err := tea.NewProgram(idxModel).Run()
	if err != nil {
		return nil, err
	}

	im, ok := finalModel.(*progress.IndexProgressModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type %T after indexing", finalModel)
	}

	return im.Tree(), nil
}

// fetchMetadataIfEnabled fetches metadata when any registry provider is enabled.
func fetchMetadataIfEnabled(t *treeview.Tree[treeview.FileInfo], formatConfig *config.FormatConfig) map[string]*provider.Metadata {
	_ = config.EnsureBuiltinProviders()
	shouldFetch := false
	for _, runtime := range formatConfig.ProviderRuntimeConfigs(nil) {
		if runtime.Enabled {
			shouldFetch = true
			break
		}
	}
	if !shouldFetch {
		return nil
	}

	metaModel := progress.NewMetadataProgressModel(t, formatConfig, theme.Default())
	finalMetaModel, err := tea.NewProgram(metaModel).Run()
	if err != nil {
		return nil
	}

	if mm, ok := finalMetaModel.(*progress.MetadataProgressModel); ok {
		return mm.Metadata()
	}
	return nil
}

// executeInstantMode runs the rename operation in non-interactive mode
func executeInstantMode(model *rename.RenameModel, commandName string, commandArgs []string) error {
	model.Command = commandName
	model.CommandArgs = commandArgs
	result := model.NewOperationEngine().RunToCompletion()
	if result.ErrorCount() > 0 {
		return fmt.Errorf("%d errors occurred during renaming", result.ErrorCount())
	}
	return nil
}

func createMediaFilter(includeDirectories bool) func(info treeview.FileInfo) bool {
	return func(info treeview.FileInfo) bool {
		if isHiddenMediaName(info.Name()) {
			return false
		}
		if includeDirectories && info.IsDir() {
			return true
		}
		return isSupportedMediaFile(info.Name())
	}
}

func createIndexFilter() func(info treeview.FileInfo) bool {
	return func(info treeview.FileInfo) bool {
		if isHiddenMediaName(info.Name()) {
			return false
		}
		if info.IsDir() {
			return true
		}
		return isSupportedMediaFile(info.Name())
	}
}

func isHiddenMediaName(name string) bool { return len(name) > 0 && name[0] == '.' }

func isSupportedMediaFile(name string) bool {
	return local.IsSubtitle(name) || local.IsVideo(name) || local.IsNFO(name) || local.IsImage(name)
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

		if noSample && local.IsSample(name) {
			shouldDelete = true
		}

		if !ni.Node.Data().IsDir() {
			if noNfo && local.IsNFO(name) {
				shouldDelete = true
			}

			if noImg && local.IsImage(name) {
				shouldDelete = true
			}
		}

		if shouldDelete {
			meta := core.EnsureMeta(ni.Node)
			meta.MarkedForDeletion = true
		}
	}
}

func createFormatContext(cfg *config.FormatConfig, showName, movieName string, year string, season, episode int, metadata *provider.Metadata) *config.FormatContext {
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

func fetchLocalMetadata(ctx context.Context, node *treeview.Node[treeview.FileInfo], mediaType provider.MediaType) (*provider.Metadata, error) {
	if node == nil {
		return nil, nil
	}

	return localProvider.Fetch(ctx, provider.FetchRequest{
		MediaType: mediaType,
		Name:      node.Name(),
		Extra: map[string]any{
			provider.FetchRequestExtraNode: node,
		},
	})
}

func detectLocalMedia(node *treeview.Node[treeview.FileInfo]) (provider.MediaType, *provider.Metadata, error) {
	if node == nil {
		var mt provider.MediaType
		return mt, nil, nil
	}

	return localProvider.Detect(node)
}
