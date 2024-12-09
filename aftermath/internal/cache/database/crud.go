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

// CreateLinkByPaths creates a new link between two zettels in the database using their paths.
func (db *DB) CreateLink(sourcePath, targetPath string) error {
	createLinkSQL := `
		INSERT INTO links (source_id, target_id)
		SELECT s.id, t.id
		FROM zettels s
		JOIN zettels t ON t.path = ?
		WHERE s.path = ?;
	`

	result, err := db.Conn.Exec(createLinkSQL, targetPath, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	// Check if the link was created by verifying affected rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("link creation failed: one or both paths do not exist")
	}

	return nil
}

// DeleteLinks deletes all outgoing links from a specified zettel path in the database.
func (db *DB) DeleteLinks(path string) error {
	deleteLinksSQL := `
		DELETE FROM links
		WHERE source_id = (
			SELECT id FROM zettels WHERE path = ? LIMIT 1);
	`
	return db.executeTransaction(deleteLinksSQL, path)
}

// UpsertZettel inserts a new zettel or updates it if it already exists.
func (db *DB) UpsertZettel(zettel Zettel) error {
	query := `
		INSERT INTO zettels (path, checksum, last_updated)
		VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			checksum = excluded.checksum,
			last_updated = excluded.last_updated;
	`
	return db.executeTransaction(query, zettel.Path, zettel.Checksum, zettel.LastUpdated)
}

// UpsertMetadata inserts or updates metadata based on the key.
func (db *DB) UpsertMetadata(key string, value []byte) error {
	upsertMetadataSQL := `
		INSERT INTO metadata ("key", value) 
		VALUES (?, ?)
		ON CONFLICT("key") 
		DO UPDATE SET value = excluded.value;
	`
	return db.executeTransaction(upsertMetadataSQL, key, value)
}

// GetMetadata retrieves metadata based on the key.
func (db *DB) GetMetadata(key string) ([]byte, error) {
	var value []byte

	// SQL command to retrieve metadata by key
	getMetadataSQL := `SELECT value FROM metadata WHERE "key" = ?`

	// Execute the query
	err := db.Conn.QueryRow(getMetadataSQL, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metadata not found for key: %s", key)
		}
		return nil, fmt.Errorf("failed to retrieve metadata: %w", err)
	}

	return value, nil
}
