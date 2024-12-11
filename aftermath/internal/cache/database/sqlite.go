package database

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDB struct {
	db *sql.DB
}

func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and WAL mode
	if _, err := db.Exec(`
        PRAGMA foreign_keys = ON;
        PRAGMA journal_mode = WAL;
    `); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set PRAGMA: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

func (db *SQLiteDB) WithTx(fn func(Transaction) error) error {
	tx, err := db.db.Begin()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}
	defer tx.Rollback()

	if err := fn(&SQLiteTx{tx}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}

	return nil
}

func (db *SQLiteDB) GetFile(path string) (*FileRecord, error) {
	var record FileRecord
	err := db.db.QueryRow(
		"SELECT path, last_modified FROM files WHERE path = ?",
		path,
	).Scan(&record.Path, &record.LastModified)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query file: %w", err)
	}

	return &record, nil
}

func (db *SQLiteDB) GetAllFiles() ([]FileRecord, error) {
	rows, err := db.db.Query("SELECT path, last_modified FROM files")
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		var record FileRecord
		if err := rows.Scan(&record.Path, &record.LastModified); err != nil {
			return nil, fmt.Errorf("failed to scan file record: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file records: %w", err)
	}

	return records, nil
}

func (db *SQLiteDB) UpsertFile(file *FileRecord) error {
	result, err := db.db.Exec(`
        INSERT INTO files (path, last_modified, file_exists)
        VALUES (?, ?, 1)
        ON CONFLICT(path) DO UPDATE SET
            last_modified = excluded.last_modified,
            file_exists = 1
    `, file.Path, file.LastModified)

	if err != nil {
		return fmt.Errorf("failed to upsert file: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return ErrConstraintViolation
	}

	return nil
}

func (db *SQLiteDB) DeleteFile(path string) error {
	result, err := db.db.Exec("DELETE FROM files WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (db *SQLiteDB) GetLinks(sourcePath string) ([]LinkRecord, error) {
	rows, err := db.db.Query(`
        SELECT source_path, target_path 
        FROM links 
        WHERE source_path = ?
    `, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query links: %w", err)
	}
	defer rows.Close()

	return scanLinkRecords(rows)
}

func (db *SQLiteDB) GetBacklinks(targetPath string) ([]LinkRecord, error) {
	rows, err := db.db.Query(`
        SELECT source_path, target_path 
        FROM links 
        WHERE target_path = ?
    `, targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query backlinks: %w", err)
	}
	defer rows.Close()

	return scanLinkRecords(rows)
}

func (db *SQLiteDB) UpsertLinks(sourcePath string, targetPaths []string) error {
	return db.WithTx(func(tx Transaction) error {
		return tx.UpsertLinks(sourcePath, targetPaths)
	})
}

func (db *SQLiteDB) DeleteLinks(sourcePath string) error {
	result, err := db.db.Exec("DELETE FROM links WHERE source_path = ?", sourcePath)
	if err != nil {
		return fmt.Errorf("failed to delete links: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (db *SQLiteDB) Clear() error {
	_, err := db.db.Exec(`
        DELETE FROM links;
        DELETE FROM files;
    `)
	if err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}
	return nil
}

func (db *SQLiteDB) Close() error {
	if _, err := db.db.Exec("DELETE FROM files WHERE file_exists = 0"); err != nil {
		return fmt.Errorf("failed to clean up non-existent files: %w", err)
	}
	return db.db.Close()
}

func scanLinkRecords(rows *sql.Rows) ([]LinkRecord, error) {
	var records []LinkRecord
	for rows.Next() {
		var record LinkRecord
		if err := rows.Scan(&record.SourcePath, &record.TargetPath); err != nil {
			return nil, fmt.Errorf("failed to scan link record: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating link records: %w", err)
	}

	return records, nil
}
