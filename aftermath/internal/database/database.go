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

// UpdateLink updates the source and target IDs of an existing link in the database.
func (db *DB) UpdateLink(link Link) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	updateLinkSQL := `
		UPDATE links
		SET source_id = ?, target_id = ?
		WHERE id = ?;
	`
	if _, err := tx.Exec(updateLinkSQL, link.SourceID, link.TargetID, link.ID); err != nil {
		return fmt.Errorf("failed to update link: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
