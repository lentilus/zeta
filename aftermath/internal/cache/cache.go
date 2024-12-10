package cache

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
	"aftermath/internal/parser"
	"aftermath/internal/utils"
	con "context"
	"fmt"
	"log"

	sitter "github.com/smacker/go-tree-sitter"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Document *parser.IncrementalParser
type Reference *parser.Reference

// Store holds the data shared across all clients.
// It manages long term caching.
type Store struct {
	root string
	db   database.DB
	bib  bibliography.Bibliography
}

func NewStore(root string) *Store {
	// Todo generate database connection
	// and bib from the root
	// This should also start the incremental Indexing
	return &Store{
		root: root,
		// db: todo
		// bib: todo
	}
}

// NewCache initializes a new Cache with all shared ressources.
func (s *Store) NewCache() *Cache {
	return &Cache{
		db:   s.db,
		bib:  s.bib,
		root: s.root,
		docs: make(map[protocol.DocumentUri]Document),
	}
}

// Cache holds all the data for one client
type Cache struct {
	db   database.DB
	bib  bibliography.Bibliography
	root string
	docs map[string]Document
}

// OpenDocument initializes a new Document and returns its initial references.
func (c *Cache) OpenDocument(identifier string, content []byte) ([]parser.Reference, error) {
	// parse initial content
	parser, err := parser.NewIncrementalParser(content)
	if err != nil {
		return nil, err
	}

	// store parser in docs
	c.docs[identifier] = parser

	// Returns initial references
	return parser.GetReferences(), nil
}

func (c *Cache) UpdateDocument(
	identifier protocol.DocumentUri,
	changes any,
) ([]parser.Reference, error) {
	log.Println("Updating document")
	log.Printf("Type of changes received: %T", changes)

	// Handle type assertion for changes manually in case it's not directly castable
	contentChanges, ok := changes.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected changes format, expected a slice")
	}

	var parsedChanges []protocol.TextDocumentContentChangeEvent
	for _, change := range contentChanges {
		// Attempt to cast each change into the expected type
		tChange, ok := change.(protocol.TextDocumentContentChangeEvent)
		if !ok {
			return nil, fmt.Errorf("invalid change type: %T", change)
		}
		parsedChanges = append(parsedChanges, tChange)
	}

	// Retrieve the document from the cache
	doc, ok := c.docs[identifier]
	if !ok {
		return nil, fmt.Errorf("identifier %s not found in active documents", identifier)
	}

	// Explicit type conversion from Document (which is *parser.IncrementalParser) to parser.IncrementalParser
	var p *parser.IncrementalParser = doc

	// Process each change event and apply only incremental changes
	for _, change := range parsedChanges {
		// Ensure change has a valid range for incremental updates
		if change.Range == nil {
			continue // Skip changes that don't specify a range
		}

		// Map protocol change data into the DocumentChange type
		docChange := parser.DocumentChange{
			StartPos: p.CalculateOffset(parser.Position{
				Line:      uint32(change.Range.Start.Line),
				Character: uint32(change.Range.Start.Character),
			}),
			EndPos: p.CalculateOffset(parser.Position{
				Line:      uint32(change.Range.End.Line),
				Character: uint32(change.Range.End.Character),
			}),
			NewText:   []byte(change.Text),
			IsPartial: true,
		}

		// Apply the change incrementally to the parser
		log.Println("Applying Changes")
		if err := p.ApplyChanges(con.Background(), []parser.DocumentChange{docChange}); err != nil {
			return nil, fmt.Errorf("failed to apply incremental changes: %w", err)
		}
	}

	// Fetch references for diagnostics computation after updates
	references := p.GetReferences()
	return references, nil
}

// CloseDocument frees all ressources associated with an open document
func (c *Cache) CloseDocument(identifier string) {}

// Commit applies the references from Document to the shared store
func (c *Cache) Commit(identifier string) {}

// Returns the referenced zettel at a given position in the Document
// returns protocol.Position or nil
func (c *Cache) ChildAt(identifier string, position protocol.Position) (any, error) {
	// Just a dummy implementation
	doc, ok := c.docs[identifier]
	if !ok {
		return nil, fmt.Errorf("identifier %s not found in active documents", identifier)
	}

	var p *parser.IncrementalParser = doc
	ref := p.GetReferenceAt(sitter.Point{
		Row:    position.Line,
		Column: position.Character,
	})

	path, err := utils.Reference2Path(ref.Text, c.root)
	if err != nil {
		return nil, err
	}

	return protocol.Location{
		URI: "file://" + path,
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 1},
		},
	}, nil
}

// Index returns a list of all zettels,
// compiled from the store and all documents
func (c *Cache) Index() {}

// Parents returns a list of all zettels linking to this one,
// compiled from the store and all documents
func (c *Cache) Parents(identifier string) ([]protocol.Location, error) {
	return []protocol.Location{
		{
			URI: "file:///home/lentilus/haha",
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 1},
			},
		},
		{
			URI: "file:///home/lentilus/huhu",
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 1},
			},
		},
	}, nil
}
