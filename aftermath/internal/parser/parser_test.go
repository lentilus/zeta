package parser_test

import (
	"aftermath/internal/parser"
	"testing"
)

func TestGetReferences(t *testing.T) {
	// Sample content to be parsed
	content := []byte(`
		some text with a ref to @foo
		another line without a ref
		and a ref here as well @bar
	`)

	// Expected results based on the refQuery pattern
	expectedReferences := []string{"@foo", "@bar"}

	// Initialize parser
	p := parser.NewParser()
	defer p.CloseParser() // Ensure parser is properly closed after test

	// Run GetReferences to extract references from the content
	references, err := p.GetReferences(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check if the results match the expected references
	if len(references) != len(expectedReferences) {
		t.Errorf("expected %d references, got %d", len(expectedReferences), len(references))
	}

	for i, ref := range references {
		if ref != expectedReferences[i] {
			t.Errorf("expected reference %q at index %d, got %q", expectedReferences[i], i, ref)
		}
	}
}
