package parser_test

import (
	"context"
	"reflect"
	"testing"

	"aftermath/internal/parser"
)

func TestIncrementalParser(t *testing.T) {
	initialContent := []byte(`first @ref1 middle @ref2 last`)

	p, err := parser.NewIncrementalParser(initialContent)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer p.Close()

	t.Run("Initial Parse", func(t *testing.T) {
		refs := p.GetReferences()
		if len(refs) != 2 {
			t.Errorf("Expected 2 references, got %d", len(refs))
		}

		expected := []parser.Reference{
			{Text: "@ref1", Line: 0, Col: 6, Start: 6, End: 11},
			{Text: "@ref2", Line: 0, Col: 19, Start: 19, End: 24},
		}

		if !reflect.DeepEqual(refs, expected) {
			t.Errorf("References mismatch\nGot: %+v\nWant: %+v", refs, expected)
		}
	})

	t.Run("Reference Texts Only", func(t *testing.T) {
		texts := p.GetReferenceTexts()
		expected := []string{"@ref1", "@ref2"}
		if !reflect.DeepEqual(texts, expected) {
			t.Errorf("Reference texts mismatch\nGot: %v\nWant: %v", texts, expected)
		}
	})

	t.Run("Get Reference At Position", func(t *testing.T) {
		testCases := []struct {
			name     string
			pos      parser.Position
			expected *parser.Reference
		}{
			{
				name: "First Reference",
				pos:  parser.Position{Line: 0, Col: 7},
				expected: &parser.Reference{
					Text: "@ref1",
					Line: 0, Col: 6,
					Start: 6, End: 11,
				},
			},
			{
				name: "Second Reference",
				pos:  parser.Position{Line: 0, Col: 20},
				expected: &parser.Reference{
					Text: "@ref2",
					Line: 0, Col: 19,
					Start: 19, End: 24,
				},
			},
			{
				name:     "No Reference",
				pos:      parser.Position{Line: 0, Col: 0},
				expected: nil,
			},
			{
				name:     "Wrong Line",
				pos:      parser.Position{Line: 1, Col: 7},
				expected: nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got := p.GetReferenceAt(tc.pos)
				if !reflect.DeepEqual(got, tc.expected) {
					t.Errorf("GetReferenceAt(%+v) = %+v, want %+v", tc.pos, got, tc.expected)
				}
			})
		}
	})

	t.Run("Incremental Update", func(t *testing.T) {
		newContent := []byte(`first @ref1 modified @ref3 last`)
		err := p.ParseContent(context.Background(), newContent)
		if err != nil {
			t.Fatalf("Failed to parse new content: %v", err)
		}

		refs := p.GetReferences()
		// Let's first print the actual references to help debug
		t.Logf("Actual references: %#v", refs)

		// Add a helper to extract the actual text from content
		extractText := func(content []byte, start, end uint32) string {
			if int(start) >= len(content) || int(end) > len(content) || start >= end {
				return ""
			}
			return string(content[start:end])
		}

		// Print the actual text segments to verify what we're matching
		for _, ref := range refs {
			t.Logf("Reference text from content: %q", extractText(newContent, ref.Start, ref.End))
		}

		expected := []parser.Reference{
			{Text: "@ref1", Line: 0, Col: 6, Start: 6, End: 11},
			{Text: "@ref3", Line: 0, Col: 21, Start: 21, End: 26}, // Updated positions
		}

		if !reflect.DeepEqual(refs, expected) {
			t.Errorf("References after update mismatch\nGot: %+v\nWant: %+v", refs, expected)
		}
	})

	t.Run("Empty Content", func(t *testing.T) {
		err := p.ParseContent(context.Background(), []byte{})
		if err != nil {
			t.Fatalf("Failed to parse empty content: %v", err)
		}

		refs := p.GetReferences()
		if len(refs) != 0 {
			t.Errorf("Expected no references for empty content, got %d", len(refs))
		}
	})
}

func TestOneTimeParser(t *testing.T) {
	otp := parser.NewOneTimeParser()
	defer otp.CloseParser()

	t.Run("Basic Parsing", func(t *testing.T) {
		content := []byte(`test @ref1 and @ref2 here`)
		refs, err := otp.GetReferences(content)
		if err != nil {
			t.Fatalf("Failed to get references: %v", err)
		}

		expected := []string{"@ref1", "@ref2"}
		if !reflect.DeepEqual(refs, expected) {
			t.Errorf("References mismatch\nGot: %v\nWant: %v", refs, expected)
		}
	})

	t.Run("Empty Content", func(t *testing.T) {
		refs, err := otp.GetReferences([]byte{})
		if err != nil {
			t.Fatalf("Failed to parse empty content: %v", err)
		}

		if len(refs) != 0 {
			t.Errorf("Expected no references for empty content, got %d", len(refs))
		}
	})

	t.Run("No References", func(t *testing.T) {
		content := []byte(`test with no references`)
		refs, err := otp.GetReferences(content)
		if err != nil {
			t.Fatalf("Failed to parse content: %v", err)
		}

		if len(refs) != 0 {
			t.Errorf("Expected no references, got %d", len(refs))
		}
	})
}

func TestParserConcurrency(t *testing.T) {
	initialContent := []byte(`initial @ref`)
	p, err := parser.NewIncrementalParser(initialContent)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer p.Close()

	// Run multiple goroutines accessing the parser concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			p.GetReferences()
			p.GetReferenceTexts()
			p.GetReferenceAt(parser.Position{Line: 0, Col: 0})
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestContextCancellation(t *testing.T) {
	p, err := parser.NewIncrementalParser([]byte(`test @ref`))
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = p.ParseContent(ctx, []byte(`new content`))
	if err == nil {
		t.Error("Expected error due to cancelled context, got nil")
	}
}
