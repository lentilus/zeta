package cache

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
)

type Document struct {
}

// Store holds the data shared across all clients.
// It manages long term caching.
type Store struct {
	root string
	db   database.DB               // shared between clients
	bib  bibliography.Bibliography // shared between clients
}

func NewStore(root string) *Store {
	// Todo generate database connection
	// and bib from the root
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
		docs: make(map[string]Document),
	}
}

// Cache holds all the data for one client
type Cache struct {
	db   database.DB
	bib  bibliography.Bibliography
	docs map[string]Document
}

// UpsertDocument updates an existing docuemnt or inserts a new one
func (c *Cache) UpsertDocument()

// CloseDocument frees all ressources associated with an open document
func (c *Cache) CloseDocument()

// Commit writes applies the references from Document to the shared store
func (c *Cache) Commit()

// Returns the referenced zettel at a given position in the Document
func (c *Cache) ChildAt()

// Index returns a list of all zettels,
// compiled from the store and all documents
func (c *Cache) Index()

// Parents returns a list of all zettels linking to this one,
// compiled from the store and all documents
func (c *Cache) Parents()
