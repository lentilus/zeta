package tree_sitter_typst_test

import (
	"testing"
	bindings "zeta/tree-sitter-typst"

	tree_sitter "github.com/smacker/go-tree-sitter"
)

func TestCanLoadGrammar(t *testing.T) {
	language := tree_sitter.NewLanguage(bindings.Language())
	if language == nil {
		t.Errorf("Error loading Typst grammar")
	}
}
