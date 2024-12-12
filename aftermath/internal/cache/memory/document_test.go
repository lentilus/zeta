package memory_test

import (
	"aftermath/internal/cache/memory"
	"aftermath/internal/parser"
	"context"
	"fmt"
	"sync"
	"testing"
)

// MockParser implements parser.Parser for testing
type MockParser struct {
	content    []byte
	references []parser.Reference
	closed     bool
	parseFails bool
	mu         sync.RWMutex
}

func NewMockParser() *MockParser {
	return &MockParser{}
}

func (m *MockParser) Parse(_ context.Context, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.parseFails {
		return fmt.Errorf("mock parse failure")
	}
	m.content = make([]byte, len(content))
	copy(m.content, content)
	m.updateReferences()
	return nil
}

func (m *MockParser) ApplyChanges(_ context.Context, changes []parser.Change) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple implementation that just updates content
	for _, change := range changes {
		startOffset := m.positionToOffset(change.Range.Start)
		endOffset := m.positionToOffset(change.Range.End)

		newContent := make([]byte, 0, len(m.content)-int(endOffset-startOffset)+len(change.NewText))
		newContent = append(newContent, m.content[:startOffset]...)
		newContent = append(newContent, []byte(change.NewText)...)
		newContent = append(newContent, m.content[endOffset:]...)
		m.content = newContent
	}
	m.updateReferences()
	return nil
}

func (m *MockParser) GetReferences() []parser.Reference {
	m.mu.RLock()
	defer m.mu.RUnlock()

	refs := make([]parser.Reference, len(m.references))
	copy(refs, m.references)
	return refs
}

func (m *MockParser) GetReferenceAt(pos parser.Position) (parser.Reference, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ref := range m.references {
		if pos.Line == ref.Range.Start.Line &&
			pos.Character >= ref.Range.Start.Character &&
			pos.Character <= ref.Range.End.Character {
			return ref, true
		}
	}
	return parser.Reference{}, false
}

func (m *MockParser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

// Helper methods for MockParser
func (m *MockParser) updateReferences() {
	m.references = nil
	content := string(m.content)
	start := 0
	for {
		idx := findNextReference(content[start:])
		if idx == -1 {
			break
		}
		refStart := start + idx
		refEnd := findReferenceEnd(content[refStart:])
		if refEnd == -1 {
			break
		}
		refEnd += refStart

		m.references = append(m.references, parser.Reference{
			Range: parser.Range{
				Start: parser.Position{Line: 0, Character: uint32(refStart)},
				End:   parser.Position{Line: 0, Character: uint32(refEnd)},
			},
			Target: content[refStart+1 : refEnd], // Skip @ symbol
		})
		start = refEnd
	}
}

func (m *MockParser) positionToOffset(pos parser.Position) uint32 {
	var offset uint32
	var currentLine uint32

	for i := 0; i < len(m.content) && currentLine < pos.Line; i++ {
		if m.content[i] == '\n' {
			currentLine++
		}
		offset++
	}

	offset += pos.Character
	if offset > uint32(len(m.content)) {
		offset = uint32(len(m.content))
	}

	return offset
}

// Test helper functions
func findNextReference(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return i
		}
	}
	return -1
}

func findReferenceEnd(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\n' {
			return i
		}
	}
	return len(s)
}

