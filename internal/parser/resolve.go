package parser

import (
	"fmt"
	"path/filepath"
)

// files starting with . are interpreted relative to file
// otherwise interpreted relative to root
func Resolve(note string, path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("Invalid path.")
	}

	// Add `.typ` extension if none is specified.
	if filepath.Ext(path) == "" {
		path += ".typ"
	}

	// Check if path should be relative to note.
	if path[0] == []byte(`.`)[0] {
		base := filepath.Dir(note)
		joined := filepath.Join(base, path)
		return filepath.Clean(joined), nil
	}

	return filepath.Clean(path), nil
}
