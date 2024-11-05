package database

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// GetPathByID retrieves the path of a zettel given its ID.
func (db *DB) GetPathByID(zettelID int) (string, error) {
	var path string
	query := `SELECT path FROM zettels WHERE id = ?`
	err := db.Conn.QueryRow(query, zettelID).Scan(&path)
	if err != nil {
		return "", fmt.Errorf("failed to get path for zettel with ID %d: %w", zettelID, err)
	}
	return path, nil
}

// GetAllZettels retrieves all zettel paths from the database.
func (db *DB) GetAllZettels() ([]string, error) {
	query := `SELECT path FROM zettels`
	rows, err := db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all zettels: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan path: %w", err)
		}
		paths = append(paths, path)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return paths, nil
}

// GetForwardLinks retrieves all forward links for a specific zettel, returning the paths of the linked zettels.
func (db *DB) GetForwardLinks(sourceID int) ([]string, error) {
	// First, get the target IDs for the forward links
	query := `SELECT target_id FROM links WHERE source_id = ?`
	rows, err := db.Conn.Query(query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get forward links: %w", err)
	}
	defer rows.Close()

	var targetIDs []int
	for rows.Next() {
		var targetID int
		if err := rows.Scan(&targetID); err != nil {
			return nil, fmt.Errorf("failed to scan target_id: %w", err)
		}
		targetIDs = append(targetIDs, targetID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	// Get the paths for the IDs
	var paths []string
	for _, targetID := range targetIDs {
		path, err := db.GetPathByID(targetID)
		if err != nil {
			return nil, fmt.Errorf("failed to get path for target_id %d: %w", targetID, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// GetBackLinks retrieves all backward links for a specific zettel, returning the paths of the linking zettels.
func (db *DB) GetBackLinks(targetID int) ([]string, error) {
	// First, get the source IDs for the backward links
	query := `SELECT source_id FROM links WHERE target_id = ?`
	rows, err := db.Conn.Query(query, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backward links: %w", err)
	}
	defer rows.Close()

	var sourceIDs []int
	for rows.Next() {
		var sourceID int
		if err := rows.Scan(&sourceID); err != nil {
			return nil, fmt.Errorf("failed to scan source_id: %w", err)
		}
		sourceIDs = append(sourceIDs, sourceID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	// Get the paths for the IDs
	var paths []string
	for _, sourceID := range sourceIDs {
		path, err := db.GetPathByID(sourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get path for source_id %d: %w", sourceID, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}
