package database

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 2

func initSchema(db *sql.DB) error {
	// Check schema version
	var version int
	err := db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	if version == schemaVersion {
		return nil
	}

	// Create or update schema
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create tables
	if err := createTables(tx); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Update schema version
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("failed to update schema version: %w", err)
	}

	return tx.Commit()
}

func createTables(tx *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS files (
            path TEXT PRIMARY KEY,
            last_modified INTEGER NOT NULL,
            file_exists INTEGER NOT NULL DEFAULT 1
        )`,
		`CREATE TABLE IF NOT EXISTS links (
            source_path TEXT NOT NULL,
            target_path TEXT NOT NULL,
            FOREIGN KEY (source_path) REFERENCES files(path) ON DELETE CASCADE,
            FOREIGN KEY (target_path) REFERENCES files(path) ON DELETE CASCADE,
            PRIMARY KEY (source_path, target_path)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_links_target 
            ON links(target_path)`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", query, err)
		}
	}

	return nil
}
