package database_test

import (
	"aftermath/internal/database"
	"testing"
)

// TestCreateZettel verifies that the CreateZettel function creates a zettel correctly.
func TestCreateZettel(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Define the zettel parameters
	path := "test_create_path"
	checksum := []byte("test_create_checksum")

	// Create the zettel using the CreateZettel function
	err := db.CreateZettel(path, checksum)
	if err != nil {
		t.Fatalf("CreateZettel failed: %v", err)
	}

	// Verify that the zettel has been inserted into the database
	var retrievedZettel database.Zettel
	query := `SELECT id, path, checksum FROM zettels WHERE path = ?`
	err = db.Conn.QueryRow(query, path).
		Scan(&retrievedZettel.ID, &retrievedZettel.Path, &retrievedZettel.Checksum)
	if err != nil {
		t.Fatalf("failed to query created zettel: %v", err)
	}

	// Verify the retrieved zettel matches the input
	if retrievedZettel.Path != path {
		t.Errorf("expected path %s, got %s", path, retrievedZettel.Path)
	}
	if string(retrievedZettel.Checksum) != string(checksum) {
		t.Errorf("expected checksum %s, got %s", checksum, retrievedZettel.Checksum)
	}

	// Check for duplicate creation
	err = db.CreateZettel(path, checksum)
	if err == nil {
		t.Fatal("expected error for duplicate zettel creation, got nil")
	}
}

// TestGetZettel verifies that the GetZettel function retrieves a zettel correctly.
func TestGetZettel(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert a zettel to test retrieval
	path := "test_path"
	checksum := []byte("test_checksum")
	if err := db.CreateZettel(path, checksum); err != nil {
		t.Fatalf("failed to create test zettel: %v", err)
	}

	// Retrieve the zettel using the GetZettel function
	retrievedZettel, err := db.GetZettel(path)
	if err != nil {
		t.Fatalf("GetZettel failed: %v", err)
	}

	// Verify the retrieved zettel matches the inserted zettel
	if retrievedZettel.Path != path {
		t.Errorf("expected path %s, got %s", path, retrievedZettel.Path)
	}
	if string(retrievedZettel.Checksum) != string(checksum) {
		t.Errorf("expected checksum %s, got %s", checksum, retrievedZettel.Checksum)
	}

	// Test for a non-existent zettel
	_, err = db.GetZettel("non_existent_path")
	if err != database.ErrZettelNotFound {
		t.Fatalf("expected ErrZettelNotFound, got: %v", err)
	}
}

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

// TestCreateLink verifies that the CreateLink function creates a link correctly.
func TestCreateLink(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert two zettels
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"path1",
		"checksum1",
	)
	if err != nil {
		t.Fatalf("failed to insert first zettel: %v", err)
	}
	_, err = db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"path2",
		"checksum2",
	)
	if err != nil {
		t.Fatalf("failed to insert second zettel: %v", err)
	}

	// Create a link between the two zettels
	err = db.CreateLink(1, 2)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Verify the link was created
	var sourceID, targetID int
	row := db.Conn.QueryRow(
		"SELECT source_id, target_id FROM links WHERE source_id = ? AND target_id = ?",
		1,
		2,
	)
	if err := row.Scan(&sourceID, &targetID); err != nil {
		t.Fatalf("failed to query created link: %v", err)
	}
	if sourceID != 1 || targetID != 2 {
		t.Errorf("expected (1, 2), got (%d, %d)", sourceID, targetID)
	}
}

// TestDeleteLinks verifies that the DeleteLinks function deletes all outgoing links correctly.
func TestDeleteLinks(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert a zettel
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum) VALUES (?, ?)",
		"path1",
		"checksum1",
	)
	if err != nil {
		t.Fatalf("failed to insert zettel: %v", err)
	}

	// Create some links
	err = db.CreateLink(1, 2)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}
	err = db.CreateLink(1, 3)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Delete links for zettel with ID 1
	err = db.DeleteLinks(1)
	if err != nil {
		t.Fatalf("DeleteLinks failed: %v", err)
	}

	// Verify that the links were deleted
	var count int
	row := db.Conn.QueryRow("SELECT COUNT(*) FROM links WHERE source_id = ?", 1)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count links: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 links, got %d", count)
	}
}
