package cache

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
	"aftermath/internal/parser"
	"log"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Document parser.IncrementalParser

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
		docs: make(map[protocol.DocumentUri]Document),
	}
}

// Cache holds all the data for one client
type Cache struct {
	db   database.DB
	bib  bibliography.Bibliography
	docs map[string]Document
}

// OpenDocument initializes a new Document and returns its initial references.
func (c *Cache) OpenDocument(identifier string, content []byte) error {
	parser, err := parser.NewIncrementalParser(content)
	if err != nil {
		return err
	}

	log.Println("Hello from OpenDocument")
	log.Println("Initial References:")
	log.Println(parser.GetReferences())
	return nil

}

// UpdateDocument updates an existing documents content and parses it.
func (c *Cache) UpdateDocument() {}

// CloseDocument frees all ressources associated with an open document
func (c *Cache) CloseDocument(identifier string) {}

// Commit applies the references from Document to the shared store
func (c *Cache) Commit(identifier string) {}

// Returns the referenced zettel at a given position in the Document
func (c *Cache) ChildAt() {}

// Index returns a list of all zettels,
// compiled from the store and all documents
func (c *Cache) Index() {}

// Parents returns a list of all zettels linking to this one,
// compiled from the store and all documents
func (c *Cache) Parents() {}
