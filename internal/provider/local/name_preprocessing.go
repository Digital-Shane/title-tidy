package local

import "github.com/Digital-Shane/treeview/v2"

const namePreprocessorKey = "local_name_preprocessor"

// SetNamePreprocessor changes only names used for parsing. Source names, paths,
// extensions, and the filesystem remain untouched.
func SetNamePreprocessor(node *treeview.Node[treeview.FileInfo], preprocess func(string) string) {
	if node.Data().Extra == nil && preprocess != nil {
		node.Data().Extra = make(map[string]any)
	}
	if preprocess == nil {
		delete(node.Data().Extra, namePreprocessorKey)
	} else {
		node.Data().Extra[namePreprocessorKey] = preprocess
	}
}

func namePreprocessor(node *treeview.Node[treeview.FileInfo]) func(string) string {
	if node != nil {
		if preprocess, ok := node.Data().Extra[namePreprocessorKey].(func(string) string); ok {
			return preprocess
		}
	}
	return func(name string) string { return name }
}
