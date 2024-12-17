// database_test.go
package database_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"zeta/internal/bibliography"
	"zeta/internal/cache/database"
)

// Define the interface directly for the mock implementation
type mockBibliography struct {
	overrideCalled bool
	appendCalled   bool
	entries        []bibliography.Entry
}

// Implement the Override method
func (m *mockBibliography) Override(entries []bibliography.Entry) error {
	m.overrideCalled = true
	m.entries = entries
	return nil
}

// Implement the Append method
func (m *mockBibliography) Append(entries []bibliography.Entry) error {
	m.appendCalled = true
	m.entries = append(m.entries, entries...)
	return nil
}

// Update testHelper to use the simplified mockBibliography directly
type testHelper struct {
	db      *database.SQLiteDB
	path    string
	mockBib *mockBibliography
}

func setupTest(t *testing.T) *testHelper {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "sqlitedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	mockBib := &mockBibliography{}
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.NewSQLiteDB(dbPath, mockBib, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create test database: %v", err)
	}

	return &testHelper{
		db:      db,
		path:    tmpDir,
		mockBib: mockBib,
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

func TestBibliographySync(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	t.Run("InitialSync", func(t *testing.T) {
		if !h.mockBib.overrideCalled {
			t.Error("Expected Override to be called during initialization")
		}
	})

	t.Run("FileAddition", func(t *testing.T) {
		// Reset mock
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false

		file := &database.FileRecord{
			Path:         filepath.Join(h.path, "test.typ"),
			LastModified: 123,
		}

		// Direct UpsertFile triggers bibliography sync
		if err := h.db.UpsertFile(file); err != nil {
			t.Fatalf("Failed to insert file: %v", err)
		}

		// After UpsertFile, either Override or Append should be called
		if !h.mockBib.overrideCalled && !h.mockBib.appendCalled {
			t.Error("Expected either Override or Append to be called after adding new file")
		}

		if len(h.mockBib.entries) == 0 {
			t.Error("Expected bibliography entry to be added")
		}
	})

	t.Run("FileDeletion", func(t *testing.T) {
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false

		if err := h.db.DeleteFile(filepath.Join(h.path, "test.typ")); err != nil {
			t.Fatalf("Failed to delete file: %v", err)
		}

		// After DeleteFile, either Override or Append should be called
		if !h.mockBib.overrideCalled && !h.mockBib.appendCalled {
			t.Error("Expected either Override or Append to be called after file deletion")
		}
	})

	t.Run("DatabaseClear", func(t *testing.T) {
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false

		if err := h.db.Clear(); err != nil {
			t.Fatalf("Failed to clear database: %v", err)
		}

		if !h.mockBib.overrideCalled {
			t.Error("Expected Override to be called after database clear")
		}

		if len(h.mockBib.entries) != 0 {
			t.Error("Expected empty bibliography after clear")
		}
	})
}

func TestBibliographyOrder(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	files := []string{
		"a.typ",
		"b.typ",
		"c.typ",
	}

	for _, f := range files {
		file := &database.FileRecord{
			Path:         filepath.Join(h.path, f),
			LastModified: 123,
		}
		if err := h.db.UpsertFile(file); err != nil {
			t.Fatalf("Failed to insert file %s: %v", f, err)
		}
	}

	for i, entry := range h.mockBib.entries {
		expected := files[i]
		if filepath.Base(entry.Path) != expected {
			t.Errorf("Wrong order: got %s at position %d, want %s",
				entry.Path, i, expected)
		}
	}
}

func TestBibliographyTransactions(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup(t)

	t.Run("SuccessfulTransaction", func(t *testing.T) {
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false

		err := h.db.WithTx(func(tx database.Transaction) error {
			// Add multiple files in one transaction
			files := []database.FileRecord{
				{
					Path:         filepath.Join(h.path, "tx1.typ"),
					LastModified: 123,
				},
				{
					Path:         filepath.Join(h.path, "tx2.typ"),
					LastModified: 123,
				},
			}

			for _, file := range files {
				if err := tx.UpsertFile(&file); err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify bibliography was updated after transaction
		if !h.mockBib.overrideCalled && !h.mockBib.appendCalled {
			t.Error("Expected bibliography update after successful transaction")
		}

		// Verify both files are in the bibliography
		foundTx1 := false
		foundTx2 := false
		for _, entry := range h.mockBib.entries {
			switch filepath.Base(entry.Path) {
			case "tx1.typ":
				foundTx1 = true
			case "tx2.typ":
				foundTx2 = true
			}
		}
		if !foundTx1 || !foundTx2 {
			t.Error("Not all transaction files were added to bibliography")
		}
	})

	t.Run("FailedTransaction", func(t *testing.T) {
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false
		entriesBefore := len(h.mockBib.entries)

		err := h.db.WithTx(func(tx database.Transaction) error {
			// Add one file
			file := database.FileRecord{
				Path:         filepath.Join(h.path, "will-fail.typ"),
				LastModified: 123,
			}
			if err := tx.UpsertFile(&file); err != nil {
				return err
			}

			// Return error to rollback transaction
			return fmt.Errorf("intentional error")
		})

		if err == nil {
			t.Fatal("Expected transaction to fail")
		}

		// Verify bibliography wasn't updated after failed transaction
		if h.mockBib.overrideCalled || h.mockBib.appendCalled {
			t.Error("Bibliography should not be updated after failed transaction")
		}

		// Verify no new entries were added
		if len(h.mockBib.entries) != entriesBefore {
			t.Error("Bibliography entries changed after failed transaction")
		}
	})

	t.Run("TransactionWithLinks", func(t *testing.T) {
		h.mockBib.overrideCalled = false
		h.mockBib.appendCalled = false

		err := h.db.WithTx(func(tx database.Transaction) error {
			// Add source and target files
			files := []database.FileRecord{
				{
					Path:         filepath.Join(h.path, "source.typ"),
					LastModified: 123,
				},
				{
					Path:         filepath.Join(h.path, "target.typ"),
					LastModified: 123,
				},
			}

			for _, file := range files {
				if err := tx.UpsertFile(&file); err != nil {
					return err
				}
			}

			// Create link between files
			return tx.UpsertLinks(
				filepath.Join(h.path, "source.typ"),
				[]string{filepath.Join(h.path, "target.typ")},
			)
		})

		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify bibliography was updated
		if !h.mockBib.overrideCalled && !h.mockBib.appendCalled {
			t.Error("Expected bibliography update after transaction with links")
		}

		// Verify both files are in the bibliography
		foundSource := false
		foundTarget := false
		for _, entry := range h.mockBib.entries {
			switch filepath.Base(entry.Path) {
			case "source.typ":
				foundSource = true
			case "target.typ":
				foundTarget = true
			}
		}
		if !foundSource || !foundTarget {
			t.Error("Not all linked files were added to bibliography")
		}
	})
}
