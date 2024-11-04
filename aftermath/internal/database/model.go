package database

import "database/sql"

// DB struct to manage the SQLite connection and schema operations
type DB struct {
	Conn *sql.DB
}

// Zettel represents a single zettel in the database.
type Zettel struct {
	ID       int
	Path     string
	Checksum []byte
}

// Link represents a relationship between two zettels in the database.
type Link struct {
	ID       int
	SourceID int
	TargetID int
}
