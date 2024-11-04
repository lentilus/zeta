package database

import (
	"fmt"
)

// UpdateZettel updates the path and checksum of a zettel in the database.
func (db *DB) UpdateZettel(zettel Zettel) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	updateZettelSQL := `
		UPDATE zettels
		SET path = ?, checksum = ?
		WHERE id = ?;
	`
	if _, err := tx.Exec(updateZettelSQL, zettel.Path, zettel.Checksum, zettel.ID); err != nil {
		return fmt.Errorf("failed to update zettel: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreateLink creates a new link between two zettels in the database.
func (db *DB) CreateLink(sourceID, targetID int) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	createLinkSQL := `
		INSERT INTO links (source_id, target_id)
		VALUES (?, ?);
	`
	if _, err := tx.Exec(createLinkSQL, sourceID, targetID); err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteLinks deletes all outgoing links from a specified zettel in the database.
func (db *DB) DeleteLinks(sourceID int) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	deleteLinksSQL := `
		DELETE FROM links
		WHERE source_id = ?;
	`
	if _, err := tx.Exec(deleteLinksSQL, sourceID); err != nil {
		return fmt.Errorf("failed to delete links: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
