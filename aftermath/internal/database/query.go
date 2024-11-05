package database

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// GetAllZettels retrieves all zettels from the database.
func (db *DB) GetAllZettels() ([]Zettel, error) {
	query := `SELECT id, path, checksum, last_updated FROM zettels`
	rows, err := db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all zettels: %w", err)
	}
	defer rows.Close()

	var zettels []Zettel
	for rows.Next() {
		var zettel Zettel
		if err := rows.Scan(&zettel.ID, &zettel.Path, &zettel.Checksum, &zettel.LastUpdated); err != nil {
			return nil, fmt.Errorf("failed to scan zettel: %w", err)
		}
		zettels = append(zettels, zettel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return zettels, nil
}

// GetForwardLinks retrieves all forward links for a specific zettel identified by its ID.
func (db *DB) GetForwardLinks(sourceID int) ([]Link, error) {
	query := `SELECT id, source_id, target_id FROM links WHERE source_id = ?`
	rows, err := db.Conn.Query(query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get forward links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.SourceID, &link.TargetID); err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return links, nil
}

// GetBackLinks retrieves all backward links for a specific zettel identified by its ID.
func (db *DB) GetBackLinks(targetID int) ([]Link, error) {
	query := `SELECT id, source_id, target_id FROM links WHERE target_id = ?`
	rows, err := db.Conn.Query(query, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backward links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.SourceID, &link.TargetID); err != nil {
			return nil, fmt.Errorf("failed to scan link: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered while iterating over rows: %w", err)
	}

	return links, nil
}
