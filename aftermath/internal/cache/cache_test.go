package cache_test

import (
	"aftermath/internal/cache"
	"aftermath/internal/database"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

type TestSetup struct {
	rootDir   string
	dbPath    string
	db        *database.DB
	testFiles []string
}

func setupTest(t *testing.T) (*TestSetup, error) {
	// Create temporary directory for test files
	rootDir, err := ioutil.TempDir("", "zettelkasten_test")
	if err != nil {
		return nil, err
	}

	// Create temporary database file
	dbFile, err := ioutil.TempFile("", "zettelkasten_test.db")
	if err != nil {
		os.RemoveAll(rootDir)
		return nil, err
	}
	dbPath := dbFile.Name()
	dbFile.Close()

	// Initialize database
	db, err := database.NewDB(dbPath)
	if err != nil {
		os.RemoveAll(rootDir)
		os.Remove(dbPath)
		return nil, err
	}

	return &TestSetup{
		rootDir: rootDir,
		dbPath:  dbPath,
		db:      db,
	}, nil
}

func (ts *TestSetup) cleanup() {
	ts.db.Close()
	os.RemoveAll(ts.rootDir)
	os.Remove(ts.dbPath)
}

func createTestFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	return ioutil.WriteFile(path, []byte(content), 0644)
}

func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}

func TestZettelkastenBasicOperation(t *testing.T) {
	setup, err := setupTest(t)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}
	defer setup.cleanup()

	// Create test files
	testFiles := map[string]string{
		"test1.typ": "Test content with @ref2",
		"test2.typ": "Another test with @ref1 and @ref3",
		"ref1.typ":  "Reference 1 content",
		"ref2.typ":  "Reference 2 content",
		"ref3.typ":  "Reference 3 content",
	}

	for name, content := range testFiles {
		err := createTestFile(setup.rootDir, name, content)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}

	zk := cache.NewZettelkasten(setup.rootDir, setup.db)

	// Run update
	err = zk.UpdateIncremental()
	if err != nil {
		t.Fatalf("UpdateIncremental failed: %v", err)
	}

	// Verify database contents
	zettels, err := setup.db.GetAll()
	if err != nil {
		t.Fatalf("Failed to get zettels from database: %v", err)
	}

	if len(zettels) != len(testFiles) {
		t.Errorf("Expected %d zettels, got %d", len(testFiles), len(zettels))
	}

	// Verify forward links
	expectedForwardLinks := map[string][]string{
		filepath.Join(setup.rootDir, "test1.typ"): {
			filepath.Join(setup.rootDir, "ref2.typ"),
		},
		filepath.Join(setup.rootDir, "test2.typ"): {
			filepath.Join(setup.rootDir, "ref1.typ"),
			filepath.Join(setup.rootDir, "ref3.typ"),
		},
	}

	for source, expectedTargets := range expectedForwardLinks {
		actualTargets, err := setup.db.GetForwardLinks(source)
		if err != nil {
			t.Errorf("Failed to get forward links for %s: %v", source, err)
			continue
		}

		if !compareStringSlices(actualTargets, expectedTargets) {
			t.Errorf("Forward link mismatch for %s:\nExpected: %v\nGot: %v",
				source, expectedTargets, actualTargets)
		}
	}

	// Verify back links
	expectedBackLinks := map[string][]string{
		filepath.Join(setup.rootDir, "ref1.typ"): {
			filepath.Join(setup.rootDir, "test2.typ"),
		},
		filepath.Join(setup.rootDir, "ref2.typ"): {
			filepath.Join(setup.rootDir, "test1.typ"),
		},
		filepath.Join(setup.rootDir, "ref3.typ"): {
			filepath.Join(setup.rootDir, "test2.typ"),
		},
	}

	for target, expectedSources := range expectedBackLinks {
		actualSources, err := setup.db.GetBackLinks(target)
		if err != nil {
			t.Errorf("Failed to get back links for %s: %v", target, err)
			continue
		}

		if !compareStringSlices(actualSources, expectedSources) {
			t.Errorf("Back link mismatch for %s:\nExpected: %v\nGot: %v",
				target, expectedSources, actualSources)
		}
	}
}

