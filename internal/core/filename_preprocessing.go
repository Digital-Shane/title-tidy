package core

import (
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
	"github.com/Digital-Shane/treeview/v2"
)

// PrepareFilenamePreprocessing shares a compiled regex pipeline with every local
// parser, including metadata lookups and ancestor-name fallbacks.
func PrepareFilenamePreprocessing(nodes []*treeview.Node[treeview.FileInfo], preprocess func(string) string) {
	var visit func(*treeview.Node[treeview.FileInfo])
	visit = func(node *treeview.Node[treeview.FileInfo]) {
		local.SetNamePreprocessor(node, preprocess)
		for _, child := range node.Children() {
			visit(child)
		}
	}
	for _, node := range nodes {
		visit(node)
	}
}
