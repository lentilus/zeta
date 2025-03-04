package cache

import (
	"database/sql"
	"time"
    "fmt"

	_ "embed"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// Filecache implements the Cache interface using a SQLite database.
type Filecache struct {
	db *sql.DB
}

// NewFilecache opens (or creates) the SQLite database at the provided path,
// enables WAL mode, initializes the schema from the embedded file, and returns a Filecache.
func NewFilecache(dbPath string) (*Filecache, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable Write-Ahead Logging (WAL)
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, err
	}

	// Execute the embedded schema.
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
        fmt.Println("Error in shemaSQL")
		return nil, err
	}
	return &Filecache{db: db}, nil
}

// withTx is a helper function to execute a function within a transaction.
func (fc *Filecache) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := fc.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpsertNote inserts or updates a note and its associated links using a transaction.
func (fc *Filecache) UpsertNote(note Note, links []Link) error {
	return fc.withTx(func(tx *sql.Tx) error {
		now := time.Now().Unix()
		// Insert a new note or update the last_modified timestamp if it exists.
		if _, err := tx.Exec(`
            INSERT INTO notes (path, last_modified, on_disk) VALUES (?, ?, 1)
            ON CONFLICT(path) DO UPDATE SET last_modified = excluded.last_modified, on_disk = 1
        `, string(note), now); err != nil {
			return err
		}

		// Remove any existing links for this note.
		if _, err := tx.Exec(`DELETE FROM links WHERE source_path = ?`, string(note)); err != nil {
			return err
		}

		// Insert the provided links.
		for _, link := range links {
			if _, err := tx.Exec(`
                INSERT OR IGNORE INTO links (reference, row, col, source_path, target_path)
                VALUES (?, ?, ?, ?, ?)
            `, link.Reference, link.Row, link.Col, string(note), link.Target); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteNote removes a note from the database using a transaction.
func (fc *Filecache) DeleteNote(note Note) error {
	return fc.withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM notes WHERE path = ?`, string(note))
		return err
	})
}

// GetLastModified returns the last modification time of the note.
func (fc *Filecache) GetLastModified(note Note) (time.Time, error) {
	var ts int64
	err := fc.db.QueryRow(`SELECT last_modified FROM notes WHERE path = ?`, string(note)).Scan(&ts)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// getNotes is a helper function to retrieve notes from the database.
func (fc *Filecache) getNotes(query string, args ...interface{}) ([]Note, error) {
	rows, err := fc.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		notes = append(notes, Note(path))
	}
	return notes, rows.Err()
}

// GetExistingNotes retrieves notes that exist (on_disk = 1).
func (fc *Filecache) GetExistingNotes() ([]Note, error) {
	return fc.getNotes(`SELECT path FROM notes WHERE on_disk = 1`)
}

// GetMissingNotes retrieves notes marked as missing (on_disk = 0).
func (fc *Filecache) GetMissingNotes() ([]Note, error) {
	return fc.getNotes(`SELECT path FROM notes WHERE on_disk = 0`)
}

// GetAllNotes returns all notes.
func (fc *Filecache) GetAllNotes() ([]Note, error) {
	return fc.getNotes(`SELECT path FROM notes`)
}

// getLinks is a helper function to retrieve links from the database.
func (fc *Filecache) getLinks(query string, args ...interface{}) ([]Link, error) {
	rows, err := fc.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		var src, tgt string
		if err := rows.Scan(&l.Reference, &l.Row, &l.Col, &src, &tgt); err != nil {
			return nil, err
		}
		l.Source = src
		l.Target = tgt
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetForwardLinks returns links originating from the specified note.
func (fc *Filecache) GetForwardLinks(source Note) ([]Link, error) {
	return fc.getLinks(`SELECT reference, row, col, source_path, target_path FROM links WHERE source_path = ?`, string(source))
}

// GetBackLinks returns links pointing to the specified note.
func (fc *Filecache) GetBackLinks(target Note) ([]Link, error) {
	return fc.getLinks(`SELECT reference, row, col, source_path, target_path FROM links WHERE target_path = ?`, string(target))
}

func (fc *Filecache) Subscribe() (<-chan ChangeLogEvent, error) {
    return nil, nil
}

