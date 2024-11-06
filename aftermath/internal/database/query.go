package database

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// GetAllZettels retrieves all zettel paths from the database.
func (db *DB) GetAllZettels() ([]string, error) {
	query := `SELECT path FROM zettels`
	return db.getStringsFromQuery(query)
}

// GetForwardLinks retrieves all forward links for a zettel based on its path, returning the paths of the linked zettels.
func (db *DB) GetForwardLinks(sourcePath string) ([]string, error) {
	query := `
		SELECT z2.path 
		FROM links l
		JOIN zettels z1 ON l.source_id = z1.id
		JOIN zettels z2 ON l.target_id = z2.id
		WHERE z1.path = ?`
	return db.getStringsFromQuery(query, sourcePath)
}

// GetBackLinks retrieves all backward links for a specific zettel, returning the paths of the linking zettels.
func (db *DB) GetBackLinks(targetPath string) ([]string, error) {
	query := `
		SELECT z1.path
		FROM links l
		JOIN zettels z1 ON l.source_id = z1.id
		JOIN zettels z2 ON l.target_id = z2.id
		WHERE z2.path = ?`
	return db.getStringsFromQuery(query, targetPath)
}

// Helper function to execute a query and return a list of strings
func (db *DB) getStringsFromQuery(query string, args ...interface{}) ([]string, error) {
	rows, err := db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return results, nil
}
