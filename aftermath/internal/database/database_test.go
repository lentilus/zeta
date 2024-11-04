package database_test

import (
	"aftermath/internal/database"
	"testing"
)

// TestUpdateZettel verifies that the UpdateZettel function updates a zettel correctly.
func TestUpdateZettel(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert a zettel
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"original_path",
		[]byte("original_checksum"),
	)
	if err != nil {
		t.Fatalf("failed to insert initial zettel: %v", err)
	}

	// Update zettel using the struct
	zettel := database.Zettel{ID: 1, Path: "new_path", Checksum: []byte("new_checksum")}
	err = db.UpdateZettel(zettel)
	if err != nil {
		t.Fatalf("UpdateZettel failed: %v", err)
	}

	// Verify update
	var path string
	var checksum []byte
	row := db.Conn.QueryRow("SELECT path, checksum FROM zettels WHERE id = ?", 1)
	if err := row.Scan(&path, &checksum); err != nil {
		t.Fatalf("failed to query updated zettel: %v", err)
	}
	if path != "new_path" || string(checksum) != "new_checksum" {
		t.Errorf("expected (new_path, new_checksum), got (%s, %s)", path, checksum)
	}
}

// TestUpdateLink verifies that the UpdateLink function updates a link correctly.
func TestUpdateLink(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert two zettels
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"path1",
		"checksum1",
	)
	if err != nil {
		t.Fatalf("failed to insert initial zettel: %v", err)
	}
	_, err = db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"path2",
		"checksum2",
	)
	if err != nil {
		t.Fatalf("failed to insert second zettel: %v", err)
	}

	// Insert a link
	_, err = db.Conn.Exec("INSERT INTO links (source_id, target_id) VALUES (?, ?)", 1, 2)
	if err != nil {
		t.Fatalf("failed to insert initial link: %v", err)
	}

	// Update link using the struct
	link := database.Link{ID: 1, SourceID: 2, TargetID: 1}
	err = db.UpdateLink(link)
	if err != nil {
		t.Fatalf("UpdateLink failed: %v", err)
	}

	// Verify update
	var sourceID, targetID int
	row := db.Conn.QueryRow("SELECT source_id, target_id FROM links WHERE id = ?", 1)
	if err := row.Scan(&sourceID, &targetID); err != nil {
		t.Fatalf("failed to query updated link: %v", err)
	}
	if sourceID != 2 || targetID != 1 {
		t.Errorf("expected (2, 1), got (%d, %d)", sourceID, targetID)
	}
}
