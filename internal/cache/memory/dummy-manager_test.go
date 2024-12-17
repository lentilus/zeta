package memory_test

import (
	"testing"
	"zeta/internal/cache/memory"
)

func TestDummyManager(t *testing.T) {
	manager := memory.NewDummyManager()

	// Test opening a document
	content := "Hello @world and @everyone"
	doc, err := manager.OpenDocument("test.txt", content)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Test getting references
	refs := doc.GetReferences()
	if len(refs) != 2 {
		t.Errorf("Expected 2 references, got %d", len(refs))
	}

	// Test getting reference at position
	ref, found := doc.GetReferenceAt(memory.Position{Line: 0, Character: 7})
	if !found {
		t.Error("Expected to find reference at position 7")
	}
	if found && ref.Target != "world" {
		t.Errorf("Expected reference target 'world', got '%s'", ref.Target)
	}

	// Test applying changes
	changes := []memory.Change{
		{
			Range: memory.Range{
				Start: memory.Position{Line: 0, Character: 6},
				End:   memory.Position{Line: 0, Character: 12},
			},
			NewText: "@earth",
		},
	}
	err = doc.ApplyChanges(changes)
	if err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Verify content after changes
	expectedContent := "Hello @earth and @everyone"
	if doc.GetContent() != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, doc.GetContent())
	}

	// Test closing document
	err = manager.CloseDocument("test.txt")
	if err != nil {
		t.Fatalf("Failed to close document: %v", err)
	}

	// Verify document is closed
	_, exists := manager.GetDocument("test.txt")
	if exists {
		t.Error("Document still exists after closing")
	}
}
