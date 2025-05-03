package manager

import (
	"fmt"
	"sync"
	"zeta/internal/cache"
	"zeta/internal/parser"
	"zeta/internal/resolver"
	"zeta/internal/sitteradapter"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// DocumentManager encapsulates parser and document state for each open URI.
type DocumentManager struct {
	mu      sync.Mutex
	parsers map[string]*parser.Parser
	docs    map[string][]byte
}

// NewDocumentManager creates an initialized DocumentManager.
func NewDocumentManager() *DocumentManager {
	return &DocumentManager{
		parsers: make(map[string]*parser.Parser),
		docs:    make(map[string][]byte),
	}
}

// EnsureParser returns the parser for a URI, creating it if needed.
func (dm *DocumentManager) EnsureParser(uri string) (*parser.Parser, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if p, ok := dm.parsers[uri]; ok && p != nil {
		return p, nil
	}

	p, err := parser.NewParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create parser for %s: %w", uri, err)
	}
	dm.parsers[uri] = p
	return p, nil
}

// GetDocument returns the current document bytes for a URI.
func (dm *DocumentManager) GetDocument(uri string) ([]byte, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, ok := dm.docs[uri]
	if !ok {
		return nil, fmt.Errorf("document not loaded for %s", uri)
	}
	return doc, nil
}

// UpdateDocument replaces the document bytes for a URI.
func (dm *DocumentManager) UpdateDocument(uri string, content []byte) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.docs[uri] = content
}

// ApplyIncrementalEdit applies a Tree-sitter edit and updates stored bytes.
func (dm *DocumentManager) ApplyIncrementalEdit(
	uri string,
	change protocol.TextDocumentContentChangeEvent,
) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	p, ok := dm.parsers[uri]
	if !ok {
		return fmt.Errorf("no parser for document %s", uri)
	}
	oldDoc, ok := dm.docs[uri]
	if !ok {
		return fmt.Errorf("no document for %s", uri)
	}

	tsEdit := sitteradapter.CreateTSEditAdapter(change, string(oldDoc))
	p.Update(tsEdit)

	newText := sitteradapter.ApplyTextEdit(change, string(oldDoc))
	dm.docs[uri] = []byte(newText)
	return nil
}

// GetLinks runs the full parse → query → extract pipeline.
func (dm *DocumentManager) GetLinks(
	uri string,
	queryString string,
) ([]cache.Link, error) {
	// Ensure parser + doc
	p, err := dm.EnsureParser(uri)
	if err != nil {
		return nil, err
	}
	doc, err := dm.GetDocument(uri)
	if err != nil {
		return nil, err
	}

	// Parse & query
	if err := p.Parse(doc); err != nil {
		return nil, err
	}
	nodes, err := p.Query([]byte(queryString), doc)
	if err != nil {
		return nil, err
	}

	// Resolve note metadata
	note, err := resolver.Resolve(uri)
	if err != nil {
		return nil, err
	}

	// Extract and return links
	return resolver.ExtractLinks(note, nodes, doc), nil
}

// Release frees parser and document for a URI.
func (dm *DocumentManager) Release(uri string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	delete(dm.parsers, uri)
	delete(dm.docs, uri)
}

// CloseAll cleans up all parsers.
func (dm *DocumentManager) CloseAll() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for uri, p := range dm.parsers {
		if err := p.Close(); err != nil {
			return fmt.Errorf("error closing parser for %s: %w", uri, err)
		}
	}
	dm.parsers = make(map[string]*parser.Parser)
	dm.docs = make(map[string][]byte)
	return nil
}
