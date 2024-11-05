package parser_test

import (
	"aftermath/internal/parser"
	"reflect"
	"sort"
	"testing"
)

// Example Typst content that contains references
var content = `
	= This is a Typst file
	Here's a reference to @foo, and another to @bar.

    // this one is @commented

	Some more content and another reference: @baz.
	`

// TestGetReferences tests the GetReferences function with a Typst file example.
func TestGetReferences(t *testing.T) {
	references, err := parser.GetReferences([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Define the expected references
	expectedReferences := []string{"@foo", "@bar", "@baz"}

	// Check if the references match the expected ones
	if len(references) != len(expectedReferences) {
		t.Fatalf("expected %d references, got %d", len(expectedReferences), len(references))
	}

	sort.Strings(references)
	sort.Strings(expectedReferences)

	if !reflect.DeepEqual(references, expectedReferences) {
		t.Fatalf("expected %s references, got %s", expectedReferences, references)
	}
}
