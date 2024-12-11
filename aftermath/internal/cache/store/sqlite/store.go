package sqlite

import (
	"aftermath/internal/cache/database"
	"fmt"
)

type SQLiteStore struct {
	db       database.Database
	rootPath string
}

func NewSQLiteStore(config Config) (*SQLiteStore, error) {
	db, err := database.NewSQLiteDB(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	return &SQLiteStore{
		db:       db,
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
