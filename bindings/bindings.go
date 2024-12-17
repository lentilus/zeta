package bindings

// #cgo CFLAGS: -std=c11 -fPIC
// #include "../tree-sitter-typst/src/parser.c"
// #include "../tree-sitter-typst/src/scanner.c"
import "C"

import "unsafe"

// Get the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_typst())
}
