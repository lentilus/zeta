package sqlite

import (
	"aftermath/internal/cache/database"
	"fmt"
	"log"
	"sync"
)

func (s *SQLiteStore) processFile(file *FileInfo) error {
	links, err := parseLinks(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse links: %w", err)
	}

	return s.db.WithTx(func(tx database.Transaction) error {
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