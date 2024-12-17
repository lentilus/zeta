package sqlite

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"zeta/internal/cache/database"
)

func (s *SQLiteStore) processFile(file *FileInfo) error {
	log.Printf("(SqliteStore) Processing File: %s", file.Path)

	// Check if the file is already in the database
	dbFile, _ := s.db.GetFile(file.Path)

	// Skip parsing links if the file's LastModified is not newer
	if dbFile != nil && file.LastModified <= dbFile.LastModified {
		log.Printf("File %s not modified since last processing. Skipping.", file.Path)
		return nil
	}

	// Use the parser to extract links from the content
	refs, err := s.parser.ParseReferences(context.Background(), file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse links: %w", err)
	}

	// Convert references to string links if necessary
	links := make([]string, len(refs))
	for i, ref := range refs {
		links[i] = filepath.Join(s.rootPath, ref+".typ")
	}

	err = s.db.WithTx(func(tx database.Transaction) error {
		// Update file record
		if err := tx.UpsertFile(&database.FileRecord{
			Path:         file.Path,
			LastModified: file.LastModified,
		}); err != nil {
			return fmt.Errorf("failed to update file record: %w", err)
		}

		// Update links
		return tx.UpsertLinks(file.Path, links)
	})

	if err != nil {
		log.Printf("Error in processor: %s", err)
		return err
	}

	return nil
}

func (s *SQLiteStore) processFiles(files []*FileInfo) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(files))
	semaphore := make(chan struct{}, 4) // Limit concurrent operations

	for _, file := range files {
		wg.Add(1)
		go func(f *FileInfo) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			if err := s.processFile(f); err != nil {
				log.Printf("Failed to process file %s: %v", f.Path, err)
				errors <- err
			}
		}(file)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors while processing files", len(errs))
	}

	return nil
}
