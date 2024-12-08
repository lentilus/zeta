package database_test

import (
	"database/sql"
	"testing"

	"aftermath/internal/database"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
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

// TestDBSetup verifies that the tables and schema version are set up correctly in the database.
func TestDBSetup(t *testing.T) {
	// Open test database
	db := openTestDB(t)
	defer closeTestDB(t, db)

	// Check if the zettels and links tables were created
	if err := verifyTableExists(db.Conn, "zettels"); err != nil {
		t.Errorf("zettels table verification failed: %v", err)
	}
	if err := verifyTableExists(db.Conn, "links"); err != nil {
		t.Errorf("links table verification failed: %v", err)
	}

	// Check the schema version
	version, err := getSchemaVersion(db.Conn)
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != database.SchemaVersion {
		t.Errorf("expected schema version %d, got %d", database.SchemaVersion, version)
	}
}

// verifyTableExists checks if a table with the given name exists in the database.
func verifyTableExists(conn *sql.DB, tableName string) error {
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?;`
	row := conn.QueryRow(query, tableName)

	var name string
	if err := row.Scan(&name); err != nil {
		return err
	}
	return nil
}

// getSchemaVersion retrieves the user_version pragma value (the schema version).
func getSchemaVersion(conn *sql.DB) (int, error) {
	var version int
	row := conn.QueryRow(`PRAGMA user_version;`)
	err := row.Scan(&version)
	return version, err
}
