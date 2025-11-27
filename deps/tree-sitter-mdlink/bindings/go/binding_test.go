package tree_sitter_mdlink_test

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_mdlink "github.com/tree-sitter/tree-sitter-mdlink/bindings/go"
)

func TestCanLoadGrammar(t *testing.T) {
	language := tree_sitter.NewLanguage(tree_sitter_mdlink.Language())
	if language == nil {
		t.Errorf("Error loading mdlink grammar")
	}
}
