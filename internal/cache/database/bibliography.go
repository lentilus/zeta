package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"zeta/internal/bibliography"
)

func (db *SQLiteDB) syncBib() error {
	db.mu.Lock()         // Lock the mutex
	defer db.mu.Unlock() // Ensure the mutex is unlocked on method exit

	// Get last written ID
	var lastID int64
	err := db.db.QueryRow(`
        SELECT CAST(value AS INTEGER) 
        FROM metadata 
        WHERE key = 'last_bibliography_id'
    `).Scan(&lastID)
	if err == sql.ErrNoRows {
		lastID = 0
	} else if err != nil {
		return fmt.Errorf("failed to get last bibliography ID: %w", err)
	}

	// If lastID is 0, we need to rewrite the entire bibliography
	if lastID == 0 {
		return db.overrideBib()
	}

	return db.appendBib(lastID)
}

func (db *SQLiteDB) overrideBib() error {
	rows, err := db.db.Query(`
        SELECT path, id 
        FROM bibliography 
        ORDER BY id
    `)
	if err != nil {
		return fmt.Errorf("failed to query all entries: %w", err)
	}
	defer rows.Close()

	var entries []bibliography.Entry
	var maxID int64
	for rows.Next() {
		var path string
		var id int64
		if err := rows.Scan(&path, &id); err != nil {
			return fmt.Errorf("failed to scan path: %w", err)
		}
		target, _ := filepath.Rel(db.root, path)
		entries = append(entries, bibliography.Entry{
			Target: strings.TrimSuffix(target, ".typ"),
			Title:  target,
			Path:   target,
		})
		maxID = id
	}

	if err := db.bib.Override(entries); err != nil {
		return err
	}

	// Update last written ID
	_, err = db.db.Exec(`
        UPDATE metadata 
        SET value = ? 
        WHERE key = 'last_bibliography_id'
    `, maxID)
	return err
}

func (db *SQLiteDB) appendBib(lastID int64) error {
	rows, err := db.db.Query(`
        SELECT path, id 
        FROM bibliography 
        WHERE id > ?
        ORDER BY id
    `, lastID)
	if err != nil {
		return fmt.Errorf("failed to query new entries: %w", err)
	}
	defer rows.Close()

	var entries []bibliography.Entry
	var maxID int64
	for rows.Next() {
		var path string
		var id int64
		if err := rows.Scan(&path, &id); err != nil {
			return fmt.Errorf("failed to scan path: %w", err)
		}
		target, _ := filepath.Rel(db.root, path)
		entries = append(entries, bibliography.Entry{
			Target: strings.TrimSuffix(target, ".typ"),
			Title:  target,
			Path:   target,
		})
		maxID = id
	}

	if len(entries) == 0 {
		return nil
	}

	if err := db.bib.Append(entries); err != nil {
		return err
	}

	// Update last written ID
	_, err = db.db.Exec(`
        UPDATE metadata 
        SET value = ? 
        WHERE key = 'last_bibliography_id'
    `, maxID)
	return err
}
