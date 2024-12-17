package dummy_test

import (
	"testing"
	"zeta/internal/cache/store/dummy"
)

func TestDummyStore(t *testing.T) {
	store := dummy.NewDummyStore()

	// Test GetAll
	paths, err := store.GetAll()
	if err != nil {
		t.Errorf("GetAll failed: %v", err)
	}
	if len(paths) == 0 {
		t.Error("Expected some paths, got none")
	}

	// Test GetChildren
	children, err := store.GetChildren("zettel1.typ")
	if err != nil {
		t.Errorf("GetChildren failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}

	// Test GetParents
	parents, err := store.GetParents("zettel3.typ")
	if err != nil {
		t.Errorf("GetParents failed: %v", err)
	}
	if len(parents) != 2 {
		t.Errorf("Expected 2 parents, got %d", len(parents))
	}

	// Test UpdateOne
	err = store.UpdateOne("zettel1.typ")
	if err != nil {
		t.Errorf("UpdateOne failed: %v", err)
	}

	// Test error handling
	_, err = store.GetChildren("nonexistent.typ")
	if err == nil {
		t.Error("Expected error for nonexistent zettel, got nil")
	}
}
