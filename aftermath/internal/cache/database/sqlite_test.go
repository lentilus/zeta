package database_test

import (
	"aftermath/internal/cache/database"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testHelper struct {
	db   *database.SQLiteDB
	path string
}

func setupTest(t *testing.T) *testHelper {
	t.Helper()

	// Create temporary database file
	tmpDir, err := os.MkdirTemp("", "sqlitedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.NewSQLiteDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create test database: %v", err)
	}

	return &testHelper{
		db:   db,
		path: tmpDir,
	}
}

func (h *testHelper) cleanup(t *testing.T) {
	t.Helper()
	if err := h.db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
	if err := os.RemoveAll(h.path); err != nil {
		t.Errorf("Failed to remove test directory: %v", err)
	}
}

func TestFileOperations(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	t.Run("UpsertAndGetFile", func(t *testing.T) {
		file := &database.FileRecord{
			Path:         "/test/file1.md",
			LastModified: time.Now().Unix(),
		}

		// Test insert
		if err := h.db.UpsertFile(file); err != nil {
			t.Fatalf("Failed to insert file: %v", err)
		}

		// Test get
		retrieved, err := h.db.GetFile(file.Path)
		if err != nil {
			t.Fatalf("Failed to get file: %v", err)
		}

		if retrieved.Path != file.Path || retrieved.LastModified != file.LastModified {
			t.Errorf("Retrieved file doesn't match: got %+v, want %+v", retrieved, file)
		}

		// Test update
		file.LastModified = time.Now().Unix()
		if err := h.db.UpsertFile(file); err != nil {
			t.Fatalf("Failed to update file: %v", err)
		}

		updated, err := h.db.GetFile(file.Path)
		if err != nil {
			t.Fatalf("Failed to get updated file: %v", err)
		}

		if updated.LastModified != file.LastModified {
			t.Errorf("Updated timestamp doesn't match: got %d, want %d",
				updated.LastModified, file.LastModified)
		}
	})

	t.Run("GetNonExistentFile", func(t *testing.T) {
		_, err := h.db.GetFile("/nonexistent.md")
		if err != database.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteFile", func(t *testing.T) {
		file := &database.FileRecord{
			Path:         "/test/file2.md",
			LastModified: time.Now().Unix(),
		}

		if err := h.db.UpsertFile(file); err != nil {
			t.Fatalf("Failed to insert file: %v", err)
		}

		if err := h.db.DeleteFile(file.Path); err != nil {
			t.Fatalf("Failed to delete file: %v", err)
		}

		_, err := h.db.GetFile(file.Path)
		if err != database.ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("GetAllFiles", func(t *testing.T) {
		// Clear database first
		if err := h.db.Clear(); err != nil {
			t.Fatalf("Failed to clear database: %v", err)
		}

		files := []database.FileRecord{
			{Path: "/test/file1.md", LastModified: time.Now().Unix()},
			{Path: "/test/file2.md", LastModified: time.Now().Unix()},
			{Path: "/test/file3.md", LastModified: time.Now().Unix()},
		}

		for _, f := range files {
			if err := h.db.UpsertFile(&f); err != nil {
				t.Fatalf("Failed to insert file: %v", err)
			}
		}

		retrieved, err := h.db.GetAllFiles()
		if err != nil {
			t.Fatalf("Failed to get all files: %v", err)
		}

		if len(retrieved) != len(files) {
			t.Errorf("Retrieved wrong number of files: got %d, want %d",
				len(retrieved), len(files))
		}
	})
}

func TestLinkOperations(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	// Helper function to create test files
	createTestFiles := func(t *testing.T) {
		files := []database.FileRecord{
			{Path: "/test/source.md", LastModified: time.Now().Unix()},
			{Path: "/test/target1.md", LastModified: time.Now().Unix()},
			{Path: "/test/target2.md", LastModified: time.Now().Unix()},
		}

		for _, f := range files {
			if err := h.db.UpsertFile(&f); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
	}

	t.Run("UpsertAndGetLinks", func(t *testing.T) {
		createTestFiles(t)

		sourcePath := "/test/source.md"
		targetPaths := []string{
			"/test/target1.md",
			"/test/target2.md",
		}

		if err := h.db.UpsertLinks(sourcePath, targetPaths); err != nil {
			t.Fatalf("Failed to upsert links: %v", err)
		}

		links, err := h.db.GetLinks(sourcePath)
		if err != nil {
			t.Fatalf("Failed to get links: %v", err)
		}

		if len(links) != len(targetPaths) {
			t.Errorf("Wrong number of links: got %d, want %d",
				len(links), len(targetPaths))
		}

		// Verify each link
		for i, link := range links {
			if link.SourcePath != sourcePath {
				t.Errorf("Wrong source path: got %s, want %s",
					link.SourcePath, sourcePath)
			}
			if link.TargetPath != targetPaths[i] {
				t.Errorf("Wrong target path: got %s, want %s",
					link.TargetPath, targetPaths[i])
			}
		}
	})

	t.Run("GetBacklinks", func(t *testing.T) {
		createTestFiles(t)

		sourcePath := "/test/source.md"
		targetPath := "/test/target1.md"

		if err := h.db.UpsertLinks(sourcePath, []string{targetPath}); err != nil {
			t.Fatalf("Failed to create link: %v", err)
		}

		backlinks, err := h.db.GetBacklinks(targetPath)
		if err != nil {
			t.Fatalf("Failed to get backlinks: %v", err)
		}

		if len(backlinks) != 1 {
			t.Fatalf("Wrong number of backlinks: got %d, want 1", len(backlinks))
		}

		if backlinks[0].SourcePath != sourcePath || backlinks[0].TargetPath != targetPath {
			t.Errorf("Wrong backlink: got %+v, want source=%s target=%s",
				backlinks[0], sourcePath, targetPath)
		}
	})

	t.Run("DeleteLinks", func(t *testing.T) {
		createTestFiles(t)

		sourcePath := "/test/source.md"
		targetPaths := []string{
			"/test/target1.md",
			"/test/target2.md",
		}

		if err := h.db.UpsertLinks(sourcePath, targetPaths); err != nil {
			t.Fatalf("Failed to create links: %v", err)
		}

		if err := h.db.DeleteLinks(sourcePath); err != nil {
			t.Fatalf("Failed to delete links: %v", err)
		}

		links, err := h.db.GetLinks(sourcePath)
		if err != nil {
			t.Fatalf("Failed to get links after deletion: %v", err)
		}

		if len(links) != 0 {
			t.Errorf("Expected no links after deletion, got %d", len(links))
		}
	})
}

func TestTransactions(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	t.Run("SuccessfulTransaction", func(t *testing.T) {
		// First create the target file (because of foreign key constraints)
		targetFile := &database.FileRecord{
			Path:         "/test/target.md",
			LastModified: time.Now().Unix(),
		}
		if err := h.db.UpsertFile(targetFile); err != nil {
			t.Fatalf("Failed to create target file: %v", err)
		}

		// Now run the transaction that creates the source file and link
		err := h.db.WithTx(func(tx database.Transaction) error {
			file := &database.FileRecord{
				Path:         "/test/file.md",
				LastModified: time.Now().Unix(),
			}
			if err := tx.UpsertFile(file); err != nil {
				return err
			}
			return tx.UpsertLinks(file.Path, []string{targetFile.Path})
		})

		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify changes were committed
		_, err = h.db.GetFile("/test/file.md")
		if err != nil {
			t.Errorf("Failed to get file after transaction: %v", err)
		}

		// Verify links were created
		links, err := h.db.GetLinks("/test/file.md")
		if err != nil {
			t.Errorf("Failed to get links after transaction: %v", err)
		}
		if len(links) != 1 || links[0].TargetPath != targetFile.Path {
			t.Errorf("Unexpected links after transaction: %+v", links)
		}
	})

	t.Run("FailedTransaction", func(t *testing.T) {
		originalFile := &database.FileRecord{
			Path:         "/test/original.md",
			LastModified: time.Now().Unix(),
		}
		if err := h.db.UpsertFile(originalFile); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := h.db.WithTx(func(tx database.Transaction) error {
			// This should succeed
			if err := tx.UpsertFile(&database.FileRecord{
				Path:         "/test/new.md",
				LastModified: time.Now().Unix(),
			}); err != nil {
				return err
			}

			// Return an error to trigger rollback
			return fmt.Errorf("intentional error")
		})

		if err == nil {
			t.Fatal("Expected transaction to fail")
		}

		// Verify new file wasn't created
		_, err = h.db.GetFile("/test/new.md")
		if err != database.ErrNotFound {
			t.Error("Expected new file not to exist after rollback")
		}

		// Verify original file still exists
		_, err = h.db.GetFile("/test/original.md")
		if err != nil {
			t.Error("Expected original file to still exist after rollback")
		}
	})
}
