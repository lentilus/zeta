package index

import (
	"aftermath/internal/database"
	"aftermath/internal/utils"
	"fmt"
	"os"
)

// processZettel checks if a zettel exists and updates or creates it as necessary
func (indexer *Indexer) processZettel(path string, changedZettels *[]string) error {
	zettel, err := indexer.db.GetZettel(path)
	if err != nil && err != database.ErrZettelNotFound {
		return fmt.Errorf("failed to check zettel in db: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	lastUpdated := info.ModTime().Unix()

	if zettel != nil && zettel.LastUpdated >= lastUpdated {
		return nil // No changes
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}
	checksum := utils.ComputeChecksum(content)

	if zettel != nil {
		zettel.Checksum = checksum
		zettel.LastUpdated = lastUpdated
		if err := indexer.db.UpdateZettel(*zettel); err != nil {
			return fmt.Errorf("failed to update zettel: %w", err)
		}
	} else {
		newZettel := database.Zettel{
			Path:        path,
			Checksum:    checksum,
			LastUpdated: lastUpdated,
		}
		if err := indexer.db.CreateZettel(newZettel); err != nil {
			return fmt.Errorf("failed to create zettel: %w", err)
		}
	}

	*changedZettels = append(*changedZettels, path)
	return nil
}
