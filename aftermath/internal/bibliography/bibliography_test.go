package bibliography_test

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/database"
	"os"
	"testing"
)

// openTestDB initializes an in-memory SQLite database using the database.NewDB function.
func openTestDB(t *testing.T) *database.DB {
	t.Helper()
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

func TestRegenerate(t *testing.T) {
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Create a temporary file to store the output of Regenerate.
	tempFile, err := os.CreateTemp("", "bibliography_test_*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up temp file after test.

	// Insert test zettels into the database.
	zettels := []database.Zettel{
		{Path: "/path1", Checksum: []byte("checksum1"), LastUpdated: 1630454400, ID: 1},
		{Path: "/path2", Checksum: []byte("checksum2"), LastUpdated: 1630454401, ID: 2},
	}
	for _, z := range zettels {
		if err := db.CreateZettel(z); err != nil {
			t.Fatalf("failed to insert zettel: %v", err)
		}
	}

	// Create a Bibliography instance.
	bib := &bibliography.Bibliography{
		Path: tempFile.Name(),
		DB:   db,
	}

	// Call the Regenerate method to generate the YAML file.
	err = bib.Regenerate("/")
	if err != nil {
		t.Fatalf("Regenerate failed: %v", err)
	}

	// Read the contents of the generated YAML file.
	yamlData, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to read generated YAML file: %v", err)
	}

	// Verify the contents of the YAML file.
	expectedYaml := `"path1":
  type: Misc
  title: "path1"
  path: "path1"
  id: 1
"path2":
  type: Misc
  title: "path2"
  path: "path2"
  id: 2
`

	if string(yamlData) != expectedYaml {
		t.Errorf(
			"unexpected YAML content:\nExpected:\n%s\nGot:\n%s",
			expectedYaml,
			string(yamlData),
		)
	}
}
