package local

import (
	"path/filepath"
	"strings"

	"github.com/Digital-Shane/treeview/v2"
)

// ParseContext captures precomputed details about a media item name and its tree node.
// Parsers use it to avoid re-running basic normalization work like extension removal
// and parent traversal.
type ParseContext struct {
	Name      string
	BaseName  string
	Extension string
	Node      *treeview.Node[treeview.FileInfo]
	IsFile    bool
	IsDir     bool
}

// NewParseContext builds a ParseContext from the raw name and optional node.
func NewParseContext(name string, node *treeview.Node[treeview.FileInfo]) ParseContext {
	ctx := ParseContext{
		Name: name,
		Node: node,
	}

	if node != nil {
		data := node.Data()
		ctx.IsDir = data.IsDir()
		ctx.IsFile = !ctx.IsDir
	}

	if ctx.IsFile {
		ctx.Extension = ExtractExtension(name)
		ctx.BaseName = strings.TrimSuffix(name, ctx.Extension)
	} else {
		ctx.BaseName = name
	}

	return ctx
}

// WorkingName returns the most useful representation for pattern matching:
// file base name when we have an extension, otherwise the raw name.
func (ctx ParseContext) WorkingName() string {
	if ctx.BaseName != "" {
		return ctx.BaseName
	}
	return ctx.Name
}

// ParentNames collects ancestor names up to the requested depth, continuing
// along the source path when containing folders are outside the indexed tree.
func (ctx ParseContext) ParentNames(maxDepth int) []string {
	if ctx.Node == nil || maxDepth <= 0 {
		return nil
	}

	names := make([]string, 0, maxDepth)
	node := ctx.Node
	for node.Parent() != nil && len(names) < maxDepth {
		node = node.Parent()
		names = append(names, node.Name())
	}

	// Rename previews detach their roots from the containing directory.
	// Recover its name for parsing without changing the preview hierarchy.
	path := node.Data().Path
	for len(names) < maxDepth {
		dir := filepath.Dir(path)
		if dir == "." || dir == ".." || dir == filepath.Dir(dir) {
			break
		}
		names = append(names, filepath.Base(dir))
		path = dir
	}

	return names
}

// TitleAndYear derives cleaned title/year values using the working name.
func (ctx ParseContext) TitleAndYear() (string, string) {
	return ExtractNameAndYear(ctx.WorkingName())
}
