package sqlite

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const neverUpdate = (1 << 63) - 1

type FileInfo struct {
	Path         string
	LastModified int64
	Content      []byte
}

func scanFile(path string) (*FileInfo, error) {
	// If the file is not supported, we still track it
	// but dont scan it for links.
	// We achieve this by setting LastModified to a very large value
	if filepath.Ext(path) != ".typ" {
		return &FileInfo{
			Path:         path,
			LastModified: neverUpdate,
		}, nil
	}

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
		// skip hidden directories
		if info.IsDir() {
			if strings.HasPrefix(filepath.Base(path), ".") {
				log.Printf("Ignoring hidden directory %s during scan.", root)
				return filepath.SkipDir
			}
		} else {
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
