package bindings_test

import (
	"aftermath/bindings"
	"testing"

	tree_sitter "github.com/smacker/go-tree-sitter"
)

func TestCanLoadGrammar(t *testing.T) {
	language := tree_sitter.NewLanguage(bindings.Language())
	if language == nil {
		t.Errorf("Error loading Typst grammar")
	}
}
