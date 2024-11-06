package index

import (
	"aftermath/internal/database"
	"fmt"
	"path/filepath"
)

// Indexer handles indexing of zettel files
type Indexer struct {
	db  *database.DB
	dir string
}

// NewIndexer creates a new instance of Indexer
func NewIndexer(db *database.DB, dir string) *Indexer {
	return &Indexer{db: db, dir: dir}
}

// Index orchestrates the update of zettel and link indexes
func (indexer *Indexer) Index() error {
	changedZettels, err := indexer.UpdateZettelIndex()
	if err != nil {
		return err
	}
	return indexer.UpdateLinkIndex(changedZettels)
}

// UpdateZettelIndex scans the directory for zettels and updates the database
func (indexer *Indexer) UpdateZettelIndex() ([]string, error) {
	var changedZettels []string

	paths := indexer.FindPaths()
	for _, path := range paths {
		if err := indexer.processZettel(path, &changedZettels); err != nil {
			return nil, err
		}
	}

	return changedZettels, nil
}

// UpdateLinkIndex computes links between changed zettels
func (indexer *Indexer) UpdateLinkIndex(zettelPaths []string) error {
	for _, path := range zettelPaths {
		if err := indexer.processLinks(path); err != nil {
			return err
		}
	}
	return nil
}

// FindPaths retrieves paths of all `.typ` files in the specified directory
func (indexer *Indexer) FindPaths() []string {
	pattern := filepath.Join(indexer.dir, "*.typ")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Error finding paths: %v\n", err)
		return nil
	}
	return matches
}
