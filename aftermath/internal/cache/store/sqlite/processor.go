package sqlite

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
)

func (s *SQLiteStore) processFile(file *FileInfo) error {
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
		return err
	}

	// First, check if the file is already in the database
	_, err = s.db.GetFile(file.Path)
	isNewFile := err == database.ErrNotFound

	// If this is a new file, add it to the bibliography
	if isNewFile {
		// Create a bibliography entry for the new file
		target, _ := filepath.Rel(s.rootPath, file.Path)
		entry := bibliography.Entry{
			Target: strings.TrimSuffix(target, ".typ"),
			Title:  target,
			Path:   target,
		}

		if err := s.bib.Append([]bibliography.Entry{entry}); err != nil {
			return fmt.Errorf("failed to append to bibliography: %w", err)
		}
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

	// After all files are processed, update the bibliography with all files
	records, err := s.db.GetAllFiles()
	if err != nil {
		return fmt.Errorf("failed to get all files from database: %w", err)
	}

	// Convert database records to bibliography entries
	entries := make([]bibliography.Entry, len(records))
	for i, record := range records {

		target, _ := filepath.Rel(s.rootPath, record.Path)
		entries[i] = bibliography.Entry{
			Target: strings.TrimSuffix(target, ".typ"),
			Title:  target,
			Path:   target,
		}
	}

	// Override the bibliography with all entries
	if err := s.bib.Override(entries); err != nil {
		return fmt.Errorf("failed to override bibliography: %w", err)
	}

	return nil
}
