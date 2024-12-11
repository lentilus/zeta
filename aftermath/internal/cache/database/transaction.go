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
        INSERT INTO files (path, last_modified)
        VALUES (?, ?)
        ON CONFLICT(path) DO UPDATE SET
            last_modified = excluded.last_modified
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

	// Prepare statement for inserting new links
	stmt, err := tx.tx.Prepare(
		"INSERT INTO links (source_path, target_path) VALUES (?, ?)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare link insert statement: %w", err)
	}
	defer stmt.Close()

	// Insert new links
	for _, targetPath := range targetPaths {
		if _, err := stmt.Exec(sourcePath, targetPath); err != nil {
			return fmt.Errorf("failed to insert link: %w", err)
		}
	}

	return nil
}
