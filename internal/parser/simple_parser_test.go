package parser_test

import (
	"aftermath/internal/parser"
	"context"
	"testing"
)

func TestSimpleParser(t *testing.T) {
	p, err := parser.NewOneTimeParser()
	if err != nil {
		t.Fatalf("Failed to create simple parser: %v", err)
	}
	defer p.Close()

	t.Run("Simple Content", func(t *testing.T) {
		refs, err := p.ParseReferences(context.Background(), []byte(simpleContent))
		if err != nil {
			t.Fatalf("Failed to parse references: %v", err)
		}

		expected := []string{"simple/reference"}
		if !stringSlicesEqual(refs, expected) {
			t.Errorf("Expected %v, got %v", expected, refs)
		}
	})

	t.Run("Complex Content", func(t *testing.T) {
		refs, err := p.ParseReferences(context.Background(), []byte(complexContent))
		if err != nil {
			t.Fatalf("Failed to parse references: %v", err)
		}

		expected := []string{
			"first/reference",
			"second/reference",
			"nested/reference/with/colons",
		}
		if !stringSlicesEqual(refs, expected) {
			t.Errorf("Expected %v, got %v", expected, refs)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := p.ParseReferences(ctx, []byte(complexContent))
		if err == nil {
			t.Error("Expected error due to cancelled context")
		}
	})
}
