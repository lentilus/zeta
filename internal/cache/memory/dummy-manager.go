package memory

import (
	"fmt"
	"strings"
	"sync"
)

// DummyDocument implements the Document interface
type DummyDocument struct {
	content    string
	references []Reference
	mu         sync.RWMutex
}

// DummyManager implements the DocumentManager interface
type DummyManager struct {
	documents map[string]*DummyDocument
	mu        sync.RWMutex
}

// NewDummyManager creates a new DummyManager
func NewDummyManager() *DummyManager {
	return &DummyManager{
		documents: make(map[string]*DummyDocument),
	}
}

// DummyDocument implementation
func NewDummyDocument(content string) *DummyDocument {
	doc := &DummyDocument{
		content: content,
	}
	// Generate some dummy references based on @mentions in the content
	doc.scanReferences()
	return doc
}

func (d *DummyDocument) scanReferences() {
	d.references = nil
	words := strings.Split(d.content, " ")
	currentPos := uint32(0)

	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			ref := Reference{
				Range: Range{
					Start: Position{Line: 0, Character: currentPos},
					End:   Position{Line: 0, Character: currentPos + uint32(len(word))},
				},
				Target: strings.TrimPrefix(word, "@"),
			}
			d.references = append(d.references, ref)
		}
		currentPos += uint32(len(word)) + 1 // +1 for space
	}
}

func (d *DummyDocument) GetContent() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.content
}

func (d *DummyDocument) ApplyChanges(changes []Change) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Simple implementation that just replaces the entire content
	// In a real implementation, you'd apply changes incrementally
	newContent := d.content
	for _, change := range changes {
		// Very naive implementation - assumes single-line content
		start := change.Range.Start.Character
		end := change.Range.End.Character
		if start > uint32(len(newContent)) || end > uint32(len(newContent)) {
			return fmt.Errorf("invalid range: content length is %d, range is %d-%d",
				len(newContent), start, end)
		}
		newContent = newContent[:start] + change.NewText + newContent[end:]
	}

	d.content = newContent
	d.scanReferences() // Rescan references after changes
	return nil
}

func (d *DummyDocument) Close() error {
	// Nothing to do in this dummy implementation
	return nil
}

func (d *DummyDocument) GetReferences() []Reference {
	d.mu.RLock()
	defer d.mu.RUnlock()
	refs := make([]Reference, len(d.references))
	copy(refs, d.references)
	return refs
}

func (d *DummyDocument) GetReferenceAt(pos Position) (Reference, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, ref := range d.references {
		if pos.Line == ref.Range.Start.Line &&
			pos.Character >= ref.Range.Start.Character &&
			pos.Character <= ref.Range.End.Character {
			return ref, true
		}
	}
	return Reference{}, false
}

// DummyManager implementation
func (m *DummyManager) OpenDocument(path string, content string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.documents[path]; exists {
		return nil, fmt.Errorf("document already open: %s", path)
	}

	doc := NewDummyDocument(content)
	m.documents[path] = doc
	return doc, nil
}

func (m *DummyManager) GetDocument(path string) (Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.documents[path]
	return doc, exists
}

func (m *DummyManager) CommitDocument(path string) error {
	return nil
}

func (m *DummyManager) CloseDocument(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[path]
	if !exists {
		return fmt.Errorf("document not found: %s", path)
	}

	err := doc.Close()
	if err != nil {
		return fmt.Errorf("error closing document: %w", err)
	}

	delete(m.documents, path)
	return nil
}

func (m *DummyManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string
	for path, doc := range m.documents {
		if err := doc.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		}
	}

	m.documents = make(map[string]*DummyDocument)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing documents: %s", strings.Join(errors, "; "))
	}
	return nil
}
