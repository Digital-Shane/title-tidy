package local

import (
	"strings"

	"github.com/Digital-Shane/treeview"
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

// ParentNames collects ancestor names up to the requested depth.
func (ctx ParseContext) ParentNames(maxDepth int) []string {
	if ctx.Node == nil || maxDepth <= 0 {
		return nil
	}

	names := make([]string, 0, maxDepth)
	parent := ctx.Node.Parent()
	depth := 0
	for parent != nil && depth < maxDepth {
		names = append(names, parent.Name())
		parent = parent.Parent()
		depth++
	}

	return names
}

// TitleAndYear derives cleaned title/year values using the working name.
func (ctx ParseContext) TitleAndYear() (string, string) {
	return ExtractNameAndYear(ctx.WorkingName())
}
