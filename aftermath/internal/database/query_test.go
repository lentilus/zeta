package database_test

import (
	"aftermath/internal/database"
	"testing"
	"time"
)

// TestGetAllZettels verifies that the GetAllZettels function retrieves all zettels correctly.
func TestGetAllZettels(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert some test zettels
	testZettels := []database.Zettel{
		{Path: "path1", Checksum: []byte("checksum1"), LastUpdated: time.Now().Unix()},
		{Path: "path2", Checksum: []byte("checksum2"), LastUpdated: time.Now().Unix()},
	}

	for _, zettel := range testZettels {
		if err := db.CreateZettel(zettel); err != nil {
			t.Fatalf("failed to create zettel: %v", err)
		}
	}

	// Retrieve all zettels
	zettels, err := db.GetAllZettels()
	if err != nil {
		t.Fatalf("GetAllZettels failed: %v", err)
	}

	if len(zettels) != len(testZettels) {
		t.Errorf("expected %d zettels, got %d", len(testZettels), len(zettels))
	}
}

// TestGetForwardLinks verifies that the GetForwardLinks function retrieves forward links correctly.
func TestGetForwardLinks(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert two zettels
	if err := db.CreateZettel(database.Zettel{Path: "path1", Checksum: []byte("checksum1"), LastUpdated: time.Now().Unix()}); err != nil {
		t.Fatalf("failed to create first zettel: %v", err)
	}
	if err := db.CreateZettel(database.Zettel{Path: "path2", Checksum: []byte("checksum2"), LastUpdated: time.Now().Unix()}); err != nil {
		t.Fatalf("failed to create second zettel: %v", err)
	}

	// Create a link
	if err := db.CreateLink(1, 2); err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Retrieve forward links
	links, err := db.GetForwardLinks(1)
	if err != nil {
		t.Fatalf("GetForwardLinks failed: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
}

// TestGetBackLinks verifies that the GetBackLinks function retrieves back links correctly.
func TestGetBackLinks(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Insert two zettels
	if err := db.CreateZettel(database.Zettel{Path: "path1", Checksum: []byte("checksum1"), LastUpdated: time.Now().Unix()}); err != nil {
		t.Fatalf("failed to create first zettel: %v", err)
	}
	if err := db.CreateZettel(database.Zettel{Path: "path2", Checksum: []byte("checksum2"), LastUpdated: time.Now().Unix()}); err != nil {
		t.Fatalf("failed to create second zettel: %v", err)
	}

	// Create a link
	if err := db.CreateLink(1, 2); err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Retrieve back links
	links, err := db.GetBackLinks(2)
	if err != nil {
		t.Fatalf("GetBackLinks failed: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
}
