package parser

import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

/*
#cgo CFLAGS: -std=c11 -fPIC

#include "../../deps/tree-sitter-typst/src/parser.c"
#include "../../deps/tree-sitter-typst/src/scanner.c"

#include "../../deps/tree-sitter-markdown/src/parser.c"
#include "../../deps/tree-sitter-markdown/src/scanner.c"
*/
import "C"

var (
	typst = unsafe.Pointer(C.tree_sitter_typst())
	markdown = unsafe.Pointer(C.tree_sitter_markdown())
)

var languages = map[string]*sitter.Language{
	"typst" : sitter.NewLanguage(typst),
	"markdown" : sitter.NewLanguage(markdown),
}

