package database

import (
	"database/sql"
	"fmt"
)

type SQLiteTx struct {
	tx *sql.Tx
}

func (tx *SQLiteTx) UpsertFile(file *FileRecord) error {
	_, err := tx.tx.Exec(`
        INSERT INTO files (path, last_modified, file_exists)
        VALUES (?, ?, 1)
        ON CONFLICT(path) DO UPDATE SET
            last_modified = excluded.last_modified,
            file_exists = 1
    `, file.Path, file.LastModified)

	if err != nil {
		return fmt.Errorf("failed to upsert file in transaction: %w", err)
	}

	return nil
}

func (tx *SQLiteTx) UpsertLinks(sourcePath string, targetPaths []string) error {
	// Delete existing links
	_, err := tx.tx.Exec("DELETE FROM links WHERE source_path = ?", sourcePath)
	if err != nil {
		return fmt.Errorf("failed to delete existing links: %w", err)
	}

	// If no new links, we're done
	if len(targetPaths) == 0 {
		return nil
	}

	// Ensure source file exists if not already in database
	_, err = tx.tx.Exec(`
        INSERT INTO files (path, last_modified, file_exists)
        VALUES (?, 0, 0)
        ON CONFLICT(path) DO NOTHING
    `, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to ensure source file exists: %w", err)
	}

	// Prepare statement for inserting new links
	stmt, err := tx.tx.Prepare(
		"INSERT INTO links (source_path, target_path) VALUES (?, ?)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare link insert statement: %w", err)
	}
	defer stmt.Close()

	// Insert new links and ensure target files exist
	for _, targetPath := range targetPaths {
		// Ensure target file exists if not already in database
		_, err = tx.tx.Exec(`
            INSERT INTO files (path, last_modified, file_exists)
            VALUES (?, 0, 0)
            ON CONFLICT(path) DO NOTHING
        `, targetPath)
		if err != nil {
			return fmt.Errorf("failed to ensure target file exists: %w", err)
		}

		// Insert the link
		if _, err := stmt.Exec(sourcePath, targetPath); err != nil {
			return fmt.Errorf("failed to insert link: %w", err)
		}
	}

	return nil
}
