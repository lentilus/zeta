package index_test

import (
	"aftermath/internal/database"
	"aftermath/pkg/index"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// openTestDB initializes an in-memory SQLite database using the database.NewDB function.
func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	// Use SQLite's in-memory database for testing
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return db
}

// closeTestDB closes the database connection to release resources after each test.
func closeTestDB(t *testing.T, db *database.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("failed to close test database: %v", err)
	}
}

func TestFindPaths(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files
	files := []string{"test1.typ", "test2.typ", ".hidden.typ", "test.txt"}
	for _, file := range files {
		os.WriteFile(filepath.Join(tempDir, file), []byte(""), 0644)
	}

	// Initialize the Indexer with a nil database (not used in this test)
	indexer := index.NewIndexer(nil, tempDir)

	// Call findPaths and verify the results
	paths := indexer.FindPaths()
	expectedPaths := []string{
		filepath.Join(tempDir, "test1.typ"),
		filepath.Join(tempDir, "test2.typ"),
		filepath.Join(tempDir, ".hidden.typ"),
	}

	sort.Strings(paths)
	sort.Strings(expectedPaths)

	if !reflect.DeepEqual(paths, expectedPaths) {
		t.Errorf("Expected %v, got %v", expectedPaths, paths)
	}
}

func TestUpdateZettelIndex(t *testing.T) {
	// Setup the in-memory SQLite database
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Initialize the Indexer with the in-memory DB
	tempDir := t.TempDir()
	indexer := index.NewIndexer(db, tempDir)

	// Create initial test files
	os.WriteFile(filepath.Join(tempDir, "newfile.typ"), []byte("new content"), 0644)
	os.WriteFile(filepath.Join(tempDir, "oldfile.typ"), []byte("updated content"), 0644)

	// Add an existing zettel to the DB
	err := db.CreateZettel(database.Zettel{
		Path:        filepath.Join(tempDir, "oldfile.typ"),
		Checksum:    []byte("old_checksum"),
		LastUpdated: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Call UpdateZettelIndex
	changedZettels, err := indexer.UpdateZettelIndex()
	if err != nil {
		t.Fatal(err)
	}

	// Expected paths that should be returned as changed
	expectedChanged := []string{
		filepath.Join(tempDir, "newfile.typ"),
		filepath.Join(tempDir, "oldfile.typ"),
	}
	if !reflect.DeepEqual(changedZettels, expectedChanged) {
		t.Errorf("Expected %v, got %v", expectedChanged, changedZettels)
	}
}

// TestIndexLinks tests the indexing of links between zettels.
func TestUpdateLinkIndex(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Create an indexer
	indexer := index.NewIndexer(db, "")

	// Prepare test zettels
	targetZettel := database.Zettel{Path: "target.typ", Checksum: []byte("checksum1")}
	randomZettel := database.Zettel{Path: "random.typ", Checksum: []byte("checksum2")}

	// Insert test zettels into the database
	if err := db.CreateZettel(targetZettel); err != nil {
		t.Fatalf("failed to create zettel1: %v", err)
	}
	if err := db.CreateZettel(randomZettel); err != nil {
		t.Fatalf("failed to create zettel2: %v", err)
	}

	// Create a zettel that references the first zettel
	content := "This is a reference to @target."
	path := filepath.Join(t.TempDir(), "source.typ")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create referencing zettel: %v", err)
	}

	// Insert the referencing zettel into the database
	sourceZettel := database.Zettel{Path: path, Checksum: []byte("checksum3")}
	if err := db.CreateZettel(sourceZettel); err != nil {
		t.Fatalf("failed to create referencing zettel: %v", err)
	}

	// get the IDs from the database
	source, err := db.GetZettel(path)
	if err != nil {
		t.Fatal(err)
	}
	target, err := db.GetZettel("target.typ")
	if err != nil {
		t.Fatal(err)
	}

	// Update the indexer with the changed zettels
	changedZettels := []string{path}
	if err := indexer.UpdateLinkIndex(changedZettels); err != nil {
		t.Fatalf("failed to update link index: %v", err)
	}

	// Verify the link was created
	var links []database.Link
	query := `SELECT source_id, target_id FROM links;`
	rows, err := db.Conn.Query(query)
	if err != nil {
		t.Fatalf("failed to query links: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var link database.Link
		if err := rows.Scan(&link.SourceID, &link.TargetID); err != nil {
			t.Fatalf("failed to scan link: %v", err)
		}
		links = append(links, link)
	}

	// Assert that exactly one link was created
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	// Verify the correct source and target IDs
	if links[0].SourceID != source.ID || links[0].TargetID != target.ID {
		t.Errorf(
			"expected link from %d to %d, got from %d to %d",
			sourceZettel.ID,
			targetZettel.ID,
			links[0].SourceID,
			links[0].TargetID,
		)
	}
}

// openTestDB initializes an in-memory SQLite database using the database.NewDB function.
func openBenchDB(b *testing.B) *database.DB {
	b.Helper()
	// Use SQLite's in-memory database for testing
	db, err := database.NewDB(":memory:")
	if err != nil {
		b.Fatalf("failed to open test database: %v", err)
	}
	return db
}

// closeTestDB closes the database connection to release resources after each test.
func closeBenchDB(b *testing.B, db *database.DB) {
	b.Helper()
	if err := db.Close(); err != nil {
		b.Errorf("failed to close test database: %v", err)
	}
}

// GenerateSyntheticFiles creates a number of .typ files with references to other files.
func GenerateSyntheticFiles(dir string, numFiles int, numReferences int) error {
	for i := 0; i < numFiles; i++ {
		content := ""
		// Generate content with random references to other files
		for j := 0; j < numReferences; j++ {
			ref := "@file" + fmt.Sprint('A'+(i+j)%numFiles) // reference to other files
			content += ref + "\n"
		}
		// Write the file
		if err := os.WriteFile(filepath.Join(dir, "file"+fmt.Sprint(i)+".typ"), []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func BenchmarkIndex(b *testing.B) {
	// Setup the in-memory SQLite database
	db := openBenchDB(b)
	defer closeBenchDB(b, db)

	// Create a temporary directory for synthetic files
	tempDir := b.TempDir()

	// Generate 1000 synthetic files with 3 references each
	err := GenerateSyntheticFiles(tempDir, 1000, 3)
	if err != nil {
		b.Fatalf("Failed to generate synthetic files: %v", err)
	}

	// Initialize the Indexer with the in-memory DB
	indexer := index.NewIndexer(db, tempDir)

	// Benchmark the initial indexing of all files
	b.Run("Index All Files", func(b *testing.B) {
		b.ResetTimer() // Reset timer before benchmarking
		for i := 0; i < b.N; i++ {
			if err := indexer.Index(); err != nil {
				b.Fatalf("Indexing failed: %v", err)
			}
		}
	})

	// Modify five files to simulate changes
	for i := 0; i < 5; i++ {
		err := os.WriteFile(
			filepath.Join(tempDir, "file"+fmt.Sprint(i)+".typ"),
			[]byte("@file1\n"),
			0644,
		)
		if err != nil {
			b.Fatalf("Failed to modify file: %v", err)
		}
	}

	// Benchmark indexing of changed files
	b.Run("Index Changed Files", func(b *testing.B) {
		b.ResetTimer() // Reset timer before benchmarking
		for i := 0; i < b.N; i++ {
			if err := indexer.Index(); err != nil {
				b.Fatalf("Indexing changed files failed: %v", err)
			}
		}
	})
}
