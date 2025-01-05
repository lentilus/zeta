package database

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 4

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
		// Main table storing all zettels (notes)
		// - path: unique identifier and file path of the zettel
		// - last_modified: timestamp to detect file changes
		// - file_exists: flag to handle deleted files without removing their relationships
		`CREATE TABLE IF NOT EXISTS files (
            path TEXT PRIMARY KEY,
            last_modified INTEGER NOT NULL,
            file_exists INTEGER NOT NULL DEFAULT 1
        )`,

		// Represents connections between zettels through references/links
		// Both source and target must exist in the files table
		// Links are automatically removed when either zettel is deleted (CASCADE)
		`CREATE TABLE IF NOT EXISTS links (
            source_path TEXT NOT NULL,
            target_path TEXT NOT NULL,
            FOREIGN KEY (source_path) REFERENCES files(path) ON DELETE CASCADE,
            FOREIGN KEY (target_path) REFERENCES files(path) ON DELETE CASCADE,
            PRIMARY KEY (source_path, target_path)
        )`,

		// Index to efficiently find backlinks (which zettels reference a given zettel)
		`CREATE INDEX IF NOT EXISTS idx_links_target 
            ON links(target_path)`,

		// Maintains an ordered list of all zettels for the bibliography
		// The id serves as the entry order and is used to track what has been written
		// Entries are automatically removed when the corresponding zettel is deleted
		`CREATE TABLE IF NOT EXISTS bibliography (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            path TEXT NOT NULL,
            FOREIGN KEY (path) REFERENCES files(path) ON DELETE CASCADE
        )`,

		// Stores system-wide settings and state
		// Currently used to track the last bibliography entry that was written to disk
		`CREATE TABLE IF NOT EXISTS metadata (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL
        )`,

		// Initialize the bibliography tracker
		// A value of 0 indicates that the bibliography file needs to be completely rewritten
		`INSERT OR IGNORE INTO metadata (key, value) VALUES ('last_bibliography_id', '0')`,

		// Automatically add new zettels to the bibliography when they are created
		// The AUTOINCREMENT id in bibliography will maintain the order of addition
		`CREATE TRIGGER IF NOT EXISTS after_file_insert
        AFTER INSERT ON files
        WHEN NEW.file_exists = 1
        BEGIN
            INSERT INTO bibliography (path)
            VALUES (NEW.path);
        END`,

		// If exists is updated
		`CREATE TRIGGER IF NOT EXISTS after_file_update
        AFTER UPDATE OF file_exists ON files
        WHEN NEW.file_exists = 1 AND OLD.file_exists = 0
        BEGIN
            INSERT INTO bibliography (path)
            VALUES (NEW.path);
        END`,

		// Reset the bibliography tracker when an already-written entry is deleted
		// This ensures the bibliography file will be completely rewritten
		// to maintain the correct order of entries
		`CREATE TRIGGER IF NOT EXISTS after_bibliography_delete
         AFTER DELETE ON bibliography
         WHEN EXISTS (
             SELECT 1 FROM metadata 
             WHERE key = 'last_bibliography_id' 
             AND CAST(value AS INTEGER) >= OLD.id
         )
         BEGIN
             UPDATE metadata 
             SET value = '0' 
             WHERE key = 'last_bibliography_id';
         END`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", query, err)
		}
	}

	return nil
}
