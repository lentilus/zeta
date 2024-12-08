package parser_test

import (
	"aftermath/internal/parser"
	"sync"
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

func TestIncrementalParser(t *testing.T) {
	content := []byte(`
		some text with a ref to @foo
		another line without a ref
		and a ref here as well @bar
	`)

	// Test basic functionality
	ip := parser.NewIncrementalParser(content)
	defer ip.Close()

	// Initial parse happens in constructor
	refs = ip.GetReferences()
	expected = []string{"@foo", "@bar", "@baz"}

	if len(refs) != len(expected) {
		t.Errorf("expected %d references, got %d", len(expected), len(refs))
	}

	// Test incremental update
	newContent := []byte(`
		some text with a ref to @foo
		another line without a ref
		and a ref here as well @bar
		and a new ref @baz
	`)

	err := ip.Parse(context.Background(), newContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	refs := ip.GetReferences()
	expected := []string{"@foo", "@bar"}

	if len(refs) != len(expected) {
		t.Errorf("expected %d references, got %d", len(expected), len(refs))
	}

	for i, ref := range refs {
		if ref != expected[i] {
			t.Errorf("expected reference %q at index %d, got %q", expected[i], i, ref)
		}
	}

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			refs := ip.GetReferences()
			if len(refs) != 2 {
				t.Errorf("concurrent access: expected 2 references, got %d", len(refs))
			}
		}()
	}
	wg.Wait()
}
