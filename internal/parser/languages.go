package parser

/*
#cgo CFLAGS: -std=c11 -fPIC

extern void *tree_sitter_typst(void);
extern void *tree_sitter_markdown(void);
*/
import "C"

import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

var (
	typst = unsafe.Pointer(C.tree_sitter_typst())
	markdown = unsafe.Pointer(C.tree_sitter_markdown())
)

var languages = map[string]*sitter.Language{
	"typst" : sitter.NewLanguage(typst),
	"markdown" : sitter.NewLanguage(markdown),
}

