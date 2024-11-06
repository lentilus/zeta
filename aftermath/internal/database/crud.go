package database

import (
	"database/sql"
	"fmt"
)

var ErrZettelNotFound = fmt.Errorf("zettel does not exist in db")

// Helper function to perform transactions and execute SQL statements
func (db *DB) executeTransaction(query string, args ...interface{}) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the query
	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetZettel retrieves a zettel by path, or returns an error if it does not exist
func (db *DB) GetZettel(path string) (*Zettel, error) {
	var zettel Zettel
	query := `SELECT id, path, checksum, last_updated FROM zettels WHERE path = ?`
	err := db.Conn.QueryRow(query, path).
		Scan(&zettel.ID, &zettel.Path, &zettel.Checksum, &zettel.LastUpdated)

	if err == sql.ErrNoRows {
		// Zettel not found
		return nil, ErrZettelNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve zettel: %w", err)
	}

	return &zettel, nil
}

// CreateZettel creates a new zettel in the database.
func (db *DB) CreateZettel(zettel Zettel) error {
	createZettelSQL := `
		INSERT INTO zettels (path, checksum, last_updated)
		VALUES (?, ?, ?);
	`
	return db.executeTransaction(
		createZettelSQL,
		zettel.Path,
		zettel.Checksum,
		zettel.LastUpdated,
	)
}

// UpdateZettel updates the path, checksum, and last_updated fields of a zettel in the database.
func (db *DB) UpdateZettel(zettel Zettel) error {
	updateZettelSQL := `
		UPDATE zettels
		SET path = ?, checksum = ?, last_updated = ?
		WHERE id = ?;
	`
	return db.executeTransaction(
		updateZettelSQL,
		zettel.Path,
		zettel.Checksum,
		zettel.LastUpdated,
		zettel.ID,
	)
}

// CreateLink creates a new link between two zettels in the database.
func (db *DB) CreateLink(sourceID, targetID int) error {
	createLinkSQL := `
		INSERT INTO links (source_id, target_id)
		VALUES (?, ?);
	`
	return db.executeTransaction(
		createLinkSQL,
		sourceID,
		targetID,
	)
}

// DeleteLinks deletes all outgoing links from a specified zettel in the database.
func (db *DB) DeleteLinks(sourceID int) error {
	deleteLinksSQL := `
		DELETE FROM links
		WHERE source_id = ?;
	`
	return db.executeTransaction(deleteLinksSQL, sourceID)
}
