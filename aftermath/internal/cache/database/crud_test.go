package database_test

import (
	"aftermath/internal/cache/database"
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

// TestUpsertMetadata verifies that the UpsertMetadata function inserts or updates metadata correctly.
func TestUpsertMetadata(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Define test metadata key and value
	key := "test_key"
	value := []byte("test_value")

	// Upsert metadata using the UpsertMetadata function
	err := db.UpsertMetadata(key, value)
	if err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	// Verify that the metadata has been inserted into the database
	var retrievedValue []byte
	query := `SELECT value FROM metadata WHERE "key" = ?`
	err = db.Conn.QueryRow(query, key).Scan(&retrievedValue)
	if err != nil {
		t.Fatalf("failed to query inserted metadata: %v", err)
	}

	// Verify that the retrieved value matches the inserted value
	if string(retrievedValue) != string(value) {
		t.Errorf("expected value %s, got %s", value, retrievedValue)
	}

	// Upsert the metadata again with a new value to test update functionality
	newValue := []byte("new_value")
	err = db.UpsertMetadata(key, newValue)
	if err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	// Verify that the metadata has been updated
	err = db.Conn.QueryRow(query, key).Scan(&retrievedValue)
	if err != nil {
		t.Fatalf("failed to query updated metadata: %v", err)
	}
	if string(retrievedValue) != string(newValue) {
		t.Errorf("expected updated value %s, got %s", newValue, retrievedValue)
	}
}

// TestGetMetadata verifies that the GetMetadata function retrieves metadata correctly.
func TestGetMetadata(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert metadata for testing
	key := "existing_key"
	value := []byte("existing_value")
	err := db.UpsertMetadata(key, value)
	if err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	// Retrieve the metadata using the GetMetadata function
	retrievedValue, err := db.GetMetadata(key)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify that the retrieved value matches the inserted value
	if string(retrievedValue) != string(value) {
		t.Errorf("expected value %s, got %s", value, retrievedValue)
	}

	// Test for a non-existent key
	_, err = db.GetMetadata("non_existent_key")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
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
	err = db.Conn.QueryRow(
		`SELECT source_id, target_id FROM links WHERE source_id = (SELECT id FROM zettels WHERE path = ?) AND target_id = (SELECT id FROM zettels WHERE path = ?)`,
		"path1",
		"path2",
	).Scan(&sourceID, &targetID)
	if err != nil {
		t.Fatalf("failed to query created link: %v", err)
	}

	if sourceID == 0 || targetID == 0 {
		t.Fatalf("link not created between zettels")
	}
}
