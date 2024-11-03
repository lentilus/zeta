package database

import (
	"database/sql"
	"fmt"
)

// Database schema version
const SchemaVersion = 1

// DB struct to manage the SQLite connection and schema operations
type DB struct {
	Conn *sql.DB
}

// NewDB initializes a new SQLite database connection, creates tables if they don’t exist,
// and returns a DB struct with the connection.
func NewDB(dbPath string) (*DB, error) {
	// Open the SQLite database
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Initialize the DB struct
	db := &DB{Conn: conn}

	// Run database setup (creates tables if not present)
	if err := db.setup(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set up database: %w", err)
	}

	return db, nil
}

// setup checks the schema version, creates tables if they don’t exist, and runs migrations if needed
func (db *DB) setup() error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create tables if they don't exist
	if err := db.createTables(tx); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// createTables runs the SQL commands to create the necessary tables (zettels, links)
func (db *DB) createTables(tx *sql.Tx) error {
	// SQL command to create zettels table
	createZettelsTable := `
	CREATE TABLE IF NOT EXISTS zettels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		checksum TEXT NOT NULL
	);
	`

	// SQL command to create links table
	createLinksTable := `
	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL,
		target_id INTEGER NOT NULL,
		FOREIGN KEY (source_id) REFERENCES zettels(id) ON DELETE CASCADE,
		FOREIGN KEY (target_id) REFERENCES zettels(id) ON DELETE CASCADE
	);
	`

	// Execute SQL statements
	if _, err := tx.Exec(createZettelsTable); err != nil {
		return fmt.Errorf("failed to create zettels table: %w", err)
	}
	if _, err := tx.Exec(createLinksTable); err != nil {
		return fmt.Errorf("failed to create links table: %w", err)
	}

	// Check and set schema version
	if err := db.setSchemaVersion(tx, SchemaVersion); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	return nil
}

// setSchemaVersion stores the current schema version in the database (for migrations)
func (db *DB) setSchemaVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version))
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.Conn.Close()
}
