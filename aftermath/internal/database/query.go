package database

import (
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// GetAllZettels retrieves all zettel paths from the database.
func (db *DB) GetAllZettels() ([]string, error) {
	query := `SELECT path FROM zettels`
	return db.getStringsFromQuery(query)
}

// GetAll loads all zettels from the database and stores them in a map keyed by path for fast retrieval.
func (db *DB) GetAll() (map[string]Zettel, error) {
	query := `SELECT id, path, checksum, last_updated FROM zettels`

	rows, err := db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve zettels: %w", err)
	}
	defer rows.Close()

	zettelMap := make(map[string]Zettel)
	for rows.Next() {
		var z Zettel
		if err := rows.Scan(&z.ID, &z.Path, &z.Checksum, &z.LastUpdated); err != nil {
			return nil, fmt.Errorf("failed to scan zettel: %w", err)
		}
		// Use z.Path as the key for quick lookup by path
		zettelMap[z.Path] = z
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return zettelMap, nil
}

// DeleteZettels deletes zettels from the database based on a list of IDs.
func (db *DB) DeleteZettels(ids []int) error {
	if len(ids) == 0 {
		return nil // No IDs provided, nothing to delete.
	}

	// Prepare placeholders for the SQL query.
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?" // Add a placeholder for each ID
		args[i] = id          // Add the ID as an argument
	}

	query := fmt.Sprintf(`DELETE FROM zettels WHERE id IN (%s)`, strings.Join(placeholders, ","))

	// Execute the delete query with the provided IDs.
	_, err := db.Conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete zettels: %w", err)
	}

	return nil
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
