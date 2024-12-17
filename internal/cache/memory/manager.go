package memory

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"
	"zeta/internal/cache/store"
	"zeta/internal/parser"
	"zeta/internal/scheduler"
)

type SQLiteDocumentManager struct {
	store        store.Store
	docs         map[string]Document
	root         string
	mu           sync.RWMutex
	schedule     *scheduler.Scheduler
	parserConfig parser.Config
}

type Config struct {
	Root         string
	Store        store.Store
	ParserConfig parser.Config
}

func NewSQLiteDocumentManager(config Config) *SQLiteDocumentManager {

	schedule := scheduler.NewScheduler(16)
	schedule.RunScheduler()

	update := scheduler.Task{
		Name: "Periodic Store Update",
		Execute: func() error {
			err := config.Store.UpdateAll()
			if err != nil {
				log.Printf("Error during periodic store update: %s", err)
			}
			return nil
		},
	}

	schedule.SchedulePeriodicTask(5*time.Minute, update)
	log.Println("Moving on from scheduled task")

	return &SQLiteDocumentManager{
		store:        config.Store,
		docs:         make(map[string]Document),
		root:         config.Root,
		schedule:     schedule,
		parserConfig: config.ParserConfig,
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
	parser, err := parser.NewIncrementalParser(m.parserConfig)
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

// CommitDocument updates the document and its references in the store
func (m *SQLiteDocumentManager) CommitDocument(path string) error {
	m.mu.RLock()
	_, exists := m.docs[path]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("document not found: %s", path)
	}

	// Even though we're not directly managing the database anymore,
	// we still need to update the store with the latest state
	// of the document, which will in turn update the database
	return m.store.UpdateOne(path)
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

	m.schedule.StopScheduler()

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

// GetAllPaths returns all paths from both store and memory
func (m *SQLiteDocumentManager) GetAllPaths() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get paths from store
	storePaths, err := m.store.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get paths from store: %w", err)
	}

	// Create set of paths
	paths := make(map[string]struct{})
	for _, path := range storePaths {
		paths[path] = struct{}{}
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

// GetParents returns all documents that link to the given path
func (m *SQLiteDocumentManager) GetParents(path string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get parents from store
	storeParents, err := m.store.GetParents(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get parents from store: %w", err)
	}

	// Create set of parents
	parents := make(map[string]struct{})
	for _, parent := range storeParents {
		parents[parent] = struct{}{}
	}

	// Add parents from memory
	// Check each open document for references to the target path
	for docPath, doc := range m.docs {
		for _, ref := range doc.GetReferences() {
			if filepath.Join(m.root, ref.Target) == path {
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
