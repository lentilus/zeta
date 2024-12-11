package memory

import (
	"aftermath/internal/cache/database"
	"aftermath/internal/parser"
	"fmt"
	"sync"
	"time"
)

type SQLiteDocumentManager struct {
	db   database.Database
	docs map[string]Document
	mu   sync.RWMutex
}

func NewSQLiteDocumentManager(db database.Database) *SQLiteDocumentManager {
	return &SQLiteDocumentManager{
		db:   db,
		docs: make(map[string]Document),
	}
}

func (m *SQLiteDocumentManager) OpenDocument(path string, content string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if document is already open
	if doc, exists := m.docs[path]; exists {
		return doc, fmt.Errorf("document already open: %s", path)
	}
	// Create Incremental Parser
	parser, err := parser.NewIncrementalParser()
	if err != nil {
		return nil, err
	}

	// Create new document
	doc, err := NewParserDocument(content, parser)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	m.docs[path] = doc
	return doc, nil
}

func (m *SQLiteDocumentManager) GetDocument(path string) (Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.docs[path]
	return doc, exists
}

func (m *SQLiteDocumentManager) CommitDocument(path string) error {
	m.mu.RLock()
	doc, exists := m.docs[path]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("document not found: %s", path)
	}

	// Get references from document
	refs := doc.GetReferences()
	targets := make([]string, len(refs))
	for i, ref := range refs {
		targets[i] = ref.Target
	}

	// Update database
	return m.db.WithTx(func(tx database.Transaction) error {
		// Update file record
		if err := tx.UpsertFile(&database.FileRecord{
			Path:         path,
			LastModified: time.Now().Unix(),
		}); err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}

		// Update links
		return tx.UpsertLinks(path, targets)
	})
}

func (m *SQLiteDocumentManager) CloseDocument(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.docs[path]
	if !exists {
		return fmt.Errorf("document not found: %s", path)
	}

	if err := doc.Close(); err != nil {
		return fmt.Errorf("failed to close document: %w", err)
	}

	delete(m.docs, path)
	return nil
}

func (m *SQLiteDocumentManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for path, doc := range m.docs {
		if err := doc.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close %s: %w", path, err))
		}
	}

	m.docs = make(map[string]Document)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing documents: %v", errs)
	}
	return nil
}

// Helper methods for merging database and memory state

func (m *SQLiteDocumentManager) GetAllPaths() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get paths from database
	records, err := m.db.GetAllFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get files from database: %w", err)
	}

	// Create set of paths
	paths := make(map[string]struct{})
	for _, record := range records {
		paths[record.Path] = struct{}{}
	}

	// Add paths from memory
	for path := range m.docs {
		paths[path] = struct{}{}
	}

	// Convert to slice
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}

	return result, nil
}

func (m *SQLiteDocumentManager) GetParents(path string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get parents from database
	records, err := m.db.GetBacklinks(path)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to get backlinks from database: %w", err)
	}

	// Create set of parents
	parents := make(map[string]struct{})
	for _, record := range records {
		parents[record.SourcePath] = struct{}{}
	}

	// Add parents from memory
	for docPath, doc := range m.docs {
		for _, ref := range doc.GetReferences() {
			if ref.Target == path {
				parents[docPath] = struct{}{}
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(parents))
	for parent := range parents {
		result = append(result, parent)
	}

	return result, nil
}
