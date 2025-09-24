package tui

import (
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/tui/components"
	tuiconfig "github.com/Digital-Shane/title-tidy/internal/tui/config"
	tuiprogress "github.com/Digital-Shane/title-tidy/internal/tui/progress"
	tuirename "github.com/Digital-Shane/title-tidy/internal/tui/rename"
	"github.com/Digital-Shane/title-tidy/internal/tui/theme"
	tuiundo "github.com/Digital-Shane/title-tidy/internal/tui/undo"
	"github.com/Digital-Shane/treeview"
)

// Type aliases to preserve the public API under the legacy tui package path.
type (
	RenameModel           = tuirename.RenameModel
	RenameCompleteMsg     = tuirename.RenameCompleteMsg
	UndoCompleteMsg       = tuiundo.UndoCompleteMsg
	MetadataProgressModel = tuiprogress.MetadataProgressModel
	IndexProgressModel    = tuiprogress.IndexProgressModel
	IndexConfig           = tuiprogress.IndexConfig
	ConfigModel           = tuiconfig.Model
)

// NewWithRegistry delegates to the configuration UI constructor.
func NewWithRegistry(templateReg *config.TemplateRegistry) (*tuiconfig.Model, error) {
	return tuiconfig.NewWithRegistry(templateReg)
}

// NewRenameModel constructs the rename UI model.
func NewRenameModel(tree *treeview.Tree[treeview.FileInfo]) *tuirename.RenameModel {
	return tuirename.NewRenameModel(tree)
}

// NewUndoModel constructs the undo selection UI model.
func NewUndoModel(tree *treeview.Tree[log.SessionSummary]) *tuiundo.UndoModel {
	return tuiundo.NewUndoModel(tree)
}

// NewMetadataProgressModel constructs the metadata progress UI model.
func NewMetadataProgressModel(tree *treeview.Tree[treeview.FileInfo], cfg *config.FormatConfig, th theme.Theme) *tuiprogress.MetadataProgressModel {
	return tuiprogress.NewMetadataProgressModel(tree, cfg, th)
}

// NewIndexProgressModel constructs the media indexing progress UI model.
func NewIndexProgressModel(path string, cfg tuiprogress.IndexConfig, th theme.Theme) *tuiprogress.IndexProgressModel {
	return tuiprogress.NewIndexProgressModel(path, cfg, th)
}

// CreateRenameProvider re-exports the shared rename tree provider.
func CreateRenameProvider() *treeview.DefaultNodeProvider[treeview.FileInfo] {
	return components.CreateRenameProvider()
}
