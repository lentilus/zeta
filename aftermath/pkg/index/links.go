package index

import (
	"aftermath/internal/database"
	"aftermath/internal/parser"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validCharPattern = regexp.MustCompile(`[^\w.-]+`)

// processLinks retrieves the zettel content and indexes its links
func (indexer *Indexer) processLinks(path string) error {
	zettel, err := indexer.db.GetZettel(path)
	if err != nil {
		return fmt.Errorf("failed to retrieve zettel %s: %w", path, err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read zettel file %s: %w", path, err)
	}

	return indexer.IndexLinks(content, zettel.ID)
}

// IndexLinks processes the content for links and inserts them into the database
func (indexer *Indexer) IndexLinks(content []byte, sourceID int) error {
	references, err := parser.GetReferences(content)
	if err != nil {
		return fmt.Errorf("failed to extract references: %w", err)
	}

	for _, ref := range references {
		if err := indexer.addLink(sourceID, ref); err != nil {
			return err
		}
	}

	return nil
}

// addLink retrieves the target zettel by reference and creates a link in the database
func (indexer *Indexer) addLink(sourceID int, ref string) error {
	refPath := indexer.Link2File(ref)

	targetZettel, err := indexer.db.GetZettel(refPath)
	if err != nil {
		if err == database.ErrZettelNotFound {
			return nil // Ignore if not found
		}
		return fmt.Errorf("failed to retrieve referenced zettel: %w", err)
	}

	return indexer.db.CreateLink(sourceID, targetZettel.ID)
}

// Link2File converts a reference to a file path
func (indexer *Indexer) Link2File(reference string) string {
	validRef := validCharPattern.ReplaceAllString(reference, "")
	path := strings.ReplaceAll(validRef, ".", "/")
	return filepath.Join(indexer.dir, path) + ".typ"
}
