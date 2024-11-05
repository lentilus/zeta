package index

import (
	"aftermath/internal/database"
	"aftermath/internal/parser"
	"aftermath/internal/utils"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Precompiled regex pattern to remove invalid characters (anything other than A-Z, a-z, 0-9, ., -, and _)
var validCharPattern = regexp.MustCompile(`[^\w.-]+`)

// Indexer struct holds the reference to the database and other configurations
type Indexer struct {
	db  *database.DB
	dir string
}

// NewIndexer creates a new instance of Indexer
func NewIndexer(db *database.DB, dir string) *Indexer {
	return &Indexer{db: db, dir: dir}
}

func (indexer *Indexer) Index() error {
	changes, err := indexer.UpdateZettelIndex()
	if err != nil {
		return err
	}
	fmt.Println("Changes are:")
	fmt.Println(changes)
	err = indexer.UpdateLinkIndex(changes)
	if err != nil {
		return err
	}
	return nil
}

// UpdateZettelIndex scans a directory, identifies new or changed zettels, and updates the database.
// It returns a list of paths for zettels that were added or updated.
func (indexer *Indexer) UpdateZettelIndex() ([]string, error) {
	var changedZettels []string

	paths := indexer.FindPaths()

	for _, path := range paths {
		// Check if zettel exists in the database
		zettel, err := indexer.db.GetZettel(path)
		if err != nil && err != database.ErrZettelNotFound {
			return nil, fmt.Errorf("failed to check zettel in db: %w", err)
		}

		// Get file info (for the timestamp)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
		}
		lastUpdated := info.ModTime().Unix()

		// Determine if we need to update or insert this zettel
		if zettel != nil && zettel.LastUpdated >= lastUpdated {
			// No changes, skip to next file
			continue
		}

		// Compute checksum only if the file was modified since the timestamp in the database
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		checksum := utils.ComputeChecksum(content)

		if zettel != nil {
			// Update the existing zettel in the database
			zettel.Checksum = checksum
			zettel.LastUpdated = lastUpdated
			if err := indexer.db.UpdateZettel(*zettel); err != nil {
				return nil, fmt.Errorf("failed to update zettel: %w", err)
			}
		} else {
			// Insert a new zettel if it does not exist
			newZettel := database.Zettel{
				Path:        path,
				Checksum:    checksum,
				LastUpdated: lastUpdated,
			}
			if err := indexer.db.CreateZettel(newZettel); err != nil {
				return nil, fmt.Errorf("failed to create zettel: %w", err)
			}
		}

		// Add the changed zettel path to the list
		changedZettels = append(changedZettels, path)
	}

	return changedZettels, nil
}

func (indexer *Indexer) FindPaths() []string {
	// Use Glob to find all `.typ` files in the specified directory (non-recursive).
	pattern := filepath.Join(indexer.dir, "*.typ")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Error finding paths: %v\n", err)
		return nil
	}

	return matches
}

// UpdateLinkIndex takes a list of changed zettels and computes the links between them.
func (indexer *Indexer) UpdateLinkIndex(zettelPaths []string) error {
	for _, path := range zettelPaths {
		// Retrieve the zettel to get the content for link indexing
		zettel, err := indexer.db.GetZettel(path)
		if err != nil {
			return fmt.Errorf("failed to retrieve zettel %s: %w", path, err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read zettel file %s: %w", path, err)
		}

		// Parse references or links in the file content and update links in the database
		err = indexer.IndexLinks(content, zettel.ID)
		if err != nil {
			return fmt.Errorf("failed to index links for zettel %s: %w", path, err)
		}
	}

	return nil
}

func (indexer *Indexer) IndexLinks(content []byte, sourceID int) error {
	// Define the query to find references in the content

	// Use GetReferences to extract links from content
	references, err := parser.GetReferences(content)
	if err != nil {
		return fmt.Errorf("failed to extract references: %w", err)
	}

	for _, ref := range references {
		// Convert reference to a file path format
		refPath := indexer.Link2File(ref)

		// Retrieve the zettel by path from the database
		targetZettel, err := indexer.db.GetZettel(refPath)
		if err != nil {
			if err == database.ErrZettelNotFound {
				// Skip if the referenced zettel is not found
				continue
			}
			return fmt.Errorf("failed to retrieve referenced zettel: %w", err)
		}

		// Insert a link between sourceID and the target zettel's ID
		err = indexer.db.CreateLink(sourceID, targetZettel.ID)
		if err != nil {
			return fmt.Errorf("failed to insert link for zettel %s: %w", refPath, err)
		}
	}

	return nil
}

// Link2File converts a given reference to a file path
func (indexer *Indexer) Link2File(reference string) string {
	// Remove any invalid characters from the reference
	validRef := validCharPattern.ReplaceAllString(reference, "")
	path := strings.ReplaceAll(validRef, ".", "/")
	return filepath.Join(indexer.dir, path) + ".typ"
}
