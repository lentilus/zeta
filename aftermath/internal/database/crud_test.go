package database_test

import (
	"aftermath/internal/database"
	"testing"
	"time"
)

// TestCreateZettel verifies that the CreateZettel function creates a zettel correctly.
func TestCreateZettel(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Define the zettel parameters
	path := "test_create_path"
	checksum := []byte("test_create_checksum")
	lastUpdated := time.Now().Unix() // Use the current time as last_updated

	// Create the zettel using the CreateZettel function
	err := db.CreateZettel(
		database.Zettel{Path: path, Checksum: checksum, LastUpdated: lastUpdated},
	)
	if err != nil {
		t.Fatalf("CreateZettel failed: %v", err)
	}

	// Verify that the zettel has been inserted into the database
	var retrievedZettel database.Zettel
	query := `SELECT id, path, checksum, last_updated FROM zettels WHERE path = ?`
	err = db.Conn.QueryRow(query, path).
		Scan(&retrievedZettel.ID, &retrievedZettel.Path, &retrievedZettel.Checksum, &retrievedZettel.LastUpdated)
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
	if retrievedZettel.LastUpdated != lastUpdated {
		t.Errorf("expected last_updated %d, got %d", lastUpdated, retrievedZettel.LastUpdated)
	}

	// Check for duplicate creation
	err = db.CreateZettel(database.Zettel{Path: path, Checksum: checksum, LastUpdated: lastUpdated})
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
	lastUpdated := time.Now().Unix()
	err := db.CreateZettel(
		database.Zettel{Path: path, Checksum: checksum, LastUpdated: lastUpdated},
	)
	if err != nil {
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
	if retrievedZettel.LastUpdated != lastUpdated {
		t.Errorf("expected last_updated %d, got %d", lastUpdated, retrievedZettel.LastUpdated)
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
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"original_path",
		[]byte("original_checksum"),
		time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("failed to insert initial zettel: %v", err)
	}

	// Update zettel using the struct
	lastUpdated := time.Now().Unix()
	zettel := database.Zettel{
		ID:          1,
		Path:        "new_path",
		Checksum:    []byte("new_checksum"),
		LastUpdated: lastUpdated,
	}
	err = db.UpdateZettel(zettel)
	if err != nil {
		t.Fatalf("UpdateZettel failed: %v", err)
	}

	// Verify update
	var path string
	var checksum []byte
	var updatedTimestamp int64
	row := db.Conn.QueryRow("SELECT path, checksum, last_updated FROM zettels WHERE id = ?", 1)
	if err := row.Scan(&path, &checksum, &updatedTimestamp); err != nil {
		t.Fatalf("failed to query updated zettel: %v", err)
	}
	if path != "new_path" || string(checksum) != "new_checksum" || updatedTimestamp != lastUpdated {
		t.Errorf(
			"expected (new_path, new_checksum, %d), got (%s, %s, %d)",
			lastUpdated,
			path,
			checksum,
			updatedTimestamp,
		)
	}
}

func TestCreateLinkByPaths(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Example UNIX timestamp
	lastUpdated := int64(1630454400)

	// Insert two zettels with last_updated timestamps
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"path1",
		"checksum1",
		lastUpdated,
	)
	if err != nil {
		t.Fatalf("failed to insert first zettel: %v", err)
	}

	_, err = db.Conn.Exec(
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"path2",
		"checksum2",
		lastUpdated,
	)
	if err != nil {
		t.Fatalf("failed to insert second zettel: %v", err)
	}

	// Create a link between the two zettels using paths
	err = db.CreateLink("path1", "path2")
	if err != nil {
		t.Fatalf("CreateLinkByPaths failed: %v", err)
	}

	// Verify that the link was created
	var sourceID, targetID int
	row := db.Conn.QueryRow(
		"SELECT source_id, target_id FROM links WHERE source_id = (SELECT id FROM zettels WHERE path = ?) AND target_id = (SELECT id FROM zettels WHERE path = ?)",
		"path1",
		"path2",
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

	// Example UNIX timestamp
	lastUpdated := int64(1630454400)

	// Insert a zettel with last_updated
	_, err := db.Conn.Exec(
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"path1",
		"checksum1",
		lastUpdated,
	)
	if err != nil {
		t.Fatalf("failed to insert zettel: %v", err)
	}

	// Insert other zettels to link to
	_, err = db.Conn.Exec(
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"path2",
		"checksum2",
		lastUpdated,
	)
	if err != nil {
		t.Fatalf("failed to insert second zettel: %v", err)
	}
	_, err = db.Conn.Exec(
		"INSERT INTO zettels (path, checksum, last_updated) VALUES (?, ?, ?)",
		"path3",
		"checksum3",
		lastUpdated,
	)
	if err != nil {
		t.Fatalf("failed to insert third zettel: %v", err)
	}

	// Create links from zettel with path "path1" to zettels with path "path2" and "path3"
	err = db.CreateLink("path1", "path2")
	if err != nil {
		t.Fatalf("CreateLinkByPaths failed: %v", err)
	}
	err = db.CreateLink("path1", "path3")
	if err != nil {
		t.Fatalf("CreateLinkByPaths failed: %v", err)
	}

	// Delete links for zettel with path "path1"
	err = db.DeleteLinks(1) // Assuming the ID for "path1" is 1
	if err != nil {
		t.Fatalf("DeleteLinks failed: %v", err)
	}

	// Verify that the links were deleted
	var count int
	row := db.Conn.QueryRow(
		"SELECT COUNT(*) FROM links WHERE source_id = (SELECT id FROM zettels WHERE path = ?)",
		"path1",
	)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count links: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 links, got %d", count)
	}
}
