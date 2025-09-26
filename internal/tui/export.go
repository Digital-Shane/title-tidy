package tui

import (
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	tuiconfig "github.com/Digital-Shane/title-tidy/internal/tui/config"
	"github.com/Digital-Shane/title-tidy/internal/tui/progress"
	"github.com/Digital-Shane/title-tidy/internal/tui/rename"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	"github.com/Digital-Shane/title-tidy/internal/tui/undo"
	"github.com/Digital-Shane/treeview"
)

// Type aliases to preserve the public API under the legacy tui package path.
type (
	RenameModel           = rename.RenameModel
	RenameCompleteMsg     = rename.RenameCompleteMsg
	UndoCompleteMsg       = undo.UndoCompleteMsg
	MetadataProgressModel = progress.MetadataProgressModel
	IndexProgressModel    = progress.IndexProgressModel
	IndexConfig           = progress.IndexConfig
	ConfigModel           = tuiconfig.Model
)

// NewWithRegistry delegates to the configuration UI constructor.
func NewWithRegistry(templateReg *config.TemplateRegistry) (*tuiconfig.Model, error) {
	return tuiconfig.NewWithRegistry(templateReg)
}

// NewRenameModel constructs the rename UI model.
func NewRenameModel(tree *treeview.Tree[treeview.FileInfo]) *rename.RenameModel {
	return rename.NewRenameModel(tree)
}

// NewUndoModel constructs the undo selection UI model.
func NewUndoModel(tree *treeview.Tree[log.SessionSummary]) *undo.UndoModel {
	return undo.NewUndoModel(tree)
}

// NewMetadataProgressModel constructs the metadata progress UI model.
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, th theme.Theme) *progress.MetadataProgressModel {
	return progress.NewMetadataProgressModel(tree, cfg, th)
}

// NewIndexProgressModel constructs the media indexing progress UI model.
func NewIndexProgressModel(path string, cfg progress.IndexConfig, th theme.Theme) *progress.IndexProgressModel {
	return progress.NewIndexProgressModel(path, cfg, th)
}

// CreateRenameProvider re-exports the shared rename tree provider.
func CreateRenameProvider() *treeview.DefaultNodeProvider[treeview.FileInfo] {
	return components.CreateRenameProvider()
}
