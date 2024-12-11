package sqlite

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type FileInfo struct {
	Path         string
	LastModified int64
	Content      []byte
}

func scanFile(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &FileInfo{
		Path:         path,
		LastModified: info.ModTime().Unix(),
		Content:      content,
	}, nil
}

func scanDirectory(root string) ([]*FileInfo, error) {
	var files []*FileInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".typ" {
			fileInfo, err := scanFile(path)
			if err != nil {
				log.Printf("Failed to scan file %s: %v", path, err)
				return nil // Continue walking despite error
			}
			files = append(files, fileInfo)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

func parseLinks(content []byte) ([]string, error) {
	// Implementation depends on your link format
	// This is a placeholder - implement according to your needs
	return nil, nil
}
