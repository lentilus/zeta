package sqlite

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
	"aftermath/internal/parser"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db       database.Database
	parser   parser.SimpleParser
	rootPath string
}

func NewSQLiteStore(config Config) (*SQLiteStore, error) {
	bib := bibliography.NewHyagrivaBib(config.BibPath)

	db, err := database.NewSQLiteDB(config.DBPath, bib, config.RootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	simpleParser, err := parser.NewOneTimeParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return &SQLiteStore{
		db:       db,
		parser:   simpleParser,
		rootPath: config.RootPath,
	}, nil
}

// Core operations
func (s *SQLiteStore) UpdateOne(path string) error {
	fileInfo, err := scanFile(path)
	if err != nil {
		return fmt.Errorf("failed to scan file: %w", err)
	}

	return s.processFile(fileInfo)
}

func (s *SQLiteStore) UpdateAll() error {
	files, err := scanDirectory(s.rootPath)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	return s.processFiles(files)
}

func (s *SQLiteStore) Recompute() error {
	if err := s.db.Clear(); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}
	return s.UpdateAll()
}

// Query operations
func (s *SQLiteStore) GetAll() ([]string, error) {
	records, err := s.db.GetAllFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get files from database: %w", err)
	}

	paths := make([]string, len(records))
	for i, record := range records {
		paths[i] = record.Path
	}

	return paths, nil
}

func (s *SQLiteStore) GetParents(path string) ([]string, error) {
	log.Printf("Getting parents for %s", path)
	records, err := s.db.GetBacklinks(path)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to get backlinks from database: %w", err)
	}

	// If no records found but no error, return empty slice
	if err == database.ErrNotFound {
		return []string{}, nil
	}

	paths := make([]string, len(records))
	for i, record := range records {
		paths[i] = record.SourcePath
	}

	return paths, nil
}

func (s *SQLiteStore) GetChildren(path string) ([]string, error) {
	records, err := s.db.GetLinks(path)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to get links from database: %w", err)
	}

	// If no records found but no error, return empty slice
	if err == database.ErrNotFound {
		return []string{}, nil
	}

	paths := make([]string, len(records))
	for i, record := range records {
		paths[i] = record.TargetPath
	}

	return paths, nil
}

// Cleanup operations
func (s *SQLiteStore) Close() error {
	var errors []error

	if err := s.parser.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close parser: %w", err))
	}

	if err := s.db.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close database: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errors)
	}

	return nil
}
