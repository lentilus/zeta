package parser_test

import (
	"context"
	"testing"
	"zeta/internal/parser"
)

func TestIncrementalParser(t *testing.T) {
	// Test creation
	config := parser.Config{
		ReferenceQuery: `(ref) @reference`,
		TargetRegex:    `^\@(.*)$`,
	}

	p, err := parser.NewIncrementalParser(config)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer p.Close()

	t.Run("Initial Parse", func(t *testing.T) {
		err := p.Parse(context.Background(), []byte(simpleContent))
		if err != nil {
			t.Fatalf("Failed to parse content: %v", err)
		}

		refs := p.GetReferences()
		if len(refs) != 1 {
			t.Errorf("Expected 1 reference, got %d", len(refs))
		}
		if refs[0].Target != "simple/reference" {
			t.Errorf("Expected 'simple/reference', got '%s'", refs[0].Target)
		}
	})

	t.Run("Apply Changes", func(t *testing.T) {
		changes := []parser.Change{
			{
				Range: parser.Range{
					Start: parser.Position{Line: 0, Character: 0},
					End:   parser.Position{Line: 0, Character: uint32(len(simpleContent))},
				},
				NewText: updatedContent,
			},
		}

		err := p.ApplyChanges(context.Background(), changes)
		if err != nil {
			t.Fatalf("Failed to apply changes: %v", err)
		}

		refs := p.GetReferences()
		if len(refs) != 1 {
			t.Errorf("Expected 1 reference, got %d", len(refs))
		}
		if refs[0].Target != "new/reference" {
			t.Errorf("Expected 'new/reference', got '%s'", refs[0].Target)
		}
	})

	t.Run("Get Reference At Position", func(t *testing.T) {
		// Position inside the reference
		ref, found := p.GetReferenceAt(parser.Position{Line: 0, Character: 15})
		if !found {
			t.Error("Expected to find reference at position")
		}
		if ref.Target != "new/reference" {
			t.Errorf("Expected 'new/reference', got '%s'", ref.Target)
		}

		// Position outside any reference
		_, found = p.GetReferenceAt(parser.Position{Line: 0, Character: 0})
		if found {
			t.Error("Expected no reference at position")
		}
	})
}

func TestIncrementalParserMultipleReferences(t *testing.T) {
	config := parser.Config{
		ReferenceQuery: `(ref) @reference`,
		TargetRegex:    `^\@(.*)$`,
	}

	p, err := parser.NewIncrementalParser(config)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer p.Close()

	err = p.Parse(context.Background(), []byte(complexContent))
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	refs := p.GetReferences()
	expected := []string{
		"first/reference",
		"second/reference",
		"nested/reference/with/colons",
	}

	if len(refs) != len(expected) {
		t.Errorf("Expected %d references, got %d", len(expected), len(refs))
	}

	for i, ref := range refs {
		if ref.Target != expected[i] {
			t.Errorf("Expected '%s', got '%s'", expected[i], ref.Target)
		}
	}
}