func TestParserDocument(t *testing.T) {
	t.Run("Constructor", func(t *testing.T) {
		// Test with nil parser (should create IncrementalParser)
		doc, err := memory.NewParserDocument("Hello @world ", nil)
		if err != nil {
			t.Fatalf("Failed to create document with nil parser: %v", err)
		}
		defer doc.Close()

		refs := doc.GetReferences()
		if len(refs) != 1 {
			t.Errorf("Expected 1 reference, got %d", len(refs))
		}

		// Test with mock parser
		mockParser := NewMockParser()
		doc, err = memory.NewParserDocument("Hello @world ", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document with mock parser: %v", err)
		}
		defer doc.Close()

		refs = doc.GetReferences()
		if len(refs) != 1 {
			t.Errorf("Expected 1 reference, got %d", len(refs))
		}
	})

	t.Run("Constructor Error Cases", func(t *testing.T) {
		// Create a failing mock parser
		failingParser := &MockParser{
			parseFails: true,
		}

		_, err := memory.NewParserDocument("Hello @world ", failingParser)
		if err == nil {
			t.Error("Expected error when parser fails, got nil")
		}
	})

	t.Run("Basic Operations", func(t *testing.T) {
		mockParser := NewMockParser()
		doc, err := memory.NewParserDocument("Hello @world  How are you?", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		defer doc.Close()

		// Test GetContent
		expectedContent := "Hello @world  How are you?"
		if content := doc.GetContent(); content != expectedContent {
			t.Errorf("Expected content %q, got %q", expectedContent, content)
		}

		// Test GetReferences
		refs := doc.GetReferences()
		if len(refs) != 1 {
			t.Fatalf("Expected 1 reference, got %d", len(refs))
		}
		if refs[0].Target != "world" {
			t.Errorf("Expected reference target 'world', got %q", refs[0].Target)
		}

		// Test GetReferenceAt
		ref, found := doc.GetReferenceAt(memory.Position{Line: 0, Character: 7})
		if !found {
			t.Error("Expected to find reference at position 7")
		}
		if found && ref.Target != "world" {
			t.Errorf("Expected reference target 'world', got %q", ref.Target)
		}
	})

	t.Run("Apply Changes", func(t *testing.T) {
		mockParser := NewMockParser()
		doc, err := memory.NewParserDocument("Hello @world ", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		defer doc.Close()

		changes := []memory.Change{
			{
				Range: memory.Range{
					Start: memory.Position{Line: 0, Character: 6},
					End:   memory.Position{Line: 0, Character: 12},
				},
				NewText: "@universe",
			},
		}

		if err := doc.ApplyChanges(changes); err != nil {
			t.Fatalf("Failed to apply changes: %v", err)
		}

		expectedContent := "Hello @universe "
		if content := doc.GetContent(); content != expectedContent {
			t.Errorf("Expected content %q, got %q", expectedContent, content)
		}

		refs := doc.GetReferences()
		if len(refs) != 1 {
			t.Fatalf("Expected 1 reference, got %d", len(refs))
		}
		if refs[0].Target != "universe" {
			t.Errorf("Expected reference target 'universe', got %q", refs[0].Target)
		}
	})

	t.Run("Multiple References", func(t *testing.T) {
		mockParser := NewMockParser()
		doc, err := memory.NewParserDocument("Hello @world and @universe and @everything ", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		defer doc.Close()

		refs := doc.GetReferences()
		expectedTargets := []string{"world", "universe", "everything"}

		if len(refs) != len(expectedTargets) {
			t.Fatalf("Expected %d references, got %d", len(expectedTargets), len(refs))
		}

		for i, ref := range refs {
			if ref.Target != expectedTargets[i] {
				t.Errorf("Expected reference target %q, got %q", expectedTargets[i], ref.Target)
			}
		}
	})

	t.Run("Close", func(t *testing.T) {
		mockParser := NewMockParser()
		doc, err := memory.NewParserDocument("Hello @world ", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		if err := doc.Close(); err != nil {
			t.Errorf("Failed to close document: %v", err)
		}

		if !mockParser.closed {
			t.Error("Parser was not closed")
		}
	})

	t.Run("Thread Safety", func(t *testing.T) {
		mockParser := NewMockParser()
		doc, err := memory.NewParserDocument("Hello @world ", mockParser)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		defer doc.Close()

		// Run concurrent operations
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				doc.GetContent()
				doc.GetReferences()
				doc.GetReferenceAt(memory.Position{Line: 0, Character: 7})
			}()
		}

		wg.Wait()
	})

	// t.Run("Multi-line Content", func(t *testing.T) {
	// 	content := "First line\nSecond @reference \nThird line"
	// 	mockParser := NewMockParser()
	// 	doc, err := memory.NewParserDocument(content, mockParser)
	// 	if err != nil {
	// 		t.Fatalf("Failed to create document: %v", err)
	// 	}
	// 	defer doc.Close()
	//
	// 	ref, found := doc.GetReferenceAt(memory.Position{Line: 1, Character: 8})
	// 	if !found {
	// 		t.Error("Expected to find reference on second line")
	// 	}
	// 	if found && ref.Target != "reference" {
	// 		t.Errorf("Expected reference target 'reference', got %q", ref.Target)
	// 	}
	// })
}