func TestFileModification(t *testing.T) {
	setup, err := setupTest(t)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}
	defer setup.cleanup()

	// Create initial files
	files := map[string]string{
		"test.typ": "Initial content @ref1",
		"ref1.typ": "Reference 1 content",
		"ref2.typ": "Reference 2 content",
	}

	for name, content := range files {
		err := createTestFile(setup.rootDir, name, content)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}
	}

	zk := cache.NewZettelkasten(setup.rootDir, setup.db)

	// First update
	err = zk.UpdateIncremental()
	if err != nil {
		t.Fatalf("First UpdateIncremental failed: %v", err)
	}

	// Verify initial links
	testPath := filepath.Join(setup.rootDir, "test.typ")
	ref1Path := filepath.Join(setup.rootDir, "ref1.typ")

	forwardLinks, err := setup.db.GetForwardLinks(testPath)
	if err != nil {
		t.Fatalf("Failed to get initial forward links: %v", err)
	}
	if !compareStringSlices(forwardLinks, []string{ref1Path}) {
		t.Errorf("Initial forward links incorrect. Expected [%s], got %v", ref1Path, forwardLinks)
	}

	// Modify file
	time.Sleep(1 * time.Second) // Ensure modification time is different
	err = createTestFile(setup.rootDir, "test.typ", "Modified content @ref2")
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Second update
	err = zk.UpdateIncremental()
	if err != nil {
		t.Fatalf("Second UpdateIncremental failed: %v", err)
	}

	// Verify modified links
	ref2Path := filepath.Join(setup.rootDir, "ref2.typ")
	forwardLinks, err = setup.db.GetForwardLinks(testPath)
	if err != nil {
		t.Fatalf("Failed to get modified forward links: %v", err)
	}
	if !compareStringSlices(forwardLinks, []string{ref2Path}) {
		t.Errorf("Modified forward links incorrect. Expected [%s], got %v", ref2Path, forwardLinks)
	}

	// Verify old reference is removed
	backLinks, err := setup.db.GetBackLinks(ref1Path)
	if err != nil {
		t.Fatalf("Failed to get back links: %v", err)
	}
	if len(backLinks) != 0 {
		t.Errorf("Expected no back links to ref1, got %v", backLinks)
	}
}

func TestFileDeletion(t *testing.T) {
	setup, err := setupTest(t)
	if err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}
	defer setup.cleanup()

	// Create initial files
	files := map[string]string{
		"test.typ": "Test content @ref1",
		"ref1.typ": "Reference 1 content",
	}

	for name, content := range files {
		err := createTestFile(setup.rootDir, name, content)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}
	}

	zk := cache.NewZettelkasten(setup.rootDir, setup.db)

	// First update
	err = zk.UpdateIncremental()
	if err != nil {
		t.Fatalf("First UpdateIncremental failed: %v", err)
	}

	// Delete file
	testPath := filepath.Join(setup.rootDir, "test.typ")
	err = os.Remove(testPath)
	if err != nil {
		t.Fatalf("Failed to delete test file: %v", err)
	}

	// Second update
	err = zk.UpdateIncremental()
	if err != nil {
		t.Fatalf("Second UpdateIncremental failed: %v", err)
	}

	// Verify no forward links from deleted file
	forwardLinks, err := setup.db.GetForwardLinks(testPath)
	if err != nil {
		t.Fatalf("Failed to get forward links: %v", err)
	}
	if len(forwardLinks) != 0 {
		t.Errorf("Expected no forward links from deleted file, got %v", forwardLinks)
	}

	// Verify no back links to deleted file
	ref1Path := filepath.Join(setup.rootDir, "ref1.typ")
	backLinks, err := setup.db.GetBackLinks(ref1Path)
	if err != nil {
		t.Fatalf("Failed to get back links: %v", err)
	}
	if len(backLinks) != 0 {
		t.Errorf("Expected no back links after deletion, got %v", backLinks)
	}
}
